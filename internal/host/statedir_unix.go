//go:build !windows

package host

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// CheckStateDir refuses a state directory that is not a 0700 directory owned
// by root, or that sits under an ancestor someone else controls (#561). A
// symlink is refused rather than followed.
func CheckStateDir(path string) error {
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
var stateDirAncestors = func(path string) error { return walkStateDirAncestors(path, lstatOwner) }

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

// walkStateDirAncestors checks every existing ancestor of path, nearest first,
// without following links: a root-owned link is root's choice, so its target
// is not walked. An absent ancestor is skipped; createStateDir makes it 0700.
func walkStateDirAncestors(path string, lstat func(string) (fs.FileInfo, uint32, error)) error {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		info, uid, err := lstat(dir)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return fmt.Errorf("state_dir %s: ancestor %s: %w", path, dir, err)
		default:
			if err := checkUnixAncestor(path, dir, info, uid); err != nil {
				return err
			}
		}
		if parent := filepath.Dir(dir); parent == dir {
			return nil
		}
	}
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
// file, or that group or others may read. A symlink is refused, not followed.
func CheckSecretFile(path string) error {
	path = filepath.Clean(path)
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
