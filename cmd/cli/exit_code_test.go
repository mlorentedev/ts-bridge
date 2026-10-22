package cmd_test

import (
	"errors"
	"fmt"
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
