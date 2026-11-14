//go:build !windows

package agent

import (
	"fmt"
	"os"
	"syscall"
)

// prepareDir creates the state directory private to the user, or checks the one
// that exists. The directory is what keeps other users away from the socket
// path, so it must be a real directory (not a symlink another user could
// redirect) owned by this user. Looser permissions on our own directory are
// tightened.
func prepareDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create agent directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("inspect agent directory: %w", err)
	}
	if err := verifyDir(dir, info, os.Geteuid()); err != nil {
		return err
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil { // #nosec G302 -- a directory needs x to be entered; 0700 is owner-only
			return fmt.Errorf("restrict agent directory %s: %w", dir, err)
		}
	}
	return nil
}

// verifyDir refuses what is not a directory, or is owned by another user.
func verifyDir(dir string, info os.FileInfo, euid int) error {
	if !info.IsDir() {
		return fmt.Errorf("agent directory %s is not a directory (a symlink or a file)", dir)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("agent directory %s: owner is not readable on this platform", dir)
	}
	if int(stat.Uid) != euid {
		return fmt.Errorf("agent directory %s is owned by uid %d, not by the current user (uid %d)", dir, stat.Uid, euid)
	}
	return nil
}

// checkPrivateDir is the client's view of the socket directory: it creates and
// tightens nothing, and trusts only a real directory owned by euid with mode
// 0700, because whoever controls that directory controls the socket in it.
func checkPrivateDir(dir string, euid int) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("inspect agent socket directory: %w", err)
	}
	if err := verifyDir(dir, info, euid); err != nil {
		return err
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		return fmt.Errorf("agent socket directory %s has mode %o, want 700", dir, perm)
	}
	return nil
}
