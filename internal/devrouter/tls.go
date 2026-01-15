package devrouter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	traefikTLSCertPath   = "/certs/devrouter.pem"
	traefikTLSKeyPath    = "/certs/devrouter-key.pem"
	traefikTLSConfigPath = "/etc/traefik/tls.yaml"
)

type traefikTLSConfig struct {
	TLS struct {
		Certificates []traefikTLSCert `yaml:"certificates"`
	} `yaml:"tls"`
}

type traefikTLSCert struct {
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
}

func NormalizeTLSConfig(cfg TLSConfig) (TLSConfig, error) {
	if !cfg.Enabled {
		return TLSConfig{}, nil
	}
	if cfg.CertFile == "" || cfg.KeyFile == "" {
		return TLSConfig{}, fmt.Errorf("tls.enabled is true but certFile/keyFile is missing")
	}
	certPath, err := expandUserPath(cfg.CertFile)
	if err != nil {
		return TLSConfig{}, err
	}
	keyPath, err := expandUserPath(cfg.KeyFile)
	if err != nil {
		return TLSConfig{}, err
	}
	certPath, err = filepath.Abs(certPath)
	if err != nil {
		return TLSConfig{}, err
	}
	keyPath, err = filepath.Abs(keyPath)
	if err != nil {
		return TLSConfig{}, err
	}
	if err := validateFile(certPath, "tls.certFile"); err != nil {
		return TLSConfig{}, err
	}
	if err := validateFile(keyPath, "tls.keyFile"); err != nil {
		return TLSConfig{}, err
	}
	return TLSConfig{
		Enabled:  true,
		CertFile: certPath,
		KeyFile:  keyPath,
	}, nil
}

func SchemeForTLS(cfg TLSConfig) string {
	if cfg.Enabled {
		return "https"
	}
	return "http"
}

func WriteTraefikTLSConfig(path string) error {
	cfg := traefikTLSConfig{}
	cfg.TLS.Certificates = []traefikTLSCert{
		{
			CertFile: traefikTLSCertPath,
			KeyFile:  traefikTLSKeyPath,
		},
	}
	payload, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func expandUserPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func validateFile(path string, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s not found: %w", label, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s must be a file: %s", label, path)
	}
	return nil
}
