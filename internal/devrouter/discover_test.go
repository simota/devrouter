package devrouter

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEnsureUniqueServiceNames(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{name: "empty"},
		{name: "unique", input: []string{"web", "api", "web-2"}, want: []string{"web", "api", "web-2"}},
		{name: "simple duplicates", input: []string{"web", "web", "web"}, want: []string{"web", "web-2", "web-3"}},
		{name: "suffix before duplicate", input: []string{"web-2", "web", "web"}, want: []string{"web-2", "web", "web-3"}},
		{name: "suffix between duplicates", input: []string{"web", "web-2", "web"}, want: []string{"web", "web-2", "web-3"}},
		{name: "suffix after duplicates", input: []string{"web", "web", "web-2"}, want: []string{"web", "web-3", "web-2"}},
		{name: "consecutive reserved suffixes", input: []string{"web", "web", "web-2", "web-3", "web"}, want: []string{"web", "web-4", "web-2", "web-3", "web-5"}},
		{name: "available suffix gap", input: []string{"web", "web", "web-3"}, want: []string{"web", "web-2", "web-3"}},
		{name: "duplicate suffixed names", input: []string{"web-2", "web-2", "web-2-2", "web-2"}, want: []string{"web-2", "web-2-3", "web-2-2", "web-2-4"}},
		{name: "nested reservations", input: []string{"web", "web", "web-2", "web-2", "web-2-2"}, want: []string{"web", "web-3", "web-2", "web-2-3", "web-2-2"}},
		{name: "independent bases", input: []string{"web", "api", "web", "api", "web-2", "api-2"}, want: []string{"web", "api", "web-3", "api-3", "web-2", "api-2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services := make([]ServiceSpec, len(tt.input))
			for i, name := range tt.input {
				services[i] = ServiceSpec{Name: name, WorkspacePath: fmt.Sprintf("apps/service-%d", i)}
			}
			ensureUniqueServiceNames(services)
			seen := make(map[string]bool, len(services))
			for i, svc := range services {
				if svc.Name != tt.want[i] {
					t.Errorf("service %d name = %q, want %q", i, svc.Name, tt.want[i])
				}
				if seen[svc.Name] {
					t.Errorf("service %d has duplicate name %q", i, svc.Name)
				}
				seen[svc.Name] = true
				if svc.WorkspacePath != fmt.Sprintf("apps/service-%d", i) {
					t.Errorf("service %d changed workspace to %q", i, svc.WorkspacePath)
				}
			}
			first := append([]ServiceSpec(nil), services...)
			ensureUniqueServiceNames(services)
			for i := range services {
				if !reflect.DeepEqual(services[i], first[i]) {
					t.Errorf("reapplying name assignment changed service %d: got %+v, want %+v", i, services[i], first[i])
				}
			}
		})
	}
}

func TestDiscoverStackPreservesCollidingServices(t *testing.T) {
	tests := []struct {
		name          string
		paths         []string
		overrideNames []string
		tls           bool
		customHost    string
	}{
		{name: "workspace basenames", paths: []string{"apps/web", "packages/web", "services/web-2"}},
		{name: "normalized basenames", paths: []string{"apps/WEB", "packages/web", "services/Web_2"}},
		{name: "normalized overrides", paths: []string{"apps/first", "packages/second", "services/third"}, overrideNames: []string{"WEB", "web", "Web_2"}, tls: true, customHost: "custom.dev.test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			repo := writeDiscoveryWorkspaces(t, tt.paths)
			cfg := DefaultConfig()
			cfg.Stack = "Review App"
			cfg.Domain = "dev.test"
			cfg.TLS.Enabled = tt.tls
			cfg.Defaults.Env["SHARED"] = "default"
			for i, path := range tt.paths {
				diff := ServiceConfigDiff{
					Port:    3100 + i,
					Command: fmt.Sprintf("node entry-%d.js", i),
					Env:     map[string]string{"WORKSPACE": path},
					HealthCheck: HealthCheckConfig{
						Endpoint: fmt.Sprintf("/health/%d", i), Method: "GET", Timeout: 2000, Status: []int{200 + i},
					},
				}
				if len(tt.overrideNames) != 0 {
					diff.Name = tt.overrideNames[i]
				}
				if i == 0 {
					diff.Host = tt.customHost
				}
				cfg.Overrides[path] = diff
			}
			stack, err := DiscoverStack(repo, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(stack.Services) != len(tt.paths) {
				t.Fatalf("discovered %d services, want %d", len(stack.Services), len(tt.paths))
			}
			// Repeated discovery must assign the same identifiers independently of
			// configuration map iteration, and applying the same config is stable.
			again, err := DiscoverStack(repo, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(again, stack) {
				t.Errorf("repeated discovery changed the stack: got %+v, want %+v", again, stack)
			}
			applyConfig(&again, cfg)
			if !reflect.DeepEqual(again, stack) {
				t.Errorf("reapplying config changed the stack: got %+v, want %+v", again, stack)
			}

			composePath, err := WriteCompose(stack, cfg.TLS)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(composePath)
			if err != nil {
				t.Fatal(err)
			}
			var compose ComposeFile
			if err := yaml.Unmarshal(data, &compose); err != nil {
				t.Fatal(err)
			}
			if len(compose.Services) != len(tt.paths) {
				t.Errorf("generated %d Compose services for %d workspaces", len(compose.Services), len(tt.paths))
			}
			scheme, entrypoint := "http", "web"
			if tt.tls {
				scheme, entrypoint = "https", "websecure"
			}
			record := StackRecordFromSpec(stack, composePath, "generated", scheme)
			if len(record.Services) != len(tt.paths) {
				t.Errorf("recorded %d services for %d workspaces", len(record.Services), len(tt.paths))
			}
			wantNames := []string{"web", "web-3", "web-2"}
			for i, path := range tt.paths {
				name := wantNames[i]
				key := "review-app-" + name
				host := key + ".dev.test"
				if i == 0 && tt.customHost != "" {
					host = tt.customHost
				}
				wantEnv := map[string]string{"HOST": "0.0.0.0", "SHARED": "default", "WORKSPACE": path, "PORT": strconv.Itoa(3100 + i)}
				wantSpec := ServiceSpec{
					Name: name, WorkspacePath: path, WorkspaceAbs: filepath.Join(repo, path),
					Command: fmt.Sprintf("node entry-%d.js", i), Port: 3100 + i, Env: wantEnv,
					Host: host, ComposeService: key, HealthCheck: cfg.Overrides[path].HealthCheck,
				}
				if !reflect.DeepEqual(stack.Services[i], wantSpec) {
					t.Errorf("workspace %q spec = %+v, want %+v", path, stack.Services[i], wantSpec)
				}
				service, ok := compose.Services[key]
				if !ok {
					t.Errorf("workspace %q lost Compose service %q", path, key)
					continue
				}
				if service.WorkingDir != "/repo/"+path {
					t.Errorf("Compose %q working_dir = %q, want %q", key, service.WorkingDir, "/repo/"+path)
				}
				if !reflect.DeepEqual(service.Environment, wantEnv) {
					t.Errorf("Compose %q environment = %v, want %v", key, service.Environment, wantEnv)
				}
				wantCommand := []string{"sh", "-lc", fmt.Sprintf("node entry-%d.js", i)}
				if !reflect.DeepEqual(service.Command, wantCommand) {
					t.Errorf("Compose %q command = %v, want %v", key, service.Command, wantCommand)
				}
				wantLabels := []string{
					"traefik.enable=true",
					"traefik.docker.network=devrouter_net",
					fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", key, host),
					fmt.Sprintf("traefik.http.routers.%s.entrypoints=%s", key, entrypoint),
					fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%d", key, 3100+i),
					fmt.Sprintf("traefik.http.routers.%s.service=%s", key, key),
				}
				if tt.tls {
					wantLabels = append(wantLabels, fmt.Sprintf("traefik.http.routers.%s.tls=true", key))
				}
				sort.Strings(service.Labels)
				sort.Strings(wantLabels)
				if !reflect.DeepEqual(service.Labels, wantLabels) {
					t.Errorf("Compose %q labels = %v, want %v", key, service.Labels, wantLabels)
				}
				wantRecord := ServiceRecord{
					Name: name, WorkspacePath: path, Host: host, URL: scheme + "://" + host,
					Port: 3100 + i, ComposeService: key, HealthCheck: cfg.Overrides[path].HealthCheck,
				}
				for _, selector := range []string{name, key} {
					got, ok := FindService(record, selector)
					if !ok || !reflect.DeepEqual(got, wantRecord) {
						t.Errorf("FindService(%q) = %+v, %t; want %+v, true", selector, got, ok, wantRecord)
					}
				}
			}
		})
	}
}

func TestInitConfigPreservesDisambiguatedServiceOverrides(t *testing.T) {
	for _, mode := range []string{"cli", "preview", "apply"} {
		t.Run(mode, func(t *testing.T) {
			paths := []string{"apps/web", "packages/web", "services/web-2"}
			repo := writeDiscoveryWorkspaces(t, paths)
			composePath := filepath.Join(repo, "compose.yml")
			compose := "services:\n" +
				"  web:\n    image: node\n    expose: [3100]\n    command: node first.js\n    environment: {ROLE: first}\n" +
				"  web-2:\n    image: node\n    expose: [3200]\n    command: node second.js\n    environment: {ROLE: second}\n" +
				"  web-3:\n    image: node\n    expose: [3300]\n    command: node third.js\n    environment: {ROLE: third}\n"
			if err := os.WriteFile(composePath, []byte(compose), 0o644); err != nil {
				t.Fatal(err)
			}
			var content []byte
			var warnings []string
			switch mode {
			case "cli":
				result, err := InitConfig(InitOptions{RepoPath: repo, ComposePath: composePath})
				if err != nil {
					t.Fatal(err)
				}
				content, err = os.ReadFile(result.OutputPath)
				if err != nil {
					t.Fatal(err)
				}
				warnings = result.Warnings
			case "preview":
				result, err := PreviewInitConfig(InitPreviewOptions{RepoPath: repo, ComposePath: composePath})
				if err != nil {
					t.Fatal(err)
				}
				if !result.ApplyAllowed {
					t.Error("preview does not allow applying the valid Compose configuration")
				}
				content, warnings = []byte(result.Content), result.Warnings
			case "apply":
				result, err := ApplyInitConfig(InitApplyOptions{RepoPath: repo, ComposePath: composePath})
				if err != nil {
					t.Fatal(err)
				}
				content, err = os.ReadFile(result.OutputPath)
				if err != nil {
					t.Fatal(err)
				}
				if string(content) != result.Content {
					t.Error("applied configuration differs from the returned preview")
				}
				warnings = result.Warnings
			}
			if len(warnings) != 0 {
				t.Errorf("unexpected unmatched-service warnings: %v", warnings)
			}
			var cfg ConfigFile
			if err := yaml.Unmarshal(content, &cfg); err != nil {
				t.Fatal(err)
			}
			want := map[string]ServiceConfig{
				"apps/web":       {Port: 3100, Command: "node first.js", Env: map[string]string{"ROLE": "first"}},
				"packages/web":   {Port: 3300, Command: "node third.js", Env: map[string]string{"ROLE": "third"}},
				"services/web-2": {Port: 3200, Command: "node second.js", Env: map[string]string{"ROLE": "second"}},
			}
			if !reflect.DeepEqual(cfg.Overrides, want) {
				t.Errorf("overrides = %+v, want %+v", cfg.Overrides, want)
			}
		})
	}
}

func writeDiscoveryWorkspaces(t *testing.T, paths []string) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "pnpm-workspace.yaml"), []byte("packages:\n  - apps/*\n  - packages/*\n  - services/*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		dir := filepath.Join(repo, path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"dev":"node app.js"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}
