package agent

import (
	"fmt"
	"os"
)

// prepareDir creates the base directory. On Windows it holds only the lock, and
// LocalAppData is already private to the user's profile.
func prepareDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create agent directory: %w", err)
	}
	return nil
}
