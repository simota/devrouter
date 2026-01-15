package devrouter

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ContainerStats represents resource usage statistics for a container
type ContainerStats struct {
	Name        string  `json:"name"`
	Service     string  `json:"service"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryUsage int64   `json:"memoryUsage"` // bytes
	MemoryLimit int64   `json:"memoryLimit"` // bytes
	MemPercent  float64 `json:"memPercent"`
	NetIO       string  `json:"netIO"`
	BlockIO     string  `json:"blockIO"`
	PIDs        int     `json:"pids"`
}

// StackStatsResponse represents resource stats for all services in a stack
type StackStatsResponse struct {
	StackID     string           `json:"stackId"`
	Containers  []ContainerStats `json:"containers"`
	CollectedAt string           `json:"collectedAt"`
}

// dockerStatsEntry represents the JSON output from docker stats
type dockerStatsEntry struct {
	Name     string `json:"Name"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
	NetIO    string `json:"NetIO"`
	BlockIO  string `json:"BlockIO"`
	PIDs     string `json:"PIDs"`
}

type statsCacheEntry struct {
	stats     []ContainerStats
	timestamp time.Time
}

var (
	statsCache   = make(map[string]statsCacheEntry)
	statsCacheMu sync.Mutex
)

// GetStackStats collects resource statistics for all containers in a stack
func GetStackStats(composePath string) (*StackStatsResponse, error) {
	// Check cache
	statsCacheMu.Lock()
	if entry, ok := statsCache[composePath]; ok {
		if time.Since(entry.timestamp) < 2*time.Second {
			statsCacheMu.Unlock()
			return &StackStatsResponse{
				Containers:  entry.stats,
				CollectedAt: entry.timestamp.UTC().Format(time.RFC3339),
			}, nil
		}
	}
	statsCacheMu.Unlock()

	// Get list of services in the stack with full container info
	// This uses the shared cache in docker.go, avoiding redundant calls
	containersList, err := GetComposeContainers(composePath)
	if err != nil {
		return nil, err
	}

	if len(containersList) == 0 {
		return &StackStatsResponse{
			Containers:  []ContainerStats{},
			CollectedAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	// Extract container names
	var containerNames []string
	for _, c := range containersList {
		if c.Name != "" {
			containerNames = append(containerNames, c.Name)
		}
	}

	if len(containerNames) == 0 {
		return &StackStatsResponse{
			Containers:  []ContainerStats{},
			CollectedAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	// Get stats for these containers
	// Use custom format with tab delimiter to avoid JSON parsing overhead
	// Format: Name, CPUPerc, MemUsage, MemPerc, NetIO, BlockIO, PIDs
	const format = "{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}"
	args := []string{"stats", "--no-stream", "--format", format}
	args = append(args, containerNames...)

	// Optimized: Use DockerOutputBytes to avoid string allocation
	statsOutput, err := DockerOutputBytes(args...)
	if err != nil {
		return nil, err
	}

	containers, err := parseDockerStats(statsOutput)
	if err != nil {
		return nil, err
	}

	statsCacheMu.Lock()
	statsCache[composePath] = statsCacheEntry{
		stats:     containers,
		timestamp: time.Now(),
	}
	statsCacheMu.Unlock()

	return &StackStatsResponse{
		Containers:  containers,
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func parseDockerStats(output []byte) ([]ContainerStats, error) {
	var containers []ContainerStats
	// Optimized: Use bytes.Split to avoid creating many string copies
	lines := bytes.Split(bytes.TrimSpace(output), []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		parts := bytes.Split(line, []byte("\t"))
		if len(parts) < 7 {
			continue
		}

		// Parts indices matching the format:
		// 0: Name
		// 1: CPUPerc
		// 2: MemUsage
		// 3: MemPerc
		// 4: NetIO
		// 5: BlockIO
		// 6: PIDs

		name := string(parts[0])
		stats := ContainerStats{
			Name:       name,
			Service:    extractServiceName(name),
			CPUPercent: parsePercent(parts[1]),
			MemPercent: parsePercent(parts[3]),
			NetIO:      string(parts[4]),
			BlockIO:    string(parts[5]),
			PIDs:       parseInt(parts[6]),
		}

		// Parse memory usage (e.g., "100MiB / 8GiB")
		// Optimized: Use bytes.Cut to avoid allocating a slice for split parts
		if before, after, found := bytes.Cut(parts[2], []byte(" / ")); found {
			stats.MemoryUsage = parseBytes(before)
			stats.MemoryLimit = parseBytes(after)
		}

		containers = append(containers, stats)
	}
	return containers, nil
}

// StreamStackStats continuously collects and sends stats through a channel
func StreamStackStats(ctx context.Context, composePath string, interval time.Duration) <-chan *StackStatsResponse {
	ch := make(chan *StackStatsResponse)

	go func() {
		defer close(ch)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Send initial stats
		if stats, err := GetStackStats(composePath); err == nil {
			select {
			case ch <- stats:
			case <-ctx.Done():
				return
			}
		}

		for {
			select {
			case <-ticker.C:
				stats, err := GetStackStats(composePath)
				if err != nil {
					continue
				}
				select {
				case ch <- stats:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

// extractServiceName extracts the service name from a container name
// Container names are typically in the format: project-service-1
// Optimized: Avoids strings.Split allocation
func extractServiceName(containerName string) string {
	first := strings.Index(containerName, "-")
	if first == -1 {
		return containerName
	}
	last := strings.LastIndex(containerName, "-")

	// If there are multiple parts (e.g. project-service-1),
	// we want everything between first and last dash.
	if last > first {
		return containerName[first+1 : last]
	}
	// If only two parts (project-1), return the second part.
	return containerName[first+1:]
}

// parsePercent parses a percentage string like "0.50%" to float64
// Optimized: Takes []byte to minimize allocation
func parsePercent(b []byte) float64 {
	b = bytes.TrimSpace(b)
	b = bytes.TrimSuffix(b, []byte("%"))
	// Trim again in case there was a space before the %
	b = bytes.TrimSpace(b)

	val, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return 0
	}
	return val
}

// parseInt parses an integer string
// Optimized: Takes []byte to minimize allocation
func parseInt(b []byte) int {
	b = bytes.TrimSpace(b)
	val, err := strconv.Atoi(string(b))
	if err != nil {
		return 0
	}
	return val
}

// parseBytes parses a byte size string like "100MiB" to int64
// Optimized: Takes []byte to minimize allocation
func parseBytes(b []byte) int64 {
	b = bytes.TrimSpace(b)
	multiplier := int64(1)

	// Check suffix
	switch {
	case bytes.HasSuffix(b, []byte("GiB")):
		multiplier = 1024 * 1024 * 1024
		b = bytes.TrimSuffix(b, []byte("GiB"))
	case bytes.HasSuffix(b, []byte("MiB")):
		multiplier = 1024 * 1024
		b = bytes.TrimSuffix(b, []byte("MiB"))
	case bytes.HasSuffix(b, []byte("KiB")):
		multiplier = 1024
		b = bytes.TrimSuffix(b, []byte("KiB"))
	case bytes.HasSuffix(b, []byte("B")):
		b = bytes.TrimSuffix(b, []byte("B"))
	}

	b = bytes.TrimSpace(b)
	val, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return 0
	}
	return int64(val * float64(multiplier))
}
