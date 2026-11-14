package agent

import (
	"os"
	"testing"
)

// shortDir returns a directory with a short path: a Unix socket path is limited
// to about 104 bytes, and t.TempDir embeds the test name.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tsb")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
