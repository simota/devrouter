package devrouter

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestLoadStackFromCompose_HealthCheckOverride(t *testing.T) {
	// Create temporary compose file
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	composeContent := `
services:
  api-service:
    image: myapi:latest
    labels:
      - devrouter.enabled=true
      - traefik.enable=true
      - traefik.http.routers.test-api.rule=Host(` + "`test-api.localtest.me`" + `)
      - traefik.http.services.test-api.loadbalancer.server.port=8080
      - traefik.docker.network=devrouter_net
    networks:
      - devrouter_net

networks:
  devrouter_net:
    external: true
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create config with healthcheck override
	cfg := Config{
		Stack:  "test",
		Domain: "localtest.me",
		Overrides: map[string]ServiceConfigDiff{
			"api-service": {
				HealthCheck: HealthCheckConfig{
					Endpoint: "/health",
					Method:   "GET",
					Timeout:  5000,
					Status:   []int{200},
				},
			},
		},
	}

	// Load stack
	stack, warnings, err := LoadStackFromCompose(tmpDir, cfg, composePath)
	if err != nil {
		t.Fatalf("LoadStackFromCompose failed: %v", err)
	}

	// Check warnings (network warning is expected since we can't actually check docker)
	t.Logf("Warnings: %v", warnings)

	// Find api-service
	var apiService *ServiceSpec
	for i := range stack.Services {
		if stack.Services[i].Name == "api-service" {
			apiService = &stack.Services[i]
			break
		}
	}

	if apiService == nil {
		t.Fatal("api-service not found in stack")
	}

	// Verify healthcheck was applied
	if apiService.HealthCheck.Endpoint != "/health" {
		t.Errorf("HealthCheck.Endpoint = %q, want %q", apiService.HealthCheck.Endpoint, "/health")
	}
	if apiService.HealthCheck.Method != "GET" {
		t.Errorf("HealthCheck.Method = %q, want %q", apiService.HealthCheck.Method, "GET")
	}
	if apiService.HealthCheck.Timeout != 5000 {
		t.Errorf("HealthCheck.Timeout = %d, want %d", apiService.HealthCheck.Timeout, 5000)
	}
	if len(apiService.HealthCheck.Status) != 1 || apiService.HealthCheck.Status[0] != 200 {
		t.Errorf("HealthCheck.Status = %v, want [200]", apiService.HealthCheck.Status)
	}
}

func loadComposeForTest(t *testing.T, composeContent string) (StackSpec, []string) {
	t.Helper()
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatal(err)
	}
	stack, warnings, err := LoadStackFromCompose(tmpDir, Config{Stack: "test", Domain: "localtest.me"}, composePath)
	if err != nil {
		t.Fatalf("LoadStackFromCompose failed: %v", err)
	}
	return stack, warnings
}

func findServiceSpec(t *testing.T, stack StackSpec, name string) ServiceSpec {
	t.Helper()
	for _, svc := range stack.Services {
		if svc.Name == name {
			return svc
		}
	}
	t.Fatalf("service %s not found in stack", name)
	return ServiceSpec{}
}

func TestLoadStackFromCompose_Worker(t *testing.T) {
	stack, warnings := loadComposeForTest(t, `
services:
  bff-worker:
    image: worker:latest
    labels:
      - devrouter.enabled=true
      - devrouter.kind=worker
`)
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	worker := findServiceSpec(t, stack, "bff-worker")
	if worker.Kind != ServiceKindWorker {
		t.Errorf("Kind = %q, want %q", worker.Kind, ServiceKindWorker)
	}
	if worker.Host != "" || worker.Port != 0 {
		t.Errorf("Host, Port = %q, %d, want empty", worker.Host, worker.Port)
	}
	if worker.ComposeService != "bff-worker" {
		t.Errorf("ComposeService = %q, want %q", worker.ComposeService, "bff-worker")
	}

	record := StackRecordFromSpec(stack, "docker-compose.yml", "compose", "https")
	if got := record.Services[0]; got.Host != "" || got.URL != "" || got.Kind != ServiceKindWorker {
		t.Errorf("record = host %q url %q kind %q, want empty host and url, kind worker", got.Host, got.URL, got.Kind)
	}
}

func TestLoadStackFromCompose_WorkerWithRouterRule(t *testing.T) {
	stack, warnings := loadComposeForTest(t, `
services:
  bff-worker:
    image: worker:latest
    labels:
      - devrouter.enabled=true
      - devrouter.kind=worker
      - traefik.http.routers.w.rule=Host(`+"`w.localtest.me`"+`)
`)
	want := []string{"service bff-worker: devrouter.kind=worker but has a traefik router rule"}
	if !reflect.DeepEqual(warnings, want) {
		t.Errorf("warnings = %v, want %v", warnings, want)
	}
	worker := findServiceSpec(t, stack, "bff-worker")
	if worker.Kind != ServiceKindWorker || worker.Host != "" {
		t.Errorf("Kind, Host = %q, %q, want worker with empty host", worker.Kind, worker.Host)
	}
}

func TestLoadStackFromCompose_UnknownKindIsHTTP(t *testing.T) {
	stack, warnings := loadComposeForTest(t, `
services:
  api:
    image: api:latest
    labels:
      - devrouter.enabled=true
      - devrouter.kind=cron
      - traefik.enable=true
      - traefik.http.routers.api.rule=Host(`+"`api.localtest.me`"+`)
      - traefik.http.services.api.loadbalancer.server.port=8080
      - traefik.docker.network=devrouter_net
    networks:
      - devrouter_net
`)
	want := []string{`service api: unknown devrouter.kind "cron", treating as http`}
	if !reflect.DeepEqual(warnings, want) {
		t.Errorf("warnings = %v, want %v", warnings, want)
	}
	api := findServiceSpec(t, stack, "api")
	if api.Kind != "" || api.Host != "api.localtest.me" || api.Port != 8080 {
		t.Errorf("api = kind %q host %q port %d, want http service", api.Kind, api.Host, api.Port)
	}
}

func TestLoadStackFromCompose_HTTPServicesUnchanged(t *testing.T) {
	stack, warnings := loadComposeForTest(t, `
services:
  api:
    image: api:latest
    labels:
      - devrouter.enabled=true
      - traefik.enable=true
      - traefik.http.routers.api.rule=Host(`+"`api.localtest.me`"+`)
      - traefik.http.services.api.loadbalancer.server.port=8080
      - traefik.docker.network=devrouter_net
    networks:
      - devrouter_net
  bare:
    image: bare:latest
    labels:
      - devrouter.enabled=true
  db:
    image: mysql:8
`)
	sort.Strings(warnings)
	want := []string{
		"service bare: missing traefik router rule",
		"service bare: missing traefik service port",
		"service bare: missing traefik.docker.network label",
		"service bare: not connected to devrouter_net",
		"service bare: traefik.enable is not true",
	}
	if !reflect.DeepEqual(warnings, want) {
		t.Errorf("warnings = %v, want %v", warnings, want)
	}
	if len(stack.Services) != 2 {
		t.Fatalf("services = %d, want 2", len(stack.Services))
	}
	bare := findServiceSpec(t, stack, "bare")
	if bare.Kind != "" || bare.Host != "test-bare.localtest.me" {
		t.Errorf("bare = kind %q host %q, want http with fallback host", bare.Kind, bare.Host)
	}

	record := StackRecordFromSpec(stack, "docker-compose.yml", "compose", "https")
	urls := map[string]string{}
	for _, svc := range record.Services {
		urls[svc.Name] = svc.URL
	}
	wantURLs := map[string]string{"api": "https://api.localtest.me", "bare": "https://test-bare.localtest.me"}
	if !reflect.DeepEqual(urls, wantURLs) {
		t.Errorf("urls = %v, want %v", urls, wantURLs)
	}
}

func TestCheckServiceHealth_Worker(t *testing.T) {
	worker := ServiceRecord{Name: "bff-worker", ComposeService: "bff-worker", Kind: ServiceKindWorker}
	tests := []struct {
		state         string
		wantStatus    string
		wantContainer string
	}{
		{state: "running", wantStatus: "healthy", wantContainer: "running"},
		{state: "exited", wantStatus: "unknown", wantContainer: "exited"},
		{state: "", wantStatus: "unknown", wantContainer: "not_found"},
	}
	for _, tt := range tests {
		states := map[string]string{}
		if tt.state != "" {
			states["bff-worker"] = tt.state
		}
		got := checkServiceHealth(worker, states)
		if got.Status != tt.wantStatus || got.ContainerState != tt.wantContainer || got.URL != "" {
			t.Errorf("state %q: got status %q container %q url %q, want %q %q and empty url",
				tt.state, got.Status, got.ContainerState, got.URL, tt.wantStatus, tt.wantContainer)
		}
	}
}
