package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"ts-bridge/internal/config"
	"ts-bridge/internal/profile"
)

func TestConnectProfileResolvedBeforeValidation(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "profiles.yaml")
	store := profile.NewStore(storePath)
	if err := store.Set("saas", "profile-saas:22", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("headscale", "profile-headscale:22", "https://profile.example.com"); err != nil {
		t.Fatal(err)
	}

	originalStorePath := defaultProfileStorePath
	originalRunner := Runner
	originalLoggerInit := LoggerInit
	t.Cleanup(func() {
		defaultProfileStorePath = originalStorePath
		Runner = originalRunner
		LoggerInit = originalLoggerInit
	})
	defaultProfileStorePath = storePath
	LoggerInit = nil

	tests := []struct {
		name        string
		profileName string
		authKey     string
		extraArgs   []string
		envTarget   string
		envControl  string
		yaml        string
		wantTarget  string
		wantControl string
	}{
		{
			name:        "SaaS profile supplies target",
			profileName: "saas",
			authKey:     "tskey-auth-test",
			wantTarget:  "profile-saas:22",
		},
		{
			name:        "Headscale profile supplies target and control URL",
			profileName: "headscale",
			authKey:     "hskey-auth-test",
			wantTarget:  "profile-headscale:22",
			wantControl: "https://profile.example.com",
		},
		{
			name:        "flags override profile",
			profileName: "headscale",
			authKey:     "tskey-auth-test",
			extraArgs:   []string{"--target", "flag-host:22", "--control-url", "https://flag.example.com"},
			wantTarget:  "flag-host:22",
			wantControl: "https://flag.example.com",
		},
		{
			name:        "environment overrides profile",
			profileName: "headscale",
			authKey:     "tskey-auth-test",
			envTarget:   "env-host:22",
			envControl:  "https://env.example.com",
			wantTarget:  "env-host:22",
			wantControl: "https://env.example.com",
		},
		{
			name:        "YAML overrides profile",
			profileName: "headscale",
			authKey:     "tskey-auth-test",
			yaml:        "version: 1\ntarget: yaml-host:22\ncontrol_url: https://yaml.example.com\n",
			wantTarget:  "yaml-host:22",
			wantControl: "https://yaml.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TS_TARGET", tt.envTarget)
			t.Setenv("TS_CONTROL_URL", tt.envControl)
			t.Setenv("TS_AUTHKEY", "")

			args := []string{"--profile", tt.profileName, "--auth-key", tt.authKey}
			args = append(args, tt.extraArgs...)
			if tt.yaml != "" {
				configPath := filepath.Join(t.TempDir(), "config.yaml")
				if err := os.WriteFile(configPath, []byte(tt.yaml), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--config", configPath)
			}

			var captured config.Config
			Runner = func(cfg config.Config) error {
				captured = cfg
				return nil
			}

			cmd := newConnectCmd()
			cmd.SetArgs(args)
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if captured.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", captured.Target, tt.wantTarget)
			}
			if captured.ControlURL != tt.wantControl {
				t.Errorf("ControlURL = %q, want %q", captured.ControlURL, tt.wantControl)
			}
		})
	}
}
