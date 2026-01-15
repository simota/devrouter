package devrouter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DoctorLevel string

const (
	DoctorOK    DoctorLevel = "ok"
	DoctorWarn  DoctorLevel = "warn"
	DoctorError DoctorLevel = "error"
)

type DoctorCheck struct {
	Level   DoctorLevel
	Name    string
	Message string
	Hint    string
}

type DoctorReport struct {
	Checks []DoctorCheck
}

type DoctorOptions struct {
	ConfigPath  string
	ComposePath string
}

func (r DoctorReport) HasErrors() bool {
	for _, check := range r.Checks {
		if check.Level == DoctorError {
			return true
		}
	}
	return false
}

func Doctor(repoPath string, opts DoctorOptions) (DoctorReport, error) {
	repoPath = resolveRepoPath(repoPath)
	report := DoctorReport{Checks: []DoctorCheck{}}

	if err := CheckDocker(); err != nil {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorError,
			Name:    "docker",
			Message: err.Error(),
			Hint:    "docker/colima を起動してください",
		})
		return report, nil
	}
	report.Checks = append(report.Checks, DoctorCheck{
		Level:   DoctorOK,
		Name:    "docker",
		Message: "docker info OK",
	})

	networkExists, err := NetworkExists(DevrouterNetwork)
	if err != nil {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorError,
			Name:    "network",
			Message: err.Error(),
			Hint:    "devrouter daemon up を実行してください",
		})
	} else if networkExists {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorOK,
			Name:    "network",
			Message: fmt.Sprintf("%s exists", DevrouterNetwork),
		})
	} else {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorWarn,
			Name:    "network",
			Message: fmt.Sprintf("%s not found", DevrouterNetwork),
			Hint:    "devrouter daemon up を実行してください",
		})
	}

	traefikRunning, err := TraefikRunning()
	if err != nil {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorError,
			Name:    "traefik",
			Message: err.Error(),
		})
	} else if traefikRunning {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorOK,
			Name:    "traefik",
			Message: "running",
		})
	} else {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorWarn,
			Name:    "traefik",
			Message: "not running",
			Hint:    "devrouter daemon up を実行してください",
		})
	}

	cfg, configPath, err := LoadConfig(repoPath, opts.ConfigPath)
	if err != nil {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorError,
			Name:    "config",
			Message: err.Error(),
		})
		return report, nil
	}
	if configPath == "" {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorWarn,
			Name:    "config",
			Message: "devrouter.yaml not found",
			Hint:    "devrouter init で生成できます",
		})
	} else {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorOK,
			Name:    "config",
			Message: fmt.Sprintf("%s", configPath),
		})
	}
	if cfg.TLS.Enabled {
		normalizedTLS, err := NormalizeTLSConfig(cfg.TLS)
		if err != nil {
			report.Checks = append(report.Checks, DoctorCheck{
				Level:   DoctorWarn,
				Name:    "tls",
				Message: err.Error(),
				Hint:    "mkcert で証明書を作成し、devrouter.yaml の tls.certFile/keyFile を設定してください",
			})
		} else {
			report.Checks = append(report.Checks, DoctorCheck{
				Level:   DoctorOK,
				Name:    "tls",
				Message: fmt.Sprintf("enabled (%s)", normalizedTLS.CertFile),
			})
		}
	}

	composePath, useCompose, err := resolveComposeForDoctor(repoPath, cfg, opts)
	if err != nil {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorError,
			Name:    "compose",
			Message: err.Error(),
		})
		return report, nil
	}
	if useCompose {
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorOK,
			Name:    "compose",
			Message: composePath,
		})
		_, warnings, err := LoadStackFromCompose(repoPath, cfg, composePath)
		if err != nil {
			report.Checks = append(report.Checks, DoctorCheck{
				Level:   DoctorError,
				Name:    "compose",
				Message: err.Error(),
			})
			return report, nil
		}
		for _, warning := range warnings {
			report.Checks = append(report.Checks, DoctorCheck{
				Level:   DoctorWarn,
				Name:    "compose",
				Message: warning,
			})
		}
	} else {
		stack, err := DiscoverStack(repoPath, cfg)
		if err != nil {
			report.Checks = append(report.Checks, DoctorCheck{
				Level:   DoctorError,
				Name:    "discover",
				Message: err.Error(),
			})
			return report, nil
		}
		report.Checks = append(report.Checks, DoctorCheck{
			Level:   DoctorOK,
			Name:    "discover",
			Message: fmt.Sprintf("%d services", len(stack.Services)),
		})
	}

	return report, nil
}

func resolveRepoPath(repoPath string) string {
	if repoPath == "" {
		repoPath = "."
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return repoPath
	}
	info, err := os.Stat(abs)
	if err != nil {
		return abs
	}
	if !info.IsDir() {
		return filepath.Dir(abs)
	}
	return abs
}

func resolveComposeForDoctor(repoPath string, cfg Config, opts DoctorOptions) (string, bool, error) {
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
	composePath = filepath.Clean(composePath)
	return composePath, true, nil
}

func TraefikRunning() (bool, error) {
	output, err := DockerOutput("ps", "--filter", "ancestor=traefik", "--format", "{{.Names}}")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}
