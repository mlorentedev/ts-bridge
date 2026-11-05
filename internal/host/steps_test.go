package host

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// recordingRunner records every command a step list issues, in order, and
// answers from canned results. The golden sequences below are copied verbatim
// from the pre-HOST-004 build-tagged implementations: PR A ports the steps
// without changing a single command (specs/HOST-004-host-step-model).
type recordingRunner struct {
	calls  []string
	fail   map[string]bool   // joined command -> Run/Output returns an error
	output map[string]string // joined command -> Output stdout
	onPath map[string]bool   // LookPath hits
	exit   map[string]int    // joined command -> Run/Output fails with this exit status
	// elevated is what Elevated reports; it records no call.
	elevated bool
}

// exitError carries an exit status the way *exec.ExitError does.
type exitError int

func (e exitError) Error() string { return "exit status " + strconv.Itoa(int(e)) }
func (e exitError) ExitCode() int { return int(e) }

func (r *recordingRunner) Elevated() bool { return r.elevated }

func (r *recordingRunner) record(kind, name string, args []string) string {
	cmd := strings.Join(append([]string{name}, args...), " ")
	r.calls = append(r.calls, kind+" "+cmd)
	return cmd
}

func (r *recordingRunner) Run(name string, args ...string) error {
	cmd := r.record("run", name, args)
	if code, ok := r.exit[cmd]; ok {
		return exitError(code)
	}
	if r.fail[cmd] {
		return errors.New("exit status 1")
	}
	return nil
}

func (r *recordingRunner) Output(name string, args ...string) ([]byte, error) {
	cmd := r.record("output", name, args)
	if code, ok := r.exit[cmd]; ok {
		return []byte(r.output[cmd]), exitError(code) // stdout survives a non-zero exit, as with exec
	}
	if r.fail[cmd] {
		return nil, errors.New("exit status 1")
	}
	return []byte(r.output[cmd]), nil
}

func (r *recordingRunner) LookPath(name string) (string, error) {
	r.calls = append(r.calls, "lookpath "+name)
	if r.onPath[name] {
		return "/usr/bin/" + name, nil
	}
	return "", errors.New("not found")
}

const (
	psPrefix      = "powershell -NoProfile -ExecutionPolicy Bypass -Command "
	psProfile     = `Get-NetConnectionProfile | Where-Object { $_.InterfaceAlias -match "Tailscale" -or $_.Name -match "Tailscale" } | ForEach-Object { $_ | Set-NetConnectionProfile -NetworkCategory Private }`
	psEnable      = `Set-ItemProperty -Path 'HKLM:\System\CurrentControlSet\Control\Terminal Server' -Name "fDenyTSConnections" -Value 0; $RdpPort = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp' -ErrorAction SilentlyContinue).PortNumber; if (-not $RdpPort) { $RdpPort = 3389 }; Write-Output $RdpPort`
	psPort        = `$RdpPort = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp' -ErrorAction SilentlyContinue).PortNumber; if (-not $RdpPort) { $RdpPort = 3389 }; Write-Output $RdpPort`
	psFw          = `Remove-NetFirewallRule -DisplayName "Rule" -ErrorAction SilentlyContinue; New-NetFirewallRule -DisplayName "Rule" -Direction Inbound -Action Allow -Protocol TCP -LocalPort 3390 -Profile Any -Enabled True | Out-Null; Enable-NetFirewallRule -DisplayGroup "Remote Desktop" -ErrorAction SilentlyContinue`
	psFwCheck     = `$r = @(Get-NetFirewallRule -DisplayName "Rule" -ErrorAction SilentlyContinue); Write-Output "count=$($r.Count)"; foreach ($x in $r) { Write-Output "enabled=$($x.Enabled)"; Write-Output "action=$($x.Action)"; Write-Output "direction=$($x.Direction)"; Write-Output "profile=$($x.Profile)"; Write-Output "remote=$(($x | Get-NetFirewallAddressFilter).RemoteAddress)"; $f = $x | Get-NetFirewallPortFilter; Write-Output "protocol=$($f.Protocol)"; Write-Output "port=$($f.LocalPort)" }`
	psProfileRead = `Get-NetConnectionProfile | Where-Object { $_.InterfaceAlias -match "Tailscale" -or $_.Name -match "Tailscale" } | ForEach-Object { Write-Output "category=$($_.NetworkCategory)" }`
)

// powercfgCalls is the power step on a host whose AC timeouts are not 0:
// one read and one /change per setting (#603).
var powercfgCalls = []string{
	"output powercfg /query SCHEME_CURRENT SUB_SLEEP STANDBYIDLE", "run powercfg /change standby-timeout-ac 0",
	"output powercfg /query SCHEME_CURRENT SUB_SLEEP HIBERNATEIDLE", "run powercfg /change hibernate-timeout-ac 0",
	"output powercfg /query SCHEME_CURRENT SUB_VIDEO VIDEOIDLE", "run powercfg /change monitor-timeout-ac 0",
}

// serviceCalls is a service step on a host where the service is not set to
// start automatically: read both, then configure and start what is missing.
func serviceCalls(name string, running bool) []string {
	calls := []string{"output sc qc " + name, "output sc query " + name, "run sc config " + name + " start=auto"}
	if !running {
		calls = append(calls, "run net start "+name)
	}
	return calls
}

func windowsSetupCalls(noSleep bool) []string {
	calls := []string{"output tailscale debug prefs", "run tailscale set --unattended"}
	calls = append(calls, serviceCalls("Tailscale", true)...)
	calls = append(calls, serviceCalls("SSDPSRV", false)...)
	calls = append(calls, serviceCalls("upnphost", false)...)
	calls = append(calls, "output "+psPrefix+psProfileRead, "output "+psPrefix+psDeny, "output "+psPrefix+psEnable, "output "+psPrefix+psFwCheck, "run "+psPrefix+psFw)
	if !noSleep {
		calls = append(calls, powercfgCalls...)
	}
	return calls
}

func TestSetupFor(t *testing.T) {
	cfg := Config{Port: 3389, FirewallRule: "Rule"}
	tests := []struct {
		name     string
		goos     string
		cfg      Config
		runner   *recordingRunner
		calls    []string
		port     int
		outcomes []Outcome
	}{
		{
			name: "windows", goos: "windows", cfg: Config{FirewallRule: "Rule"},
			runner: &recordingRunner{output: map[string]string{"sc query Tailscale": "STATE : 4 RUNNING", psPrefix + psEnable: "3390\r\n"}},
			calls:  windowsSetupCalls(false), port: 3390,
			outcomes: []Outcome{OutcomeChanged, OutcomeChanged, OutcomeChanged, OutcomeChanged, OutcomeManual, OutcomeChanged, OutcomeChanged, OutcomeChanged},
		},
		{
			name: "windows --no-sleep", goos: "windows", cfg: Config{FirewallRule: "Rule", NoSleep: true},
			runner: &recordingRunner{output: map[string]string{"sc query Tailscale": "RUNNING", psPrefix + psEnable: "3390"}},
			calls:  windowsSetupCalls(true), port: 3390,
			outcomes: []Outcome{OutcomeChanged, OutcomeChanged, OutcomeChanged, OutcomeChanged, OutcomeManual, OutcomeChanged, OutcomeChanged},
		},
		{name: "darwin stub", goos: "darwin", cfg: cfg, runner: &recordingRunner{}, port: 3389},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setupFor(tt.goos, tt.cfg, tt.runner)
			if !reflect.DeepEqual(tt.runner.calls, tt.calls) {
				t.Errorf("calls:\n got %q\nwant %q", tt.runner.calls, tt.calls)
			}
			if got.RDPPort != tt.port {
				t.Errorf("RDPPort = %d, want %d", got.RDPPort, tt.port)
			}
			var outcomes []Outcome
			for _, s := range got.Steps {
				outcomes = append(outcomes, s.Outcome)
				if s.Success != s.Outcome.Succeeded() {
					t.Errorf("step %q: Success = %v but outcome %q", s.Name, s.Success, s.Outcome)
				}
				if !s.Success && s.Fix == "" {
					t.Errorf("step %q failed without a fix", s.Name)
				}
			}
			if !reflect.DeepEqual(outcomes, tt.outcomes) {
				t.Errorf("outcomes = %v, want %v", outcomes, tt.outcomes)
			}
		})
	}
}

func TestSetupForWindowsRDPFailureFallsBackToDefaultPort(t *testing.T) {
	r := &recordingRunner{fail: map[string]bool{psPrefix + psEnable: true}, output: map[string]string{"sc query Tailscale": "RUNNING"}}
	got := setupFor("windows", Config{FirewallRule: "Rule", NoSleep: true}, r)
	if got.RDPPort != 3389 {
		t.Fatalf("RDPPort = %d, want 3389", got.RDPPort)
	}
	want := "run " + psPrefix + strings.Replace(psFw, "3390", "3389", 1)
	if last := r.calls[len(r.calls)-1]; last != want {
		t.Errorf("firewall call = %q, want %q", last, want)
	}
}

// TestCheckFor pins check as the read half of the setup steps (AC3): the
// recorded argv is today's read sequence, no call is a Run (the runner's only
// mutating verb), each step reports an outcome, and the legacy keys are derived
// from services (ADR-017 §3).
func TestCheckFor(t *testing.T) {
	active := (*bool)(nil)
	tests := []struct {
		name     string
		goos     string
		cfg      Config
		runner   *recordingRunner
		calls    []string
		steps    []string // "name=outcome", in order
		services []Service
		legacy   CheckResult // only the pre-HOST-004 fields
	}{
		{
			name: "windows", goos: "windows", cfg: Config{FirewallRule: "Rule"},
			runner:   &recordingRunner{output: map[string]string{"tailscale ip -4": "100.64.0.9", psPrefix + psPort: "3390", psPrefix + psDeny: "policy=\r\nlocal=0", psPrefix + psFwCheck: "count=1\r\n" + fwRule("True", "Allow", "Inbound", "3390")}},
			calls:    []string{"output tailscale ip -4", "output " + psPrefix + psPort, "output " + psPrefix + psDeny, "output " + psPrefix + psFwCheck},
			steps:    []string{"Tailscale IP=ok", "RDP configuration=ok", "Firewall rule=ok"},
			services: []Service{{Kind: "rdp", Port: 3390, Enabled: true, Active: active, FirewallOK: true}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.9", TailscaleUp: true, RDPPort: 3390, RDPEnabled: true, FirewallOK: true},
		},
		{
			name: "windows, registry unreadable and no rule", goos: "windows", cfg: Config{FirewallRule: "Rule"},
			runner:   &recordingRunner{fail: map[string]bool{"tailscale ip -4": true, psPrefix + psPort: true}},
			calls:    []string{"output tailscale ip -4", "output " + psPrefix + psPort, "output " + psPrefix + psFwCheck},
			steps:    []string{"Tailscale IP=failed", "RDP configuration=failed", "Firewall rule=changed"},
			services: []Service{{Kind: "rdp", Active: active}},
			legacy:   CheckResult{},
		},
		{
			// readPortPS maps a read below 1 to 3389, so a successful read
			// never yields an enabled service on port 0.
			name: "windows, registry reads 0", goos: "windows", cfg: Config{FirewallRule: "Rule"},
			runner:   &recordingRunner{output: map[string]string{"tailscale ip -4": "100.64.0.9", psPrefix + psPort: "0", psPrefix + psDeny: "local=0"}},
			calls:    []string{"output tailscale ip -4", "output " + psPrefix + psPort, "output " + psPrefix + psDeny, "output " + psPrefix + psFwCheck},
			steps:    []string{"Tailscale IP=ok", "RDP configuration=ok", "Firewall rule=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: active}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.9", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
		},
		{
			// No service is declared on macOS until #459, so the hard-coded
			// rdp_port 3389 this branch used to report is gone.
			name: "darwin stub", goos: "darwin", cfg: Config{},
			runner: &recordingRunner{output: map[string]string{"tailscale ip -4": "100.64.0.7"}},
			calls:  []string{"output tailscale ip -4"},
			steps:  []string{"Tailscale IP=ok"},
			legacy: CheckResult{TailscaleIP: "100.64.0.7", TailscaleUp: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkFor(tt.goos, tt.cfg, tt.runner)
			if !reflect.DeepEqual(tt.runner.calls, tt.calls) {
				t.Errorf("calls:\n got %q\nwant %q", tt.runner.calls, tt.calls)
			}
			for _, c := range tt.runner.calls {
				if strings.HasPrefix(c, "run ") {
					t.Errorf("check issued a mutating command: %q", c)
				}
			}
			var steps []string
			for _, s := range got.Steps {
				steps = append(steps, s.Name+"="+string(s.Outcome))
				if s.Success != s.Outcome.Succeeded() {
					t.Errorf("step %q: Success = %v but outcome %q", s.Name, s.Success, s.Outcome)
				}
				if strings.Contains(s.Message, "port 0") {
					t.Errorf("step %q names port 0 for an unknown port: %q", s.Name, s.Message)
				}
				if s.Outcome != OutcomeOK && s.Fix == "" {
					t.Errorf("step %q is %s with no fix", s.Name, s.Outcome)
				}
			}
			if !reflect.DeepEqual(steps, tt.steps) {
				t.Errorf("steps = %q, want %q", steps, tt.steps)
			}
			if !reflect.DeepEqual(got.Services, tt.services) {
				t.Errorf("services = %+v, want %+v", got.Services, tt.services)
			}
			got.Steps, got.Services = nil, nil
			if !reflect.DeepEqual(got, tt.legacy) {
				t.Errorf("legacy keys = %+v, want %+v", got, tt.legacy)
			}
		})
	}
}

func TestOutcomeSucceeded(t *testing.T) {
	for o, want := range map[Outcome]bool{
		OutcomeOK: true, OutcomeChanged: true, OutcomeFailed: false, OutcomeManual: false, OutcomeUnverified: false,
	} {
		if got := o.Succeeded(); got != want {
			t.Errorf("%q.Succeeded() = %v, want %v", o, got, want)
		}
	}
}

// The declared control_url reaches both results, for the descriptor (AC6).
func TestCheckFor_ControlURL(t *testing.T) {
	for _, goos := range []string{goosWindows, goosLinux, goosDarwin} {
		got := checkFor(goos, Config{Port: 3389, ControlURL: "https://hs.example"}, &recordingRunner{})
		if got.ControlURL != "https://hs.example" {
			t.Errorf("%s: ControlURL = %q, want the declared one", goos, got.ControlURL)
		}
	}
}

func TestSetupFor_ControlURL(t *testing.T) {
	for _, goos := range []string{goosWindows, goosLinux, goosDarwin} {
		got := setupFor(goos, Config{Port: 3389, FirewallRule: "Rule", ControlURL: "https://hs.example"}, &recordingRunner{elevated: true})
		if got.ControlURL != "https://hs.example" {
			t.Errorf("%s: ControlURL = %q, want the declared one", goos, got.ControlURL)
		}
	}
}
