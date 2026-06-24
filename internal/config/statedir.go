package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Path components and GOOS values shared across the per-user state directories.
// Extracted as consts so they are not repeated string literals (goconst).
const (
	appDirName   = "ts-bridge"
	stateDirLeaf = "state"
	osWindows    = "windows"
	osDarwin     = "darwin"
)

// StateDirForPlatform returns the fixed, per-user, CWD-independent directory
// where tsnet node state is stored — including tailscaled.state, which holds
// the node's private identity.
//
// It mirrors logging.LogDirForPlatform but uses the semantically-correct XDG
// *state* base on Linux, and is guaranteed to return an absolute path. A
// missing HOME / LOCALAPPDATA / XDG_STATE_HOME must never produce a
// CWD-relative path, because that is exactly how node identity leaked into the
// working directory (and any auto-committing git tree) in #207.
func StateDirForPlatform() string {
	return stateDirFor(runtime.GOOS, os.Getenv)
}

// stateDirFor is the pure core of StateDirForPlatform. goos and getenv are
// injected so the per-OS branches can be exercised deterministically in tests.
//
//nolint:unparam // goos is parameterized for cross-platform table tests; the sole production caller passes runtime.GOOS (test callers are excluded from unparam).
func stateDirFor(goos string, getenv func(string) string) string {
	dir := platformStateBase(goos, getenv)
	if !filepath.IsAbs(dir) {
		// The platform base was empty (no HOME/LOCALAPPDATA/XDG) — fall back to
		// an absolute temp path rather than a CWD-relative one (#207). TempDir
		// is absolute on every supported platform.
		dir = filepath.Join(os.TempDir(), appDirName, stateDirLeaf)
	}
	return dir
}

// stateDirIsCWDRelative reports whether dir is a non-empty CWD-relative path.
// A relative state dir risks leaking the tsnet node identity (tailscaled.state)
// into the working directory and any auto-committing git tree (#207). The
// resolved default is always absolute, so this only fires for an explicit
// relative --state-dir / TS_STATE_DIR / yaml override.
func stateDirIsCWDRelative(dir string) bool {
	return dir != "" && !filepath.IsAbs(dir)
}

// EphemeralStateDir returns a temp-based, CWD-independent state directory for
// auto-instance (throwaway-identity) mode. Like StateDirForPlatform it is
// always absolute, so ephemeral node state never lands in the working dir.
func EphemeralStateDir(hostname string) string {
	return filepath.Join(os.TempDir(), appDirName, ephemeralSegment(hostname))
}

// ephemeralSegment reduces hostname to a single safe path segment so a value
// containing path separators or ".." cannot escape the temp state root.
func ephemeralSegment(hostname string) string {
	if seg := SanitizeHostnameLabel(hostname); seg != "" {
		return seg
	}
	return "instance"
}

// platformStateBase resolves the per-OS state directory before the
// absolute-path guard in stateDirFor.
func platformStateBase(goos string, getenv func(string) string) string {
	switch goos {
	case osWindows:
		return filepath.Join(getenv("LOCALAPPDATA"), appDirName, stateDirLeaf)
	case osDarwin:
		return filepath.Join(getenv("HOME"), "Library", "Application Support", appDirName, stateDirLeaf)
	default:
		// Linux and others: honor $XDG_STATE_HOME, fall back to ~/.local/state.
		base := getenv("XDG_STATE_HOME")
		if base == "" {
			base = filepath.Join(getenv("HOME"), ".local", stateDirLeaf)
		}
		return filepath.Join(base, appDirName, stateDirLeaf)
	}
}
