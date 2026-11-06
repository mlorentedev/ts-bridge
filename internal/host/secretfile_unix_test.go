//go:build !windows

package host

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWalkAncestors_AuthKeyFile pins that the walk of #561 serves the key file
// too (#595): the same rule, with the refusal naming auth_key_file.
func TestWalkAncestors_AuthKeyFile(t *testing.T) {
	rootDir := fakeNode{fs.ModeDir | 0o755, 0}
	tests := []struct {
		name    string
		path    string
		tree    map[string]fakeNode
		links   map[string]string
		wantErr string // "" accepts
	}{
		{"default layout", "/etc/ts-bridge/authkey",
			map[string]fakeNode{"/": rootDir, "/etc": rootDir, "/etc/ts-bridge": rootDir}, nil, ""},
		{"directly under /etc", "/etc/authkey",
			map[string]fakeNode{"/": rootDir, "/etc": rootDir}, nil, ""},
		{"under /tmp", "/tmp/authkey",
			map[string]fakeNode{"/": rootDir, "/tmp": {fs.ModeDir | fs.ModeSticky | 0o777, 0}}, nil,
			"auth_key_file /tmp/authkey: ancestor /tmp: mode 0777"},
		{"a user's link", "/srv/link/authkey",
			map[string]fakeNode{"/": rootDir, "/srv": rootDir, "/srv/link": {fs.ModeSymlink | 0o777, 1000}}, nil,
			"auth_key_file /srv/link/authkey: ancestor /srv/link is a symlink owned by uid 1000"},
		{"a user's dir", "/home/u/authkey",
			map[string]fakeNode{"/": rootDir, "/home": rootDir, "/home/u": {fs.ModeDir | 0o700, 1000}}, nil,
			"auth_key_file /home/u/authkey: ancestor /home/u: owned by uid 1000"},
		{"a root-owned link to a writable dir", "/srv/link/authkey",
			map[string]fakeNode{"/": rootDir, "/srv": rootDir, "/srv/link": {fs.ModeSymlink | 0o777, 0}, "/var/tmp": {fs.ModeDir | fs.ModeSticky | 0o777, 0}, "/var": rootDir},
			map[string]string{"/srv/link": "/var/tmp"}, "ancestor /var/tmp: mode 0777"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lstat := func(p string) (fs.FileInfo, uint32, error) {
				if p == tc.path {
					t.Errorf("the walk read the leaf %s; CheckSecretFile owns it", p)
				}
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
			err := walkAncestors("auth_key_file", tc.path, ancestorFS{lstat, readlink, nil})
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

// Through the real filesystem: a 0600 key file under a writable tree is
// refused for its ancestor, which only the walk running before the leaf check
// can report (a non-root owner would otherwise be named first).
func TestCheckSecretFile_WalksAncestors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authkey")
	requireRefusedAncestors(t, path)
	if err := os.WriteFile(path, []byte("tskey-auth-x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckSecretFile(path); err == nil || !strings.Contains(err.Error(), "auth_key_file "+path+": ancestor") {
		t.Errorf("CheckSecretFile(%s) = %v, want it refused for an ancestor", path, err)
	}
}

// With the walk stubbed the leaf checks still run, so the walk adds a check
// and replaces none.
func TestCheckSecretFile_LeafStillChecked(t *testing.T) {
	prev := secretFileAncestors
	secretFileAncestors = func(string) error { return nil }
	t.Cleanup(func() { secretFileAncestors = prev })
	path := filepath.Join(t.TempDir(), "authkey")
	if err := os.WriteFile(path, []byte("tskey-auth-x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckSecretFile(path); err == nil || !strings.Contains(err.Error(), "mode 0644") {
		t.Errorf("CheckSecretFile(0644 file) = %v, want it refused naming the mode", err)
	}
}

// A .. in the declared path is refused, as for state_dir, naming auth_key_file.
func TestCheckSecretFile_RefusesDotDot(t *testing.T) {
	err := CheckSecretFile("/srv/link/../authkey")
	if err == nil || !strings.Contains(err.Error(), "auth_key_file") || !strings.Contains(err.Error(), "..") {
		t.Errorf("CheckSecretFile(/srv/link/../authkey) = %v, want it refused for its ..", err)
	}
}
