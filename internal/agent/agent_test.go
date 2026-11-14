package agent

import (
	"os"
	"path/filepath"
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

// testPaths keeps the lock and the socket in separate directories under one
// short root, as the defaults do on Linux.
func testPaths(t *testing.T) Paths {
	t.Helper()
	root := shortDir(t)
	return Paths{Base: filepath.Join(root, "b"), Run: filepath.Join(root, "r")}
}
