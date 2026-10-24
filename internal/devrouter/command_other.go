//go:build !linux && !darwin

package devrouter

import "os/exec"

// Other platforms retain exec.CommandContext's direct-process cancellation.
func configureCommandCancellation(cmd *exec.Cmd) {}
