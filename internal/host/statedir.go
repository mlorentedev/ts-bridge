package host

import (
	"fmt"
	"io/fs"
)

// The state directory holds the serve node's key, so only the account serve
// runs as may touch it (HOST-008 AC5): root with mode 0700 on Unix, SYSTEM and
// Administrators on Windows. CheckStateDir and EnsureStateDir are per OS.

// specialBits are mode bits beyond rwx that a key store must not carry: setgid
// on a directory hands new files its group, sticky changes who may delete.
const specialBits = fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky

// checkUnixStateDir accepts a real directory with mode 0700 owned by uid 0.
func checkUnixStateDir(path string, info fs.FileInfo, uid uint32) error {
	if !info.IsDir() {
		return fmt.Errorf("state_dir %s: not a directory", path)
	}
	if perm := info.Mode().Perm(); perm != 0o700 || info.Mode()&specialBits != 0 {
		return fmt.Errorf("state_dir %s: mode %04o%s, want 0700", path, perm, specialSuffix(info.Mode()))
	}
	if uid != 0 {
		return fmt.Errorf("state_dir %s: owned by uid %d, want root (0)", path, uid)
	}
	return nil
}

// checkUnixAncestor accepts an ancestor dir of state_dir path that only root
// can change: a root-owned directory no group or other may write (a sticky
// world-writable /tmp included), or a root-owned symlink, whose target the
// walk then checks the same way (#561).
func checkUnixAncestor(path, dir string, info fs.FileInfo, uid uint32) error {
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		if uid != 0 {
			return fmt.Errorf("state_dir %s: ancestor %s is a symlink owned by uid %d, want root (0)", path, dir, uid)
		}
		return nil
	case !info.IsDir():
		return fmt.Errorf("state_dir %s: ancestor %s: not a directory", path, dir)
	case uid != 0:
		return fmt.Errorf("state_dir %s: ancestor %s: owned by uid %d, want root (0)", path, dir, uid)
	case info.Mode().Perm()&0o022 != 0:
		return fmt.Errorf("state_dir %s: ancestor %s: mode %04o lets group or others write", path, dir, info.Mode().Perm())
	}
	return nil
}

// specialSuffix names the special bits of m, for an error message.
func specialSuffix(m fs.FileMode) string {
	s := ""
	for _, b := range []struct {
		bit  fs.FileMode
		name string
	}{{fs.ModeSetuid, "setuid"}, {fs.ModeSetgid, "setgid"}, {fs.ModeSticky, "sticky"}} {
		if m&b.bit != 0 {
			s += " " + b.name
		}
	}
	return s
}

// checkUnixSecretFile accepts a regular file owned by uid 0 that neither group
// nor others may read or write: it holds a live enrolment key.
func checkUnixSecretFile(path string, info fs.FileInfo, uid uint32) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("auth_key_file %s: not a regular file", path)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 || info.Mode()&specialBits != 0 {
		return fmt.Errorf("auth_key_file %s: mode %04o%s, want no group or other bits (0600 or 0400)", path, perm, specialSuffix(info.Mode()))
	}
	if uid != 0 {
		return fmt.Errorf("auth_key_file %s: owned by uid %d, want root (0)", path, uid)
	}
	return nil
}
