package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
			authKey:     "tskey-auth-test-secret",
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
			authKey:     "tskey-auth-test-secret",
			extraArgs:   []string{"--target", "flag-host:22", "--control-url", "https://flag.example.com"},
			wantTarget:  "flag-host:22",
			wantControl: "https://flag.example.com",
		},
		{
			name:        "environment overrides profile",
			profileName: "headscale",
			authKey:     "tskey-auth-test-secret",
			envTarget:   "env-host:22",
			envControl:  "https://env.example.com",
			wantTarget:  "env-host:22",
			wantControl: "https://env.example.com",
		},
		{
			name:        "YAML overrides profile",
			profileName: "headscale",
			authKey:     "tskey-auth-test-secret",
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

// After `ts-bridge import home tsb://...` the next documented step is
// `connect --profile home`; without a credential it must point at the managed
// path (ADR-015) the profile is built for, not only at TS_AUTHKEY.
func TestConnectProfileWithoutCredentialPointsToAuthSet(t *testing.T) {
	t.Chdir(t.TempDir())
	storePath := filepath.Join(t.TempDir(), "profiles.yaml")
	if err := profile.NewStore(storePath).Set("home", "acemagic-office:45000", ""); err != nil {
		t.Fatal(err)
	}
	oldStore, oldRunner, oldLogger := defaultProfileStorePath, Runner, LoggerInit
	t.Cleanup(func() { defaultProfileStorePath, Runner, LoggerInit = oldStore, oldRunner, oldLogger })
	defaultProfileStorePath, LoggerInit = storePath, nil
	Runner = func(config.Config) error { return nil }
	t.Setenv("TS_AUTHKEY", "")
	t.Setenv("TS_TARGET", "")

	cmd := newConnectCmd()
	cmd.SetArgs([]string{"--profile", "home"})
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	err := cmd.Execute()
	if err == nil {
		t.Fatal("connect --profile home without any credential succeeded")
	}
	if !strings.Contains(err.Error(), "auth set --profile home") {
		t.Fatalf("error = %q; want it to name `ts-bridge auth set --profile home`", err)
	}
	if !errors.Is(err, config.ErrAuthKeyRequired) {
		t.Fatalf("error = %v; want it to still wrap config.ErrAuthKeyRequired", err)
	}
}

// Without --profile there is no profile to attach a credential to, so the hint
// must not invent one.
func TestConnectWithoutProfileDoesNotSuggestAuthSet(t *testing.T) {
	t.Chdir(t.TempDir())
	oldRunner, oldLogger := Runner, LoggerInit
	t.Cleanup(func() { Runner, LoggerInit = oldRunner, oldLogger })
	LoggerInit = nil
	Runner = func(config.Config) error { return nil }
	t.Setenv("TS_AUTHKEY", "")
	t.Setenv("TS_TARGET", "")

	cmd := newConnectCmd()
	cmd.SetArgs([]string{"--target", "host:22"})
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	err := cmd.Execute()
	if !errors.Is(err, config.ErrAuthKeyRequired) {
		t.Fatalf("error = %v; want config.ErrAuthKeyRequired, or this guard does not exercise the no-hint branch", err)
	}
	if strings.Contains(err.Error(), "auth set") {
		t.Fatalf("error = %q; must not suggest auth set without --profile", err)
	}
}
