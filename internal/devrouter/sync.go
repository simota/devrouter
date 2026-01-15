package devrouter

import (
	"time"
)

// SyncRegistry synchronizes the registry with actual container states
func SyncRegistry() error {
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}

	updated := false
	now := time.Now().UTC().Format(time.RFC3339)

	for i, stack := range reg.Stacks {
		states, err := ComposePs(stack.ComposeFilePath)
		if err != nil {
			// Compose file doesn't exist or Docker error - skip
			continue
		}

		// Check if all services are stopped
		allStopped := true
		for _, state := range states {
			if state == "running" {
				allStopped = false
				break
			}
		}

		// If no containers or all stopped, and LastDownAt is not set
		if (len(states) == 0 || allStopped) && reg.Stacks[i].LastDownAt == "" {
			reg.Stacks[i].LastDownAt = now
			updated = true
		}

		// If containers are running, clear LastDownAt
		if len(states) > 0 && !allStopped && reg.Stacks[i].LastDownAt != "" {
			reg.Stacks[i].LastDownAt = ""
			updated = true
		}
	}

	if updated {
		return SaveRegistry(reg)
	}
	return nil
}
