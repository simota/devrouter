package devrouter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryIncrementHelper(t *testing.T) {
	if os.Getenv("DEVROUTER_TEST_REGISTRY_INCREMENT") != "1" {
		return
	}
	if err := os.WriteFile(os.Getenv("DEVROUTER_TEST_READY"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(os.Getenv("DEVROUTER_TEST_RELEASE")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("increment barrier timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
	for i := 0; i < 20; i++ {
		if err := updateRegistry(func(reg *Registry) (bool, error) {
			reg.Stacks[0].Services[0].Port++
			return true, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRegistryUpdatesSerializeAcrossProcesses(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reg := snapshotTestRegistry()
	reg.Stacks[0].Services[0].Port = 0
	saveSnapshotTestRegistry(t, reg)
	const workers = 6
	dir := t.TempDir()
	release := filepath.Join(dir, "start")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	type worker struct {
		cmd    *exec.Cmd
		output bytes.Buffer
		done   chan error
		waited bool
	}
	children := make([]worker, workers)
	defer func() {
		cancel()
		for i := range children {
			if children[i].done != nil && !children[i].waited {
				<-children[i].done
			}
		}
	}()
	for i := range children {
		child := &children[i]
		child.cmd = exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRegistryIncrementHelper$")
		child.cmd.Env = append(os.Environ(),
			"DEVROUTER_TEST_REGISTRY_INCREMENT=1",
			fmt.Sprintf("DEVROUTER_TEST_READY=%s", filepath.Join(dir, fmt.Sprint(i))),
			"DEVROUTER_TEST_RELEASE="+release,
		)
		child.cmd.Stdout, child.cmd.Stderr = &child.output, &child.output
		if err := child.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		child.done = make(chan error, 1)
		go func() { child.done <- child.cmd.Wait() }()
	}
	deadline := time.Now().Add(10 * time.Second)
	for i := range children {
		for {
			if _, err := os.Stat(filepath.Join(dir, fmt.Sprint(i))); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("increment processes did not reach start barrier")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for i := range children {
		err := <-children[i].done
		children[i].waited = true
		if err != nil {
			t.Fatalf("worker %d: %v\n%s", i, err, children[i].output.String())
		}
	}
	got := loadSnapshotTestRegistry(t).Stacks[0].Services[0].Port
	if got != workers*20 {
		t.Fatalf("concurrent transactions lost updates: got %d, want %d", got, workers*20)
	}
}

func TestRegistryReadersSeeOnlyCommittedSnapshots(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := snapshotTestRegistry()
	saveSnapshotTestRegistry(t, want)
	modified, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- updateRegistry(func(reg *Registry) (bool, error) {
			reg.Stacks[0].Name = "committed"
			close(modified)
			<-release
			return true, nil
		})
	}()
	waited, released := false, false
	defer func() {
		if !released {
			close(release)
		}
		if !waited {
			<-done
		}
	}()
	select {
	case <-modified:
	case err := <-done:
		waited = true
		t.Fatalf("update failed before callback: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("update did not reach callback")
	}
	// Load must remain available and expose only the last atomic replacement.
	read := make(chan Registry, 1)
	readError := make(chan error, 1)
	go func() {
		reg, err := LoadRegistry()
		read <- reg
		readError <- err
	}()
	select {
	case reg := <-read:
		if err := <-readError; err != nil {
			t.Fatal(err)
		}
		requireSnapshotTestRegistry(t, reg, want)
	case <-time.After(5 * time.Second):
		t.Fatal("registry reader blocked behind uncommitted update")
	}
	close(release)
	released = true
	err := <-done
	waited = true
	if err != nil {
		t.Fatal(err)
	}
	want.Stacks[0].Name = "committed"
	requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), want)
}

func TestStackRevisionChangesOnEveryUp(t *testing.T) {
	h := newRegistryHarness(t)
	repo := h.repo(t, "a")
	first := upRegistryTestStack(t, repo)
	second := upRegistryTestStack(t, repo)
	if first.Revision == "" || second.Revision == "" || first.Revision == second.Revision {
		t.Fatalf("each up needs a distinct non-empty revision: first=%q second=%q", first.Revision, second.Revision)
	}
	reg := loadSnapshotTestRegistry(t)
	if len(reg.Stacks) != 1 || reg.Stacks[0].Revision != second.Revision {
		t.Fatalf("latest revision was not persisted: %#v", reg.Stacks)
	}
}

func TestRegistryProtectsLegacyAndRenamedStacks(t *testing.T) {
	for _, operation := range []string{"down", "sync"} {
		for _, change := range []string{"legacy", "renamed"} {
			t.Run(operation+"/"+change, func(t *testing.T) {
				h := newRegistryHarness(t)
				repo := h.repo(t, "a")
				initial := upRegistryTestStack(t, repo)
				if change == "legacy" {
					initial.Revision = ""
					if err := SaveRegistry(Registry{Stacks: []StackRecord{initial}}); err != nil {
						t.Fatal(err)
					}
				}
				block := "down"
				if operation == "sync" {
					block = "ps"
				}
				pending := startRegistryOperation(t, operation, initial.ID, block)
				if change == "renamed" {
					if err := os.WriteFile(filepath.Join(repo, "devrouter.yaml"), []byte("stack: renamed\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				latest := upRegistryTestStack(t, repo)
				pending.finish(t, operation == "down")
				requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), Registry{Version: 1, Stacks: []StackRecord{latest}})
			})
		}
	}
}

func TestSyncRegistryStateTransitions(t *testing.T) {
	h := newRegistryHarness(t)
	initial := upRegistryTestStack(t, h.repo(t, "a"))
	if err := SyncRegistry(); err != nil {
		t.Fatal(err)
	}
	stopped := loadSnapshotTestRegistry(t)
	if stopped.Stacks[0].LastDownAt == "" || stopped.Stacks[0].Revision != initial.Revision {
		t.Fatalf("stopped observation lost revision or timestamp: %#v", stopped.Stacks)
	}
	path, err := RegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := SyncRegistry(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("unchanged observation rewrote registry")
	}
	h.setState(t, "running")
	if err := SyncRegistry(); err != nil {
		t.Fatal(err)
	}
	requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), Registry{Version: 1, Stacks: []StackRecord{initial}})
}

func TestRegistryUpdateDoesNotPublishFailedChanges(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := snapshotTestRegistry()
	path := saveSnapshotTestRegistry(t, want)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("transaction rejected")
	err = updateRegistry(func(reg *Registry) (bool, error) {
		reg.Stacks[0].Name = "must not be published"
		reg.Stacks[0].Services[0].HealthCheck.Status[0] = 500
		return true, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("callback error not preserved: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed callback changed persisted registry")
	}
	requireSnapshotTestRegistry(t, loadSnapshotTestRegistry(t), want)
	// A failure must release the lock, allowing a later transaction to succeed.
	if err := updateRegistry(func(reg *Registry) (bool, error) {
		reg.Stacks[0].Name = "committed"
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := loadSnapshotTestRegistry(t).Stacks[0].Name; got != "committed" {
		t.Fatalf("next update not committed: %q", got)
	}
}

func TestRegistryUpdatePreservesInvalidJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := saveSnapshotTestRegistry(t, snapshotTestRegistry())
	invalid := []byte("{invalid registry")
	if err := os.WriteFile(path, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	err := updateRegistry(func(reg *Registry) (bool, error) {
		called = true
		reg.Stacks = nil
		return true, nil
	})
	if err == nil || called {
		t.Fatalf("invalid JSON must prevent callback and save: called=%v err=%v", called, err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, invalid) {
		t.Fatal("invalid registry was silently overwritten")
	}
}

func TestRegistryUpdateNoChangeDoesNotReplaceFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := saveSnapshotTestRegistry(t, snapshotTestRegistry())
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := updateRegistry(func(reg *Registry) (bool, error) { return false, nil }); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("no-op transaction replaced the registry")
	}
}
