package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"ts-bridge/internal/profile"
)

// BUG-001/002: init should not overwrite existing config without confirmation.

func TestWriteEnvConfig_DetectsExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	// Pre-create .env file.
	if err := os.WriteFile(envPath, []byte("TS_TARGET=100.64.0.1:3389\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a mock command with non-terminal stdin (non-interactive).
	cmd := &cobra.Command{}
	f := initFlags{
		AuthKey: "tskey-test",
		Target:  "100.64.0.2:443",
		Format:  "env",
		Config:  envPath,
		Force:   false,
	}

	err := writeEnvConfig(cmd, f)
	if err == nil {
		t.Fatal("expected error for existing .env without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}

	// File should be unchanged.
	data, _ := os.ReadFile(envPath)
	if strings.Contains(string(data), "100.64.0.2") {
		t.Error("file should not have been overwritten")
	}
}

func TestWriteEnvConfig_OverwriteWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	if err := os.WriteFile(envPath, []byte("TS_TARGET=100.64.0.1:3389\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	f := initFlags{
		AuthKey: "tskey-test",
		Target:  "100.64.0.2:443",
		Format:  "env",
		Config:  envPath,
		Force:   true,
	}

	err := writeEnvConfig(cmd, f)
	if err != nil {
		t.Fatalf("unexpected error with --force: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	if !strings.Contains(string(data), "100.64.0.2") {
		t.Error("file should have been overwritten with --force")
	}
}

func TestWriteYAMLConfig_DetectsExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "ts-bridge.yaml")

	if err := os.WriteFile(yamlPath, []byte("version: 1\ntarget: 100.64.0.1:3389\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	f := initFlags{
		AuthKey: "tskey-test",
		Target:  "100.64.0.2:443",
		Format:  "yaml",
		Config:  yamlPath,
		Force:   false,
	}

	err := writeYAMLConfig(cmd, f)
	if err == nil {
		t.Fatal("expected error for existing YAML without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestWriteYAMLConfig_CreatesYamlAndEnv(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "ts-bridge.yaml")
	envPath := filepath.Join(tmpDir, ".env")

	cmd := &cobra.Command{}
	f := initFlags{
		AuthKey:  "tskey-test",
		Target:   "100.64.0.1:3389",
		Format:   "yaml",
		Config:   yamlPath,
		Instance: "my-server",
		PortRange: "33389-34388",
		Force:    true,
	}

	err := writeYAMLConfig(cmd, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// YAML file should exist with target and hostname.
	yamlData, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("YAML file not created: %v", err)
	}
	if !strings.Contains(string(yamlData), "target:") {
		t.Error("YAML should contain target")
	}
	if !strings.Contains(string(yamlData), "my-server") {
		t.Error("YAML should contain hostname derived from instance")
	}

	// .env file should exist with TS_AUTHKEY.
	envData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf(".env file not created: %v", err)
	}
	if !strings.Contains(string(envData), "TS_AUTHKEY=tskey-test") {
		t.Error(".env should contain TS_AUTHKEY")
	}
}

func TestRunInitProfile(t *testing.T) {
	cases := []struct {
		name          string
		flags         initFlags
		preExisting   string // if non-empty, pre-write this target under f.Profile name
		wantErr       string // substring expected in error; empty = no error
		wantTarget    string
		wantControlURL string
	}{
		{
			name:       "writes profile without control URL",
			flags:      initFlags{Target: "host:3389", Profile: "work"},
			wantTarget: "host:3389",
		},
		{
			name:           "writes Headscale profile with control URL",
			flags:          initFlags{Target: "host:3389", Profile: "kubelab", ControlURL: "https://vpn.kubelab.live"},
			wantTarget:     "host:3389",
			wantControlURL: "https://vpn.kubelab.live",
		},
		{
			name:        "existing profile without --force returns error",
			flags:       initFlags{Target: "new-host:45000", Profile: "work", Force: false},
			preExisting: "old-host:3389",
			wantErr:     "already exists",
			wantTarget:  "old-host:3389", // unchanged
		},
		{
			name:        "existing profile with --force overwrites",
			flags:       initFlags{Target: "new-host:45000", Profile: "work", Force: true},
			preExisting: "old-host:3389",
			wantTarget:  "new-host:45000",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storePath := filepath.Join(t.TempDir(), "profiles.yaml")
			if tc.preExisting != "" {
				if err := profile.NewStore(storePath).Set(tc.flags.Profile, tc.preExisting, ""); err != nil {
					t.Fatalf("pre-write error: %v", err)
				}
			}

			err := runInitProfile(storePath, tc.flags)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantTarget != "" {
				p, getErr := profile.NewStore(storePath).Get(tc.flags.Profile)
				if getErr != nil {
					t.Fatalf("Get error: %v", getErr)
				}
				if p.Target != tc.wantTarget {
					t.Errorf("Target = %q, want %q", p.Target, tc.wantTarget)
				}
				if tc.wantControlURL != "" && p.ControlURL != tc.wantControlURL {
					t.Errorf("ControlURL = %q, want %q", p.ControlURL, tc.wantControlURL)
				}
			}
		})
	}
}

func TestWriteEnvConfig_CreatesFullConfig(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	cmd := &cobra.Command{}
	f := initFlags{
		AuthKey:   "tskey-test",
		Target:    "100.64.0.1:3389",
		Format:    "env",
		Config:    envPath,
		Instance:  "my-server",
		PortRange: "33389-34388",
	}

	err := writeEnvConfig(cmd, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf(".env file not created: %v", err)
	}

	// Should contain both required fields.
	content := string(data)
	if !strings.Contains(content, "TS_AUTHKEY=tskey-test") {
		t.Error("should contain TS_AUTHKEY")
	}
	if !strings.Contains(content, "TS_TARGET=100.64.0.1:3389") {
		t.Error("should contain TS_TARGET")
	}
	if !strings.Contains(content, "TS_INSTANCE_NAME=my-server") {
		t.Error("should contain TS_INSTANCE_NAME")
	}
	if !strings.Contains(content, "TS_PORT_RANGE=33389-34388") {
		t.Error("should contain TS_PORT_RANGE")
	}
}
