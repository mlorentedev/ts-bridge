package agent

import (
	"fmt"
	"os"
)

// prepareDir creates the state directory. It holds only the lock: the pipe has
// its own security descriptor, and the per-user cache directory the default
// lives in is already private to the user's profile.
func prepareDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create agent directory: %w", err)
	}
	return nil
}
