package browser

import (
	"fmt"
	"os"
)

// PrepareProfileDir creates dir. The default lives under the user's
// LocalAppData, whose inherited ACL already admits only that user.
func PrepareProfileDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create browser profile directory: %w", err)
	}
	return nil
}
