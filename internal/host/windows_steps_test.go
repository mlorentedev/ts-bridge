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
	psDeny       = `$p = (Get-ItemProperty -Path 'HKLM:\SOFTWARE\Policies\Microsoft\Windows NT\Terminal Services' -Name fDenyTSConnections -ErrorAction SilentlyContinue).fDenyTSConnections; $l = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server' -Name fDenyTSConnections -ErrorAction SilentlyContinue).fDenyTSConnections; Write-Output "policy=$p"; Write-Output "local=$l"`
)

// denyRunner answers the fDenyTSConnections read with policy and local
// ("" = the value is absent) and the RDP port reads with 3390.
func denyRunner(policy, local string, readFails bool) *recordingRunner {
	return &recordingRunner{
		output: map[string]string{
			psPrefix + psDeny:   "policy=" + policy + "\r\nlocal=" + local + "\r\n",
			psPrefix + psPort:   "3390",
			psPrefix + psEnable: "3390",
		},
		fail: map[string]bool{psPrefix + psDeny: readFails},
	}
}

// TestWindowsRDPDenyState pins W2 (#455): RDP is enabled only when the
// effective fDenyTSConnections is 0, the Group Policy value taking precedence
// over the local one and an absent value meaning denied (the Windows default).
// Setup writes the local value only when the local value is what denies; a
// policy denial is the user's, since a local write is reverted on refresh.
func TestWindowsRDPDenyState(t *testing.T) {
	tests := []struct {
		name          string
		policy, local string
		readFails     bool
		setup         Outcome
		wrote         bool
		check         Outcome
		enabled       bool
	}{
		{name: "local allows", local: "0", setup: OutcomeOK, check: OutcomeOK, enabled: true},
		{name: "local denies", local: "1", setup: OutcomeChanged, wrote: true, check: OutcomeChanged},
		{name: "nothing set", setup: OutcomeChanged, wrote: true, check: OutcomeChanged},
		{name: "policy denies over local allow", policy: "1", local: "0", setup: OutcomeManual, check: OutcomeManual},
		{name: "policy allows over local deny", policy: "0", local: "1", setup: OutcomeOK, check: OutcomeOK, enabled: true},
		{name: "read fails", readFails: true, setup: OutcomeFailed, check: OutcomeFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := denyRunner(tt.policy, tt.local, tt.readFails)
			step, port := rdpSetupStep(r)
			wrote := false
			for _, c := range r.calls {
				wrote = wrote || c == "output "+psPrefix+psEnable
			}
			if step.Outcome != tt.setup || wrote != tt.wrote {
				t.Errorf("setup: outcome %q wrote %v, want %q wrote %v (%s)", step.Outcome, wrote, tt.setup, tt.wrote, step.Message)
			}
			if port != 3390 && !tt.readFails {
				t.Errorf("setup port = %d, want 3390", port)
			}
			if tt.setup == OutcomeManual && !strings.Contains(step.Fix, `Policies\Microsoft\Windows NT\Terminal Services`) {
				t.Errorf("manual fix does not name the policy key: %q", step.Fix)
			}

			var res CheckResult
			checkWindows(Config{FirewallRule: "Rule"}, denyRunner(tt.policy, tt.local, tt.readFails), &res)
			if got := res.Steps[0]; got.Outcome != tt.check || (got.Outcome != OutcomeOK && got.Fix == "") {
				t.Errorf("check: outcome %q fix %q, want %q with a fix", got.Outcome, got.Fix, tt.check)
			}
			if res.Services[0].Enabled != tt.enabled {
				t.Errorf("check: enabled = %v, want %v", res.Services[0].Enabled, tt.enabled)
			}
		})
	}
}

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
