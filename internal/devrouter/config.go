package devrouter

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const DefaultDomain = "localtest.me"

func DefaultConfig() Config {
	return Config{
		Domain: DefaultDomain,
		TLS:    TLSConfig{},
		Defaults: DefaultsConfig{
			Env: map[string]string{
				"HOST": "0.0.0.0",
			},
		},
		Overrides: map[string]ServiceConfigDiff{},
	}
}

func LoadConfig(repoPath, configPath string) (Config, string, error) {
	defaultCfg := DefaultConfig()
	if configPath == "" {
		candidate := filepath.Join(repoPath, "devrouter.yaml")
		info, err := os.Stat(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return defaultCfg, "", nil
			}
			return Config{}, "", err
		}
		if info.IsDir() {
			return Config{}, "", errors.New("devrouter.yaml is a directory")
		}
		configPath = candidate
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, configPath, err
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, configPath, err
	}
	cfg.Overrides = normalizeOverrides(cfg.Overrides)
	if cfg.Domain == "" {
		cfg.Domain = DefaultDomain
	}
	if cfg.Defaults.Env == nil {
		cfg.Defaults.Env = defaultCfg.Defaults.Env
	} else if _, ok := cfg.Defaults.Env["HOST"]; !ok {
		cfg.Defaults.Env["HOST"] = "0.0.0.0"
	}
	return cfg, configPath, nil
}

func normalizeOverrides(input map[string]ServiceConfigDiff) map[string]ServiceConfigDiff {
	if input == nil {
		return map[string]ServiceConfigDiff{}
	}
	out := make(map[string]ServiceConfigDiff, len(input))
	for key, value := range input {
		out[NormalizeRelPath(key)] = value
	}
	return out
}
