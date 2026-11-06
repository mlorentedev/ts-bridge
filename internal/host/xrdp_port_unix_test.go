//go:build !windows

package host

import (
	"os"
	"path/filepath"
	"testing"
)

// execRunner.WriteFile replaces the file whole and keeps its mode: xrdp.ini is
// root's 0644, and a 0600 rewrite would lock xrdp's own reader out (#557).
func TestExecRunnerWriteFileKeepsTheMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xrdp.ini")
	// #nosec G306 -- a mode CreateTemp never produces, so keeping it is visible.
	if err := os.WriteFile(path, []byte("port=3389\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := (execRunner{}).WriteFile(path, []byte("port=3390\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "port=3390\n" || info.Mode().Perm() != 0o640 {
		t.Errorf("after WriteFile: %q mode %v, want port=3390 at 0640", data, info.Mode().Perm())
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("temporary file left behind: %v", entries)
	}
}

func TestExecRunnerWriteFileMissing(t *testing.T) {
	if err := (execRunner{}).WriteFile(filepath.Join(t.TempDir(), "absent.ini"), nil); !os.IsNotExist(err) {
		t.Errorf("WriteFile on a missing file = %v, want not-exist: setup never creates a service's configuration", err)
	}
}
