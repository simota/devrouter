package devrouter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DevrouterNetwork = "devrouter_net"

// TODO: make the image configurable if needed.
const defaultNodeImage = "node:20"

const defaultEntrypoint = "web"
const defaultTLSEntrypoint = "websecure"

const defaultTraefikServicePortLabel = "traefik.http.services.%s.loadbalancer.server.port=%d"
const defaultTraefikRouterRuleLabel = "traefik.http.routers.%s.rule=Host(`%s`)"
const defaultTraefikRouterEntryLabel = "traefik.http.routers.%s.entrypoints=%s"
const defaultTraefikEnableLabel = "traefik.enable=true"
const defaultTraefikNetworkLabel = "traefik.docker.network=" + DevrouterNetwork

// ComposeFile is a minimal docker-compose schema for MVP.
type ComposeFile struct {
	Version  string                    `yaml:"version,omitempty"`
	Services map[string]ComposeService `yaml:"services"`
	Networks map[string]ComposeNetwork `yaml:"networks"`
}

type ComposeService struct {
	Image       string            `yaml:"image"`
	WorkingDir  string            `yaml:"working_dir,omitempty"`
	Command     []string          `yaml:"command,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Ports       []string          `yaml:"ports,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Labels      []string          `yaml:"labels,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
}

type ComposeNetwork struct {
	External bool `yaml:"external"`
}

func WriteCompose(stack StackSpec, tls TLSConfig) (string, error) {
	composePath, err := StackComposePath(stack.ID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(composePath), 0o755); err != nil {
		return "", err
	}

	services := make(map[string]ComposeService, len(stack.Services))
	entrypoint := defaultEntrypoint
	if tls.Enabled {
		entrypoint = defaultTLSEntrypoint
	}
	for _, svc := range stack.Services {
		command := buildCommand(svc.Command)
		serviceLabels := buildTraefikLabels(svc, entrypoint, tls.Enabled)
		workDir := filepath.ToSlash(filepath.Join("/repo", svc.WorkspacePath))
		services[svc.ComposeService] = ComposeService{
			Image:       defaultNodeImage,
			WorkingDir:  workDir,
			Command:     command,
			Volumes:     []string{fmt.Sprintf("%s:/repo", stack.RepoPath)},
			Environment: svc.Env,
			Labels:      serviceLabels,
			Networks:    []string{DevrouterNetwork},
		}
	}

	compose := ComposeFile{
		Version:  "3.9",
		Services: services,
		Networks: map[string]ComposeNetwork{
			DevrouterNetwork: {External: true},
		},
	}

	payload, err := yaml.Marshal(&compose)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(composePath, payload, 0o644); err != nil {
		return "", err
	}
	return composePath, nil
}

func buildCommand(command string) []string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		trimmed = "pnpm dev"
	}
	if strings.Contains(trimmed, "pnpm") {
		trimmed = "corepack enable && " + trimmed
	}
	return []string{"sh", "-lc", trimmed}
}

// buildTraefikLabels generates Traefik labels for a service.
// It creates labels for the main port and any extra ports defined in ExtraPorts.
func buildTraefikLabels(svc ServiceSpec, entrypoint string, tlsEnabled bool) []string {
	labels := []string{
		defaultTraefikEnableLabel,
		defaultTraefikNetworkLabel,
	}

	// Main port labels
	labels = append(labels, buildRouterLabels(svc.ComposeService, svc.Host, svc.Port, entrypoint, tlsEnabled)...)

	// Extra ports labels (e.g., for MinIO console on port 9001)
	for _, ep := range svc.ExtraPorts {
		routerName := svc.ComposeService + "-" + ep.Suffix
		host := buildHostWithSuffix(svc.Host, ep.Suffix)
		labels = append(labels, buildRouterLabels(routerName, host, ep.Port, entrypoint, tlsEnabled)...)
	}

	return labels
}

// buildRouterLabels generates Traefik router and service labels for a single port.
func buildRouterLabels(routerName, host string, port int, entrypoint string, tlsEnabled bool) []string {
	labels := []string{
		fmt.Sprintf(defaultTraefikRouterRuleLabel, routerName, host),
		fmt.Sprintf(defaultTraefikRouterEntryLabel, routerName, entrypoint),
		fmt.Sprintf(defaultTraefikServicePortLabel, routerName, port),
		fmt.Sprintf("traefik.http.routers.%s.service=%s", routerName, routerName),
	}
	if tlsEnabled {
		labels = append(labels, fmt.Sprintf("traefik.http.routers.%s.tls=true", routerName))
	}
	return labels
}

// buildHostWithSuffix inserts a suffix before the domain.
// Example: "stack-minio.localtest.me" + "console" -> "stack-minio-console.localtest.me"
func buildHostWithSuffix(host, suffix string) string {
	if suffix == "" {
		return host
	}
	parts := strings.SplitN(host, ".", 2)
	if len(parts) == 2 {
		return parts[0] + "-" + suffix + "." + parts[1]
	}
	return host + "-" + suffix
}
