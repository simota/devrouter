package devrouter

import (
	"reflect"
	"time"
)

// SyncRegistry synchronizes the registry with actual container states
func SyncRegistry() error {
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}

	type observation struct {
		stack      StackRecord
		lastDownAt string
	}
	observations := make(map[string]observation)
	now := time.Now().UTC().Format(time.RFC3339)

	for _, stack := range reg.Stacks {
		states, err := composePsFresh(stack.ComposeFilePath)
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

		if allStopped && stack.LastDownAt == "" {
			observations[stack.ID] = observation{stack: stack, lastDownAt: now}
		}

		// If containers are running, clear LastDownAt
		if !allStopped && stack.LastDownAt != "" {
			observations[stack.ID] = observation{stack: stack}
		}
	}

	if len(observations) == 0 {
		return nil
	}
	return updateRegistry(func(current *Registry) (bool, error) {
		updated := false
		for i := range current.Stacks {
			stack := &current.Stacks[i]
			observed, ok := observations[stack.ID]
			// Docker was queried without holding the registry lock. Discard
			// results if this stack changed or was replaced in the meantime.
			if !ok || !reflect.DeepEqual(*stack, observed.stack) {
				continue
			}
			stack.LastDownAt = observed.lastDownAt
			updated = true
		}
		return updated, nil
	})
}
