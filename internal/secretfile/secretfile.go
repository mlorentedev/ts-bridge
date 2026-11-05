// Package secretfile writes files that hold a credential: owner-only, and
// replaced atomically so a reader never sees a partial file.
package secretfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile atomically replaces path with data, owner-only. Unlike
// os.WriteFile it also tightens a file that already exists with a looser mode.
func WriteFile(path string, data []byte) error {
	return WriteFileWith(path, data, os.Rename)
}

// WriteFileWith is WriteFile with the final rename injected, so a caller can
// test the failure path.
func WriteFileWith(path string, data []byte, rename func(string, string) error) error {
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }() // a no-op once renamed

	if err := writeTemp(temp, data); err != nil {
		return err
	}
	if err := Harden(tempPath, false); err != nil {
		return fmt.Errorf("secure temporary file: %w", err)
	}
	if err := rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	if err := Harden(path, false); err != nil {
		return fmt.Errorf("secure %s: %w", path, err)
	}
	return nil
}

func writeTemp(file *os.File, data []byte) error {
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("set file permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write file: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}
	return nil
}
