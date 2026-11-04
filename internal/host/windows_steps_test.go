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

// fwRule renders the firewall read's output for one rule.
func fwRule(enabled, action, direction, port string) string {
	return fwRuleScoped(enabled, action, direction, "TCP", "Any", port)
}

// fwRuleScoped is fwRule with the protocol and profile the recreate sets.
func fwRuleScoped(enabled, action, direction, protocol, profile, port string) string {
	return "enabled=" + enabled + "\r\naction=" + action + "\r\ndirection=" + direction +
		"\r\nprofile=" + profile + "\r\nremote=Any\r\nprotocol=" + protocol + "\r\nport=" + port + "\r\n"
}

// TestWindowsFirewallRule pins W3 (#455): setup reads the named rule and
// recreates it only when it is absent, duplicated, or differs in state,
// action, direction or port; check uses the same read.
func TestWindowsFirewallRule(t *testing.T) {
	good := fwRule("True", "Allow", "Inbound", "3390")
	tests := []struct {
		name      string
		read      string
		readFails bool
		setup     Outcome
		recreated bool
		check     Outcome
		ok        bool
	}{
		{name: "converged", read: "count=1\r\n" + good, setup: OutcomeOK, check: OutcomeOK, ok: true},
		{name: "absent", read: "count=0\r\n", setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "other port", read: "count=1\r\n" + fwRule("True", "Allow", "Inbound", "3389"), setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "disabled", read: "count=1\r\n" + fwRule("False", "Allow", "Inbound", "3390"), setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "blocks", read: "count=1\r\n" + fwRule("True", "Block", "Inbound", "3390"), setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "outbound", read: "count=1\r\n" + fwRule("True", "Allow", "Outbound", "3390"), setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "udp", read: "count=1\r\n" + fwRuleScoped("True", "Allow", "Inbound", "UDP", "Any", "3390"), setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "public profile only", read: "count=1\r\n" + fwRuleScoped("True", "Allow", "Inbound", "TCP", "Public", "3390"), setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "restricted remote address", read: "count=1\r\n" + strings.Replace(good, "remote=Any", "remote=10.0.0.0/255.0.0.0", 1), setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "duplicated", read: "count=2\r\n" + good + good, setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		// The recreate is idempotent, so setup acts on an unreadable rule;
		// check cannot say what setup would do, so it fails with the read.
		{name: "read fails", readFails: true, setup: OutcomeChanged, recreated: true, check: OutcomeFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newRunner := func() *recordingRunner {
				return &recordingRunner{
					output: map[string]string{psPrefix + psFwCheck: tt.read},
					fail:   map[string]bool{psPrefix + psFwCheck: tt.readFails},
				}
			}
			r := newRunner()
			step := windowsFirewallSetupStep(r, "Rule", 3390)
			recreated := false
			for _, c := range r.calls {
				recreated = recreated || c == "run "+psPrefix+psFw
				if strings.HasPrefix(c, "run ") && c != "run "+psPrefix+psFw {
					t.Errorf("setup ran %q; only the recreate may change the host", c)
				}
			}
			if step.Outcome != tt.setup || recreated != tt.recreated {
				t.Errorf("setup: outcome %q recreated %v, want %q recreated %v (%s)", step.Outcome, recreated, tt.setup, tt.recreated, step.Message)
			}

			step, ok := windowsFirewallCheckStep(newRunner(), "Rule", 3390)
			if step.Outcome != tt.check || ok != tt.ok {
				t.Errorf("check: outcome %q ok %v, want %q ok %v", step.Outcome, ok, tt.check, tt.ok)
			}
			if step.Outcome != OutcomeOK && step.Fix == "" {
				t.Errorf("check: %s with no fix", step.Outcome)
			}
			if step.Outcome == OutcomeChanged {
				want := "differs from what setup creates"
				if tt.name == "absent" {
					want = "No firewall rule found"
				}
				if !strings.Contains(step.Message, want) {
					t.Errorf("check message %q, want it to say %q", step.Message, want)
				}
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
			// A failed read is not trusted even when its stdout says true.
			name: "prefs unreadable",
			runner: &recordingRunner{exit: map[string]int{prefsRead: 1},
				output: map[string]string{prefsRead: `{"ForceDaemon": true}`}},
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

// An invalid rule name is a config error in setup as in check: both point at
// the key, not at opening the port by hand.
func TestWindowsFirewallInvalidName(t *testing.T) {
	setup := windowsFirewallSetupStep(&recordingRunner{}, "bad rule", 3390)
	check, _ := windowsFirewallCheckStep(&recordingRunner{}, "bad rule", 3390)
	for name, step := range map[string]SetupStep{"setup": setup, "check": check} {
		if step.Outcome != OutcomeFailed || !strings.Contains(step.Fix, "host.firewall.rule") {
			t.Errorf("%s: %s, fix %q, want failed naming host.firewall.rule", name, step.Outcome, step.Fix)
		}
	}
}
