package cmd_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	cmdpkg "ts-bridge/cmd/cli"
	"ts-bridge/internal/host"
)

// Exit status follows `terraform plan -detailed-exitcode`: 0 converged,
// 1 the command failed, 2 the command ran and the host is not converged.
func TestExitCode(t *testing.T) {
	notConverged := &cmdpkg.ExitError{Code: 2, Err: errors.New("host not converged")}
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, 0},
		{"plain error", errors.New("boom"), 1},
		{"exit error", notConverged, 2},
		{"wrapped exit error", fmt.Errorf("host check: %w", notConverged), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cmdpkg.ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestCheckConvergence(t *testing.T) {
	ok := host.SetupStep{Name: "Tailscale IP", Outcome: host.OutcomeOK}
	tests := []struct {
		name    string
		steps   []host.SetupStep
		wantErr string // empty: converged
	}{
		{"every step ok", []host.SetupStep{ok, ok}, ""},
		{"no steps", nil, ""},
		// changed counts: in check it means setup would change something.
		{"drift", []host.SetupStep{ok, {Name: "Firewall rule", Outcome: host.OutcomeChanged}}, "1 of 2 steps not ok: Firewall rule (changed)"},
		{"failed and unverified", []host.SetupStep{
			{Name: "Tailscale IP", Outcome: host.OutcomeFailed},
			{Name: "Firewall rule", Outcome: host.OutcomeUnverified},
		}, "2 of 2 steps not ok: Tailscale IP (failed), Firewall rule (unverified)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmdpkg.CheckConvergenceForTest(tt.steps)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			if cmdpkg.ExitCode(err) != 2 || err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v (exit %d), want exit 2 naming %q", err, cmdpkg.ExitCode(err), tt.wantErr)
			}
		})
	}
}

// TestHostCheckCmd_ExitStatus drives the host check command end to end with an
// injected result, in text and JSON mode. The smoke suite accepts 0 or 2 on any
// runner, so this is the test that fails if check stops reporting a host that
// needs work, whether in checkConvergence, reportCheck or runHostCheck. It also
// pins that the result goes to the command's writer, not the process stdout
// (#472).
func TestHostCheckCmd_ExitStatus(t *testing.T) {
	t.Chdir(t.TempDir()) // no .env from the working tree
	converged := host.CheckResult{Steps: []host.SetupStep{{Name: "Tailscale IP", Outcome: host.OutcomeOK}}}
	drift := host.CheckResult{Steps: []host.SetupStep{{Name: "Firewall rule", Outcome: host.OutcomeChanged, Fix: "run ts-bridge host setup"}}}
	tests := []struct {
		name   string
		result host.CheckResult
		args   []string
		want   int
		stdout string
	}{
		{"converged, text", converged, nil, 0, "HOST CHECK"},
		{"converged, json", converged, []string{"--json"}, 0, `"steps"`},
		{"drift, text", drift, nil, 2, "! Firewall rule (changed): run ts-bridge host setup"},
		{"drift, json", drift, []string{"--json"}, 2, `"outcome": "changed"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cmdpkg.HostCheckCmdForTest(t, func(host.Config, *slog.Logger) (host.CheckResult, error) { return tt.result, nil })
			c.SetArgs(tt.args)
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetErr(io.Discard)
			if got := cmdpkg.ExitCode(c.Execute()); got != tt.want {
				t.Errorf("exit %d, want %d", got, tt.want)
			}
			if !strings.Contains(out.String(), tt.stdout) {
				t.Errorf("stdout = %q, want it to contain %q", out.String(), tt.stdout)
			}
		})
	}
}

func TestHostSetupCmd_ExitStatus(t *testing.T) {
	t.Chdir(t.TempDir()) // no .env from the working tree
	step := func(name string, o host.Outcome, fix string) host.SetupStep {
		return host.SetupStep{Name: name, Outcome: o, Success: o.Succeeded(), Message: name + " " + string(o), Fix: fix}
	}
	converged := host.SetupResult{RDPPort: 3389, Steps: []host.SetupStep{
		step("Tailscale IP", host.OutcomeOK, ""), step("Firewall rule", host.OutcomeChanged, ""),
		step("Screen Sharing", host.OutcomeManual, "turn it on in System Settings")}}
	failed := host.SetupResult{RDPPort: 3389, Steps: []host.SetupStep{
		step("Tailscale IP", host.OutcomeOK, ""), step("xrdp service", host.OutcomeFailed, "sudo apt install xrdp")}}
	tests := []struct {
		name     string
		elevated bool
		result   host.SetupResult
		args     []string
		want     int
		stdout   string
	}{
		{"ok, changed and manual, text", true, converged, nil, 0, "Firewall rule"},
		{"ok, changed and manual, json", true, converged, []string{"--json"}, 0, `"outcome": "manual"`},
		{"a failed step, text", true, failed, nil, 2, "xrdp service"},
		{"a failed step, json", true, failed, []string{"--json"}, 2, `"outcome": "failed"`},
		{"not elevated", false, converged, nil, 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cmdpkg.HostSetupCmdForTest(t, tt.elevated, func(host.Config, *slog.Logger) (host.SetupResult, error) { return tt.result, nil })
			c.SetArgs(tt.args)
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetErr(io.Discard)
			if got := cmdpkg.ExitCode(c.Execute()); got != tt.want {
				t.Errorf("exit %d, want %d", got, tt.want)
			}
			if tt.stdout == "" && out.Len() > 0 {
				t.Errorf("stdout = %q, want nothing", out.String())
			}
			if !strings.Contains(out.String(), tt.stdout) {
				t.Errorf("stdout = %q, want it to contain %q", out.String(), tt.stdout)
			}
		})
	}
}
