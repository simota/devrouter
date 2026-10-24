//go:build !linux && !darwin

package devrouter

import (
	"fmt"
	"os"
	"runtime"
)

func lockRegistry(path string) (*os.File, error) {
	return nil, fmt.Errorf("registry updates are not supported on %s", runtime.GOOS)
}
