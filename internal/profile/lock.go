package profile

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errProfileInUse = errors.New("already connected; close this profile's existing ts-bridge terminal with Ctrl+C before retrying")

// Lock acquires exclusive ownership until the returned file or process closes.
// The lock file must not be removed: other processes may already have it open.
func (s *Store) Lock(name string) (*os.File, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("profile name must not be empty")
	}
	directory := filepath.Join(filepath.Dir(s.path), "locks")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create profile ownership directory: %w", err)
	}
	path := filepath.Join(directory, fmt.Sprintf("%x.lock", sha256.Sum256([]byte(name))))
	// #nosec G304 -- private store directory and a hashed, not path-interpreted, name.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open ownership file for profile %q: %w", name, err)
	}
	if err := lockFile(file); err != nil {
		return nil, fmt.Errorf("lock profile %q: %w", name, errors.Join(err, file.Close()))
	}
	return file, nil
}
