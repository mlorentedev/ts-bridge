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
	link := fakeNode{fs.ModeSymlink | 0o777, 0}
	tests := []struct {
		name    string
		path    string
		tree    map[string]fakeNode
		links   map[string]string // link -> target, as readlink returns it
		wantErr string            // "" accepts
	}{
		{"default layout", "/var/lib/ts-bridge/serve",
			map[string]fakeNode{"/": rootDir, "/var": rootDir, "/var/lib": rootDir, "/var/lib/ts-bridge": rootDir}, nil, ""},
		{"absent ancestors are created later", "/var/lib/ts-bridge/serve",
			map[string]fakeNode{"/": rootDir, "/var": rootDir, "/var/lib": rootDir}, nil, ""},
		{"macOS /var is a root-owned link", "/var/lib/ts-bridge/serve",
			map[string]fakeNode{"/": rootDir, "/var": link, "/private": rootDir, "/private/var": rootDir, "/var/lib": rootDir},
			map[string]string{"/var": "private/var"}, ""},
		{"root-owned /var/run link", "/var/run/ts-bridge/serve",
			map[string]fakeNode{"/": rootDir, "/var": rootDir, "/var/run": link, "/run": rootDir},
			map[string]string{"/var/run": "/run"}, ""},
		{"macOS /tmp links to a world-writable dir", "/tmp/ts-bridge/serve",
			map[string]fakeNode{"/": rootDir, "/tmp": link, "/private": rootDir, "/private/tmp": {fs.ModeDir | fs.ModeSticky | 0o777, 0}},
			map[string]string{"/tmp": "private/tmp"}, "ancestor /private/tmp: mode 0777"},
		{"a root-owned link under a user's dir", "/srv/link/serve",
			map[string]fakeNode{"/": rootDir, "/srv": rootDir, "/srv/link": link, "/home/u": {fs.ModeDir | 0o755, 1000}, "/home": rootDir},
			map[string]string{"/srv/link": "/home/u"}, "ancestor /home/u: owned by uid 1000"},
		{"a link loop", "/a/serve",
			map[string]fakeNode{"/": rootDir, "/a": link, "/b": link},
			map[string]string{"/a": "/b", "/b": "/a"}, "too many links"},
		{"under /tmp", "/tmp/x/serve",
			map[string]fakeNode{"/": rootDir, "/tmp": {fs.ModeDir | fs.ModeSticky | 0o777, 0}}, nil, "ancestor /tmp: mode 0777"},
		{"a user's link", "/srv/link/serve",
			map[string]fakeNode{"/": rootDir, "/srv": rootDir, "/srv/link": {fs.ModeSymlink | 0o777, 1000}}, nil, "ancestor /srv/link is a symlink owned by uid 1000"},
		{"a user's dir deep down", "/srv/home/serve",
			map[string]fakeNode{"/": rootDir, "/srv": rootDir, "/srv/home": {fs.ModeDir | 0o700, 1000}}, nil, "ancestor /srv/home: owned by uid 1000"},
		// The kernel takes .. from where /srv/alias points, /tmp/nested, not
		// from /srv: the walk must reach /tmp/store, not stop at /srv/store.
		{"a link target's .. follows the link", "/srv/link/serve",
			map[string]fakeNode{"/": rootDir, "/srv": rootDir, "/srv/link": link, "/srv/alias": link,
				"/tmp": rootDir, "/tmp/nested": rootDir, "/tmp/store": {fs.ModeDir | 0o755, 1000}},
			map[string]string{"/srv/link": "/srv/alias/../store", "/srv/alias": "/tmp/nested"}, "ancestor /tmp/store: owned by uid 1000"},
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
			readlink := func(p string) (string, error) {
				if target, ok := tc.links[p]; ok {
					return target, nil
				}
				return "", fs.ErrInvalid
			}
			err := walkStateDirAncestors(tc.path, ancestorFS{lstat, readlink})
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

// requireRefusedAncestors skips unless the real walk refuses the ancestors of
// path, the exact path the test passes to the function under test. A default
// TMPDIR (/tmp, 1777, or a user-owned tree) is refused; a TMPDIR whose every
// ancestor only root can write would let the walk pass.
func requireRefusedAncestors(t *testing.T, path string) {
	t.Helper()
	if walkStateDirAncestors(path, ancestorFS{lstatOwner, os.Readlink}) == nil {
		t.Skipf("the ancestors of %s are all root-controlled; this test needs a writable one", path)
	}
}

// Through the real filesystem: EnsureStateDir refuses a hostile ancestor and
// creates nothing.
func TestEnsureStateDir_RefusedAncestorCreatesNothing(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "a")
	dir := filepath.Join(parent, "serve")
	requireRefusedAncestors(t, dir)
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
	requireRefusedAncestors(t, dir)
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := CheckStateDir(dir); err == nil || !strings.Contains(err.Error(), "ancestor") {
		t.Errorf("CheckStateDir(%s) = %v, want it refused for an ancestor", dir, err)
	}
}

// Links whose targets are links again cost the walk one visit per link, not
// a re-walk per level (#596 review): ten nested /srv/l -> . links stay linear.
func TestWalkStateDirAncestors_LinearInLinks(t *testing.T) {
	const depth = 10
	visits := 0
	lstat := func(p string) (fs.FileInfo, uint32, error) {
		visits++
		switch {
		case p == "/" || p == "/srv":
			return fakeInfo{fs.ModeDir | 0o755}, 0, nil
		case strings.ReplaceAll(strings.TrimPrefix(p, "/srv"), "/l", "") == "":
			return fakeInfo{fs.ModeSymlink | 0o777}, 0, nil // /srv/l, /srv/l/l, ...
		}
		return nil, 0, fs.ErrNotExist
	}
	readlink := func(string) (string, error) { return ".", nil }
	path := "/srv" + strings.Repeat("/l", depth) + "/serve"
	if err := walkStateDirAncestors(path, ancestorFS{lstat, readlink}); err != nil {
		t.Fatalf("walk(%s) = %v, want nil", path, err)
	}
	if visits > 4*depth {
		t.Errorf("walk(%s) made %d lstat calls, want at most %d", path, visits, 4*depth)
	}
}

// A .. in the declared path is refused: Clean would drop it lexically while
// the kernel resolves it after any link before it.
func TestCheckStateDir_RefusesDotDot(t *testing.T) {
	for name, fn := range map[string]func(string) error{"CheckStateDir": CheckStateDir, "EnsureStateDir": EnsureStateDir} {
		if err := fn("/srv/link/../serve"); err == nil || !strings.Contains(err.Error(), "..") {
			t.Errorf("%s(/srv/link/../serve) = %v, want it refused for its ..", name, err)
		}
	}
}
