package host

import (
	"reflect"
	"strings"
	"testing"
)

const (
	prefsRead    = "tailscale debug prefs"
	setUnattend  = "tailscale set --unattended"
	headscalePfx = `{"ControlURL": "https://hs.example.internal", "RouteAll": false, "WantRunning": true, "LoggedOut": false, `
)

// TestWindowsUnattendedStep pins W1 (#455): the step reads ForceDaemon, the
// pref --unattended sets, and changes only that pref. It never runs
// `tailscale up`, which refuses on a Headscale node whose login server it was
// not given again.
func TestWindowsUnattendedStep(t *testing.T) {
	tests := []struct {
		name    string
		runner  *recordingRunner
		calls   []string
		outcome Outcome
	}{
		{
			name:    "already unattended",
			runner:  &recordingRunner{output: map[string]string{prefsRead: headscalePfx + `"ForceDaemon": true}`}},
			calls:   []string{"output " + prefsRead},
			outcome: OutcomeOK,
		},
		{
			// ForceDaemon is omitempty, so a node that never set it omits the key.
			name:    "headscale node, not unattended",
			runner:  &recordingRunner{output: map[string]string{prefsRead: headscalePfx + `"Hostname": ""}`}},
			calls:   []string{"output " + prefsRead, "run " + setUnattend},
			outcome: OutcomeChanged,
		},
		{
			name:    "prefs unparseable",
			runner:  &recordingRunner{output: map[string]string{prefsRead: "not json"}},
			calls:   []string{"output " + prefsRead, "run " + setUnattend},
			outcome: OutcomeChanged,
		},
		{
			name:    "prefs unreadable",
			runner:  &recordingRunner{fail: map[string]bool{prefsRead: true}},
			calls:   []string{"output " + prefsRead, "run " + setUnattend},
			outcome: OutcomeChanged,
		},
		{
			name: "set fails",
			runner: &recordingRunner{
				output: map[string]string{prefsRead: `{"ForceDaemon": false}`},
				fail:   map[string]bool{setUnattend: true},
			},
			calls:   []string{"output " + prefsRead, "run " + setUnattend},
			outcome: OutcomeFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unattendedStep(tt.runner)
			if !reflect.DeepEqual(tt.runner.calls, tt.calls) {
				t.Errorf("calls:\n got %q\nwant %q", tt.runner.calls, tt.calls)
			}
			for _, c := range tt.runner.calls {
				if strings.HasPrefix(c, "run tailscale up") {
					t.Errorf("ran %q: tailscale up resets prefs it is not given", c)
				}
			}
			if got.Outcome != tt.outcome {
				t.Errorf("outcome = %q, want %q (%s)", got.Outcome, tt.outcome, got.Message)
			}
			if got.Outcome == OutcomeFailed && got.Fix == "" {
				t.Error("failed step carries no fix")
			}
		})
	}
}
