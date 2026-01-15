package devrouter

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const traefikImage = "traefik:v2.11"

type DaemonOptions struct {
	TLS TLSConfig
}

func WriteDaemonCompose(opts DaemonOptions) (string, error) {
	composePath, err := DaemonComposePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(composePath), 0o755); err != nil {
		return "", err
	}

	command := []string{
		"--providers.docker=true",
		"--providers.docker.exposedbydefault=false",
		"--providers.docker.network=" + DevrouterNetwork,
		"--entrypoints.web.address=:80",
	}
	ports := []string{"80:80"}
	volumes := []string{"/var/run/docker.sock:/var/run/docker.sock:ro"}

	if opts.TLS.Enabled {
		normalizedTLS, err := NormalizeTLSConfig(opts.TLS)
		if err != nil {
			return "", err
		}
		tlsConfigPath, err := DaemonTLSConfigPath()
		if err != nil {
			return "", err
		}
		if err := WriteTraefikTLSConfig(tlsConfigPath); err != nil {
			return "", err
		}
		command = append(command,
			"--entrypoints.websecure.address=:443",
			"--entrypoints.web.http.redirections.entrypoint.to=websecure",
			"--entrypoints.web.http.redirections.entrypoint.scheme=https",
			"--entrypoints.websecure.http.tls=true",
			"--providers.file.filename="+traefikTLSConfigPath,
		)
		ports = append(ports, "443:443")
		volumes = append(
			volumes,
			normalizedTLS.CertFile+":"+traefikTLSCertPath+":ro",
			normalizedTLS.KeyFile+":"+traefikTLSKeyPath+":ro",
			tlsConfigPath+":"+traefikTLSConfigPath+":ro",
		)
	}

	services := map[string]ComposeService{
		"traefik": {
			Image:    traefikImage,
			Command:  command,
			Ports:    ports,
			Volumes:  volumes,
			Networks: []string{DevrouterNetwork},
		},
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

func DaemonUp(opts DaemonOptions) error {
	if err := EnsureNetwork(); err != nil {
		return err
	}
	composePath, err := WriteDaemonCompose(opts)
	if err != nil {
		return err
	}
	return RunDocker("compose", "-f", composePath, "up", "-d")
}

func DaemonDown() error {
	composePath, err := DaemonComposePath()
	if err != nil {
		return err
	}
	return RunDocker("compose", "-f", composePath, "down")
}
