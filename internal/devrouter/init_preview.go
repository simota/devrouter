package devrouter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	warningComposeMissing = "compose file not found; overrides were not generated"
	warningNoOverrides    = "no overrides were generated; check compose service names or add overrides manually"
)

var (
	ErrInitComposeMissing = errors.New("init compose missing")
	ErrInitOutputExists   = errors.New("init output already exists")
)

type InitPreviewOptions struct {
	RepoPath    string
	ComposePath string
	Stack       string
	Domain      string
}

type InitApplyOptions struct {
	RepoPath    string
	ComposePath string
	Stack       string
	Domain      string
	Force       bool
}

type InitPreviewResult struct {
	OutputPath    string
	Content       string
	Warnings      []string
	ApplyAllowed  bool
	RequiresForce bool
}

func PreviewInitConfig(opts InitPreviewOptions) (InitPreviewResult, error) {
	repoPath, err := resolveInitRepoPath(opts.RepoPath)
	if err != nil {
		return InitPreviewResult{}, err
	}

	outputPath := resolveOutputPath(repoPath, "")

	config, warnings, composeFound, err := buildInitConfigFile(repoPath, opts.ComposePath, opts.Stack, opts.Domain)
	if err != nil {
		return InitPreviewResult{}, err
	}

	payload, err := yaml.Marshal(config)
	if err != nil {
		return InitPreviewResult{}, err
	}

	requiresForce, err := outputExists(outputPath)
	if err != nil {
		return InitPreviewResult{}, err
	}

	return InitPreviewResult{
		OutputPath:    outputPath,
		Content:       string(payload),
		Warnings:      warnings,
		ApplyAllowed:  composeFound,
		RequiresForce: requiresForce,
	}, nil
}

func ApplyInitConfig(opts InitApplyOptions) (InitPreviewResult, error) {
	preview, err := PreviewInitConfig(InitPreviewOptions{
		RepoPath:    opts.RepoPath,
		ComposePath: opts.ComposePath,
		Stack:       opts.Stack,
		Domain:      opts.Domain,
	})
	if err != nil {
		return InitPreviewResult{}, err
	}

	if !preview.ApplyAllowed {
		return preview, ErrInitComposeMissing
	}

	if preview.RequiresForce && !opts.Force {
		return preview, ErrInitOutputExists
	}

	if err := os.MkdirAll(filepath.Dir(preview.OutputPath), 0o755); err != nil {
		return preview, err
	}

	if err := os.WriteFile(preview.OutputPath, []byte(preview.Content), 0o644); err != nil {
		return preview, err
	}

	return preview, nil
}

func buildInitConfigFile(repoPath, composePath, stackName, domain string) (ConfigFile, []string, bool, error) {
	repoPath = filepath.Clean(repoPath)

	composeServices := map[string]ComposeServiceInfo{}
	composeWarnings := []string{}
	composeFound := false

	if composePath != "" {
		info, err := os.Stat(composePath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return ConfigFile{}, nil, false, err
			}
		} else if !info.IsDir() {
			services, warnings, err := loadComposeServices(composePath)
			if err != nil {
				return ConfigFile{}, nil, false, err
			}
			composeServices = services
			composeWarnings = warnings
			composeFound = true
		}
	}

	cfg := DefaultConfig()
	cfg.Overrides = map[string]ServiceConfigDiff{}
	if strings.TrimSpace(domain) != "" {
		cfg.Domain = strings.TrimSpace(domain)
	}

	stack, err := DiscoverStack(repoPath, cfg)
	if err != nil {
		return ConfigFile{}, nil, composeFound, err
	}

	stackName = strings.TrimSpace(stackName)
	if stackName == "" {
		stackName = filepath.Base(repoPath)
	}

	domain = strings.TrimSpace(domain)
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
	if composeFound {
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
	}

	warnings := append([]string{}, composeWarnings...)
	if composeFound {
		unmatched := collectUnmatched(composeServices, matched)
		if len(unmatched) > 0 {
			warnings = append(warnings, fmt.Sprintf("compose services not matched to workspaces: %s", strings.Join(unmatched, ", ")))
		}
		if len(output.Overrides) == 0 {
			warnings = append(warnings, warningNoOverrides)
		}
	} else {
		warnings = append(warnings, warningComposeMissing)
	}

	return output, warnings, composeFound, nil
}

func resolveInitRepoPath(path string) (string, error) {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	return abs, nil
}

func outputExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
