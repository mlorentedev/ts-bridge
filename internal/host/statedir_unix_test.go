//go:build !windows

package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureStateDir(t *testing.T) {
	loose := t.TempDir()
	if err := os.Chmod(loose, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureStateDir(loose); err == nil || !strings.Contains(err.Error(), "mode 0755") {
		t.Errorf("EnsureStateDir(0755 dir) = %v, want it refused naming the mode", err)
	}
	if info, err := os.Stat(loose); err != nil || info.Mode().Perm() != 0o755 {
		t.Errorf("EnsureStateDir repaired an existing directory: %v %v", info.Mode(), err)
	}

	dir := filepath.Join(t.TempDir(), "a", "serve")
	err := EnsureStateDir(dir)
	info, statErr := os.Stat(dir)
	if statErr != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("EnsureStateDir(absent) left %v (%v), want a 0700 directory", info, statErr)
	}
	if os.Geteuid() != 0 {
		// The uid-0 half needs root; as a user the new directory is refused for its owner.
		if err == nil || !strings.Contains(err.Error(), "uid") {
			t.Errorf("EnsureStateDir as uid %d = %v, want it refused for its owner", os.Geteuid(), err)
		}
		return
	}
	if err != nil {
		t.Errorf("EnsureStateDir(absent) as root = %v, want nil", err)
	}
	if err := EnsureStateDir(dir); err != nil {
		t.Errorf("second EnsureStateDir = %v, want nil (idempotent)", err)
	}
}
