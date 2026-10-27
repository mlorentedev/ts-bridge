package cmd

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	"ts-bridge/internal/config"
	"ts-bridge/internal/profile"
)

func TestConnectProfileTargetOverrideWarning(t *testing.T) {
	const profileTarget = "acemagic-office:45000"
	tests := []struct {
		name               string
		env                string
		dotenv             string
		yaml               string
		args               []string
		noProfile          bool
		emptyProfileTarget bool
		wantTarget         string
		wantSource         string
		wantError          bool
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
		{name: "profile without target", emptyProfileTarget: true, env: "env-host:443", wantTarget: "env-host:443"},
		{name: "SOCKS5 clears profile target", args: []string{"--socks5", "127.0.0.1:1080"}},
		{name: "quiet does not suppress warning", env: "env-host:443", args: []string{"--quiet"}, wantTarget: "env-host:443", wantSource: "TS_TARGET"},
		{name: "invalid target", env: "invalid-target", wantError: true},
	}
	for _, format := range []string{"text", "json"} {
		for _, tt := range tests {
			t.Run(format+"/"+tt.name, func(t *testing.T) {
				profilePath := setupProfileRuntimeTest(t)
				if tt.emptyProfileTarget {
					if err := profile.NewStore(profilePath).Set("office", "", ""); err != nil {
						t.Fatal(err)
					}
				}
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
	setupProfileRuntimeTest(t)
	var logs bytes.Buffer
	oldLogger, oldDefault := logger, slog.Default()
	t.Cleanup(func() {
		logger = oldLogger
		slog.SetDefault(oldDefault)
	})
	logger = nil
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Setenv("TS_TARGET", "env-host:443")

	ran := false
	Runner = func(cfg config.Config) error {
		ran = true
		assertTargetOverrideWarning(t, logs.String(), "json", "TS_TARGET", cfg.Target)
		return nil
	}
	command := newConnectCmd()
	command.SetArgs([]string{"--profile", "office"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !ran || strings.Count(logs.String(), "\n") != 1 {
		t.Fatalf("runner called = %v, want one warning before runner: %q", ran, logs.String())
	}

	assertTargetOverrideWarning(t, logs.String(), "json", "TS_TARGET", "env-host:443")
}

func TestConnectTargetOverrideProductionLogger(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			setupProfileRuntimeTest(t)
			logRoot := t.TempDir()
			t.Setenv("HOME", logRoot)
			t.Setenv("LOCALAPPDATA", logRoot)
			t.Setenv("TS_TARGET", "env-host:443")
			restoreConnectLogging(t)
			LoggerInit = InitLogger
			ran := false
			Runner = func(cfg config.Config) error {
				ran = true
				if !cfg.Quiet || cfg.LogFormat != format {
					t.Fatalf("runner quiet = %v, format = %q", cfg.Quiet, cfg.LogFormat)
				}
				assertTargetOverrideLogFile(t)
				_, err := os.Stdout.WriteString("runner-started\n")
				return err
			}
			output, stderr := captureInitOutput(t, func() {
				command := newConnectCmd()
				command.SetArgs([]string{"--profile", "office", "--quiet", "--log-format", format})
				if err := command.Execute(); err != nil {
					t.Fatal(err)
				}
			})
			warning, rest, found := strings.Cut(output, "\n")
			if !ran || !found || rest != "runner-started\n" || stderr != "" {
				t.Fatalf("runner called = %v, stdout = %q, stderr = %q", ran, output, stderr)
			}
			assertTargetOverrideWarning(t, warning, "text", "TS_TARGET", "env-host:443")
		})
	}
}

func assertTargetOverrideLogFile(t *testing.T) {
	t.Helper()
	if loggingInstance == nil || loggingInstance.LogPath() == "" {
		t.Fatal("production logger did not initialize a log file")
	}
	data, err := os.ReadFile(loggingInstance.LogPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "\n") != 1 || strings.Contains(string(data), "tskey-auth-") {
		t.Fatalf("want one credential-free log record before runner, got %q", data)
	}
	assertTargetOverrideWarning(t, string(data), "json", "TS_TARGET", "env-host:443")
}

func restoreConnectLogging(t *testing.T) {
	t.Helper()
	oldLogger, oldLogDir, oldInstance := logger, logDir, loggingInstance
	t.Cleanup(func() {
		if loggingInstance != nil && loggingInstance != oldInstance {
			if err := loggingInstance.Close(); err != nil {
				t.Errorf("close test logger: %v", err)
			}
		}
		logger, logDir, loggingInstance = oldLogger, oldLogDir, oldInstance
		config.SetLogger(oldLogger)
	})
}
