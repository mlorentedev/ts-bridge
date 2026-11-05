package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// init writes TS_AUTHKEY into .env through writeFileWithPerms. os.WriteFile
// applies its mode only when it creates the file, so re-running init over an
// existing group- or world-readable .env (init --force, or a confirmed
// overwrite) leaves the auth key readable by others, with only a warning.
func TestWriteFileWithPermsTightensAnExistingLooseFile(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("TS_TARGET=old:1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeFileWithPerms(path, []byte("TS_AUTHKEY=tskey-auth-audit-secret\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("file holding the auth key left at mode %04o, want no group/other bits", perm)
	}
}
