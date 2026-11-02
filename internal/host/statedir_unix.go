//go:build !windows

package host

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// CheckStateDir refuses a state directory that is not a 0700 directory owned
// by root. A symlink is refused rather than followed.
func CheckStateDir(path string) error {
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
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create state_dir %s: %w", path, err)
		}
		// MkdirAll applies the umask; the mode must be exactly 0700.
		if err := os.Chmod(path, 0o700); err != nil {
			return fmt.Errorf("protect state_dir %s: %w", path, err)
		}
	}
	return CheckStateDir(path)
}
