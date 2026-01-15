package devrouter

import (
	"os"
	"path/filepath"
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
