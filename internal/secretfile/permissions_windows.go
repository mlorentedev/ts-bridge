//go:build windows

package secretfile

import (
	"fmt"
	"os"
	"os/exec"
)

var runICACLS = func(args ...string) error {
	return exec.Command("icacls", args...).Run()
}

// Harden restricts path to its owner: mode 0600 (0700 for a directory) on Unix,
// an owner-only ACL on Windows.
func Harden(path string, directory bool) error {
	username := os.Getenv("USERNAME")
	if username == "" {
		return fmt.Errorf("USERNAME is not set")
	}
	grant := username + ":F"
	if directory {
		grant = username + ":(OI)(CI)F"
	}
	if err := runICACLS(path, "/inheritance:r", "/grant:r", grant); err != nil {
		return fmt.Errorf("icacls: %w", err)
	}
	return nil
}
