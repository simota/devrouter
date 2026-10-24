package devrouter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func snapshotTestRegistry() Registry {
	return Registry{
		Version: 1,
		Stacks: []StackRecord{{
			ID:              "snapshot-stack",
			Name:            "saved-stack",
			RepoPath:        "/example/repo",
			ComposeFilePath: "/example/compose.yaml",
			Services: []ServiceRecord{{
				Name:           "saved-service",
				WorkspacePath:  "apps/web",
				ComposeService: "snapshot-stack-web",
				HealthCheck: HealthCheckConfig{
					Endpoint: "/health",
					Status:   []int{200, 204},
				},
			}},
		}},
	}
}

func saveSnapshotTestRegistry(t *testing.T, reg Registry) string {
	t.Helper()
	if err := SaveRegistry(reg); err != nil {
		t.Fatal(err)
	}
	path, err := RegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func loadSnapshotTestRegistry(t *testing.T) Registry {
	t.Helper()
	reg, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func requireSnapshotTestRegistry(t *testing.T, got, want Registry) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registry mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLoadRegistrySnapshotsAreIndependent(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*Registry)
	}{
		{
			name: "stacks",
			mutate: func(reg *Registry) {
				reg.Stacks[0].Name = "unsaved-stack"
			},
		},
		{
			name: "services",
			mutate: func(reg *Registry) {
				reg.Stacks[0].Services[0].Name = "unsaved-service"
			},
		},
		{
			name: "health statuses",
			mutate: func(reg *Registry) {
				reg.Stacks[0].Services[0].HealthCheck.Status[0] = 503
			},
		},
	}
	for _, cached := range []bool{false, true} {
		phase := "initial load"
		if cached {
			phase = "repeated load"
		}
		t.Run(phase, func(t *testing.T) {
			for _, mutation := range mutations {
				t.Run(mutation.name, func(t *testing.T) {
					t.Setenv("HOME", t.TempDir())
					want := snapshotTestRegistry()
					path := saveSnapshotTestRegistry(t, want)
					if cached {
						requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), want)
					}

					snapshot := loadSnapshotTestRegistry(t)
					requireSnapshotTestRegistry(t, snapshot, want)
					independent := loadSnapshotTestRegistry(t)
					requireSnapshotTestRegistry(t, independent, want)
					mutation.mutate(&snapshot)

					if !reflect.DeepEqual(independent, want) {
						t.Error("mutating one loaded snapshot changed another loaded snapshot")
					}
					if got := loadSnapshotTestRegistry(t); !reflect.DeepEqual(got, want) {
						t.Error("LoadRegistry exposed changes that were never saved")
					}
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					var persisted Registry
					if err := json.Unmarshal(data, &persisted); err != nil {
						t.Fatal(err)
					}
					requireSnapshotTestRegistry(t, persisted, want)
				})
			}
		})
	}
}

func TestLoadRegistryConcurrentSnapshots(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := snapshotTestRegistry()
	saveSnapshotTestRegistry(t, want)

	const workers = 8
	snapshots := make([]Registry, workers)
	for i := range snapshots {
		snapshots[i] = loadSnapshotTestRegistry(t)
		requireSnapshotTestRegistry(t, snapshots[i], want)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := range snapshots {
		go func(worker int, snapshot Registry) {
			defer wg.Done()
			<-start
			// A caller owns its loaded snapshot and may change every nested field
			// without coordinating with other callers of LoadRegistry.
			for iteration := 0; iteration < 256; iteration++ {
				snapshot.Stacks[0].Name = fmt.Sprintf("worker-%d-%d", worker, iteration)
				snapshot.Stacks[0].Services[0].Name = fmt.Sprintf("service-%d-%d", worker, iteration)
				snapshot.Stacks[0].Services[0].HealthCheck.Status[0] = 400 + worker
			}
		}(i, snapshots[i])
	}
	close(start)

	// Read saved state while other callers edit their own snapshots. Run this
	// test with -race as well as checking the observable unsaved-value leak.
	leaked := false
	var loadErr error
	for i := 0; i < 64; i++ {
		got, err := LoadRegistry()
		if err != nil {
			loadErr = err
			break
		}
		if !reflect.DeepEqual(got, want) {
			leaked = true
		}
	}
	wg.Wait()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if leaked {
		t.Error("concurrent snapshot mutations became visible to LoadRegistry")
	}
	requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), want)
}

// Replace the JSON file as another process would, without invoking any
// process-local registry update or cache invalidation code.
func replaceSnapshotTestRegistry(t *testing.T, path string, reg Registry, modified time.Time) {
	t.Helper()
	data, err := json.Marshal(reg)
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "registry-replacement-*.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(tmp.Name()) })
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(tmp.Name(), modified, modified); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRegistryReadsReplacedFile(t *testing.T) {
	tests := []struct {
		name         string
		mtimeOffset  time.Duration
		deleteBefore bool
	}{
		{name: "same modification time"},
		{name: "older modification time", mtimeOffset: -24 * time.Hour},
		{name: "deleted and recreated", deleteBefore: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			original := snapshotTestRegistry()
			path := saveSnapshotTestRegistry(t, original)
			requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), original)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if tt.deleteBefore {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				got := loadSnapshotTestRegistry(t)
				if got.Version != 1 || len(got.Stacks) != 0 {
					t.Fatalf("missing registry should be empty, got %#v", got)
				}
			}

			updated := snapshotTestRegistry()
			updated.Stacks[0].Name = "replacement-stack"
			updated.Stacks[0].Services[0].Name = "replacement-service"
			updated.Stacks[0].Services[0].HealthCheck.Status = []int{201}
			replaceSnapshotTestRegistry(t, path, updated, info.ModTime().Add(tt.mtimeOffset))
			requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), updated)
		})
	}
}

func TestLoadRegistryUsesCurrentHome(t *testing.T) {
	firstHome := t.TempDir()
	secondHome := t.TempDir()
	first := snapshotTestRegistry()
	second := snapshotTestRegistry()
	second.Stacks[0].ID = "second-home-stack"
	second.Stacks[0].Name = "second-home"

	t.Setenv("HOME", firstHome)
	firstPath := saveSnapshotTestRegistry(t, first)
	info, err := os.Stat(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", secondHome)
	secondPath := saveSnapshotTestRegistry(t, second)
	older := info.ModTime().Add(-24 * time.Hour)
	if err := os.Chtimes(secondPath, older, older); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", firstHome)
	requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), first)
	t.Setenv("HOME", secondHome)
	requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), second)
	t.Setenv("HOME", firstHome)
	requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), first)
}
