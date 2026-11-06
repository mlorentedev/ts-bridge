package host

import (
	"slices"
	"strings"
	"testing"
)

// xrdpINI is the [Globals] head of xrdp 0.9.24's /etc/xrdp/xrdp.ini on Ubuntu
// 24.04, trimmed to the keys around `port`.
const xrdpINI = "[Globals]\n; xrdp.ini file version number\nini_version=1\n\n; fork a new process for each incoming connection\nfork=true\n\n; ports to listen on, number alone means listen on all interfaces\nport=%s\n\n; 'port' above should be connected to with vsock instead of tcp\nuse_vsock=false\n\n[Xorg]\nname=Xorg\nport=-1\n"

func xrdpWith(port string) map[string]string {
	return map[string]string{xrdpINIPath: strings.Replace(xrdpINI, "%s", port, 1)}
}

// A declared RDP port is applied to the xrdp listener, not only to the firewall,
// so RDP answers where setup opened the port (#557, AC2).
func TestXRDPPortStep(t *testing.T) {
	tests := []struct {
		name         string
		files        map[string]string
		declared     int
		setup, check Outcome
		included     bool   // false: everything is xrdp's default, no step
		written      string // the port line setup writes, "" for none
		message      string
	}{
		{name: "default everywhere", files: xrdpWith("3389"), declared: 3389},
		{name: "no ini, default declared", declared: 3389},
		{name: "declared 3390 on a 3389 listener", files: xrdpWith("3389"), declared: 3390, setup: OutcomeChanged, check: OutcomeChanged, included: true, written: "port=3390", message: "3390"},
		{name: "converged on 3390", files: xrdpWith("3390"), declared: 3390, setup: OutcomeOK, check: OutcomeOK, included: true, message: "3390"},
		{name: "xrdp moved off the declared default", files: xrdpWith("3390"), declared: 3389, setup: OutcomeChanged, check: OutcomeChanged, included: true, written: "port=3389", message: "3389"},
		{name: "tcp:// form keeps its address", files: xrdpWith("tcp://.:3389"), declared: 3390, setup: OutcomeChanged, check: OutcomeChanged, included: true, written: "port=tcp://.:3390", message: "3390"},
		{name: "vsock is not a TCP port", files: xrdpWith("vsock://-1:3389"), declared: 3390, setup: OutcomeFailed, check: OutcomeFailed, included: true, message: "vsock://-1:3389"},
		{name: "no ini, 3390 declared", declared: 3390, setup: OutcomeFailed, check: OutcomeFailed, included: true, message: xrdpINIPath},
		// With no [Globals] header there is nowhere to put port=: refuse rather
		// than rewrite the same bytes and report the port applied.
		{name: "no [Globals] section, 3390 declared", files: map[string]string{xrdpINIPath: "[Xorg]\nname=Xorg\nport=-1\n"}, declared: 3390, setup: OutcomeFailed, check: OutcomeFailed, included: true, message: "[Globals]"},
		{name: "no [Globals] section, default declared", files: map[string]string{xrdpINIPath: "[Xorg]\nname=Xorg\nport=-1\n"}, declared: 3389},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recordingRunner{elevated: true, files: cloneMap(tt.files)}
			s, ok := xrdpPortSetupStep(r, tt.declared)
			assertXRDPPortStep(t, "setup", s, ok, tt.included, tt.setup, tt.message)
			wrote := slices.Contains(r.calls, "writefile "+xrdpINIPath)
			if wrote != (tt.written != "") {
				t.Fatalf("setup wrote xrdp.ini = %v, want %v: %v", wrote, tt.written != "", r.calls)
			}
			if wrote {
				if !strings.Contains(r.files[xrdpINIPath], "\n"+tt.written+"\n") || !strings.Contains(r.files[xrdpINIPath], "[Xorg]\nname=Xorg\nport=-1\n") {
					t.Errorf("xrdp.ini after setup:\n%s\nwant %q in [Globals] and [Xorg] untouched", r.files[xrdpINIPath], tt.written)
				}
				if !slices.Contains(r.calls, "run systemctl restart xrdp") {
					t.Errorf("xrdp not restarted after the port changed: %v", r.calls)
				}
			}
			c := &recordingRunner{elevated: true, files: cloneMap(tt.files)}
			s, ok = xrdpPortCheckStep(c, tt.declared)
			assertXRDPPortStep(t, "check", s, ok, tt.included, tt.check, tt.message)
			if len(c.calls) != 0 {
				t.Errorf("check issued %v", c.calls)
			}
		})
	}
}

func assertXRDPPortStep(t *testing.T, which string, s SetupStep, included, want bool, outcome Outcome, message string) {
	t.Helper()
	if included != want {
		t.Fatalf("%s included the step = %v, want %v (%+v)", which, included, want, s)
	}
	if !want {
		return
	}
	if s.Outcome != outcome || !strings.Contains(s.Message, message) {
		t.Errorf("%s = %s %q, want %s naming %q", which, s.Outcome, s.Message, outcome, message)
	}
	if !s.Outcome.Succeeded() && s.Fix == "" {
		t.Errorf("%s is %s with no fix", which, s.Outcome)
	}
}

func cloneMap(m map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Setup and check carry the step on Linux once a non-default port is declared.
func TestLinuxStepsCarryTheXRDPPort(t *testing.T) {
	r := &recordingRunner{elevated: true, onPath: map[string]bool{"systemctl": true, "ufw": true}, files: xrdpWith("3389"),
		output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + "ufw allow 3390/tcp\n"})}
	got := setupLinux(Config{Port: 3390, PortDeclared: true}, r)
	names := make([]string, 0, len(got.Steps))
	for _, s := range got.Steps {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{"xrdp service", xrdpPortName, linuxFwName, "Tailscale IP"}) {
		t.Errorf("setup steps = %v", names)
	}
	var result CheckResult
	checkLinux(Config{Port: 3390, PortDeclared: true}, &recordingRunner{elevated: true, onPath: map[string]bool{"systemctl": true, "ufw": true}, files: xrdpWith("3389"),
		output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + "ufw allow 3390/tcp\n"})}, &result)
	if len(result.Steps) != 3 || result.Steps[1].Name != xrdpPortName || result.Steps[1].Outcome != OutcomeChanged {
		t.Errorf("check steps = %+v, want the xrdp port drift second", result.Steps)
	}
}

// Only a declared port moves xrdp: with nothing declared, a hand-set port= is
// the operator's, so setup neither rewrites it nor restarts xrdp, and check
// reports no drift (Windows likewise adopts the registry's port).
func TestLinuxLeavesAnUndeclaredXRDPPortAlone(t *testing.T) {
	ufwHost := map[string]bool{"systemctl": true, "ufw": true}
	r := &recordingRunner{elevated: true, onPath: ufwHost, files: xrdpWith("3390"), output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwScopedAllow})}
	got := setupLinux(Config{Port: 3389}, r)
	for _, s := range got.Steps {
		if s.Name == xrdpPortName {
			t.Errorf("setup reported %s %q with no port declared", s.Outcome, s.Message)
		}
	}
	if slices.Contains(r.calls, "writefile "+xrdpINIPath) || slices.Contains(r.calls, "run systemctl restart xrdp") {
		t.Errorf("setup moved an undeclared xrdp port: %v", r.calls)
	}
	var result CheckResult
	checkLinux(Config{Port: 3389}, &recordingRunner{elevated: true, onPath: ufwHost, files: xrdpWith("3390"), output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwScopedAllow})}, &result)
	for _, s := range result.Steps {
		if s.Name == xrdpPortName {
			t.Errorf("check reported %s %q with no port declared", s.Outcome, s.Message)
		}
	}
}
