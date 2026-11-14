package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	dirName    = "ts-bridge"
	lockName   = "agent.lock"
	socketName = "agent.sock"
)

// DefaultDir returns the per-user directory the agent keeps its lock and, on
// Unix, its socket in.
func DefaultDir() (string, error) {
	return defaultDir(runtime.GOOS, os.Getenv, os.UserCacheDir)
}

// defaultDir takes the OS and its lookups as arguments so every platform's
// choice is tested on any host. Linux prefers the runtime directory, which the
// login session creates private to the user and clears on logout; without a
// session (a container, `su`) it falls back to the user cache directory, which
// prepareDir then holds to the same ownership rules.
func defaultDir(goos string, getenv func(string) string, cacheDir func() (string, error)) (string, error) {
	if run := getenv("XDG_RUNTIME_DIR"); goos == "linux" && filepath.IsAbs(run) {
		return filepath.Join(run, dirName), nil
	}
	cache, err := cacheDir()
	if err != nil {
		return "", fmt.Errorf("locate the per-user cache directory: %w", err)
	}
	return filepath.Join(cache, dirName), nil
}
