package cmd

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	"ts-bridge/internal/config"
)

func TestConnectProfileTargetOverrideWarning(t *testing.T) {
	const profileTarget = "acemagic-office:45000"
	tests := []struct {
		name       string
		env        string
		dotenv     string
		yaml       string
		args       []string
		noProfile  bool
		wantTarget string
		wantSource string
		wantError  bool
	}{
		{name: "profile only", wantTarget: profileTarget},
		{name: "YAML overrides profile", yaml: "yaml-host:443", wantTarget: "yaml-host:443", wantSource: "YAML"},
		{name: "environment overrides YAML", env: "env-host:443", yaml: "yaml-host:443", wantTarget: "env-host:443", wantSource: "TS_TARGET"},
		{name: "flag overrides environment and YAML", env: "env-host:443", yaml: "yaml-host:443", args: []string{"--target", "flag-host:443"}, wantTarget: "flag-host:443", wantSource: "--target"},
		{name: "dotenv overrides profile", dotenv: "dotenv-host:443", wantTarget: "dotenv-host:443", wantSource: "TS_TARGET"},
		{name: "equal environment target", env: profileTarget, wantTarget: profileTarget},
		{name: "equal YAML target", yaml: profileTarget, wantTarget: profileTarget},
		{name: "flag restores profile target", env: "env-host:443", args: []string{"--target", profileTarget}, wantTarget: profileTarget},
		{name: "no profile", noProfile: true, env: "env-host:443", wantTarget: "env-host:443"},
		{name: "SOCKS5 clears profile target", args: []string{"--socks5", "127.0.0.1:1080"}},
		{name: "quiet does not suppress warning", env: "env-host:443", args: []string{"--quiet"}, wantTarget: "env-host:443", wantSource: "TS_TARGET"},
		{name: "invalid target", env: "invalid-target", wantError: true},
	}
	for _, format := range []string{"text", "json"} {
		for _, tt := range tests {
			t.Run(format+"/"+tt.name, func(t *testing.T) {
				profilePath := setupProfileRuntimeTest(t)
				t.Setenv("TS_TARGET", tt.env)
				t.Setenv("TS_AUTHKEY", "tskey-auth-synthetic-secret")
				if tt.dotenv != "" {
					if err := os.WriteFile(".env", []byte("TS_TARGET="+tt.dotenv+"\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				args := []string{"--log-format", format}
				if !tt.noProfile {
					args = append(args, "--profile", "office")
				}
				if tt.yaml != "" {
					if err := os.WriteFile("config.yaml", []byte("version: 1\ntarget: "+tt.yaml+"\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--config", "config.yaml")
				}
				args = append(args, tt.args...)
				before, err := os.ReadFile(profilePath)
				if err != nil {
					t.Fatal(err)
				}

				var logs bytes.Buffer
				oldLogger := logger
				t.Cleanup(func() { logger = oldLogger })
				LoggerInit = func(cfg config.Config) {
					if cfg.LogFormat == "json" {
						logger = slog.New(slog.NewJSONHandler(&logs, nil))
					} else {
						logger = slog.New(slog.NewTextHandler(&logs, nil))
					}
				}
				var gotTarget, logsAtRun string
				var ran bool
				Runner = func(cfg config.Config) error {
					ran, gotTarget, logsAtRun = true, cfg.Target, logs.String()
					return nil
				}
				command := newConnectCmd()
				command.SetArgs(args)
				command.SilenceUsage, command.SilenceErrors = true, true
				err = command.Execute()
				if (err != nil) != tt.wantError {
					t.Fatalf("Execute() error = %v, wantError = %v", err, tt.wantError)
				}
				if ran == tt.wantError || gotTarget != tt.wantTarget {
					t.Fatalf("runner called = %v, target = %q; want called = %v, target = %q", ran, gotTarget, !tt.wantError, tt.wantTarget)
				}
				after, err := os.ReadFile(profilePath)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) {
					t.Fatal("profile store changed during connect")
				}
				if strings.Contains(logs.String(), "tskey-auth-") {
					t.Fatal("authentication material leaked into diagnostics")
				}
				if tt.wantSource == "" {
					if logs.Len() != 0 {
						t.Fatalf("unexpected override warning: %s", &logs)
					}
					return
				}
				if logsAtRun == "" || logsAtRun != logs.String() || strings.Count(logsAtRun, "\n") != 1 {
					t.Fatalf("want exactly one warning before runner, got %q", logsAtRun)
				}
				assertTargetOverrideWarning(t, logsAtRun, format, tt.wantSource, tt.wantTarget)
			})
		}
	}
}

func assertTargetOverrideWarning(t *testing.T, output, format, source, target string) {
	t.Helper()
	want := map[string]string{
		"level": "WARN", "msg": "connection target overrides selected profile",
		"profile": "office", "source": source, "target": target,
	}
	if format == "json" {
		var got map[string]string
		if err := json.Unmarshal([]byte(output), &got); err != nil {
			t.Fatalf("invalid JSON warning: %v", err)
		}
		for key, value := range want {
			if got[key] != value {
				t.Errorf("warning %s = %q, want %q", key, got[key], value)
			}
		}

		return
	}
	for key, value := range want {
		if key == "msg" {
			value = `"` + value + `"`
		}
		if !strings.Contains(output, key+"="+value) {
			t.Errorf("warning missing %s=%s: %s", key, value, output)
		}
	}
}

func TestConnectTargetOverrideDefaultLogger(t *testing.T) {
	var logs bytes.Buffer
	oldLogger, oldDefault := logger, slog.Default()
	t.Cleanup(func() {
		logger = oldLogger
		slog.SetDefault(oldDefault)
	})
	logger = nil
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Setenv("TS_TARGET", "env-host:443")

	warnConnectTargetOverride("office", "profile-host:443", "env-host:443", "")

	assertTargetOverrideWarning(t, logs.String(), "json", "TS_TARGET", "env-host:443")
}
