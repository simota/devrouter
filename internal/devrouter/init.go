package devrouter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type InitOptions struct {
	RepoPath    string
	ComposePath string
	OutputPath  string
	Stack       string
	Domain      string
	Force       bool
}

type InitResult struct {
	OutputPath string
	Warnings   []string
}

type ConfigFile struct {
	Stack     string                   `yaml:"stack"`
	Domain    string                   `yaml:"domain"`
	TLS       TLSConfigFile            `yaml:"tls,omitempty"`
	Defaults  DefaultsConfigFile       `yaml:"defaults"`
	Overrides map[string]ServiceConfig `yaml:"overrides"`
}

type DefaultsConfigFile struct {
	Env map[string]string `yaml:"env"`
}

type TLSConfigFile struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	CertFile string `yaml:"certFile,omitempty"`
	KeyFile  string `yaml:"keyFile,omitempty"`
}

type ServiceConfig struct {
	Name    string            `yaml:"name,omitempty"`
	Port    int               `yaml:"port,omitempty"`
	Command string            `yaml:"command,omitempty"`
	Host    string            `yaml:"host,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

type ComposeServiceInfo struct {
	Name    string
	Port    int
	Env     map[string]string
	Command string
}

type composeFileRaw struct {
	Services map[string]composeServiceRaw `yaml:"services"`
}

type composeServiceRaw struct {
	Ports       []any `yaml:"ports"`
	Expose      []any `yaml:"expose"`
	Environment any   `yaml:"environment"`
	Command     any   `yaml:"command"`
}

func InitConfig(opts InitOptions) (InitResult, error) {
	repoPath := opts.RepoPath
	if repoPath == "" {
		repoPath = "."
	}
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return InitResult{}, err
	}
	info, err := os.Stat(repoPath)
	if err != nil {
		return InitResult{}, err
	}
	if !info.IsDir() {
		repoPath = filepath.Dir(repoPath)
	}

	composePath, err := resolveComposePath(repoPath, opts.ComposePath)
	if err != nil {
		return InitResult{}, err
	}
	outputPath := resolveOutputPath(repoPath, opts.OutputPath)
	if err := ensureOutputPath(outputPath, opts.Force); err != nil {
		return InitResult{}, err
	}

	composeServices, composeWarnings, err := loadComposeServices(composePath)
	if err != nil {
		return InitResult{}, err
	}

	cfg := DefaultConfig()
	cfg.Overrides = map[string]ServiceConfigDiff{}
	if opts.Domain != "" {
		cfg.Domain = opts.Domain
	}
	stack, err := DiscoverStack(repoPath, cfg)
	if err != nil {
		return InitResult{}, err
	}

	stackName := opts.Stack
	if stackName == "" {
		stackName = filepath.Base(repoPath)
	}
	domain := opts.Domain
	if domain == "" {
		domain = DefaultDomain
	}

	output := ConfigFile{
		Stack:  stackName,
		Domain: domain,
		Defaults: DefaultsConfigFile{
			Env: map[string]string{
				"HOST": "0.0.0.0",
			},
		},
		Overrides: map[string]ServiceConfig{},
	}

	matched := map[string]bool{}
	for _, svc := range stack.Services {
		key := NormalizeName(svc.Name)
		if info, ok := composeServices[key]; ok {
			diff := ServiceConfig{}
			if info.Port > 0 {
				diff.Port = info.Port
			}
			if info.Command != "" {
				diff.Command = info.Command
			}
			if len(info.Env) > 0 {
				diff.Env = info.Env
			}
			if diff.Port != 0 || diff.Command != "" || len(diff.Env) > 0 {
				output.Overrides[svc.WorkspacePath] = diff
				matched[key] = true
			}
		}
	}

	warnings := append([]string{}, composeWarnings...)
	unmatched := collectUnmatched(composeServices, matched)
	if len(unmatched) > 0 {
		warnings = append(warnings, fmt.Sprintf("compose services not matched to workspaces: %s", strings.Join(unmatched, ", ")))
	}
	if len(output.Overrides) == 0 {
		warnings = append(warnings, "no overrides were generated; check compose service names or add overrides manually")
	}

	payload, err := yaml.Marshal(output)
	if err != nil {
		return InitResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return InitResult{}, err
	}
	if err := os.WriteFile(outputPath, payload, 0o644); err != nil {
		return InitResult{}, err
	}
	return InitResult{OutputPath: outputPath, Warnings: warnings}, nil
}

func resolveComposePath(repoPath, composePath string) (string, error) {
	if composePath != "" {
		if !filepath.IsAbs(composePath) {
			composePath = filepath.Join(repoPath, composePath)
		}
		if _, err := os.Stat(composePath); err != nil {
			return "", err
		}
		return composePath, nil
	}
	candidates := []string{
		filepath.Join(repoPath, "docker-compose.yaml"),
		filepath.Join(repoPath, "docker-compose.yml"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("docker-compose.yaml not found (use --compose)")
}

func resolveOutputPath(repoPath, outputPath string) string {
	if outputPath == "" {
		return filepath.Join(repoPath, "devrouter.yaml")
	}
	if filepath.IsAbs(outputPath) {
		return outputPath
	}
	return filepath.Join(repoPath, outputPath)
}

func ensureOutputPath(outputPath string, force bool) error {
	if !force {
		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("output already exists: %s", outputPath)
		}
	}
	return nil
}

func loadComposeServices(path string) (map[string]ComposeServiceInfo, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var raw composeFileRaw
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil, err
	}
	result := map[string]ComposeServiceInfo{}
	warnings := []string{}
	for name, svc := range raw.Services {
		normalized := NormalizeName(name)
		if _, exists := result[normalized]; exists {
			warnings = append(warnings, fmt.Sprintf("compose service name collision after normalization: %s", name))
			continue
		}
		info := ComposeServiceInfo{
			Name:    name,
			Port:    parseFirstPort(svc.Ports, svc.Expose),
			Env:     parseEnv(svc.Environment),
			Command: parseCommand(svc.Command),
		}
		result[normalized] = info
	}
	return result, warnings, nil
}

func parseFirstPort(ports []any, expose []any) int {
	if port := parsePortsList(ports); port > 0 {
		return port
	}
	return parsePortsList(expose)
}

func parsePortsList(items []any) int {
	for _, item := range items {
		if port := parsePortItem(item); port > 0 {
			return port
		}
	}
	return 0
}

// parsePortItem extracts the container-side port from compose "ports"/"expose".
// Compose entries can be numbers, strings like "HOST:CONTAINER/proto", or maps
// with a "target" field (long syntax), and yaml.v3 may decode maps as map[any]any.
// We only need the service port for routing labels, so host bindings are ignored.
func parsePortItem(item any) int {
	switch v := item.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		return parsePortString(v)
	case map[string]any:
		if target, ok := v["target"]; ok {
			return parsePortItem(target)
		}
	case map[any]any:
		if target, ok := v["target"]; ok {
			return parsePortItem(target)
		}
		for key, value := range v {
			if fmt.Sprint(key) == "target" {
				return parsePortItem(value)
			}
		}
	}
	return 0
}

func parsePortString(input string) int {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0
	}
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	parts := strings.Split(trimmed, ":")
	last := parts[len(parts)-1]
	if idx := strings.Index(last, "-"); idx >= 0 {
		last = last[:idx]
	}
	var port int
	_, err := fmt.Sscanf(last, "%d", &port)
	if err != nil {
		return 0
	}
	return port
}

func parseEnv(raw any) map[string]string {
	if raw == nil {
		return nil
	}
	env := map[string]string{}
	switch v := raw.(type) {
	case map[string]any:
		for key, value := range v {
			env[key] = fmt.Sprint(value)
		}
	case map[any]any:
		for key, value := range v {
			env[fmt.Sprint(key)] = fmt.Sprint(value)
		}
	case []any:
		for _, item := range v {
			entry := strings.TrimSpace(fmt.Sprint(item))
			if entry == "" {
				continue
			}
			parts := strings.SplitN(entry, "=", 2)
			key := parts[0]
			if key == "" {
				continue
			}
			value := ""
			if len(parts) == 2 {
				value = parts[1]
			}
			env[key] = value
		}
	}
	if len(env) == 0 {
		return nil
	}
	return env
}

func parseCommand(raw any) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, " ")
	}
	return ""
}

func collectUnmatched(services map[string]ComposeServiceInfo, matched map[string]bool) []string {
	unmatched := make([]string, 0)
	for key, svc := range services {
		if !matched[key] {
			unmatched = append(unmatched, svc.Name)
		}
	}
	sort.Strings(unmatched)
	return unmatched
}
