package profile

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProfileLockLifecycle(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "profiles.yaml"))
	first, err := store.Lock("office")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if duplicate, err := store.Lock("office"); err == nil {
		_ = duplicate.Close()
		t.Fatal("same-profile ownership was granted twice")
	} else if !strings.Contains(err.Error(), "already connected") {
		t.Fatalf("duplicate error is not actionable: %v", err)
	}
	for _, name := range []string{"other", "../office"} {
		independent, err := store.Lock(name)
		if err != nil {
			t.Fatalf("independent profile %q blocked: %v", name, err)
		}
		if filepath.Dir(independent.Name()) != filepath.Dir(first.Name()) {
			t.Fatal("profile name escaped the ownership directory")
		}
		if err := independent.Close(); err != nil {
			t.Fatal(err)
		}
	}
	lockPath := first.Name()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("release removed the persistent lock file: %v", err)
	}
	reopened, err := store.Lock("office")
	if err != nil {
		t.Fatalf("normal close did not release ownership: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProfileLockReleasedAfterProcessExit(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "profiles.yaml")
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	// #nosec G204 G702 -- fixed test selector and the current test executable, without a shell.
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProfileLockHelperProcess$")
	child.Env = append(os.Environ(), "TS_BRIDGE_PROFILE_LOCK_HELPER="+storePath)
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
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || strings.TrimSpace(line) != "locked" {
		t.Fatalf("child failed to acquire ownership: %q, %v", line, err)
	}
	store := NewStore(storePath)
	if duplicate, err := store.Lock("office"); err == nil {
		_ = duplicate.Close()
		t.Fatal("another process acquired the same profile")
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err == nil {
		t.Fatal("expected the lock owner to be forcibly terminated")
	}
	recovered, err := store.Lock("office")
	if err != nil {
		t.Fatalf("abrupt process exit left stale ownership: %v", err)
	}
	if err := recovered.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProfileLockHelperProcess(t *testing.T) {
	path := os.Getenv("TS_BRIDGE_PROFILE_LOCK_HELPER")
	if path == "" {
		return
	}
	lock, err := NewStore(path).Lock("office")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	fmt.Println("locked")
	time.Sleep(time.Minute)
}
