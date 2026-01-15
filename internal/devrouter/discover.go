package devrouter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type PackageJSON struct {
	Name            string            `json:"name"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type pnpmWorkspace struct {
	Packages []string `yaml:"packages"`
}

func DiscoverStack(repoPath string, cfg Config) (StackSpec, error) {
	repoPath = filepath.Clean(repoPath)
	stackName := filepath.Base(repoPath)
	patterns, err := readWorkspacePatterns(repoPath)
	if err != nil {
		return StackSpec{}, err
	}
	workspaceDirs, err := findWorkspaceDirs(repoPath, patterns)
	if err != nil {
		return StackSpec{}, err
	}
	if len(workspaceDirs) == 0 {
		return StackSpec{}, errors.New("no workspaces found")
	}

	services := make([]ServiceSpec, 0)
	for _, dir := range workspaceDirs {
		pkg, err := readPackageJSON(dir)
		if err != nil {
			return StackSpec{}, err
		}
		if _, ok := pkg.Scripts["dev"]; !ok {
			continue
		}
		rel, err := filepath.Rel(repoPath, dir)
		if err != nil {
			return StackSpec{}, err
		}
		rel = NormalizeRelPath(rel)
		serviceName := serviceNameFromPath(stackName, rel)
		framework := detectFramework(pkg)
		port := defaultPortFor(framework)
		services = append(services, ServiceSpec{
			Name:          serviceName,
			WorkspacePath: rel,
			WorkspaceAbs:  dir,
			Command:       "pnpm dev",
			Port:          port,
			Env:           map[string]string{},
		})
	}
	if len(services) == 0 {
		return StackSpec{}, errors.New("no services with scripts.dev found")
	}

	stack := StackSpec{
		Name:     stackName,
		RepoPath: repoPath,
		Domain:   cfg.Domain,
		Services: services,
	}
	applyConfig(&stack, cfg)
	return stack, nil
}

func readWorkspacePatterns(repoPath string) ([]string, error) {
	paths := []string{
		filepath.Join(repoPath, "pnpm-workspace.yaml"),
		filepath.Join(repoPath, "pnpm-workspace.yml"),
	}
	for _, candidate := range paths {
		info, err := os.Stat(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		data, err := os.ReadFile(candidate)
		if err != nil {
			return nil, err
		}
		var ws pnpmWorkspace
		if err := yaml.Unmarshal(data, &ws); err != nil {
			return nil, err
		}
		return ws.Packages, nil
	}
	return nil, nil
}

func findWorkspaceDirs(repoPath string, patterns []string) ([]string, error) {
	include, exclude, err := compilePatterns(patterns)
	if err != nil {
		return nil, err
	}
	found := make(map[string]struct{})
	err = filepath.WalkDir(repoPath, func(pathname string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "package.json" {
			return nil
		}
		dir := filepath.Dir(pathname)
		rel, err := filepath.Rel(repoPath, dir)
		if err != nil {
			return err
		}
		rel = NormalizeRelPath(rel)
		if rel == "" {
			rel = "."
		}
		if len(include) == 0 {
			if rel == "." {
				found[dir] = struct{}{}
			}
			return nil
		}
		if matchAny(include, rel) && !matchAny(exclude, rel) {
			found[dir] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	dirs := make([]string, 0, len(found))
	for dir := range found {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs, nil
}

func readPackageJSON(dir string) (PackageJSON, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return PackageJSON{}, err
	}
	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return PackageJSON{}, err
	}
	if pkg.Scripts == nil {
		pkg.Scripts = map[string]string{}
	}
	if pkg.Dependencies == nil {
		pkg.Dependencies = map[string]string{}
	}
	if pkg.DevDependencies == nil {
		pkg.DevDependencies = map[string]string{}
	}
	return pkg, nil
}

func serviceNameFromPath(stackName, relPath string) string {
	if relPath == "." || relPath == "" {
		return stackName
	}
	return path.Base(relPath)
}

func detectFramework(pkg PackageJSON) string {
	if hasDep(pkg, "next") {
		return "next"
	}
	if hasDep(pkg, "vite") {
		return "vite"
	}
	if hasDep(pkg, "hono") || hasDepPrefix(pkg, "@hono/") {
		return "hono"
	}
	return "unknown"
}

func hasDep(pkg PackageJSON, name string) bool {
	if _, ok := pkg.Dependencies[name]; ok {
		return true
	}
	if _, ok := pkg.DevDependencies[name]; ok {
		return true
	}
	return false
}

func hasDepPrefix(pkg PackageJSON, prefix string) bool {
	for key := range pkg.Dependencies {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	for key := range pkg.DevDependencies {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func defaultPortFor(framework string) int {
	switch framework {
	case "next":
		return 3000
	case "vite":
		return 5173
	case "hono":
		return 4000
	default:
		return 3000
	}
}

type compiledPattern struct {
	raw string
	re  *regexp.Regexp
}

func compilePatterns(patterns []string) ([]compiledPattern, []compiledPattern, error) {
	include := make([]compiledPattern, 0)
	exclude := make([]compiledPattern, 0)
	for _, raw := range patterns {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		negate := strings.HasPrefix(trimmed, "!")
		if negate {
			trimmed = strings.TrimPrefix(trimmed, "!")
		}
		trimmed = NormalizeRelPath(trimmed)
		if trimmed == "" {
			continue
		}
		regex, err := globToRegex(trimmed)
		if err != nil {
			return nil, nil, err
		}
		entry := compiledPattern{raw: trimmed, re: regex}
		if negate {
			exclude = append(exclude, entry)
		} else {
			include = append(include, entry)
		}
	}
	return include, exclude, nil
}

func globToRegex(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch ch {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func matchAny(patterns []compiledPattern, rel string) bool {
	for _, entry := range patterns {
		if entry.re.MatchString(rel) {
			return true
		}
	}
	return false
}

func applyConfig(stack *StackSpec, cfg Config) {
	if cfg.Stack != "" {
		stack.Name = cfg.Stack
	}
	stack.Name = NormalizeName(stack.Name)
	if cfg.Domain == "" {
		stack.Domain = DefaultDomain
	} else {
		stack.Domain = cfg.Domain
	}

	defaults := cfg.Defaults.Env
	if defaults == nil {
		defaults = map[string]string{"HOST": "0.0.0.0"}
	}

	for i := range stack.Services {
		service := &stack.Services[i]
		if diff, ok := cfg.Overrides[NormalizeRelPath(service.WorkspacePath)]; ok {
			if diff.Name != "" {
				service.Name = diff.Name
			}
			if diff.Command != "" {
				service.Command = diff.Command
			}
			if diff.Port != 0 {
				service.Port = diff.Port
			}
			if diff.Host != "" {
				service.Host = diff.Host
			}
			service.Env = mergeEnv(service.Env, diff.Env)
			service.HealthCheck = diff.HealthCheck
		}

		service.Name = NormalizeName(service.Name)
	}

	ensureUniqueServiceNames(stack.Services)

	for i := range stack.Services {
		service := &stack.Services[i]
		service.Env = mergeEnv(defaults, service.Env)
		if service.Port > 0 {
			if _, ok := service.Env["PORT"]; !ok {
				service.Env["PORT"] = fmt.Sprintf("%d", service.Port)
			}
		}
		if service.Host == "" {
			service.Host = fmt.Sprintf("%s-%s.%s", stack.Name, service.Name, stack.Domain)
		}
		service.ComposeService = ComposeServiceName(stack.Name, service.Name)
	}

	stack.ID = StackID(stack.Name, stack.RepoPath)
}

func mergeEnv(base map[string]string, override map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(override))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range override {
		out[key] = value
	}
	return out
}

func ensureUniqueServiceNames(services []ServiceSpec) {
	seen := map[string]int{}
	for i := range services {
		name := services[i].Name
		count := seen[name] + 1
		seen[name] = count
		if count > 1 {
			services[i].Name = fmt.Sprintf("%s-%d", name, count)
		}
	}
}
