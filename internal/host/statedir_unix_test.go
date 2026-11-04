//go:build !windows

package host

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A link at state_dir is refused, not followed, even when it points at a
// 0700 directory: the refusal names the link, not the target's owner.
func TestCheckStateDir_Symlink(t *testing.T) {
	skipAncestorWalk(t)
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
	skipAncestorWalk(t)
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

// skipAncestorWalk stubs the ancestor walk for a test that exercises the leaf
// checks on a t.TempDir, whose ancestors the real walk refuses (#561).
func skipAncestorWalk(t *testing.T) {
	prev := stateDirAncestors
	stateDirAncestors = func(string) error { return nil }
	t.Cleanup(func() { stateDirAncestors = prev })
}

// fakeNode is one path of the fake tree walkStateDirAncestors reads.
type fakeNode struct {
	mode fs.FileMode
	uid  uint32
}

// TestWalkStateDirAncestors pins the walk of #561 over a fake tree: every
// existing ancestor is checked lexically, absent ones are skipped, the first
// refusal names its ancestor, and the leaf is never consulted.
func TestWalkStateDirAncestors(t *testing.T) {
	rootDir := fakeNode{fs.ModeDir | 0o755, 0}
	tests := []struct {
		name    string
		path    string
		tree    map[string]fakeNode
		wantErr string // "" accepts
	}{
		{"default layout", "/var/lib/ts-bridge/serve",
			map[string]fakeNode{"/": rootDir, "/var": rootDir, "/var/lib": rootDir, "/var/lib/ts-bridge": rootDir}, ""},
		{"absent ancestors are created later", "/var/lib/ts-bridge/serve",
			map[string]fakeNode{"/": rootDir, "/var": rootDir, "/var/lib": rootDir}, ""},
		{"macOS /var is a root-owned link", "/var/lib/ts-bridge/serve",
			map[string]fakeNode{"/": rootDir, "/var": {fs.ModeSymlink | 0o755, 0}, "/var/lib": rootDir}, ""},
		{"root-owned /var/run link", "/var/run/ts-bridge/serve",
			map[string]fakeNode{"/": rootDir, "/var": rootDir, "/var/run": {fs.ModeSymlink | 0o777, 0}}, ""},
		{"under /tmp", "/tmp/x/serve",
			map[string]fakeNode{"/": rootDir, "/tmp": {fs.ModeDir | fs.ModeSticky | 0o777, 0}}, "ancestor /tmp: mode 0777"},
		{"a user's link", "/srv/link/serve",
			map[string]fakeNode{"/": rootDir, "/srv": rootDir, "/srv/link": {fs.ModeSymlink | 0o777, 1000}}, "ancestor /srv/link is a symlink owned by uid 1000"},
		{"a user's dir deep down", "/srv/home/serve",
			map[string]fakeNode{"/": rootDir, "/srv": rootDir, "/srv/home": {fs.ModeDir | 0o700, 1000}}, "ancestor /srv/home: owned by uid 1000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var read []string
			lstat := func(p string) (fs.FileInfo, uint32, error) {
				read = append(read, p)
				n, ok := tc.tree[p]
				if !ok {
					return nil, 0, fs.ErrNotExist
				}
				return fakeInfo{n.mode}, n.uid, nil
			}
			err := walkStateDirAncestors(tc.path, lstat)
			for _, p := range read {
				if p == tc.path {
					t.Errorf("the walk read the leaf %s; CheckStateDir owns it", p)
				}
			}
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("walk(%s) = %v, want nil", tc.path, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("walk(%s) = %v, want %q", tc.path, err, tc.wantErr)
			}
		})
	}
}

// Through the real filesystem: every t.TempDir sits under /tmp (1777) on
// Linux or a user-owned tree on macOS, so the walk refuses it whatever the
// test runs as, and EnsureStateDir creates nothing.
func TestEnsureStateDir_RefusedAncestorCreatesNothing(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "a")
	dir := filepath.Join(parent, "serve")
	if err := EnsureStateDir(dir); err == nil || !strings.Contains(err.Error(), "ancestor") {
		t.Fatalf("EnsureStateDir(%s) = %v, want it refused for an ancestor", dir, err)
	}
	if _, err := os.Lstat(parent); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("EnsureStateDir created %s before refusing: %v", parent, err)
	}
}

// CheckStateDir walks the ancestors too, so an existing state_dir under a
// writable tree is refused, not only a new one.
func TestCheckStateDir_WalksAncestors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := CheckStateDir(dir); err == nil || !strings.Contains(err.Error(), "ancestor") {
		t.Errorf("CheckStateDir(%s) = %v, want it refused for an ancestor", dir, err)
	}
}
