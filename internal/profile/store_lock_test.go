package profile

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func shortStoreLockWait(t *testing.T) {
	t.Helper()
	old := storeLockWait
	storeLockWait = 200 * time.Millisecond
	t.Cleanup(func() { storeLockWait = old })
}

// A transition holds the lock from its first read to its last write: a writer
// that arrives meanwhile is applied afterwards, never in between (#391).
func TestUpdateHoldsOffAnotherWriterUntilItEnds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	store := NewStore(path)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- store.Update(func(*Tx) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	finished := make(chan error, 1)
	go func() { finished <- NewStore(path).Set("late", "host:22", "") }()
	select {
	case err := <-finished:
		t.Fatalf("a writer completed inside another transition: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatalf("waiting writer failed once the lock was released: %v", err)
	}
	if _, err := store.Get("late"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateReleasesTheLockWhenItsFunctionFails(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "profiles.yaml"))
	boom := errors.New("boom")
	if err := store.Update(func(*Tx) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("Update() = %v, want the function's error", err)
	}
	shortStoreLockWait(t)
	if err := store.Set("after", "host:22", ""); err != nil {
		t.Fatalf("a failed transition left the lock held: %v", err)
	}
}

// Another process holds the lock: mutations fail clearly after the wait, reads
// keep working and see the last complete store, and the lock frees when the
// holder is killed (crash recovery: no stale lock to clear by hand).
func TestStoreLockAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	store := NewStore(path)
	if err := store.Set("existing", "host:22", ""); err != nil {
		t.Fatal(err)
	}
	child := startStoreHelper(t, "hold", path)
	shortStoreLockWait(t)

	err := store.Set("blocked", "host:22", "")
	if !errors.Is(err, errStoreBusy) || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("Set() while another process holds the lock = %v, want a retryable busy error", err)
	}
	if got, err := store.Get("existing"); err != nil || got.Target != "host:22" {
		t.Fatalf("Get() while the lock is held = %#v, %v", got, err)
	}
	names, err := store.List()
	if err != nil || len(names) != 1 {
		t.Fatalf("List() while the lock is held = %v, %v; want only the existing profile", names, err)
	}
	if _, err := store.ListProfiles(); err != nil {
		t.Fatalf("ListProfiles() while the lock is held: %v", err)
	}

	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	if err := store.Set("recovered", "host:22", ""); err != nil {
		t.Fatalf("a killed lock holder left the store locked: %v", err)
	}
}

// Writers in separate processes and this one interleave without losing a profile.
func TestConcurrentProcessesLoseNoProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	children := make([]*exec.Cmd, 3)
	for i := range children {
		children[i] = startStoreHelper(t, fmt.Sprintf("write:c%d", i), path)
	}
	store := NewStore(path)
	for i := range 20 {
		if err := store.Set(fmt.Sprintf("parent%02d", i), "host:22", ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, child := range children {
		if err := child.Wait(); err != nil {
			t.Fatalf("helper writer: %v", err)
		}
	}
	names, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if want := 20 + 20*len(children); len(names) != want {
		t.Fatalf("store holds %d profiles, want %d: updates were lost", len(names), want)
	}
}

// startStoreHelper re-runs this test binary as TestStoreHelperProcess and, for
// "hold", waits until the child owns the lock.
func startStoreHelper(t *testing.T, mode, path string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	// #nosec G204 G702 -- fixed test selector and the current test executable, without a shell.
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStoreHelperProcess$")
	child.Env = append(os.Environ(), "TS_BRIDGE_STORE_HELPER="+mode, "TS_BRIDGE_STORE_PATH="+path)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	})
	if mode == "hold" {
		if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || strings.TrimSpace(line) != "locked" {
			t.Fatalf("helper did not take the store lock: %q, %v", line, err)
		}
	}
	return child
}

func TestStoreHelperProcess(t *testing.T) {
	mode, path := os.Getenv("TS_BRIDGE_STORE_HELPER"), os.Getenv("TS_BRIDGE_STORE_PATH")
	if mode == "" {
		return
	}
	store := NewStore(path)
	if mode == "hold" {
		err := store.Update(func(*Tx) error {
			fmt.Println("locked")
			time.Sleep(time.Minute)
			return nil
		})
		t.Fatal(err)
	}
	prefix := strings.TrimPrefix(mode, "write:")
	for i := range 20 {
		if err := store.Set(fmt.Sprintf("%s-%02d", prefix, i), "host:22", ""); err != nil {
			t.Fatal(err)
		}
	}
}
