package cmd_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	cmdpkg "ts-bridge/cmd/cli"
)

func TestConnectErrorHandling(t *testing.T) {
	// Isolate from developer's .env file and env vars.
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_AUTHKEY", "")
	t.Setenv("TS_AUTO_INSTANCE", "")
	t.Setenv("TS_MANUAL_MODE", "")
	
	tmpDir := t.TempDir()
	originalWD, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to tmpDir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("failed to chdir back to originalWD: %v", err)
		}
	})

	cases := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "missing target",
			args:        []string{"connect", "--auth-key", "tskey-test"},
			errContains: "target is required",
		},
		{
			name:        "malformed target missing port",
			args:        []string{"connect", "--target", "100.64.0.1", "--auth-key", "tskey-test"},
			errContains: "target invalid format",
		},
		{
			name:        "out of bounds port",
			args:        []string{"connect", "--target", "100.64.0.1:99999", "--auth-key", "tskey-test"},
			errContains: "invalid port",
		},
		{
			name:        "missing auth key",
			args:        []string{"connect", "--target", "100.64.0.1:3389"},
			errContains: "auth key is required",
		},
		{
			name:        "malformed auth key",
			args:        []string{"connect", "--target", "100.64.0.1:3389", "--auth-key", "invalid-format"},
			errContains: "auth key invalid format",
		},
		{
			name:        "missing auth key file",
			args:        []string{"connect", "--target", "100.64.0.1:3389", "--auth-key-file", "does-not-exist.txt"},
			errContains: "stat auth key file",
		},
		{
			name:        "invalid dial retries",
			args:        []string{"connect", "--target", "100.64.0.1:3389", "--auth-key", "tskey-test", "--dial-retries", "-1"},
			errContains: "dial retries must be non-negative",
		},
		{
			name:        "invalid idle timeout",
			args:        []string{"connect", "--target", "100.64.0.1:3389", "--auth-key", "tskey-test", "--idle-timeout", "-5s"},
			errContains: "idle timeout must be non-negative",
		},
		{
			name:        "missing config file",
			args:        []string{"connect", "--target", "100.64.0.1:3389", "--auth-key", "tskey-test", "--config", "does-not-exist.yaml"},
			errContains: "read YAML config:",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := cmdpkg.NewRootCmd()
			root.SetArgs(c.args)

			// Silence usage so we just get the error
			root.SilenceUsage = true
			root.SilenceErrors = true

			// Capture output just in case
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)

			err := root.Execute()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.errContains)
			}

			if !strings.Contains(err.Error(), c.errContains) {
				t.Errorf("expected error to contain %q, got: %v", c.errContains, err)
			}
		})
	}
}
