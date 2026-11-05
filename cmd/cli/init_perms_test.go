package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// init writes TS_AUTHKEY into .env. os.WriteFile applies its mode only when it
// creates the file, so re-running init over an existing group- or
// world-readable .env (init --force) once left the auth key readable by
// others. These tests drive the init command itself, so they fail if any write
// on that path stops tightening an existing file.
func TestInitTightensAnExistingLooseFile(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Unix permission bits")
	}
	tests := []struct {
		name   string
		format string
		config string // file the --config flag names
	}{
		{name: "env format", format: "env", config: ".env"},
		{name: "yaml format writes the .env beside it", format: formatYAML, config: "ts-bridge.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initTestEnv(t)
			dir := t.TempDir()
			keyPath := filepath.Join(dir, "authkey")
			if err := os.WriteFile(keyPath, []byte("tskey-auth-init-secret\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(dir, tt.config)
			envPath := filepath.Join(dir, ".env")
			for _, path := range []string{config, envPath} {
				if err := os.WriteFile(path, []byte("TS_TARGET=old:1\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			cmd := newInitCmd()
			cmd.SetArgs([]string{
				"--auth-key-file", keyPath, "--target", "100.64.0.1:3389",
				"--format", tt.format, "--config", config, "--force",
			})
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			var runErr error
			_, _ = captureInitOutput(t, func() { runErr = cmd.Execute() })
			if runErr != nil {
				t.Fatalf("init: %v", runErr)
			}

			data, err := os.ReadFile(envPath)
			if err != nil || !strings.Contains(string(data), "TS_AUTHKEY=tskey-auth-init-secret") {
				t.Fatalf(".env = %q, %v; want it to hold the auth key", data, err)
			}
			info, err := os.Stat(envPath)
			if err != nil {
				t.Fatal(err)
			}
			if perm := info.Mode().Perm(); perm&0o077 != 0 {
				t.Fatalf("file holding the auth key left at mode %04o, want no group/other bits", perm)
			}
		})
	}
}
