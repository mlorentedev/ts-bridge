package host

import (
	"fmt"
	"strings"
	"testing"
)

var (
	winSSHCapPS   = psPrefix + psReadSSHCapability
	winSSHPortsPS = psPrefix + psReadSSHPorts
	// winSSHMSIPortsPS is the port read when `sc qc sshd` names winMSIBin.
	winSSHMSIPortsPS = psPrefix + sshPortsScript(winMSIBin)
)

const (
	winSSHQC       = "sc qc sshd"
	winSSHQuery    = "sc query sshd"
	winAutoStart   = "START_TYPE         : 2   AUTO_START"
	winRunning     = "STATE              : 4  RUNNING"
	winSSHRuleName = "Tailscale-SSH-Ingress"
	winInstaller   = "OpenSSH-Server-In-TCP"
	winMSIBin      = `C:\Program Files\OpenSSH\sshd.exe`
)

func winSSHFwPS() string { return psPrefix + fmt.Sprintf(psReadFirewallRule, winSSHRuleName) }
func winSSHInstallerPS(port int) string {
	return psPrefix + fmt.Sprintf(psReadSSHInstallerRule, port)
}

// winSSHHost is a converged Windows host with the OpenSSH Server installed,
// sshd Automatic and Running, its tailnet-scoped rule in place and the
// installer's rule absent. over replaces answers, keyed like the runner's map.
func winSSHHost(port int, over map[string]string) *recordingRunner {
	out := map[string]string{
		winSSHCapPS:             "state=Installed\r\n",
		winSSHQC:                winAutoStart,
		winSSHQuery:             winRunning,
		winSSHFwPS():            "count=1\r\n" + fwRule("True", "Allow", "Inbound", fmt.Sprint(port)),
		winSSHInstallerPS(port): "total=0\r\nexposed=0\r\n",
		winSSHPortsPS:           fmt.Sprintf("port %d\r\nlistenaddress 0.0.0.0:%d\r\n", port, port),
	}
	for k, v := range over {
		out[k] = v
	}
	return &recordingRunner{output: out}
}

// mutating lists the commands a runner ran that change the host.
func mutating(r *recordingRunner) []string {
	var cmds []string
	for _, c := range r.calls {
		if strings.HasPrefix(c, "run ") || strings.HasPrefix(c, "writefile ") {
			cmds = append(cmds, c)
		}
	}
	return cmds
}

func outcomes(steps []SetupStep) []string {
	got := make([]string, 0, len(steps))
	for _, s := range steps {
		got = append(got, s.Name+"="+string(s.Outcome))
	}
	return got
}

func sameList(got, want []string) bool { return strings.Join(got, "|") == strings.Join(want, "|") }

func sawCall(r *recordingRunner, sub string) bool {
	for _, c := range r.calls {
		if strings.Contains(c, sub) {
			return true
		}
	}
	return false
}

// TestWindowsSSHSetup pins the setup steps for an ssh service: the capability
// is read and never installed, sshd is converged only on a difference, the
// firewall rule is read before it is written, and the installer's own rule is
// reported, never touched.
func TestWindowsSSHSetup(t *testing.T) {
	const cap, svc, fw = sshWinCapName, sshName, sshWinFwName
	tests := []struct {
		name   string
		port   int
		over   map[string]string
		fail   []string
		steps  []string
		ranHas []string // substrings, one per mutating command, in order
	}{
		{"converged", 22, nil, nil, []string{cap + "=ok", svc + "=ok", fw + "=ok"}, nil},
		{"sshd manual and stopped", 22, map[string]string{winSSHQC: "3   DEMAND_START", winSSHQuery: "1  STOPPED"}, nil,
			[]string{cap + "=ok", svc + "=changed", fw + "=ok"}, []string{"sc config sshd start=auto", "net start sshd"}},
		{"sshd automatic and stopped", 22, map[string]string{winSSHQuery: "1  STOPPED"}, nil,
			[]string{cap + "=ok", svc + "=changed", fw + "=ok"}, []string{"net start sshd"}},
		{"capability missing and no sshd service: manual, no sshd step, rule still written", 22, map[string]string{winSSHCapPS: "state=NotPresent\r\n", winSSHFwPS(): "count=0\r\n"}, []string{winSSHQC},
			[]string{cap + "=manual", fw + "=changed"}, []string{"New-NetFirewallRule"}},
		{"installed by the MSI (NotPresent, service exists): converged", 22, map[string]string{winSSHCapPS: "state=NotPresent\r\n"}, nil,
			[]string{cap + "=ok", svc + "=ok", fw + "=ok"}, nil},
		{"installed by the MSI and stopped: the service is started", 22, map[string]string{winSSHCapPS: "state=NotPresent\r\n", winSSHQuery: "1  STOPPED"}, nil,
			[]string{cap + "=ok", svc + "=changed", fw + "=ok"}, []string{"net start sshd"}},
		{"installed by the MSI, 2222: sshd -T runs from the service's own path", 2222, map[string]string{winSSHCapPS: "state=NotPresent\r\n",
			winSSHQC: winAutoStart + "\r\n        BINARY_PATH_NAME   : \"" + winMSIBin + "\" -f x\r\n", winSSHMSIPortsPS: "port 2222\r\n"}, nil,
			[]string{cap + "=ok", svc + "=ok", sshPortName + "=ok", fw + "=ok"}, nil},
		{"capability unreadable: failed, sshd not touched", 22, nil, []string{winSSHCapPS},
			[]string{cap + "=failed", fw + "=ok"}, nil},
		{"capability in a state ts-bridge does not interpret", 22, map[string]string{winSSHCapPS: "state=Staged\r\n"}, nil,
			[]string{cap + "=failed", fw + "=ok"}, nil},
		{"capability answer empty (no such capability)", 22, map[string]string{winSSHCapPS: "state=\r\n"}, nil,
			[]string{cap + "=failed", fw + "=ok"}, nil},
		{"rule absent: created scoped", 22, map[string]string{winSSHFwPS(): "count=0\r\n"}, nil,
			[]string{cap + "=ok", svc + "=ok", fw + "=changed"}, []string{"New-NetFirewallRule -DisplayName \"" + winSSHRuleName + "\""}},
		{"installer rule open to any address: manual, never disabled", 22, map[string]string{winSSHInstallerPS(22): "total=1\r\nexposed=1\r\n"}, nil,
			[]string{cap + "=ok", svc + "=ok", fw + "=manual"}, nil},
		{"declared 2222 and sshd listens there", 2222, nil, nil,
			[]string{cap + "=ok", svc + "=ok", sshPortName + "=ok", fw + "=ok"}, nil},
		{"declared 2222 and sshd listens on 22", 2222, map[string]string{winSSHPortsPS: "port 22\r\n"}, nil,
			[]string{cap + "=ok", svc + "=ok", sshPortName + "=manual", fw + "=ok"}, nil},
		{"declared 2222 and sshd -T unreadable", 2222, nil, []string{winSSHPortsPS},
			[]string{cap + "=ok", svc + "=ok", sshPortName + "=failed", fw + "=ok"}, nil},
		{"declared 2222, capability missing and no service: no port step", 2222, map[string]string{winSSHCapPS: "state=NotPresent\r\n"}, []string{winSSHQC},
			[]string{cap + "=manual", fw + "=ok"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := winSSHHost(tt.port, tt.over)
			r.fail = map[string]bool{}
			for _, k := range tt.fail {
				r.fail[k] = true
			}
			steps := sshWindowsSetupSteps(r, tt.port)
			if got := outcomes(steps); !sameList(got, tt.steps) {
				t.Errorf("steps:\n got %q\nwant %q", got, tt.steps)
			}
			mut := mutating(r)
			if len(mut) != len(tt.ranHas) {
				t.Fatalf("mutating commands:\n got %q\nwant containing %q", mut, tt.ranHas)
			}
			for i, sub := range tt.ranHas {
				if !strings.Contains(mut[i], sub) {
					t.Errorf("command %d = %q, want it to contain %q", i, mut[i], sub)
				}
			}
			for _, c := range mut {
				if strings.Contains(c, winInstaller) || strings.Contains(c, "Add-WindowsCapability") || strings.Contains(c, "sshd_config") {
					t.Errorf("setup ran %q: it must not touch the installer's rule, install the capability or write sshd_config", c)
				}
			}
		})
	}
}

// TestWindowsSSHSetupMissingCapabilityNamesTheInstall pins the manual fix: the
// capability name, and that a managed host needs a Features-on-Demand source.
func TestWindowsSSHSetupMissingCapabilityNamesTheInstall(t *testing.T) {
	r := winSSHHost(22, map[string]string{winSSHCapPS: "state=NotPresent\r\n"})
	r.fail = map[string]bool{winSSHQC: true}
	step := sshWindowsSetupSteps(r, 22)[0]
	for _, want := range []string{"Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0", "WSUS", "does not install"} {
		if !strings.Contains(step.Fix, want) {
			t.Errorf("fix %q does not contain %q", step.Fix, want)
		}
	}
	if sawCall(r, "sc query") || sawCall(r, "sc config") || sawCall(r, "net start") {
		t.Errorf("a missing server converged sshd: %q", r.calls)
	}
}

// TestWindowsSSHCheck pins the read half: no mutating command, drift is
// "changed", an answer that cannot be read is "unverified", and services[]
// carries enabled (Automatic), active (Running) and unread.
func TestWindowsSSHCheck(t *testing.T) {
	const cap, svc, fw = sshWinCapName, sshName, sshWinFwName
	tests := []struct {
		name  string
		port  int
		over  map[string]string
		fail  []string
		steps []string
		want  Service
	}{
		{"converged", 22, nil, nil, []string{cap + "=ok", svc + "=ok", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 22, Enabled: true, Active: boolPtr(true), FirewallOK: true}},
		{"stopped", 22, map[string]string{winSSHQuery: "1  STOPPED"}, nil, []string{cap + "=ok", svc + "=changed", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 22, Enabled: true, Active: boolPtr(false), FirewallOK: true}},
		{"manual start", 22, map[string]string{winSSHQC: "3   DEMAND_START"}, nil, []string{cap + "=ok", svc + "=changed", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 22, Enabled: false, Active: boolPtr(true), FirewallOK: true}},
		{"sc cannot be read", 22, nil, []string{winSSHQC, winSSHQuery}, []string{cap + "=ok", svc + "=unverified", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 22, FirewallOK: true, Unread: true}},
		{"capability missing and no sshd service", 22, map[string]string{winSSHCapPS: "state=NotPresent\r\n"}, []string{winSSHQC}, []string{cap + "=manual", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 22, FirewallOK: true}},
		{"installed by the MSI (NotPresent, service exists)", 22, map[string]string{winSSHCapPS: "state=NotPresent\r\n"}, nil, []string{cap + "=ok", svc + "=ok", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 22, Enabled: true, Active: boolPtr(true), FirewallOK: true}},
		{"installed by the MSI and stopped", 22, map[string]string{winSSHCapPS: "state=NotPresent\r\n", winSSHQuery: "1  STOPPED"}, nil, []string{cap + "=ok", svc + "=changed", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 22, Enabled: true, Active: boolPtr(false), FirewallOK: true}},
		{"installed by the MSI, 2222 on its own path, listening on 22", 2222, map[string]string{winSSHCapPS: "state=NotPresent\r\n",
			winSSHQC: winAutoStart + "\r\n        BINARY_PATH_NAME   : " + winMSIBin + "\r\n", winSSHMSIPortsPS: "port 22\r\n"}, nil,
			[]string{cap + "=ok", svc + "=ok", sshPortName + "=manual", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 2222, Enabled: true, Active: boolPtr(true), FirewallOK: true}},
		{"capability unreadable (not elevated)", 22, nil, []string{winSSHCapPS}, []string{cap + "=unverified", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 22, FirewallOK: true, Unread: true}},
		{"rule absent", 22, map[string]string{winSSHFwPS(): "count=0\r\n"}, nil, []string{cap + "=ok", svc + "=ok", fw + "=changed"},
			Service{Kind: ServiceSSH, Port: 22, Enabled: true, Active: boolPtr(true)}},
		{"installer rule open to any address", 22, map[string]string{winSSHInstallerPS(22): "total=1\r\nexposed=1\r\n"}, nil,
			[]string{cap + "=ok", svc + "=ok", fw + "=manual"},
			Service{Kind: ServiceSSH, Port: 22, Enabled: true, Active: boolPtr(true)}},
		{"installer rule read fails", 22, nil, []string{winSSHInstallerPS(22)}, []string{cap + "=ok", svc + "=ok", fw + "=unverified"},
			Service{Kind: ServiceSSH, Port: 22, Enabled: true, Active: boolPtr(true)}},
		{"2222 matches", 2222, nil, nil, []string{cap + "=ok", svc + "=ok", sshPortName + "=ok", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 2222, Enabled: true, Active: boolPtr(true), FirewallOK: true}},
		{"2222 but sshd listens on 22", 2222, map[string]string{winSSHPortsPS: "port 22\r\n"}, nil,
			[]string{cap + "=ok", svc + "=ok", sshPortName + "=manual", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 2222, Enabled: true, Active: boolPtr(true), FirewallOK: true}},
		{"2222 and sshd -T unreadable: its own step, the service stays read", 2222, nil, []string{winSSHPortsPS},
			[]string{cap + "=ok", svc + "=ok", sshPortName + "=unverified", fw + "=ok"},
			Service{Kind: ServiceSSH, Port: 2222, Enabled: true, Active: boolPtr(true), FirewallOK: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := winSSHHost(tt.port, tt.over)
			r.fail = map[string]bool{}
			for _, k := range tt.fail {
				r.fail[k] = true
			}
			var res CheckResult
			sshWindowsCheck(r, tt.port, &res)
			if got := outcomes(res.Steps); !sameList(got, tt.steps) {
				t.Errorf("steps:\n got %q\nwant %q", got, tt.steps)
			}
			if mut := mutating(r); len(mut) != 0 {
				t.Errorf("check ran mutating commands: %q", mut)
			}
			if len(res.Services) != 1 {
				t.Fatalf("services = %+v, want one", res.Services)
			}
			got, want := res.Services[0], tt.want
			if got.Kind != want.Kind || got.Port != want.Port || got.Enabled != want.Enabled || got.FirewallOK != want.FirewallOK || got.Unread != want.Unread ||
				(got.Active == nil) != (want.Active == nil) || (got.Active != nil && *got.Active != *want.Active) {
				t.Errorf("service = %+v (active %v), want %+v (active %v)", got, got.Active, want, want.Active)
			}
		})
	}
}

// TestWindowsServiceReadState pins the read serviceStep and the check share:
// the two sc answers, and an error only when a read fails, so "stopped" and
// "cannot tell" differ.
func TestWindowsServiceReadState(t *testing.T) {
	tests := []struct {
		name          string
		r             *recordingRunner
		auto, running bool
		failed        bool
	}{
		{"auto and running", &recordingRunner{output: map[string]string{winSSHQC: winAutoStart, winSSHQuery: winRunning}}, true, true, false},
		{"delayed auto", &recordingRunner{output: map[string]string{winSSHQC: winAutoStart + " (DELAYED)", winSSHQuery: winRunning}}, true, true, false},
		{"demand and stopped", &recordingRunner{output: map[string]string{winSSHQC: "3 DEMAND_START", winSSHQuery: "1 STOPPED"}}, false, false, false},
		{"qc fails", &recordingRunner{fail: map[string]bool{winSSHQC: true}, output: map[string]string{winSSHQuery: winRunning}}, false, true, true},
		{"query fails", &recordingRunner{fail: map[string]bool{winSSHQuery: true}, output: map[string]string{winSSHQC: winAutoStart}}, true, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auto, running, err := readServiceState(tt.r, "sshd")
			if auto != tt.auto || running != tt.running || (err != nil) != tt.failed {
				t.Errorf("readServiceState = %v, %v, %v; want %v, %v, failed=%v", auto, running, err, tt.auto, tt.running, tt.failed)
			}
			if !sameList(tt.r.calls, []string{"output " + winSSHQC, "output " + winSSHQuery}) {
				t.Errorf("calls = %q, want both reads, whatever the first answered", tt.r.calls)
			}
		})
	}
}

// TestWindowsSSHInstallerRule pins the guard on OpenSSH-Server-In-TCP: an
// enabled inbound allow from any address on the declared port voids the
// tailnet scope and is "manual"; no rule at all is a normal host, unlike the
// Remote Desktop group, where an empty answer means the query missed it.
func TestWindowsSSHInstallerRule(t *testing.T) {
	tests := []struct {
		name    string
		read    string
		fails   bool
		outcome Outcome
		ok      bool
	}{
		{"rule absent", "total=0\r\nexposed=0\r\n", false, OutcomeOK, true},
		{"rule present, scoped or off", "total=1\r\nexposed=0\r\n", false, OutcomeOK, true},
		{"rule enabled for any address", "total=1\r\nexposed=1\r\n", false, OutcomeManual, false},
		{"read fails", "", true, OutcomeUnverified, false},
		{"answer without a count", "garbage", false, OutcomeUnverified, false},
		{"exposed count missing", "total=1\r\n", false, OutcomeUnverified, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recordingRunner{output: map[string]string{winSSHInstallerPS(22): tt.read}, fail: map[string]bool{winSSHInstallerPS(22): tt.fails}}
			step, ok := sshInstallerRuleStep(22)(r, newStep("Firewall rule (ssh)", OutcomeOK, "converged", ""))
			if step.Outcome != tt.outcome || ok != tt.ok {
				t.Fatalf("outcome %q ok %v (%s), want %q %v", step.Outcome, ok, step.Message, tt.outcome, tt.ok)
			}
			if tt.outcome == OutcomeManual && step.Fix != "Disable-NetFirewallRule -Name "+winInstaller {
				t.Errorf("fix = %q, want the operator's Disable-NetFirewallRule -Name %s", step.Fix, winInstaller)
			}
			if mut := mutating(r); len(mut) != 0 {
				t.Errorf("guard ran %q: it reads only", mut)
			}
		})
	}
}

// TestWindowsSSHReadsUseStableNames pins the queries themselves: the Go tests
// feed canned output, so without this a switch to the localized display name
// (as in #707) would only fail on a Windows host.
func TestWindowsSSHReadsUseStableNames(t *testing.T) {
	rule := fmt.Sprintf(psReadSSHInstallerRule, 22)
	if !strings.Contains(rule, "-Name '"+winInstaller+"'") || strings.Contains(rule, "DisplayName") || strings.Contains(rule, "DisplayGroup") {
		t.Errorf("the installer rule is not selected by its Name: %s", rule)
	}
	if !strings.Contains(rule, "total=") || !strings.Contains(rule, "exposed=") {
		t.Errorf("the installer rule read does not print total= and exposed=: %s", rule)
	}
	for _, want := range []string{"Get-WindowsCapability -Online -Name 'OpenSSH.Server*'", "state="} {
		if !strings.Contains(psReadSSHCapability, want) {
			t.Errorf("capability read lacks %q: %s", want, psReadSSHCapability)
		}
	}
	if !strings.Contains(psReadSSHPorts, `$env:WINDIR\System32\OpenSSH\sshd.exe" -T`) {
		t.Errorf("port read does not run the installed sshd -T: %s", psReadSSHPorts)
	}
}

// ssh-only and rdp+ssh hosts through the entry points, so the legacy keys are
// derived as they are for a real run.
func TestWindowsSSHOnlyHost(t *testing.T) {
	cfg := Config{FirewallRule: "Rule", NoSleep: true, Services: []DeclaredService{{Kind: ServiceSSH, Port: 22}}}
	r := winSSHHost(22, nil)

	check := checkFor(goosWindows, cfg, r)
	for _, s := range check.Steps {
		if s.Name == "RDP configuration" || s.Name == "Firewall rule" {
			t.Errorf("an ssh-only host ran the RDP step %q", s.Name)
		}
	}
	if len(check.Services) != 1 || check.Services[0].Kind != ServiceSSH || !check.FirewallOK || check.RDPPort != 0 || check.RDPEnabled {
		t.Errorf("check: services %+v firewall_ok %v rdp_port %d rdp_enabled %v; want ssh alone, firewall ok, no RDP keys", check.Services, check.FirewallOK, check.RDPPort, check.RDPEnabled)
	}

	setup := setupFor(goosWindows, cfg, winSSHHost(22, nil))
	if setup.RDPPort != 0 {
		t.Errorf("setup RDPPort = %d, want 0 for an ssh-only host", setup.RDPPort)
	}
	names := strings.Join(outcomes(setup.Steps), "|")
	for _, bad := range []string{"RDP configuration", "Firewall rule="} {
		if strings.Contains(names, bad) {
			t.Errorf("an ssh-only host ran %q: %s", bad, names)
		}
	}
	for _, want := range []string{"Tailscale service", sshWinCapName, sshWinFwName} {
		if !strings.Contains(names, want) {
			t.Errorf("steps lack %q: %s", want, names)
		}
	}
}

func TestWindowsRDPBesideSSH(t *testing.T) {
	cfg := Config{FirewallRule: "Rule", NoSleep: true, Port: 3390, Services: []DeclaredService{{Kind: ServiceRDP, Port: 3390}, {Kind: ServiceSSH, Port: 22}}}
	r := winSSHHost(22, map[string]string{
		psPrefix + psPort: "3390", psPrefix + psDeny: "policy=\r\nlocal=0", psPrefix + psFwCheck: "count=1\r\n" + fwRule("True", "Allow", "Inbound", "3390"),
		psPrefix + psReadRDGroup: groupClean,
	})
	check := checkFor(goosWindows, cfg, r)
	if len(check.Services) != 2 || check.Services[0].Kind != ServiceRDP || check.Services[1].Kind != ServiceSSH {
		t.Fatalf("services = %+v, want rdp then ssh", check.Services)
	}
	if !check.RDPEnabled || check.RDPPort != 3390 || !check.FirewallOK {
		t.Errorf("legacy keys: rdp_port %d rdp_enabled %v firewall_ok %v; want them from rdp alone", check.RDPPort, check.RDPEnabled, check.FirewallOK)
	}

	check = checkFor(goosWindows, cfg, winSSHHost(22, map[string]string{
		psPrefix + psPort: "3390", psPrefix + psDeny: "policy=\r\nlocal=0", psPrefix + psFwCheck: "count=1\r\n" + fwRule("True", "Allow", "Inbound", "3390"),
		psPrefix + psReadRDGroup: groupClean, winSSHInstallerPS(22): "total=1\r\nexposed=1\r\n",
	}))
	if check.FirewallOK || !check.RDPEnabled || check.RDPPort != 3390 {
		t.Errorf("an exposed ssh rule: firewall_ok %v rdp_port %d rdp_enabled %v; want firewall_ok false and the RDP keys unchanged", check.FirewallOK, check.RDPPort, check.RDPEnabled)
	}
}

// A declared kind the Windows steps do not know fails, never converges
// nothing and reads ready (the Linux guard, #768).
func TestWindowsUnknownKindFails(t *testing.T) {
	cfg := Config{FirewallRule: "Rule", NoSleep: true, Services: []DeclaredService{{Kind: ServiceVNC, Port: 5900}}}
	check := checkFor(goosWindows, cfg, &recordingRunner{})
	var failed bool
	for _, s := range check.Steps {
		failed = failed || (strings.Contains(s.Name, ServiceVNC) && s.Outcome == OutcomeFailed)
	}
	if !failed || len(check.Services) != 1 || !check.Services[0].Unread {
		t.Errorf("check: steps %q services %+v, want a failed vnc step and an unread service", outcomes(check.Steps), check.Services)
	}
	setup := setupFor(goosWindows, cfg, &recordingRunner{})
	if names := strings.Join(outcomes(setup.Steps), "|"); !strings.Contains(names, "vnc service=failed") {
		t.Errorf("setup steps = %s, want a failed vnc step", names)
	}
}

// TestWindowsServiceBinary pins the BINARY_PATH_NAME parse: quoted with
// arguments, unquoted, and no line at all (the default path stands in).
func TestWindowsServiceBinary(t *testing.T) {
	tests := []struct{ name, qc, want string }{
		{"quoted with arguments", "SERVICE_NAME: sshd\r\n        BINARY_PATH_NAME   : \"C:\\Program Files\\OpenSSH\\sshd.exe\" -f x\r\n", `C:\Program Files\OpenSSH\sshd.exe`},
		{"unquoted", "        BINARY_PATH_NAME   : C:\\Windows\\System32\\OpenSSH\\sshd.exe\r\n", `C:\Windows\System32\OpenSSH\sshd.exe`},
		{"unquoted with arguments", "BINARY_PATH_NAME   : C:\\OpenSSH\\SSHD.EXE -D\r\n", `C:\OpenSSH\SSHD.EXE`},
		{"no such line", "START_TYPE         : 2   AUTO_START\r\n", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serviceBinary(tt.qc); got != tt.want {
				t.Errorf("serviceBinary = %q, want %q", got, tt.want)
			}
		})
	}
	if got := sshPortsScript(""); got != psReadSSHPorts {
		t.Errorf("no service path must read the capability's sshd.exe, got %q", got)
	}
	if got := sshPortsScript(`C:\a'b\sshd.exe`); !strings.Contains(got, `'C:\a''b\sshd.exe'`) {
		t.Errorf("a quote in the path is not escaped: %q", got)
	}
}
