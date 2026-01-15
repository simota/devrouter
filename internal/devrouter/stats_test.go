package devrouter

import (
	"testing"
)

func TestParseDockerStats(t *testing.T) {
	// Sample custom output from docker stats --format "{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}"
	input := []byte(`project-service-1	0.50%	100MiB / 1GiB	9.7%	1kB / 2kB	0B / 0B	12
project-db-1	0.10%	50MiB / 512MiB	9.7%	500B / 500B	10B / 0B	8`)

	stats, err := parseDockerStats(input)
	if err != nil {
		t.Fatalf("Failed to parse stats: %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(stats))
	}

	if stats[0].Name != "project-service-1" {
		t.Errorf("Expected name project-service-1, got %s", stats[0].Name)
	}
	if stats[0].CPUPercent != 0.50 {
		t.Errorf("Expected CPU 0.50, got %f", stats[0].CPUPercent)
	}
	if stats[0].MemoryUsage != 100*1024*1024 { // 100MiB
		t.Errorf("Expected Mem 104857600, got %d", stats[0].MemoryUsage)
	}
	if stats[0].PIDs != 12 {
		t.Errorf("Expected PIDs 12, got %d", stats[0].PIDs)
	}
}

func BenchmarkParseDockerStats(b *testing.B) {
	input := []byte(`project-service-1	0.50%	100MiB / 1GiB	9.7%	1kB / 2kB	0B / 0B	12
project-db-1	0.10%	50MiB / 512MiB	9.7%	500B / 500B	10B / 0B	8`)

	for i := 0; i < b.N; i++ {
		_, _ = parseDockerStats(input)
	}
}
