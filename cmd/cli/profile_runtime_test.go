package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ts-bridge/internal/config"
	"ts-bridge/internal/profile"
)

func setupProfileRuntimeTest(t *testing.T) string {
	t.Helper()
	path, _ := setupProfileCredentialTest(t)
	t.Chdir(t.TempDir())
	for _, name := range []string{
		"TS_TARGET", "TS_AUTHKEY", "TS_CONTROL_URL", "TS_BOOTSTRAP_SSH",
		"TS_BOOTSTRAP_SOCKS_ADDR", "TS_HOSTNAME", "TS_STATE_DIR", "TS_LOCAL_ADDR",
		"TS_HEALTH_ADDR", "TS_INSTANCE_NAME", "TS_AUTO_INSTANCE", "TS_MANUAL_MODE",
		"TS_SOCKS5_ADDR",
	} {
		t.Setenv(name, "")
	}
	oldRunner, oldLogger, oldBrowser := Runner, LoggerInit, BrowserRunner
	t.Cleanup(func() { Runner, LoggerInit, BrowserRunner = oldRunner, oldLogger, oldBrowser })
	LoggerInit = nil
	return path
}

func writeProfileRuntimeFixture(t *testing.T, path string) {
	t.Helper()
	source := `profiles:
  office:
    target: mesh:443
    control_url: https://vpn.example.com
    credential: office-key
    bootstrap_ssh: profile-bastion
    bootstrap_socks_addr: 127.0.0.1:1056
    hostname: profile-bridge
    local_addr: 127.0.0.1:34443
    health_addr: 127.0.0.1:9081
`
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestProfileRuntimeConnectPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		yaml    string
		args    []string
		wantSSH string
	}{
		{name: "profile only", wantSSH: "profile-bastion"},
		{name: "YAML overrides profile", yaml: "bootstrap_ssh: yaml-bastion\n", wantSSH: "yaml-bastion"},
		{name: "environment overrides YAML", env: "env-bastion", yaml: "bootstrap_ssh: yaml-bastion\n", wantSSH: "env-bastion"},
		{name: "flag overrides environment", env: "env-bastion", args: []string{"--bootstrap-ssh", "flag-bastion"}, wantSSH: "flag-bastion"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := setupProfileRuntimeTest(t)
			writeProfileRuntimeFixture(t, path)
			t.Setenv("TS_BOOTSTRAP_SSH", tt.env)
			var got config.Config
			Runner = func(cfg config.Config) error { got = cfg; return nil }
			args := append([]string{"connect", "--profile", "office"}, tt.args...)
			if tt.yaml != "" {
				configPath := filepath.Join(t.TempDir(), "override.yaml")
				if err := os.WriteFile(configPath, []byte(tt.yaml), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--config", configPath)
			}
			command := NewRootCmd()
			command.SetArgs(args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if got.BootstrapSSH != tt.wantSSH || got.BootstrapSOCKSAddr != "127.0.0.1:1056" ||
				got.Hostname != "profile-bridge" || got.LocalAddr != "127.0.0.1:34443" ||
				got.HealthAddr != "127.0.0.1:9081" || got.AuthKey != "tskey-auth-profile-secret" {
				t.Fatalf("profile runtime not resolved: SSH=%q SOCKS=%q host=%q local=%q health=%q",
					got.BootstrapSSH, got.BootstrapSOCKSAddr, got.Hostname, got.LocalAddr, got.HealthAddr)
			}
		})
	}
}

func TestProfileRuntimeImportPreservesExistingTargetAndCredential(t *testing.T) {
	path := setupProfileRuntimeTest(t)
	configPath := filepath.Join(t.TempDir(), "runtime.yaml")
	stateDir := filepath.Join(t.TempDir(), "state")
	source := "version: 1\ncontrol_url: https://vpn.example.com\nbootstrap_ssh: imported-bastion\n" +
		"bootstrap_socks_addr: 127.0.0.1:1056\nstate_dir: " + filepath.ToSlash(stateDir) + "\n" +
		"hostname: imported-bridge\nhealth_addr: 127.0.0.1:9081\nlocal_addr: 127.0.0.1:34443\n"
	if err := os.WriteFile(configPath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	var configured []byte
	for attempt := range 2 {
		command := NewRootCmd()
		command.SetArgs([]string{"init", "--profile", "office", "--config", configPath, "--force"})
		output, _ := captureInitOutput(t, func() {
			if err := command.Execute(); err != nil {
				t.Fatalf("one-time profile configuration: %v", err)
			}
		})
		if strings.Contains(output, "auth set") {
			t.Fatal("import must reuse the existing credential, not ask to replace it")
		}
		current, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if attempt > 0 && !bytes.Equal(configured, current) {
			t.Fatal("repeating profile configuration must leave the store unchanged")
		}
		configured = current
	}
	selected, err := profile.NewStore(path).Get("office")
	if err != nil {
		t.Fatal(err)
	}
	if selected.Target != "acemagic-office:45000" || selected.Credential != "office-key" {
		t.Fatalf("existing profile association was lost: %#v", selected)
	}
	var got config.Config
	Runner = func(cfg config.Config) error { got = cfg; return nil }
	command := NewRootCmd()
	command.SetArgs([]string{"connect", "--profile", "office"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.BootstrapSSH != "imported-bastion" || filepath.Clean(got.StateDir) != stateDir ||
		got.LocalAddr != "127.0.0.1:34443" || got.Hostname != "imported-bridge" {
		t.Fatalf("imported runtime: SSH=%q state=%q (want %q) local=%q hostname=%q",
			got.BootstrapSSH, got.StateDir, stateDir, got.LocalAddr, got.Hostname)
	}
}

func TestProfileRuntimeImportRejectsUnsafeOrUnsupportedInput(t *testing.T) {
	for _, tt := range []struct{ name, source string }{
		{"secret", "auth_key: tskey-auth-do-not-leak\n"},
		{"credential path", "auth_key_file: private-key.txt\n"},
		{"credential association", "credential: other-key\n"},
		{"unsupported setting", "port_range: 33000-34000\n"},
		{"unsupported version", "version: 2\n"},
		{"second document", "hostname: first\n---\nhostname: second\n"},
		{"invalid second document", "hostname: first\n---\ninvalid: [\n"},
		{"secret in runtime", "state_dir: tskey-auth-do-not-leak\n"},
		{"secret in control URL", "control_url: tskey-auth-do-not-leak\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := setupProfileRuntimeTest(t)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(t.TempDir(), "invalid.yaml")
			if err := os.WriteFile(input, []byte(tt.source), 0o600); err != nil {
				t.Fatal(err)
			}
			command := NewRootCmd()
			command.SilenceErrors, command.SilenceUsage = true, true
			command.SetArgs([]string{"init", "--profile", "office", "--config", input, "--force"})
			err = command.Execute()
			if err == nil || strings.Contains(err.Error(), "tskey-auth-do-not-leak") {
				t.Fatalf("expected an actionable error without secret content, got %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("rejected import changed the profile store: %v", err)
			}
		})
	}
}

func TestProfileRuntimeBrowserDoesNotInheritTCPTarget(t *testing.T) {
	path := setupProfileRuntimeTest(t)
	writeProfileRuntimeFixture(t, path)
	var got config.Config
	BrowserRunner = func(cfg config.Config, _ BrowserOptions) error { got = cfg; return nil }
	command := NewRootCmd()
	command.SetArgs([]string{"browser", "--profile", "office", "--url", "https://forge.example.internal/",
		"--route", "forge.example.internal:443=mesh:443"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Target != "" || got.SOCKS5Addr == "" || got.BootstrapSSH != "profile-bastion" {
		t.Fatalf("browser runtime/target separation failed: target=%q socks=%q SSH=%q",
			got.Target, got.SOCKS5Addr, got.BootstrapSSH)
	}
}

func executeProfileLockCommand(kind, name string) error {
	command := NewRootCmd()
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	args := []string{kind, "--profile", name}
	if kind == "browser" {
		args = append(args, "--url", "https://forge.example.internal/",
			"--route", "forge.example.internal:443=mesh:443")
	}
	command.SetArgs(args)
	return command.Execute()
}

func TestProfileLockRejectsDuplicateBeforeRunner(t *testing.T) {
	for _, kind := range []string{"connect", "browser"} {
		t.Run(kind, func(t *testing.T) {
			path := setupProfileRuntimeTest(t)
			writeProfileRuntimeFixture(t, path)
			runs, logs := 0, 0
			LoggerInit = func(config.Config) { logs++ }
			run := func(config.Config) error {
				runs++
				if runs == 1 {
					for _, duplicateKind := range []string{"connect", "browser"} {
						err := executeProfileLockCommand(duplicateKind, "office")
						if err == nil || !strings.Contains(err.Error(), "already connected") {
							t.Errorf("duplicate %s was not rejected before startup: %v", duplicateKind, err)
						}
					}
				}
				return nil
			}
			Runner = run
			BrowserRunner = func(cfg config.Config, _ BrowserOptions) error { return run(cfg) }
			if err := executeProfileLockCommand(kind, "office"); err != nil {
				t.Fatal(err)
			}
			if runs != 1 || logs != 1 {
				t.Fatalf("duplicate started runtime/logging: runs=%d logs=%d", runs, logs)
			}
			if err := executeProfileLockCommand(kind, "office"); err != nil {
				t.Fatalf("command return did not release ownership: %v", err)
			}
			if runs != 2 || logs != 2 {
				t.Fatalf("restarted command did not run exactly once: runs=%d logs=%d", runs, logs)
			}
		})
	}
}

func TestProfileLockAllowsOtherProfile(t *testing.T) {
	path := setupProfileRuntimeTest(t)
	writeProfileRuntimeFixture(t, path)
	store := profile.NewStore(path)
	if err := store.Set("other", "other-mesh:443", "https://vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCredential("other", "office-key"); err != nil {
		t.Fatal(err)
	}
	runs := 0
	Runner = func(config.Config) error {
		runs++
		if runs == 1 {
			return executeProfileLockCommand("connect", "other")
		}
		return nil
	}
	if err := executeProfileLockCommand("connect", "office"); err != nil {
		t.Fatal(err)
	}
	if runs != 2 {
		t.Fatalf("independent profile did not run: got %d runtime calls", runs)
	}
}
