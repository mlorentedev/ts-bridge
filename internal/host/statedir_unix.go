//go:build !windows

package host

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// CheckStateDir refuses a state directory that is not a 0700 directory owned
// by root, or that sits under an ancestor someone else controls (#561). A
// symlink at state_dir itself is refused rather than followed; a root-owned
// link among its ancestors is followed and its target checked too.
func CheckStateDir(path string) error {
	if err := refuseDotDot("state_dir", path); err != nil {
		return err
	}
	path = filepath.Clean(path) // a trailing slash would make Lstat follow a final link
	if err := stateDirAncestors(path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("state_dir %s: %w", path, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("state_dir %s: cannot read its owner", path)
	}
	return checkUnixStateDir(path, info, st.Uid)
}

// EnsureStateDir creates an absent state directory with mode 0700, then checks
// it. An existing directory is checked, never repaired.
func EnsureStateDir(path string) error {
	if err := refuseDotDot("state_dir", path); err != nil {
		return err
	}
	path = filepath.Clean(path)
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		// Refuse a hostile ancestor before creating anything under it.
		if err := stateDirAncestors(path); err != nil {
			return err
		}
		if err := createStateDir(path); err != nil {
			return fmt.Errorf("create state_dir %s: %w", path, err)
		}
	}
	return CheckStateDir(path)
}

// stateDirAncestors is the ancestor walk with the real filesystem; tests that
// exercise the leaf checks on a t.TempDir stub it.
var stateDirAncestors = func(path string) error {
	return walkStateDirAncestors(path, ancestorFS{lstatOwner, os.Readlink})
}

// secretFileAncestors is the same walk for the auth key file (#595); a test
// that exercises the leaf checks on a t.TempDir stubs it.
var secretFileAncestors = func(path string) error {
	return walkAncestors("auth_key_file", path, ancestorFS{lstatOwner, os.Readlink})
}

// maxLinkHops bounds how many links one walk follows in all, as the kernel does.
const maxLinkHops = 40

// ancestorFS is the filesystem the walk reads: lstat plus the owner's uid, and
// readlink.
type ancestorFS struct {
	lstat    func(string) (fs.FileInfo, uint32, error)
	readlink func(string) (string, error)
}

// lstatOwner is os.Lstat plus the owner's uid.
func lstatOwner(path string) (fs.FileInfo, uint32, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, 0, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, 0, errors.New("cannot read its owner")
	}
	return info, st.Uid, nil
}

// walkStateDirAncestors is walkAncestors for a state_dir.
func walkStateDirAncestors(path string, afs ancestorFS) error {
	return walkAncestors("state_dir", path, afs)
}

// walkAncestors checks every directory the kernel passes through to reach
// path's parent, path being the setting label names (state_dir, auth_key_file),
// top-down as path_resolution(7) resolves it: each component on entry, and a
// link's target in place of the link, so a .. in a target is taken from where
// the link points. A root-owned link is followed: root owning the link says
// nothing about who can write where it points (macOS /tmp -> /private/tmp is
// 1777). One link budget covers the whole walk, as in the kernel. An absent
// component ends it; for a state_dir, createStateDir makes the rest 0700.
func walkAncestors(label, path string, afs ancestorFS) error {
	if _, err := ancestorAt(label, path, "/", afs); err != nil {
		return err
	}
	rest, cur, links := components(filepath.Dir(path)), "/", 0
	for len(rest) > 0 {
		c := rest[0]
		rest = rest[1:]
		if c == ".." {
			cur = filepath.Dir(cur)
			continue
		}
		next := filepath.Join(cur, c)
		info, err := ancestorAt(label, path, next, afs)
		if err != nil || info == nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			cur = next
			continue
		}
		if links++; links > maxLinkHops {
			return fmt.Errorf("%s %s: ancestor %s: too many links", label, path, next)
		}
		target, err := afs.readlink(next)
		if err != nil {
			return fmt.Errorf("%s %s: ancestor %s: %w", label, path, next, err)
		}
		if filepath.IsAbs(target) {
			cur = "/"
		}
		rest = append(components(target), rest...)
	}
	return nil
}

// components splits a slash path into its names, dropping empty and "."
// ones; ".." is kept for the walk to resolve.
func components(p string) []string {
	var out []string
	for _, c := range strings.Split(p, "/") {
		if c != "" && c != "." {
			out = append(out, c)
		}
	}
	return out
}

// ancestorAt reads and checks one ancestor. A nil FileInfo means it is absent.
func ancestorAt(label, path, dir string, afs ancestorFS) (fs.FileInfo, error) {
	info, uid, err := afs.lstat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("%s %s: ancestor %s: %w", label, path, dir, err)
	}
	return info, checkAncestor(label, path, dir, info, uid)
}

// refuseDotDot refuses a .. in the declared path: Clean drops it lexically,
// while the kernel resolves it after any link before it.
func refuseDotDot(label, path string) error {
	for _, c := range strings.Split(path, "/") {
		if c == ".." {
			return fmt.Errorf("%s %s: has a .. component; give the path without it", label, path)
		}
	}
	return nil
}

// createStateDir makes the last element itself, so a link planted there in the
// meantime fails with EEXIST instead of being followed, and sets the mode
// through a descriptor opened without following links.
func createStateDir(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		return err
	}
	// #nosec G304 -- path is the operator's declared host.mesh.state_dir, just created.
	dir, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer dir.Close()
	// Mkdir applies the umask; the mode must be exactly 0700.
	return dir.Chmod(0o700)
}

// CheckSecretFile refuses an auth key file that is not a root-owned regular
// file, or that group or others may read, or that sits under an ancestor
// someone else controls (#595). A symlink at the file itself is refused, not
// followed; a root-owned link among its ancestors is followed and its target
// checked too.
func CheckSecretFile(path string) error {
	if err := refuseDotDot("auth_key_file", path); err != nil {
		return err
	}
	path = filepath.Clean(path)
	if err := secretFileAncestors(path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("auth_key_file %s: %w", path, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("auth_key_file %s: cannot read its owner", path)
	}
	return checkUnixSecretFile(path, info, st.Uid)
}
