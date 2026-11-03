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
// by root. A symlink is refused rather than followed.
func CheckStateDir(path string) error {
	path = filepath.Clean(path) // a trailing slash would make Lstat follow a final link
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
		if err := createStateDir(path); err != nil {
			return fmt.Errorf("create state_dir %s: %w", path, err)
		}
	}
	return CheckStateDir(path)
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
