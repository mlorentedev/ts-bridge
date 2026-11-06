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

// tailnetRemote is the RemoteAddress the recreate sets, as the read prints it.
const tailnetRemote = "100.64.0.0/10,fd7a:115c:a1e0::/48"

// fwRuleScoped is fwRule with the protocol and profile the recreate sets.
func fwRuleScoped(enabled, action, direction, protocol, profile, port string) string {
	return "enabled=" + enabled + "\r\naction=" + action + "\r\ndirection=" + direction +
		"\r\nprofile=" + profile + "\r\nremote=" + tailnetRemote + "\r\nprotocol=" + protocol + "\r\nport=" + port + "\r\n"
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
		{name: "restricted remote address", read: "count=1\r\n" + strings.Replace(good, "remote="+tailnetRemote, "remote=10.0.0.0/255.0.0.0", 1), setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "legacy any address", read: "count=1\r\n" + strings.Replace(good, "remote="+tailnetRemote, "remote=Any", 1), setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "tailnet plus one more range", read: "count=1\r\n" + strings.Replace(good, tailnetRemote, tailnetRemote+",10.0.0.0/8", 1), setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "only the IPv4 range", read: "count=1\r\n" + strings.Replace(good, tailnetRemote, "100.64.0.0/10", 1), setup: OutcomeChanged, recreated: true, check: OutcomeChanged},
		{name: "windows normalised form", read: "count=1\r\n" + strings.Replace(good, tailnetRemote, "100.64.0.0/255.192.0.0,FD7A:115C:A1E0::/48", 1), setup: OutcomeOK, check: OutcomeOK, ok: true},
		{name: "reordered", read: "count=1\r\n" + strings.Replace(good, tailnetRemote, "fd7a:115c:a1e0::/48,100.64.0.0/10", 1), setup: OutcomeOK, check: OutcomeOK, ok: true},
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
				if tt.name == "legacy any address" {
					want = "legacy"
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
		message string // checked when set
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
				onPath: map[string]bool{"tailscale": true},
			},
			calls:   []string{"output " + prefsRead, "run " + setUnattend, "lookpath tailscale"},
			outcome: OutcomeFailed,
			message: "Could not enable via CLI — check Tailscale GUI",
		},
		{
			// #609: with no CLI the cause is the install, not the GUI.
			name:    "tailscale not installed",
			runner:  &recordingRunner{fail: map[string]bool{prefsRead: true, setUnattend: true}},
			calls:   []string{"output " + prefsRead, "run " + setUnattend, "lookpath tailscale"},
			outcome: OutcomeFailed,
			message: "Tailscale CLI not found — is Tailscale installed?",
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
			if tt.message != "" && got.Message != tt.message {
				t.Errorf("message = %q, want %q", got.Message, tt.message)
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

// TestWindowsServiceStep pins #604: a service already set to start
// automatically and running is "ok" with no write; otherwise only the missing
// half is written.
func TestWindowsServiceStep(t *testing.T) {
	const qc, query = "sc qc SSDPSRV", "sc query SSDPSRV"
	const auto, running = "START_TYPE         : 2   AUTO_START", "STATE              : 4  RUNNING"
	tests := []struct {
		name    string
		runner  *recordingRunner
		calls   []string
		outcome Outcome
	}{
		{"converged", &recordingRunner{output: map[string]string{qc: auto, query: running}},
			[]string{"output " + qc, "output " + query}, OutcomeOK},
		{"delayed auto start counts as auto", &recordingRunner{output: map[string]string{qc: auto + "  (DELAYED)", query: running}},
			[]string{"output " + qc, "output " + query}, OutcomeOK},
		{"manual start, running", &recordingRunner{output: map[string]string{qc: "3   DEMAND_START", query: running}},
			[]string{"output " + qc, "output " + query, "run sc config SSDPSRV start=auto"}, OutcomeChanged},
		{"auto, stopped", &recordingRunner{output: map[string]string{qc: auto, query: "1  STOPPED"}},
			[]string{"output " + qc, "output " + query, "run net start SSDPSRV"}, OutcomeChanged},
		{"reads fail, so it acts", &recordingRunner{fail: map[string]bool{qc: true, query: true}},
			[]string{"output " + qc, "output " + query, "run sc config SSDPSRV start=auto", "run net start SSDPSRV"}, OutcomeChanged},
		{"start fails", &recordingRunner{output: map[string]string{qc: auto}, fail: map[string]bool{"net start SSDPSRV": true}},
			[]string{"output " + qc, "output " + query, "run net start SSDPSRV"}, OutcomeFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serviceStep(tt.runner, "SSDPSRV", "SSDP Discovery", "Failed to start SSDPSRV", "start the SSDPSRV service")
			if !reflect.DeepEqual(tt.runner.calls, tt.calls) {
				t.Errorf("calls:\n got %q\nwant %q", tt.runner.calls, tt.calls)
			}
			if got.Outcome != tt.outcome {
				t.Errorf("outcome = %q, want %q (%s)", got.Outcome, tt.outcome, got.Message)
			}
			if tt.outcome == OutcomeFailed && !strings.Contains(got.Message, "exit status 1") {
				t.Errorf("message %q drops the command's error", got.Message)
			}
		})
	}
}

// TestWindowsNetworkProfileStep pins #604: the profile is read first, and a
// host with no Tailscale adapter is "manual", never "changed".
func TestWindowsNetworkProfileStep(t *testing.T) {
	read, set := psPrefix+psProfileRead, psPrefix+psProfile
	tests := []struct {
		name    string
		runner  *recordingRunner
		calls   []string
		outcome Outcome
	}{
		{"no tailscale adapter", &recordingRunner{}, []string{"output " + read}, OutcomeManual},
		{"already private", &recordingRunner{output: map[string]string{read: "category=Private\r\n"}}, []string{"output " + read}, OutcomeOK},
		{"public", &recordingRunner{output: map[string]string{read: "category=Public\r\n"}}, []string{"output " + read, "run " + set}, OutcomeChanged},
		{"one of two public", &recordingRunner{output: map[string]string{read: "category=Private\r\ncategory=Public"}}, []string{"output " + read, "run " + set}, OutcomeChanged},
		{"read fails", &recordingRunner{fail: map[string]bool{read: true}}, []string{"output " + read}, OutcomeFailed},
		{"set fails", &recordingRunner{output: map[string]string{read: "category=Public"}, fail: map[string]bool{set: true}}, []string{"output " + read, "run " + set}, OutcomeFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := networkProfileStep(tt.runner)
			if !reflect.DeepEqual(tt.runner.calls, tt.calls) {
				t.Errorf("calls:\n got %q\nwant %q", tt.runner.calls, tt.calls)
			}
			if got.Outcome != tt.outcome {
				t.Errorf("outcome = %q, want %q (%s)", got.Outcome, tt.outcome, got.Message)
			}
			if !got.Success && got.Fix == "" {
				t.Errorf("%s step carries no fix", got.Outcome)
			}
		})
	}
}

// powercfgQuery is `powercfg /query SCHEME_CURRENT SUB_SLEEP STANDBYIDLE`
// output, as read on a Windows 11 guest (labels localize, the order does not).
func powercfgQuery(ac, dc string) string {
	return "Power Scheme GUID: 381b4222-f694-41f0-9685-ff5bb260df2e  (Balanced)\r\n" +
		"  Subgroup GUID: 238c9fa8-0aad-41ed-83f4-97be242c8f20  (Sleep)\r\n" +
		"    Power Setting GUID: 29f6c1db-86da-48c5-9fdb-f2b67b1f44da  (Sleep after)\r\n" +
		"      Minimum Possible Setting: 0x00000000\r\n      Maximum Possible Setting: 0xffffffff\r\n" +
		"      Possible Settings increment: 0x00000001\r\n      Possible Settings units: Seconds\r\n" +
		"    Current AC Power Setting Index: " + ac + "\r\n    Current DC Power Setting Index: " + dc + "\r\n"
}

// TestWindowsPowerStep pins #603: powercfg takes one /change per call, and a
// timeout already 0 is not written again.
func TestWindowsPowerStep(t *testing.T) {
	const standby, hibernate, monitor = "powercfg /query SCHEME_CURRENT SUB_SLEEP STANDBYIDLE",
		"powercfg /query SCHEME_CURRENT SUB_SLEEP HIBERNATEIDLE", "powercfg /query SCHEME_CURRENT SUB_VIDEO VIDEOIDLE"
	zero, on := powercfgQuery("0x00000000", "0x00000384"), powercfgQuery("0x00000384", "0x00000384")
	dcOnly := powercfgQuery("0x00000384", "0x00000000")
	tests := []struct {
		name    string
		runner  *recordingRunner
		calls   []string
		outcome Outcome
	}{
		{"all zero", &recordingRunner{output: map[string]string{standby: zero, hibernate: zero, monitor: zero}},
			[]string{"output " + standby, "output " + hibernate, "output " + monitor}, OutcomeOK},
		{"standby on", &recordingRunner{output: map[string]string{standby: on, hibernate: zero, monitor: zero}},
			[]string{"output " + standby, "run powercfg /change standby-timeout-ac 0", "output " + hibernate, "output " + monitor}, OutcomeChanged},
		{"unreadable, so all set", &recordingRunner{}, powercfgCalls, OutcomeChanged},
		{"all on", &recordingRunner{output: map[string]string{standby: on, hibernate: on, monitor: on}}, powercfgCalls, OutcomeChanged},
		{"a DC-only zero is not AC zero", &recordingRunner{output: map[string]string{standby: dcOnly, hibernate: dcOnly, monitor: dcOnly}}, powercfgCalls, OutcomeChanged},
		{"change fails", &recordingRunner{fail: map[string]bool{"powercfg /change standby-timeout-ac 0": true}},
			[]string{"output " + standby, "run powercfg /change standby-timeout-ac 0"}, OutcomeFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := powerStep(tt.runner)
			if !reflect.DeepEqual(tt.runner.calls, tt.calls) {
				t.Errorf("calls:\n got %q\nwant %q", tt.runner.calls, tt.calls)
			}
			for _, c := range tt.runner.calls {
				if strings.Count(c, "/change") > 1 {
					t.Errorf("%q: powercfg takes one /change per call", c)
				}
			}
			if got.Outcome != tt.outcome {
				t.Errorf("outcome = %q, want %q (%s)", got.Outcome, tt.outcome, got.Message)
			}
		})
	}
}

// Windows RDP listens on the registry's port. A declared port that differs is
// reported manual by setup and check, naming both, so check exits non-zero
// (#557, AC1). An undeclared port takes the registry's: there is nothing to
// compare it with.
func TestWindowsDeclaredRDPPort(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want Outcome
	}{
		{"declared 3389, registry 3390", Config{FirewallRule: "Rule", Port: 3389, PortDeclared: true}, OutcomeManual},
		{"declared 3390, registry 3390", Config{FirewallRule: "Rule", Port: 3390, PortDeclared: true}, OutcomeOK},
		{"nothing declared, registry 3390", Config{FirewallRule: "Rule", Port: 3389}, OutcomeOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var res CheckResult
			checkWindows(tt.cfg, denyRunner("", "0", false), &res)
			setup := setupWindows(tt.cfg, denyRunner("", "0", false))
			for which, s := range map[string]SetupStep{"check": res.Steps[0], "setup": stepNamedIn(setup.Steps, "RDP configuration")} {
				if s.Outcome != tt.want {
					t.Errorf("%s: %s %q, want %s", which, s.Outcome, s.Message, tt.want)
				}
				if tt.want == OutcomeManual && (!strings.Contains(s.Message, "3389") || !strings.Contains(s.Message, "3390") || s.Fix == "") {
					t.Errorf("%s: %q (fix %q) must name both ports and a fix", which, s.Message, s.Fix)
				}
			}
		})
	}
}

// check reads the power timeouts setup converges, and never writes them (#652).
// A non-zero AC timeout is drift ("changed": setup would change it, ADR-017
// §2); an unreadable one is "failed", since check cannot tell.
func TestWindowsPowerCheckStep(t *testing.T) {
	const standby, hibernate, monitor = "powercfg /query SCHEME_CURRENT SUB_SLEEP STANDBYIDLE",
		"powercfg /query SCHEME_CURRENT SUB_SLEEP HIBERNATEIDLE", "powercfg /query SCHEME_CURRENT SUB_VIDEO VIDEOIDLE"
	zero, on := powercfgQuery("0x00000000", "0x00000384"), powercfgQuery("0x00000384", "0x00000384")
	tests := []struct {
		name    string
		output  map[string]string
		fail    map[string]bool
		outcome Outcome
		message string
	}{
		{"all zero", map[string]string{standby: zero, hibernate: zero, monitor: zero}, nil, OutcomeOK, ""},
		{"standby on", map[string]string{standby: on, hibernate: zero, monitor: zero}, nil, OutcomeChanged, "standby-timeout-ac"},
		{"standby and monitor on", map[string]string{standby: on, hibernate: zero, monitor: on}, nil, OutcomeChanged, "standby-timeout-ac, monitor-timeout-ac"},
		{"monitor unreadable", map[string]string{standby: zero, hibernate: zero}, nil, OutcomeFailed, "monitor-timeout-ac"},
		{"powercfg fails", map[string]string{standby: zero, monitor: zero}, map[string]bool{hibernate: true}, OutcomeFailed, "hibernate-timeout-ac"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recordingRunner{output: tt.output, fail: tt.fail}
			got := powerCheckStep(r)
			if got.Outcome != tt.outcome || !strings.Contains(got.Message, tt.message) || (tt.outcome != OutcomeOK && got.Fix == "") {
				t.Errorf("= %s %q (fix %q), want %s naming %q with a fix", got.Outcome, got.Message, got.Fix, tt.outcome, tt.message)
			}
			for _, c := range r.calls {
				if strings.HasPrefix(c, "run ") {
					t.Errorf("check ran %q", c)
				}
			}
		})
	}
	var res CheckResult
	checkWindows(Config{FirewallRule: "Rule"}, &recordingRunner{output: map[string]string{standby: on, hibernate: zero, monitor: zero}}, &res)
	if s := stepNamedIn(res.Steps, "Power settings"); s.Outcome != OutcomeChanged {
		t.Errorf("checkWindows power step = %+v, want changed", s)
	}
	res = CheckResult{}
	checkWindows(Config{FirewallRule: "Rule", NoSleep: true}, &recordingRunner{}, &res)
	if s := stepNamedIn(res.Steps, "Power settings"); s.Name != "" {
		t.Errorf("prevent_sleep false still checks power: %+v", s)
	}
}

func stepNamedIn(steps []SetupStep, name string) SetupStep {
	for _, s := range steps {
		if s.Name == name {
			return s
		}
	}
	return SetupStep{}
}

// TestWindowsFirewallRecreateScoped pins ADR-017 Amendment B: the recreate
// carries the tailnet RemoteAddress and no longer enables the built-in
// "Remote Desktop" group, whose rules allow any address.
func TestWindowsFirewallRecreateScoped(t *testing.T) {
	r := &recordingRunner{output: map[string]string{psPrefix + psFwCheck: "count=0\r\n"}}
	windowsFirewallSetupStep(r, "Rule", 3390)
	var recreate string
	for _, c := range r.calls {
		if strings.HasPrefix(c, "run ") {
			recreate = c
		}
	}
	if !strings.Contains(recreate, "-RemoteAddress 100.64.0.0/10,fd7a:115c:a1e0::/48") {
		t.Errorf("recreate %q lacks the tailnet RemoteAddress", recreate)
	}
	if strings.Contains(recreate, "Enable-NetFirewallRule") || strings.Contains(recreate, "Remote Desktop") {
		t.Errorf("recreate %q enables the built-in Remote Desktop group", recreate)
	}
}

// TestWindowsFirewallRecreateOrder pins the order of the recreate: the scoped
// rule is created first and the rules that were there are removed after, with
// errors terminating, so a failed create removes nothing and RDP is never left
// without an allow rule.
func TestWindowsFirewallRecreateOrder(t *testing.T) {
	r := &recordingRunner{output: map[string]string{psPrefix + psFwCheck: "count=0\r\n"}}
	windowsFirewallSetupStep(r, "Rule", 3390)
	var recreate string
	for _, c := range r.calls {
		if strings.HasPrefix(c, "run ") {
			recreate = c
		}
	}
	stop := strings.Index(recreate, "$ErrorActionPreference = 'Stop'")
	read := strings.Index(recreate, "$old = @(Get-NetFirewallRule")
	create := strings.Index(recreate, "New-NetFirewallRule")
	remove := strings.Index(recreate, "Remove-NetFirewallRule")
	if stop < 0 || stop > read || read > create || create > remove {
		t.Errorf("recreate %q: want errors terminating, then the old rules read, the new one created, the old removed", recreate)
	}
}

// TestWindowsBuiltinGroup pins the report folded in from #685: an enabled
// any-address rule in the built-in "Remote Desktop" group voids the tailnet
// scope, so a converged named rule is not enough. Setup reports it and leaves
// the group alone; check says the same and firewall_ok is false.
func TestWindowsBuiltinGroup(t *testing.T) {
	good := "count=1\r\n" + fwRule("True", "Allow", "Inbound", "3390")
	tests := []struct {
		name      string
		rule      string
		group     string
		groupFail bool
		want      Outcome
		ok        bool
	}{
		{name: "none enabled", rule: good, group: "exposed=0\r\n", want: OutcomeOK, ok: true},
		{name: "unreadable output counts as none", rule: good, group: "", want: OutcomeOK, ok: true},
		{name: "two enabled", rule: good, group: "exposed=2\r\n", want: OutcomeManual},
		{name: "read fails", rule: good, groupFail: true, want: OutcomeUnverified},
		{name: "named rule drifted, group not read", rule: "count=0\r\n", group: "exposed=2\r\n", want: OutcomeChanged},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newRunner := func() *recordingRunner {
				return &recordingRunner{
					output: map[string]string{psPrefix + psFwCheck: tt.rule, psPrefix + psReadRDGroup: tt.group},
					fail:   map[string]bool{psPrefix + psReadRDGroup: tt.groupFail},
				}
			}
			step, ok := windowsFirewallCheckStep(newRunner(), "Rule", 3390)
			if step.Outcome != tt.want || ok != tt.ok {
				t.Errorf("check: outcome %q ok %v, want %q ok %v (%s)", step.Outcome, ok, tt.want, tt.ok, step.Message)
			}
			if tt.want == OutcomeManual && !strings.Contains(step.Fix, `Disable-NetFirewallRule -DisplayGroup "Remote Desktop"`) {
				t.Errorf("check: fix %q lacks the disable command", step.Fix)
			}
			r := newRunner()
			step = windowsFirewallSetupStep(r, "Rule", 3390)
			wantSetup := tt.want
			if tt.name == "named rule drifted, group not read" {
				wantSetup = OutcomeManual // the recreate converges, then the group is read
			}
			if step.Outcome != wantSetup {
				t.Errorf("setup: outcome %q, want %q (%s)", step.Outcome, wantSetup, step.Message)
			}
			for _, c := range r.calls {
				if strings.HasPrefix(c, "run ") && strings.Contains(c, "Disable-NetFirewallRule") {
					t.Errorf("setup ran %q; it must not disable the built-in group", c)
				}
			}
		})
	}
}
