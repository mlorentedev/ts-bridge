package discover

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// UpdateEnv rewrites every TS_ variable, TS_AUTHKEY included, with
// os.WriteFile(..., 0600), which keeps the mode of an existing file. A loose
// .env stays loose with the key in it, and discover does not even warn.
func TestUpdateEnvTightensAnExistingLooseFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("TS_AUTHKEY=tskey-auth-audit-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UpdateEnv(path, "office", 3389); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf(".env holding TS_AUTHKEY left at mode %04o after UpdateEnv, want no group/other bits", perm)
	}
}

// An existing .env that cannot be read must fail the update. Replacing it
// would write a file holding only TS_TARGET and lose TS_AUTHKEY and the rest.
func TestUpdateEnvRefusesAnUnreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), ".env")
	const original = "TS_AUTHKEY=tskey-auth-audit-secret\n"
	if err := os.WriteFile(path, []byte(original), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("file is readable despite mode 000 (running as root)")
	}

	if err := UpdateEnv(path, "office", 3389); err == nil {
		t.Fatal("UpdateEnv() = nil for an unreadable .env, want an error")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf(".env = %q after a refused update, want it untouched", got)
	}
}
