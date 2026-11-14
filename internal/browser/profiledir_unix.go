//go:build !windows

package browser

import (
	"fmt"
	"os"
	"syscall"
)

// PrepareProfileDir makes dir a private Edge profile directory: it creates it
// 0700, refuses a symlink, a file or a directory another user owns, and
// tightens a loose mode. The profile holds cookies for the allow-listed
// private origins, so a directory someone else controls is never used (#803).
func PrepareProfileDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create browser profile directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("inspect browser profile directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("browser profile directory %s is not a directory (a symlink or a file)", dir)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("browser profile directory %s is not owned by the current user", dir)
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil { // #nosec G302 -- a directory needs x to be entered; 0700 is owner-only
			return fmt.Errorf("restrict browser profile directory %s: %w", dir, err)
		}
	}
	return nil
}
