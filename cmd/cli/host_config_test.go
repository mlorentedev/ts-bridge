package cmd_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	cmdpkg "ts-bridge/cmd/cli"
	"ts-bridge/internal/config"
	"ts-bridge/internal/host"
)

// hostRun is what one host command run saw and printed.
type hostRun struct {
	cfg            host.Config
	calls          int
	err            error
	stdout, stderr string
}

// runHostCmd runs `host <name>` as an elevated goos with args. The fake
// setup/check records the config it received; check returns ready.
func runHostCmd(t *testing.T, name, goos string, args []string, ready host.CheckResult) hostRun {
	t.Helper()
	var run hostRun
	var c *cobra.Command
	if name == "setup" {
		c = cmdpkg.HostSetupCmdForTest(t, goos, true, func(cfg host.Config, _ *slog.Logger) (host.SetupResult, error) {
			run.cfg, run.calls = cfg, run.calls+1
			return host.SetupResult{RDPPort: cfg.Port}, nil
		})
	} else {
		c = cmdpkg.HostCheckCmdOnForTest(t, goos, func(cfg host.Config, _ *slog.Logger) (host.CheckResult, error) {
			run.cfg, run.calls = cfg, run.calls+1
			ready.ControlURL = cfg.ControlURL
			return ready, nil
		})
	}
	var out, errOut bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errOut)
	c.SetArgs(args)
	run.err = c.Execute()
	run.stdout, run.stderr = out.String(), errOut.String()
	return run
}

// isolateHostEnv runs the test in an empty directory with no TS_HOST_* set,
// and returns that directory.
func isolateHostEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	for _, k := range []string{"TS_HOST_FIREWALL_RULE", "TS_HOST_NO_SLEEP", "TS_HOST_RDP_PORT", "TS_HOST_VERBOSE", "TS_HOST_LOG_FORMAT"} {
		t.Setenv(k, "")
	}
	return dir
}

func writeTestFile(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHostSetupCmd_FirewallRuleFlagUnset(t *testing.T) {
	isolateHostEnv(t)
	t.Setenv("TS_HOST_FIREWALL_RULE", "EnvRule")
	run := runHostCmd(t, "setup", "windows", []string{"--json"}, host.CheckResult{})
	if run.cfg.FirewallRule != "EnvRule" {
		t.Errorf("FirewallRule = %q, want EnvRule (an unset flag must not override env)", run.cfg.FirewallRule)
	}
}

func TestHostSetupCmd_Config(t *testing.T) { testHostCmdConfig(t, "setup") }

func TestHostCheckCmd_Config(t *testing.T) { testHostCmdConfig(t, "check") }

// testHostCmdConfig: --config reaches host <name>, below env and .env, and an
// unusable file exits 1 before the step runner is called (AC1, AC2).
func testHostCmdConfig(t *testing.T, name string) {
	const valid = "version: 1\nhost:\n  services:\n    - kind: rdp\n      port: 3390\n  firewall:\n    rule: FileRule\n  bogus: 1\n"
	tests := []struct {
		name      string
		body      string // "" means the path does not exist
		dotEnv    string
		wantCalls int
		wantPort  int
		wantErr   string // substring of the error; "" means success
		wantWarn  string
	}{
		{"file reaches the command", valid, "", 1, 3390, "", `unknown host config key "host.bogus"`},
		{".env beats the file", valid, "TS_HOST_RDP_PORT=3395\n", 1, 3395, "", ""},
		{"invalid file", "host:\n  services:\n    - kind: ftp\n", "", 0, 0, "host.services[0].kind", ""},
		{"missing file names the path", "", "", 0, 0, "host.yaml", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := isolateHostEnv(t)
			path := filepath.Join(dir, "host.yaml")
			if tt.body != "" {
				writeTestFile(t, path, tt.body)
			}
			if tt.dotEnv != "" {
				writeTestFile(t, filepath.Join(dir, ".env"), tt.dotEnv)
			}
			run := runHostCmd(t, name, "windows", []string{"--json", "--config", path}, host.CheckResult{})
			if run.calls != tt.wantCalls {
				t.Fatalf("calls = %d, want %d (err %v)", run.calls, tt.wantCalls, run.err)
			}
			if tt.wantErr != "" {
				assertExit1Naming(t, run.err, tt.wantErr)
				return
			}
			if run.cfg.Port != tt.wantPort || run.cfg.FirewallRule != "FileRule" {
				t.Errorf("Port %d, FirewallRule %q; want %d, FileRule", run.cfg.Port, run.cfg.FirewallRule, tt.wantPort)
			}
			if !strings.Contains(run.stderr, tt.wantWarn) {
				t.Errorf("stderr = %q, want %q", run.stderr, tt.wantWarn)
			}
		})
	}
}

// TestHostSetupCmd_ConfigOSCapability: an entry linux has no step for exits 1
// naming the OS before any step runs (AC4 wiring guard).
func TestHostSetupCmd_ConfigOSCapability(t *testing.T) {
	dir := isolateHostEnv(t)
	path := writeTestFile(t, filepath.Join(dir, "host.yaml"), "host:\n  services:\n    - kind: ssh\n")
	run := runHostCmd(t, "setup", "linux", []string{"--json", "--config", path}, host.CheckResult{})
	if run.calls != 0 {
		t.Fatalf("setup ran %d time(s), want 0", run.calls)
	}
	assertExit1Naming(t, run.err, "linux")
}

func assertExit1Naming(t *testing.T, err error, want string) {
	t.Helper()
	if cmdpkg.ExitCode(err) != 1 || err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v (exit %d), want exit 1 naming %q", err, cmdpkg.ExitCode(err), want)
	}
}

// TestHostCmds_NeverReadUserStore pins ADR-017 C9: elevated host commands
// never read the per-user profile or credential store, even when it sits
// where the process's home directories point.
func TestHostCmds_NeverReadUserStore(t *testing.T) {
	dir := isolateHostEnv(t)
	home := filepath.Join(dir, "root-home")
	for _, k := range []string{"HOME", "XDG_STATE_HOME", "XDG_CONFIG_HOME", "LOCALAPPDATA", "APPDATA", "USERPROFILE"} {
		t.Setenv(k, home)
	}
	store := config.ProfileStorePath()
	if err := os.MkdirAll(filepath.Join(filepath.Dir(store), "credentials"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, store, "profiles:\n  default:\n    target: 100.64.0.9:3389\n    control_url: https://poison.invalid\n    credential: poison\n")
	writeTestFile(t, filepath.Join(filepath.Dir(store), "credentials", "poison"), "tskey-poison")
	path := writeTestFile(t, filepath.Join(dir, "host.yaml"), "host:\n  services:\n    - kind: rdp\n")
	ready := host.CheckResult{TailscaleIP: "100.64.0.1", RDPPort: 3389}
	for _, name := range []string{"setup", "check"} {
		t.Run(name, func(t *testing.T) {
			run := runHostCmd(t, name, "linux", []string{"--json", "--config", path}, ready)
			if run.calls != 1 || run.cfg.ControlURL != "" {
				t.Errorf("calls %d, ControlURL %q; want 1 and none (err %v)", run.calls, run.cfg.ControlURL, run.err)
			}
			if strings.Contains(run.stdout+run.stderr, "poison") {
				t.Errorf("output reads the user store: %q", run.stdout+run.stderr)
			}
		})
	}
}

// TestHostCmds_DescriptorControlURL: a declared control_url is the
// descriptor's control plane in check's text and JSON output (AC6).
func TestHostCmds_DescriptorControlURL(t *testing.T) {
	dir := isolateHostEnv(t)
	path := writeTestFile(t, filepath.Join(dir, "host.yaml"), "host:\n  control_url: https://hs.example\n")
	ready := host.CheckResult{TailscaleIP: "100.64.0.1", RDPPort: 3389}
	const want = "cp=https%3A%2F%2Fhs.example"
	for _, args := range [][]string{{"--config", path}, {"--json", "--config", path}} {
		run := runHostCmd(t, "check", "linux", args, ready)
		if !strings.Contains(run.stdout, want) {
			t.Errorf("%v: stdout = %q, want a descriptor with %s", args, run.stdout, want)
		}
	}
}

// TestPrintSetupSummary_ControlURL: setup's summary descriptor carries the
// declared control plane (AC6).
func TestPrintSetupSummary_ControlURL(t *testing.T) {
	var out bytes.Buffer
	cmdpkg.PrintSetupSummaryForTest(&out, "100.64.0.1", host.SetupResult{RDPPort: 3389, ControlURL: "https://hs.example"})
	if !strings.Contains(out.String(), "cp=https%3A%2F%2Fhs.example") {
		t.Errorf("summary = %q, want the declared control plane", out.String())
	}
}
