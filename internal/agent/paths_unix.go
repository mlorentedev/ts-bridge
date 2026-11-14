//go:build !windows

package agent

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
)

// DefaultPaths resolves the agent's paths for the effective user. The home
// directory comes from the user database (getpwuid_r on macOS, /etc/passwd on
// a cgo-less Linux build), never from $HOME.
func DefaultPaths() (Paths, error) {
	uid := os.Geteuid()
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return Paths{}, fmt.Errorf("resolve the home directory of uid %d from the user database: %w", uid, err)
	}
	if !filepath.IsAbs(u.HomeDir) {
		return Paths{}, fmt.Errorf("the user database gives uid %d a home directory that is not absolute: %q", uid, u.HomeDir)
	}
	usable := func(dir string) bool { return runUserUsable(dir, uid) }
	return pathsFor(runtime.GOOS, uid, u.HomeDir, usable), nil
}

// runUserUsable reports whether dir is the private runtime directory
// systemd-logind creates: a real directory owned by uid with mode 0700.
func runUserUsable(dir string, uid int) bool {
	info, err := os.Lstat(dir)
	if err != nil || verifyDir(dir, info, uid) != nil {
		return false
	}
	return info.Mode().Perm() == 0o700
}
