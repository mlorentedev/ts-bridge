//go:build !windows

package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A link at state_dir is refused, not followed, even when it points at a
// 0700 directory: the refusal names the link, not the target's owner.
func TestCheckStateDir_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "serve")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	for name, fn := range map[string]func(string) error{"CheckStateDir": CheckStateDir, "EnsureStateDir": EnsureStateDir} {
		// With a trailing slash POSIX resolves the final link; it must not help.
		for _, p := range []string{link, link + "/"} {
			if err := fn(p); err == nil || !strings.Contains(err.Error(), "not a directory") {
				t.Errorf("%s(%q) = %v, want it refused as not a directory", name, p, err)
			}
		}
	}
}

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
		// The uid-0 half needs root; as a user the new directory is refused for
		// its owner, and a second run refuses it the same way, changing nothing.
		for run := 1; run <= 2; run++ {
			if run == 2 {
				err = EnsureStateDir(dir)
			}
			if err == nil || !strings.Contains(err.Error(), "owned by uid") {
				t.Errorf("EnsureStateDir run %d as uid %d = %v, want it refused for its owner", run, os.Geteuid(), err)
			}
		}
		if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("a second EnsureStateDir changed the directory: %v %v", info, err)
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
