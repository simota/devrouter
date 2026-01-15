package devrouter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type UpOptions struct {
	ConfigPath  string
	ComposePath string
	Domain      string
	AutoDaemon  bool
	Build       bool
}

type DownOptions struct {
	Target string
}

type LogsOptions struct {
	Follow bool
}

func Up(repoPath string, opts UpOptions) (StackRecord, []string, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return StackRecord{}, nil, err
	}
	cfg, _, err := LoadConfig(repoPath, opts.ConfigPath)
	if err != nil {
		return StackRecord{}, nil, err
	}
	if opts.Domain != "" {
		cfg.Domain = opts.Domain
	}
	if cfg.TLS.Enabled {
		normalizedTLS, err := NormalizeTLSConfig(cfg.TLS)
		if err != nil {
			return StackRecord{}, nil, err
		}
		cfg.TLS = normalizedTLS
	}
	scheme := SchemeForTLS(cfg.TLS)
	composePath, useCompose, err := resolveComposeForUp(repoPath, cfg, opts)
	if err != nil {
		return StackRecord{}, nil, err
	}
	var stack StackSpec
	var warnings []string
	source := "generated"
	if useCompose {
		stack, warnings, err = LoadStackFromCompose(repoPath, cfg, composePath)
		if err != nil {
			return StackRecord{}, nil, err
		}
		source = "compose"
	} else {
		stack, err = DiscoverStack(repoPath, cfg)
		if err != nil {
			return StackRecord{}, nil, err
		}
		composePath, err = WriteCompose(stack, cfg.TLS)
		if err != nil {
			return StackRecord{}, nil, err
		}
	}
	if err := CheckDocker(); err != nil {
		return StackRecord{}, nil, err
	}
	if opts.AutoDaemon {
		if err := DaemonUp(DaemonOptions{TLS: cfg.TLS}); err != nil {
			return StackRecord{}, nil, err
		}
	} else {
		if err := EnsureNetwork(); err != nil {
			return StackRecord{}, nil, err
		}
	}
	reg, err := LoadRegistry()
	if err != nil {
		return StackRecord{}, nil, err
	}
	if existing, ok := FindStackByRepoPath(reg, repoPath); ok && existing.ID != stack.ID {
		if err := ComposeDown(existing.ComposeFilePath); err != nil {
			return StackRecord{}, nil, err
		}
		reg, _ = RemoveStack(reg, existing.ID)
	}
	if err := ComposeUp(composePath, opts.Build); err != nil {
		return StackRecord{}, nil, err
	}
	record := StackRecordFromSpec(stack, composePath, source, scheme)
	reg = UpsertStack(reg, record)
	if err := SaveRegistry(reg); err != nil {
		return StackRecord{}, nil, err
	}

	// Record stack up event
	serviceNames := make([]string, 0, len(record.Services))
	for _, svc := range record.Services {
		serviceNames = append(serviceNames, svc.Name)
	}
	_ = RecordStackUp(record.ID, record.Name, serviceNames)

	return record, warnings, nil
}

func Down(opts DownOptions) (StackRecord, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return StackRecord{}, err
	}
	stack, err := resolveStack(reg, opts.Target)
	if err != nil {
		return StackRecord{}, err
	}

	// Parse upSince for duration calculation
	var upSince time.Time
	if stack.LastUpAt != "" {
		upSince, _ = time.Parse(time.RFC3339, stack.LastUpAt)
	}

	if err := ComposeDown(stack.ComposeFilePath); err != nil {
		return StackRecord{}, err
	}
	reg, _ = RemoveStack(reg, stack.ID)
	if err := SaveRegistry(reg); err != nil {
		return StackRecord{}, err
	}

	// Record stack down event
	_ = RecordStackDown(stack.ID, stack.Name, upSince)

	return stack, nil
}

func List() (Registry, error) {
	return LoadRegistry()
}

func Logs(target string, opts LogsOptions) error {
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}
	stackName, serviceName := splitTarget(target)
	stack, err := resolveStack(reg, stackName)
	if err != nil {
		return err
	}
	serviceKey := ""
	if serviceName != "" {
		service, ok := FindService(stack, serviceName)
		if !ok {
			return fmt.Errorf("service not found: %s", serviceName)
		}
		serviceKey = service.ComposeService
	}
	return ComposeLogs(stack.ComposeFilePath, opts.Follow, serviceKey)
}

func Open(target string) (string, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return "", err
	}
	stackName, serviceName := splitTarget(target)
	if serviceName == "" {
		return "", errors.New("service name is required (use <stack>/<service>)")
	}
	stack, err := resolveStack(reg, stackName)
	if err != nil {
		return "", err
	}
	service, ok := FindService(stack, serviceName)
	if !ok {
		return "", fmt.Errorf("service not found: %s", serviceName)
	}
	return service.URL, nil
}

func OpenURL(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return runCommand(cmd, args, os.Stdout, os.Stderr)
}

func Exec(target string, command []string, opts ExecOptions) error {
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}
	stackName, serviceName := splitTarget(target)
	if serviceName == "" {
		return errors.New("service name is required (use <stack>/<service>)")
	}
	stack, err := resolveStack(reg, stackName)
	if err != nil {
		return err
	}
	service, ok := FindService(stack, serviceName)
	if !ok {
		return fmt.Errorf("service not found: %s", serviceName)
	}
	if len(command) == 0 {
		return errors.New("command is required after --")
	}
	return ComposeExec(stack.ComposeFilePath, service.ComposeService, command, opts)
}

func resolveComposeForUp(repoPath string, cfg Config, opts UpOptions) (string, bool, error) {
	composePath := opts.ComposePath
	if composePath == "" {
		composePath = cfg.Compose
	}
	if composePath == "" {
		return "", false, nil
	}
	if !filepath.IsAbs(composePath) {
		composePath = filepath.Join(repoPath, composePath)
	}
	if _, err := os.Stat(composePath); err != nil {
		return "", false, err
	}
	return composePath, true, nil
}

func resolveStack(reg Registry, target string) (StackRecord, error) {
	if target == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return StackRecord{}, err
		}
		target = cwd
	}
	if info, err := os.Stat(target); err == nil {
		repoPath := target
		if !info.IsDir() {
			repoPath = filepath.Dir(target)
		}
		repoPath, err = filepath.Abs(repoPath)
		if err != nil {
			return StackRecord{}, err
		}
		if stack, ok := FindStackByRepoPath(reg, repoPath); ok {
			return stack, nil
		}
		return StackRecord{}, fmt.Errorf("stack not found for repo path: %s", repoPath)
	}
	if stack, ok := FindStack(reg, target); ok {
		return stack, nil
	}
	return StackRecord{}, fmt.Errorf("stack not found: %s", target)
}

func splitTarget(input string) (string, string) {
	if input == "" {
		return "", ""
	}
	parts := strings.SplitN(input, "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}
