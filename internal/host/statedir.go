package host

import (
	"fmt"
	"io/fs"
)

// The state directory holds the serve node's key, so only the account serve
// runs as may touch it (HOST-008 AC5): root with mode 0700 on Unix, SYSTEM and
// Administrators on Windows. CheckStateDir and EnsureStateDir are per OS.

// checkUnixStateDir accepts a real directory with mode 0700 owned by uid 0.
func checkUnixStateDir(path string, info fs.FileInfo, uid uint32) error {
	if !info.IsDir() {
		return fmt.Errorf("state_dir %s: not a directory", path)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		return fmt.Errorf("state_dir %s: mode %04o, want 0700", path, perm)
	}
	if uid != 0 {
		return fmt.Errorf("state_dir %s: owned by uid %d, want root (0)", path, uid)
	}
	return nil
}

// checkUnixSecretFile accepts a regular file owned by uid 0 that neither group
// nor others may read or write: it holds a live enrolment key.
func checkUnixSecretFile(path string, info fs.FileInfo, uid uint32) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("auth_key_file %s: not a regular file", path)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Errorf("auth_key_file %s: mode %04o, want no group or other bits (0600 or 0400)", path, perm)
	}
	if uid != 0 {
		return fmt.Errorf("auth_key_file %s: owned by uid %d, want root (0)", path, uid)
	}
	return nil
}
