//go:build !windows

package secretfile

import "os"

// Harden restricts path to its owner: mode 0600 (0700 for a directory) on Unix,
// an owner-only ACL on Windows.
func Harden(path string, directory bool) error {
	mode := os.FileMode(0o600)
	if directory {
		mode = 0o700
	}
	return os.Chmod(path, mode)
}
