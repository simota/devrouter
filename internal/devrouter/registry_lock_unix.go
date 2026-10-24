//go:build linux || darwin

package devrouter

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// lockRegistry uses a stable sidecar: registry.json itself is replaced by rename.
// Close releases the lock. Never remove the sidecar, which could split writers
// between different lock-file inodes.
func lockRegistry(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}
