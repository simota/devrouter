package devrouter

import (
	"os"
	"path/filepath"
)

func BaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".devrouter"), nil
}

func RegistryPath() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "registry.json"), nil
}

func StackDir(stackID string) (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "stacks", stackID), nil
}

func StackComposePath(stackID string) (string, error) {
	dir, err := StackDir(stackID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "docker-compose.yml"), nil
}

func DaemonDir() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "daemon"), nil
}

func DaemonComposePath() (string, error) {
	dir, err := DaemonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "docker-compose.yml"), nil
}

func DaemonTLSConfigPath() (string, error) {
	dir, err := DaemonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tls.yaml"), nil
}
