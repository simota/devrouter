package devrouter

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const registryVersion = 1

// LoadRegistry returns an independent snapshot of the current registry file.
func LoadRegistry() (Registry, error) {
	path, err := RegistryPath()
	if err != nil {
		return Registry{}, err
	}

	return loadRegistry(path)
}

func loadRegistry(path string) (Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Registry{Version: registryVersion, Stacks: []StackRecord{}}, nil
		}
		return Registry{}, err
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return Registry{}, err
	}
	if reg.Stacks == nil {
		reg.Stacks = []StackRecord{}
	}
	if reg.Version == 0 {
		reg.Version = registryVersion
	}

	return reg, nil
}

// SaveRegistry replaces the whole registry. Read-modify-write callers must use
// updateRegistry so another process's changes are preserved.
func SaveRegistry(reg Registry) error {
	path, err := RegistryPath()
	if err != nil {
		return err
	}
	lock, err := lockRegistry(path)
	if err != nil {
		return err
	}
	defer lock.Close()
	return saveRegistry(path, reg)
}

// updateRegistry runs a short read-modify-write transaction across processes.
// The callback must not perform Docker operations or call registry writers.
func updateRegistry(update func(*Registry) (bool, error)) error {
	path, err := RegistryPath()
	if err != nil {
		return err
	}
	lock, err := lockRegistry(path)
	if err != nil {
		return err
	}
	defer lock.Close()

	reg, err := loadRegistry(path)
	if err != nil {
		return err
	}
	changed, err := update(&reg)
	if err != nil || !changed {
		return err
	}
	return saveRegistry(path, reg)
}

func saveRegistry(path string, reg Registry) error {
	reg.Version = registryVersion
	payload, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, payload)
}

func newStackRevision() (string, error) {
	var revision [16]byte
	if _, err := rand.Read(revision[:]); err != nil {
		return "", fmt.Errorf("generate stack revision: %w", err)
	}
	return hex.EncodeToString(revision[:]), nil
}

func UpsertStack(reg Registry, stack StackRecord) Registry {
	updated := make([]StackRecord, 0, len(reg.Stacks))
	replaced := false
	for _, existing := range reg.Stacks {
		if existing.ID == stack.ID || existing.RepoPath == stack.RepoPath {
			if !replaced {
				updated = append(updated, stack)
				replaced = true
			}
			continue
		}
		updated = append(updated, existing)
	}
	if !replaced {
		updated = append(updated, stack)
	}
	reg.Stacks = updated
	return reg
}

func RemoveStack(reg Registry, stackID string) (Registry, bool) {
	updated := make([]StackRecord, 0, len(reg.Stacks))
	removed := false
	for _, stack := range reg.Stacks {
		if stack.ID == stackID {
			removed = true
			continue
		}
		updated = append(updated, stack)
	}
	reg.Stacks = updated
	return reg, removed
}

func StackRecordFromSpec(stack StackSpec, composePath string, source string, scheme string) StackRecord {
	services := make([]ServiceRecord, 0, len(stack.Services))
	for _, svc := range stack.Services {
		url := ""
		if svc.Host != "" {
			url = fmt.Sprintf("%s://%s", scheme, svc.Host)
		}
		services = append(services, ServiceRecord{
			Name:           svc.Name,
			WorkspacePath:  svc.WorkspacePath,
			Host:           svc.Host,
			URL:            url,
			Port:           svc.Port,
			ComposeService: svc.ComposeService,
			HealthCheck:    svc.HealthCheck,
			Kind:           svc.Kind,
		})
	}
	return StackRecord{
		ID:              stack.ID,
		Name:            stack.Name,
		RepoPath:        stack.RepoPath,
		Source:          source,
		Domain:          stack.Domain,
		ComposeFilePath: composePath,
		LastUpAt:        time.Now().UTC().Format(time.RFC3339),
		Services:        services,
	}
}

func FindStack(reg Registry, target string) (StackRecord, bool) {
	for _, stack := range reg.Stacks {
		if stack.ID == target || stack.Name == target {
			return stack, true
		}
	}
	return StackRecord{}, false
}

func FindStackByRepoPath(reg Registry, repoPath string) (StackRecord, bool) {
	for _, stack := range reg.Stacks {
		if stack.RepoPath == repoPath {
			return stack, true
		}
	}
	return StackRecord{}, false
}

func FindService(stack StackRecord, name string) (ServiceRecord, bool) {
	for _, svc := range stack.Services {
		if svc.Name == name || svc.ComposeService == name {
			return svc, true
		}
	}
	return ServiceRecord{}, false
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".registry-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
