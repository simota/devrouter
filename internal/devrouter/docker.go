package devrouter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

func CheckDocker() error {
	cmd := exec.Command("docker", "info")
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("docker info failed: %s", msg)
	}
	return nil
}

func NetworkExists(name string) (bool, error) {
	cmd := exec.Command("docker", "network", "inspect", name)
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if strings.Contains(strings.ToLower(msg), "not found") {
			return false, nil
		}
		return false, fmt.Errorf("docker network inspect failed: %s", msg)
	}
	return true, nil
}

func EnsureNetwork() error {
	inspect := exec.Command("docker", "network", "inspect", DevrouterNetwork)
	inspect.Stdout = io.Discard
	inspect.Stderr = io.Discard
	if err := inspect.Run(); err == nil {
		return nil
	}
	return RunDocker("network", "create", DevrouterNetwork)
}

func ComposeUp(composePath string, build bool) error {
	args := []string{"compose", "-f", composePath, "up", "-d"}
	if build {
		args = append(args, "--build")
	}
	return RunDocker(args...)
}

func ComposeDown(composePath string) error {
	return RunDocker("compose", "-f", composePath, "down")
}

func ComposeBuildRestart(composePath string, service string) error {
	args := []string{"compose", "-f", composePath, "up", "-d", "--build", "--force-recreate"}
	if service != "" {
		args = append(args, service)
	}
	return RunDocker(args...)
}

func ComposeRestart(composePath string, service string) error {
	args := []string{"compose", "-f", composePath, "restart"}
	if service != "" {
		args = append(args, service)
	}
	return RunDocker(args...)
}

func ComposeLogs(composePath string, follow bool, service string) error {
	args := []string{"compose", "-f", composePath, "logs"}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	}
	return RunDocker(args...)
}

// ContainerInfo represents a single entry from docker compose ps --format json
type ContainerInfo struct {
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"`
}

type psCacheEntry struct {
	containers []ContainerInfo
	timestamp  time.Time
}

var (
	psCache   = make(map[string]psCacheEntry)
	psCacheMu sync.Mutex
)

// GetComposeContainers returns detailed information about containers in the stack
func GetComposeContainers(composePath string) ([]ContainerInfo, error) {
	psCacheMu.Lock()
	if entry, ok := psCache[composePath]; ok {
		// Cache for 2 seconds to avoid thundering herd on polling
		if time.Since(entry.timestamp) < 2*time.Second {
			psCacheMu.Unlock()
			return entry.containers, nil
		}
	}
	psCacheMu.Unlock()

	// Optimized: Use DockerOutputBytes to avoid string allocation
	output, err := DockerOutputBytes("compose", "-f", composePath, "ps", "--format", "json")
	if err != nil {
		return nil, err
	}

	var containers []ContainerInfo
	if len(bytes.TrimSpace(output)) > 0 {
		// docker compose ps --format json outputs one JSON object per line
		// Optimized: Use bytes.Split to avoid creating many string copies
		lines := bytes.Split(bytes.TrimSpace(output), []byte("\n"))
		for _, line := range lines {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			var entry ContainerInfo
			if err := json.Unmarshal(line, &entry); err != nil {
				continue
			}
			containers = append(containers, entry)
		}
	}

	psCacheMu.Lock()
	psCache[composePath] = psCacheEntry{
		containers: containers,
		timestamp:  time.Now(),
	}
	psCacheMu.Unlock()

	return containers, nil
}

// ComposePs returns a map of service name to container state (running/exited/stopped/not_found)
func ComposePs(composePath string) (map[string]string, error) {
	containers, err := GetComposeContainers(composePath)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, entry := range containers {
		// Normalize state to: running, stopped, exited
		state := strings.ToLower(entry.State)
		if state == "" {
			state = "unknown"
		}
		result[entry.Service] = state
	}

	return result, nil
}

func ComposeExec(composePath string, service string, command []string, opts ExecOptions) error {
	args := []string{"compose", "-f", composePath, "exec"}
	if opts.User != "" {
		args = append(args, "-u", opts.User)
	}
	if opts.Workdir != "" {
		args = append(args, "-w", opts.Workdir)
	}
	args = append(args, service)
	args = append(args, command...)
	return RunDocker(args...)
}

func RunDocker(args ...string) error {
	return runCommand("docker", args, os.Stdout, os.Stderr)
}

func DockerOutput(args ...string) (string, error) {
	out, err := DockerOutputBytes(args...)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func DockerOutputBytes(args ...string) ([]byte, error) {
	cmd := exec.Command("docker", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("docker %s failed: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

func runCommand(name string, args []string, stdout, stderr io.Writer) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
