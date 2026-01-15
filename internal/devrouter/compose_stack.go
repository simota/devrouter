package devrouter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type composeFileStack struct {
	Services map[string]composeServiceStack `yaml:"services"`
}

type composeServiceStack struct {
	Labels   any `yaml:"labels"`
	Networks any `yaml:"networks"`
}

func LoadStackFromCompose(repoPath string, cfg Config, composePath string) (StackSpec, []string, error) {
	if !filepath.IsAbs(composePath) {
		composePath = filepath.Join(repoPath, composePath)
	}
	data, err := os.ReadFile(composePath)
	if err != nil {
		return StackSpec{}, nil, err
	}
	var raw composeFileStack
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return StackSpec{}, nil, err
	}
	if len(raw.Services) == 0 {
		return StackSpec{}, nil, fmt.Errorf("no services found in compose: %s", composePath)
	}

	stackName := cfg.Stack
	if stackName == "" {
		stackName = filepath.Base(repoPath)
	}
	stackName = NormalizeName(stackName)
	domain := cfg.Domain
	if domain == "" {
		domain = DefaultDomain
	}

	services := make([]ServiceSpec, 0)
	warnings := make([]string, 0)
	for name, svc := range raw.Services {
		labels := parseLabels(svc.Labels)
		if !isDevrouterEnabled(labels) {
			continue
		}
		if !serviceHasNetwork(svc.Networks, DevrouterNetwork) {
			warnings = append(warnings, fmt.Sprintf("service %s: not connected to %s", name, DevrouterNetwork))
		}
		if networkLabel, ok := labels["traefik.docker.network"]; !ok {
			warnings = append(warnings, fmt.Sprintf("service %s: missing traefik.docker.network label", name))
		} else if networkLabel != DevrouterNetwork {
			warnings = append(warnings, fmt.Sprintf("service %s: traefik.docker.network is %s", name, networkLabel))
		}
		host := parseTraefikHost(labels)
		port := parseTraefikPort(labels)
		if host == "" {
			warnings = append(warnings, fmt.Sprintf("service %s: missing traefik router rule", name))
			host = fmt.Sprintf("%s-%s.%s", stackName, NormalizeName(name), domain)
		}
		if port == 0 {
			warnings = append(warnings, fmt.Sprintf("service %s: missing traefik service port", name))
		}
		if !isTraefikEnabled(labels) {
			warnings = append(warnings, fmt.Sprintf("service %s: traefik.enable is not true", name))
		}

		// Apply overrides from config
		var healthCheck HealthCheckConfig
		if override, ok := cfg.Overrides[name]; ok {
			healthCheck = override.HealthCheck
		}

		services = append(services, ServiceSpec{
			Name:           name,
			WorkspacePath:  name,
			WorkspaceAbs:   "",
			Command:        "",
			Port:           port,
			Env:            map[string]string{},
			Host:           host,
			ComposeService: name,
			HealthCheck:    healthCheck,
		})
	}
	if len(services) == 0 {
		return StackSpec{}, nil, fmt.Errorf("no services with devrouter.enabled=true in compose: %s", composePath)
	}
	SortServices(services)

	stack := StackSpec{
		ID:       StackID(stackName, repoPath),
		Name:     stackName,
		RepoPath: repoPath,
		Domain:   domain,
		Services: services,
	}
	return stack, warnings, nil
}

func serviceHasNetwork(raw any, name string) bool {
	if raw == nil {
		return false
	}
	switch v := raw.(type) {
	case string:
		return v == name
	case []string:
		for _, item := range v {
			if item == name {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if fmt.Sprint(item) == name {
				return true
			}
		}
	case map[string]any:
		if _, ok := v[name]; ok {
			return true
		}
	case map[any]any:
		for key := range v {
			if fmt.Sprint(key) == name {
				return true
			}
		}
	}
	return false
}

func SortServices(services []ServiceSpec) {
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})
}

func parseLabels(raw any) map[string]string {
	if raw == nil {
		return nil
	}
	labels := map[string]string{}
	switch v := raw.(type) {
	case map[string]any:
		for key, value := range v {
			labels[key] = fmt.Sprint(value)
		}
	case map[any]any:
		for key, value := range v {
			labels[fmt.Sprint(key)] = fmt.Sprint(value)
		}
	case []any:
		for _, item := range v {
			assignLabel(labels, fmt.Sprint(item))
		}
	case []string:
		for _, item := range v {
			assignLabel(labels, item)
		}
	case string:
		assignLabel(labels, v)
	}
	if len(labels) == 0 {
		return nil
	}
	return labels
}

func assignLabel(labels map[string]string, item string) {
	trimmed := strings.TrimSpace(item)
	if trimmed == "" {
		return
	}
	parts := strings.SplitN(trimmed, "=", 2)
	if len(parts) != 2 {
		return
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" {
		return
	}
	labels[key] = value
}

func isDevrouterEnabled(labels map[string]string) bool {
	if len(labels) == 0 {
		return false
	}
	value, ok := labels["devrouter.enabled"]
	if !ok {
		return false
	}
	return isTruthy(value)
}

func isTraefikEnabled(labels map[string]string) bool {
	if len(labels) == 0 {
		return false
	}
	value, ok := labels["traefik.enable"]
	if !ok {
		return false
	}
	return isTruthy(value)
}

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func parseTraefikHost(labels map[string]string) string {
	for key, value := range labels {
		if strings.HasPrefix(key, "traefik.http.routers.") && strings.HasSuffix(key, ".rule") {
			if host := extractHostFromRule(value); host != "" {
				return host
			}
		}
	}
	return ""
}

// hostRuleRe extracts literal Host(...) entries from Traefik rules so we can
// surface a stable hostname per service. We ignore HostRegexp/Path rules and
// fall back to the default host when no explicit Host() is present.
var hostRuleRe = regexp.MustCompile(`Host\(([^)]*)\)`)

func extractHostFromRule(rule string) string {
	match := hostRuleRe.FindStringSubmatch(rule)
	if len(match) < 2 {
		return ""
	}
	inside := match[1]
	parts := strings.Split(inside, ",")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		trimmed = strings.Trim(trimmed, "`\"'")
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseTraefikPort(labels map[string]string) int {
	for key, value := range labels {
		if strings.HasSuffix(key, ".loadbalancer.server.port") {
			port, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil && port > 0 {
				return port
			}
		}
	}
	return 0
}
