package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// writeUnreadable creates a file the process can write but not read, which is
// the case where a silent read failure turns into data loss (#408). Windows
// ignores the read bit and root bypasses it, so both skip.
func writeUnreadable(t *testing.T, path, content string) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits cannot make a file unreadable here")
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0200); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })
}

func TestWriteYAMLConfig_UnreadableEnvRefusesBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "ts-bridge.yaml")
	envPath := filepath.Join(dir, ".env")
	const original = "TS_AUTHKEY=old\nTS_INSTANCE_NAME=keep-me\n"
	writeUnreadable(t, envPath, original)

	err := writeYAMLConfig(&cobra.Command{}, initFlags{
		AuthKey: "tskey-auth-test-secret", Target: "100.64.0.1:3389",
		Format: "yaml", Config: yamlPath, Force: true,
	})
	if err == nil || !strings.Contains(err.Error(), envPath) {
		t.Fatalf("err = %v, want a read error naming %s", err, envPath)
	}
	if _, statErr := os.Stat(yamlPath); !os.IsNotExist(statErr) {
		t.Errorf("YAML written despite the refusal (stat err = %v)", statErr)
	}
	_ = os.Chmod(envPath, 0600)
	if got, _ := os.ReadFile(envPath); string(got) != original {
		t.Errorf(".env replaced despite the refusal:\n%s", got)
	}
}

func TestWriteHostEnv_UnreadableEnvRefusesToReplace(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	const original = "TS_AUTHKEY=old\nTS_TARGET=100.64.0.1:3389\n"
	writeUnreadable(t, envPath, original)

	err := writeHostEnv(hostInitFlags{Port: 3389, FirewallRule: "Rule", Config: envPath})
	if err == nil || !strings.Contains(err.Error(), envPath) {
		t.Fatalf("err = %v, want a read error naming %s", err, envPath)
	}
	_ = os.Chmod(envPath, 0600)
	if got, _ := os.ReadFile(envPath); string(got) != original {
		t.Errorf(".env replaced despite the refusal:\n%s", got)
	}
}

// A directory where the .env should be is the one unreadable case every OS and
// root produce alike, so these run on the Windows job too.
func TestWriteYAMLConfig_DirectoryAsEnvRefusesBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "ts-bridge.yaml")
	envPath := filepath.Join(dir, ".env")
	if err := os.Mkdir(envPath, 0700); err != nil {
		t.Fatal(err)
	}
	err := writeYAMLConfig(&cobra.Command{}, initFlags{
		AuthKey: "tskey-auth-test-secret", Target: "100.64.0.1:3389",
		Format: "yaml", Config: yamlPath, Force: true,
	})
	if err == nil || !strings.Contains(err.Error(), envPath) {
		t.Fatalf("err = %v, want a read error naming %s", err, envPath)
	}
	if _, statErr := os.Stat(yamlPath); !os.IsNotExist(statErr) {
		t.Errorf("YAML written despite the refusal (stat err = %v)", statErr)
	}
}

func TestWriteHostEnv_DirectoryAsEnvRefuses(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.Mkdir(envPath, 0700); err != nil {
		t.Fatal(err)
	}
	err := writeHostEnv(hostInitFlags{Port: 3389, FirewallRule: "Rule", Config: envPath})
	if err == nil || !strings.Contains(err.Error(), "read existing") {
		t.Fatalf("err = %v, want a read error, not a write error", err)
	}
}
