package devrouter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The operation helper uses the real public entry points in a separate process.
// Its fake Docker command supplies an explicit barrier at the external operation
// so these tests exercise stale snapshots without relying on scheduler timing.
func TestRegistryOperationHelper(t *testing.T) {
	operation := os.Getenv("DEVROUTER_TEST_REGISTRY_OPERATION")
	if operation == "" {
		return
	}
	var err error
	switch operation {
	case "up":
		_, _, err = Up(os.Getenv("DEVROUTER_TEST_TARGET"), UpOptions{ComposePath: "compose.yaml"})
	case "down":
		_, err = Down(DownOptions{Target: os.Getenv("DEVROUTER_TEST_TARGET")})
	case "sync":
		err = SyncRegistry()
	default:
		err = fmt.Errorf("unknown test operation: %s", operation)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestRegistryDockerHelper(t *testing.T) {
	if os.Getenv("DEVROUTER_TEST_REGISTRY_DOCKER") != "1" {
		return
	}
	var args []string
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		os.Exit(3)
	}
	if args[0] == "info" || args[0] == "network" {
		os.Exit(0)
	}
	if len(args) < 4 || args[0] != "compose" || args[1] != "-f" {
		fmt.Fprintln(os.Stderr, "unexpected Docker arguments:", args)
		os.Exit(3)
	}
	operation := args[3]
	if operation == os.Getenv("DEVROUTER_TEST_BLOCK_DOCKER") {
		if err := os.WriteFile(os.Getenv("DEVROUTER_TEST_READY"), []byte(args[2]), 0600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		deadline := time.Now().Add(15 * time.Second)
		for {
			if _, err := os.Stat(os.Getenv("DEVROUTER_TEST_RELEASE")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				fmt.Fprintln(os.Stderr, "test Docker barrier timed out")
				os.Exit(3)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if operation == os.Getenv("DEVROUTER_TEST_FAIL_DOCKER") {
		fmt.Fprintln(os.Stderr, "intentional Docker failure")
		os.Exit(4)
	}
	if operation == "ps" {
		state, err := os.ReadFile(os.Getenv("DEVROUTER_TEST_STATE_FILE"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		fmt.Printf("{\"Service\":\"web\",\"State\":%q}\n", string(state))
	}
	os.Exit(0)
}

type registryHarness struct {
	dir       string
	stateFile string
}

func newRegistryHarness(t *testing.T) *registryHarness {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
	script := "#!/bin/sh\nexec " + quoted + " -test.run=^TestRegistryDockerHelper$ -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DEVROUTER_TEST_REGISTRY_DOCKER", "1")
	h := &registryHarness{dir: dir, stateFile: filepath.Join(dir, "docker-state")}
	t.Setenv("DEVROUTER_TEST_STATE_FILE", h.stateFile)
	h.setState(t, "stopped")
	return h
}

func (h *registryHarness) setState(t *testing.T, state string) {
	t.Helper()
	if err := os.WriteFile(h.stateFile, []byte(state), 0600); err != nil {
		t.Fatal(err)
	}
}

func (h *registryHarness) repo(t *testing.T, name string) string {
	t.Helper()
	repo := filepath.Join(h.dir, name)
	if err := os.MkdirAll(repo, 0700); err != nil {
		t.Fatal(err)
	}
	compose := "services:\n  web:\n    image: test\n    labels:\n      devrouter.enabled: 'true'\n"
	if err := os.WriteFile(filepath.Join(repo, "compose.yaml"), []byte(compose), 0600); err != nil {
		t.Fatal(err)
	}
	return repo
}

func upRegistryTestStack(t *testing.T, repo string) StackRecord {
	t.Helper()
	stack, _, err := Up(repo, UpOptions{ComposePath: "compose.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	return stack
}

type registryOperation struct {
	cmd     *exec.Cmd
	output  bytes.Buffer
	done    chan error
	release string
	waited  bool
	err     error
}

func startRegistryOperation(t *testing.T, operation, target, block string) *registryOperation {
	t.Helper()
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRegistryOperationHelper$")
	ready := filepath.Join(dir, "ready")
	p := &registryOperation{cmd: cmd, done: make(chan error, 1), release: filepath.Join(dir, "release")}
	cmd.Env = append(os.Environ(),
		"DEVROUTER_TEST_REGISTRY_OPERATION="+operation,
		"DEVROUTER_TEST_TARGET="+target,
		"DEVROUTER_TEST_BLOCK_DOCKER="+block,
		"DEVROUTER_TEST_READY="+ready,
		"DEVROUTER_TEST_RELEASE="+p.release,
	)
	cmd.Stdout, cmd.Stderr = &p.output, &p.output
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	go func() { p.done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = os.WriteFile(p.release, nil, 0600)
		if !p.waited {
			select {
			case p.err = <-p.done:
			case <-time.After(5 * time.Second):
				cancel()
				p.err = <-p.done
			}
			p.waited = true
		}
		cancel()
	})
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(ready); err == nil {
			return p
		}
		select {
		case p.err = <-p.done:
			p.waited = true
			t.Fatalf("operation ended before reaching Docker barrier: %v\n%s", p.err, p.output.String())
		case <-deadline.C:
			t.Fatal("operation did not reach Docker barrier")
		case <-ticker.C:
		}
	}
}

func (p *registryOperation) finish(t *testing.T, wantConflict bool) {
	t.Helper()
	if err := os.WriteFile(p.release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case p.err = <-p.done:
		p.waited = true
	case <-time.After(10 * time.Second):
		t.Fatal("operation did not finish after releasing Docker")
	}
	if wantConflict {
		if p.err == nil || !strings.Contains(p.output.String(), "changed") {
			t.Errorf("stale operation must report a registry conflict, got %v\n%s", p.err, p.output.String())
		}
	} else if p.err != nil {
		t.Fatalf("operation failed: %v\n%s", p.err, p.output.String())
	}
}

func TestConcurrentUpPreservesOtherStacks(t *testing.T) {
	h := newRegistryHarness(t)
	a, b := h.repo(t, "a"), h.repo(t, "b")
	blocked := startRegistryOperation(t, "up", a, "up")
	other := upRegistryTestStack(t, b)
	blocked.finish(t, false)
	reg := loadSnapshotTestRegistry(t)
	if len(reg.Stacks) != 2 {
		t.Fatalf("concurrent up lost a stack: %#v", reg.Stacks)
	}
	if got, ok := FindStack(reg, other.ID); !ok || !reflect.DeepEqual(got, other) {
		t.Fatalf("concurrent up changed the other stack: %#v", reg.Stacks)
	}
}

func TestConcurrentDownPreservesOtherStacks(t *testing.T) {
	h := newRegistryHarness(t)
	a := upRegistryTestStack(t, h.repo(t, "a"))
	blocked := startRegistryOperation(t, "down", a.ID, "down")
	b := upRegistryTestStack(t, h.repo(t, "b"))
	blocked.finish(t, false)
	reg := loadSnapshotTestRegistry(t)
	if len(reg.Stacks) != 1 || !reflect.DeepEqual(reg.Stacks[0], b) {
		t.Fatalf("down discarded an unrelated concurrent up: %#v", reg.Stacks)
	}
}

func TestSyncPreservesConcurrentRegistryChanges(t *testing.T) {
	for _, change := range []string{"add", "remove", "replace same instance fields"} {
		t.Run(change, func(t *testing.T) {
			h := newRegistryHarness(t)
			repo := h.repo(t, "a")
			a := upRegistryTestStack(t, repo)
			blocked := startRegistryOperation(t, "sync", "", "ps")
			var replacement StackRecord
			switch change {
			case "add":
				replacement = upRegistryTestStack(t, h.repo(t, "b"))
			case "remove":
				if _, err := Down(DownOptions{Target: a.ID}); err != nil {
					t.Fatal(err)
				}
			case "replace same instance fields":
				replacement = upRegistryTestStack(t, repo)
				// A same-second restart can have identical public metadata. Keep
				// timestamps identical so protection cannot depend on their precision.
				replacement.LastUpAt = a.LastUpAt
				if err := SaveRegistry(Registry{Stacks: []StackRecord{replacement}}); err != nil {
					t.Fatal(err)
				}
			}
			blocked.finish(t, false)
			reg := loadSnapshotTestRegistry(t)
			if change == "remove" {
				if len(reg.Stacks) != 0 {
					t.Fatalf("sync resurrected a removed stack: %#v", reg.Stacks)
				}
				return
			}
			got, ok := FindStack(reg, replacement.ID)
			if !ok || !reflect.DeepEqual(got, replacement) {
				t.Fatalf("sync overwrote a concurrent %s: got %#v, want %#v", change, got, replacement)
			}
			if change == "add" {
				original, ok := FindStack(reg, a.ID)
				if !ok || original.LastDownAt == "" {
					t.Fatal("sync failed to apply its observation to the unchanged stack")
				}
			}
		})
	}
}

func TestDownPreservesNewStackInstance(t *testing.T) {
	h := newRegistryHarness(t)
	repo := h.repo(t, "a")
	a := upRegistryTestStack(t, repo)
	blocked := startRegistryOperation(t, "down", a.ID, "down")
	replacement := upRegistryTestStack(t, repo)
	replacement.LastUpAt = a.LastUpAt
	if err := SaveRegistry(Registry{Stacks: []StackRecord{replacement}}); err != nil {
		t.Fatal(err)
	}
	blocked.finish(t, true)
	reg := loadSnapshotTestRegistry(t)
	if len(reg.Stacks) != 1 || !reflect.DeepEqual(reg.Stacks[0], replacement) {
		t.Fatalf("old down removed a newly started stack: %#v", reg.Stacks)
	}
}

func TestDownAcceptsConcurrentStatusSync(t *testing.T) {
	h := newRegistryHarness(t)
	a := upRegistryTestStack(t, h.repo(t, "a"))
	blocked := startRegistryOperation(t, "down", a.ID, "down")
	if err := SyncRegistry(); err != nil {
		t.Fatal(err)
	}
	if reg := loadSnapshotTestRegistry(t); reg.Stacks[0].LastDownAt == "" {
		t.Fatal("test did not observe a status change before down completed")
	}
	blocked.finish(t, false)
	if reg := loadSnapshotTestRegistry(t); len(reg.Stacks) != 0 {
		t.Fatalf("status-only sync prevented removal: %#v", reg.Stacks)
	}
}

func TestSyncUsesFreshDockerState(t *testing.T) {
	h := newRegistryHarness(t)
	a := upRegistryTestStack(t, h.repo(t, "a"))
	states, err := ComposePs(a.ComposeFilePath)
	if err != nil || states["web"] != "stopped" {
		t.Fatalf("could not prime stopped cache: %#v, %v", states, err)
	}
	h.setState(t, "running")
	if err := SyncRegistry(); err != nil {
		t.Fatal(err)
	}
	reg := loadSnapshotTestRegistry(t)
	if reg.Stacks[0].LastDownAt != "" {
		t.Fatalf("sync marked a running stack down using the polling cache: %#v", reg.Stacks[0])
	}
}

func TestRegistryDockerFailuresPreserveSavedState(t *testing.T) {
	for _, operation := range []string{"up", "down", "ps"} {
		t.Run(operation, func(t *testing.T) {
			h := newRegistryHarness(t)
			repo := h.repo(t, "a")
			a := upRegistryTestStack(t, repo)
			before := loadSnapshotTestRegistry(t)
			t.Setenv("DEVROUTER_TEST_FAIL_DOCKER", operation)
			var err error
			switch operation {
			case "up":
				_, _, err = Up(repo, UpOptions{ComposePath: "compose.yaml"})
			case "down":
				_, err = Down(DownOptions{Target: a.ID})
			case "ps":
				err = SyncRegistry() // Failed observations are skipped by design.
			}
			if operation != "ps" && err == nil {
				t.Fatal("Docker failure was not propagated")
			}
			if operation == "ps" && err != nil {
				t.Fatal(err)
			}
			requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), before)
		})
	}
}
