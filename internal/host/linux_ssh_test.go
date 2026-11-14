package host

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The systemd answers below are the words `systemctl is-enabled` and
// `is-active` print: Debian and Ubuntu name the unit ssh, Fedora, RHEL and
// Arch name it sshd, and a host with neither answers not-found (exit 4).
const (
	sshIsEnabled  = "systemctl is-enabled ssh"
	sshIsActive   = "systemctl is-active ssh"
	sshdIsEnabled = "systemctl is-enabled sshd"
	sshdIsActive  = "systemctl is-active sshd"
	sockIsEnabled = "systemctl is-enabled ssh.socket"
	sockIsActive  = "systemctl is-active ssh.socket"
	sdSockEnabled = "systemctl is-enabled sshd.socket"
	sdSockActive  = "systemctl is-active sshd.socket"
	ufwSSHAllow   = "ufw allow in on tailscale0 to any port 22 proto tcp"
	iptScopedSSH  = "-A INPUT -i tailscale0 -p tcp -m tcp --dport 22 -j ACCEPT\n"
)

var (
	sshOnly   = Config{Port: 3389, Services: []DeclaredService{{Kind: ServiceSSH, Port: 22}}}
	sshAndRDP = Config{Port: 3389, Services: []DeclaredService{{Kind: ServiceRDP, Port: 3389}, {Kind: ServiceSSH, Port: 22}}}
	sshOnPath = map[string]bool{"systemctl": true, "ufw": true, "sshd": true}
)

// sshUp is a Debian-style host with ssh running and its port open on tailscale0.
func sshUp(extra map[string]string) map[string]string {
	out := map[string]string{
		"tailscale ip -4": "100.64.0.5\n", sshIsEnabled: "enabled\n", sshIsActive: "active\n",
		ufwStatus: ufwActive, ufwAdded: ufwAddedHeader + ufwSSHAllow + "\n",
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// sshSocket is the answers of a Ubuntu-style host whose ssh service and
// ssh.socket each report an (is-enabled, is-active) pair.
func sshSocket(svcEnabled, svcActive, sockEnabled, sockActive string) map[string]string {
	return map[string]string{
		sshIsEnabled: svcEnabled + "\n", sshIsActive: svcActive + "\n",
		sockIsEnabled: sockEnabled + "\n", sockIsActive: sockActive + "\n",
	}
}

// ran lists the commands a runner executed, as opposed to read.
func ran(r *recordingRunner) []string {
	var out []string
	for _, c := range r.calls {
		if strings.HasPrefix(c, "run ") {
			out = append(out, strings.TrimPrefix(c, "run "))
		}
	}
	return out
}

func outcomesOf(steps []SetupStep) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.Name+"="+string(s.Outcome))
	}
	return out
}

func TestReadSSH(t *testing.T) {
	notFound := map[string]int{sshIsEnabled: 4}
	tests := []struct {
		name   string
		runner *recordingRunner
		unit   string
		want   xrdpState
	}{
		{"debian: ssh", &recordingRunner{onPath: sshOnPath, output: sshUp(nil)},
			"ssh", xrdpState{installed: true, readable: true, enabled: true, active: true}},
		{"fedora: ssh is not found, sshd is", &recordingRunner{onPath: sshOnPath,
			output: map[string]string{sshIsEnabled: "not-found\n", sshdIsEnabled: "enabled\n", sshdIsActive: "active\n"}, exit: notFound},
			"sshd", xrdpState{installed: true, readable: true, enabled: true, active: true}},
		{"neither unit exists", &recordingRunner{onPath: sshOnPath,
			output: map[string]string{sshIsEnabled: "not-found\n", sshdIsEnabled: "not-found\n"}, exit: map[string]int{sshIsEnabled: 4, sshdIsEnabled: 4}},
			"", xrdpState{}},
		{"ssh wins over the sshd alias it carries on Debian", &recordingRunner{onPath: sshOnPath,
			output: map[string]string{sshIsEnabled: "disabled\n", sshIsActive: "inactive\n", sshdIsEnabled: "alias\n"}},
			"ssh", xrdpState{installed: true, readable: true}},
		{"socket-activated: the socket stands in for the idle service", &recordingRunner{onPath: sshOnPath,
			output: map[string]string{sshIsEnabled: "disabled\n", sshIsActive: "inactive\n", sockIsEnabled: "enabled\n", sockIsActive: "active\n"}},
			"ssh.socket", xrdpState{installed: true, readable: true, enabled: true, active: true}},
		{"socket exists but is off: the socket is the converge target", &recordingRunner{onPath: sshOnPath,
			output: map[string]string{sshIsEnabled: "disabled\n", sshIsActive: "inactive\n", sockIsEnabled: "disabled\n", sockIsActive: "inactive\n"}},
			"ssh.socket", xrdpState{installed: true, readable: true}},
		{"no socket unit: the service is the target", &recordingRunner{onPath: sshOnPath,
			output: map[string]string{sshIsEnabled: "disabled\n", sshIsActive: "inactive\n", sockIsEnabled: "not-found\n"}, exit: map[string]int{sockIsEnabled: 4}},
			"ssh", xrdpState{installed: true, readable: true}},
		{"a socket systemctl cannot answer is ignored", &recordingRunner{onPath: sshOnPath,
			output: map[string]string{sshIsEnabled: "disabled\n", sshIsActive: "inactive\n"}},
			"ssh", xrdpState{installed: true, readable: true}},
		{"a masked socket is masked, and still the target", &recordingRunner{onPath: sshOnPath,
			output: map[string]string{sshIsEnabled: "disabled\n", sshIsActive: "inactive\n", sockIsEnabled: "masked\n", sockIsActive: "inactive\n"}},
			"ssh.socket", xrdpState{installed: true, readable: true, masked: true}},
		{"fedora: sshd.socket", &recordingRunner{onPath: sshOnPath,
			output: map[string]string{sshIsEnabled: "not-found\n", sshdIsEnabled: "disabled\n", sshdIsActive: "inactive\n", sdSockEnabled: "enabled\n", sdSockActive: "active\n"},
			exit:   map[string]int{sshIsEnabled: 4}},
			"sshd.socket", xrdpState{installed: true, readable: true, enabled: true, active: true}},
		{"service running directly: the socket is not consulted", &recordingRunner{onPath: sshOnPath,
			output: sshSocket("enabled", "active", "masked", "inactive")},
			"ssh", xrdpState{installed: true, readable: true, enabled: true, active: true}},
		{"a masked service is not rescued by its socket", &recordingRunner{onPath: sshOnPath,
			output: map[string]string{sshIsEnabled: "masked\n", sshIsActive: "inactive\n", sockIsEnabled: "enabled\n", sockIsActive: "active\n"}},
			"ssh", xrdpState{installed: true, readable: true, masked: true}},
		{"no systemctl: sshd on PATH is the only evidence", &recordingRunner{onPath: map[string]bool{"sshd": true}},
			"ssh", xrdpState{installed: true}},
		{"no systemctl, no sshd", &recordingRunner{}, "", xrdpState{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unit, got := readSSH(tt.runner)
			if got != tt.want {
				t.Errorf("state = %+v, want %+v", got, tt.want)
			}
			if tt.want.installed && unit != tt.unit {
				t.Errorf("unit = %q, want %q", unit, tt.unit)
			}
		})
	}
}

// TestSetupSSH pins setup as read-then-act for ssh: a converged host runs
// nothing, each mutation runs once and only on a difference, and the rdp
// steps never appear for a host that does not declare rdp.
func TestSetupSSH(t *testing.T) {
	tests := []struct {
		name   string
		cfg    Config
		runner *recordingRunner
		steps  []string
		ran    []string
	}{
		{"converged", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(nil)},
			[]string{"SSH server=ok", sshFwName + "=ok", "Tailscale IP=ok"}, nil},
		{"enabled but stopped", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(map[string]string{sshIsActive: "inactive\n"})},
			[]string{"SSH server=changed", sshFwName + "=ok", "Tailscale IP=ok"}, []string{"systemctl enable --now ssh"}},
		{"fedora names the unit sshd", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(map[string]string{sshIsEnabled: "not-found\n", sshdIsEnabled: "disabled\n", sshdIsActive: "inactive\n"}), exit: map[string]int{sshIsEnabled: 4}},
			[]string{"SSH server=changed", sshFwName + "=ok", "Tailscale IP=ok"}, []string{"systemctl enable --now sshd"}},
		{"socket-activated and listening: nothing to run", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(sshSocket("disabled", "inactive", "enabled", "active"))},
			[]string{"SSH server=ok", sshFwName + "=ok", "Tailscale IP=ok"}, nil},
		{"socket enabled but stopped: the socket is started, not the service", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(sshSocket("disabled", "inactive", "enabled", "inactive"))},
			[]string{"SSH server=changed", sshFwName + "=ok", "Tailscale IP=ok"}, []string{"systemctl enable --now ssh.socket"}},
		{"socket and service both off: the socket is the target", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(sshSocket("disabled", "inactive", "disabled", "inactive"))},
			[]string{"SSH server=changed", sshFwName + "=ok", "Tailscale IP=ok"}, []string{"systemctl enable --now ssh.socket"}},
		{"no socket unit: the service, as before", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(map[string]string{sshIsEnabled: "disabled\n", sshIsActive: "inactive\n", sockIsEnabled: "not-found\n"}), exit: map[string]int{sockIsEnabled: 4}},
			[]string{"SSH server=changed", sshFwName + "=ok", "Tailscale IP=ok"}, []string{"systemctl enable --now ssh"}},
		{"service running directly, socket masked: converged, left alone", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(sshSocket("enabled", "active", "masked", "inactive"))},
			[]string{"SSH server=ok", sshFwName + "=ok", "Tailscale IP=ok"}, nil},
		{"masked socket: failed, never unmasked", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(sshSocket("disabled", "inactive", "masked", "inactive"))},
			[]string{"SSH server=failed", sshFwName + "=ok", "Tailscale IP=ok"}, nil},
		{"fedora: sshd.socket", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(map[string]string{sshIsEnabled: "not-found\n", sshdIsEnabled: "disabled\n", sshdIsActive: "inactive\n", sdSockEnabled: "disabled\n", sdSockActive: "inactive\n"}), exit: map[string]int{sshIsEnabled: 4}},
			[]string{"SSH server=changed", sshFwName + "=ok", "Tailscale IP=ok"}, []string{"systemctl enable --now sshd.socket"}},
		{"port closed: the rule is scoped to tailscale0", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(map[string]string{ufwAdded: ufwAddedHeader})},
			[]string{"SSH server=ok", sshFwName + "=changed", "Tailscale IP=ok"}, []string{ufwSSHAllow}},
		{"not installed: manual, nothing installed", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(map[string]string{sshIsEnabled: "not-found\n", sshdIsEnabled: "not-found\n"}), exit: map[string]int{sshIsEnabled: 4, sshdIsEnabled: 4}},
			[]string{"SSH server=manual", sshFwName + "=ok", "Tailscale IP=ok"}, nil},
		{"masked: failed, never unmasked", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(map[string]string{sshIsEnabled: "masked\n", sshIsActive: "inactive\n"})},
			[]string{"SSH server=failed", sshFwName + "=ok", "Tailscale IP=ok"}, nil},
		{"rdp and ssh: rdp steps first, rdp firewall name unchanged", sshAndRDP,
			&recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "ufw": true},
				output: sshUp(map[string]string{isEnabled: "enabled\n", isActive: "active\n", ufwAdded: ufwAddedHeader + ufwScopedAllow + ufwSSHAllow + "\n"})},
			[]string{"xrdp service=ok", linuxFwName + "=ok", "SSH server=ok", sshFwName + "=ok", "Tailscale IP=ok"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setupFor("linux", tt.cfg, tt.runner)
			if steps := outcomesOf(got.Steps); !reflect.DeepEqual(steps, tt.steps) {
				t.Errorf("steps = %q, want %q", steps, tt.steps)
			}
			if got := ran(tt.runner); !slices.Equal(got, tt.ran) {
				t.Errorf("ran %q, want %q", got, tt.ran)
			}
			for _, s := range got.Steps {
				if s.Outcome != OutcomeOK && s.Outcome != OutcomeChanged && s.Fix == "" {
					t.Errorf("step %q is %s with no fix", s.Name, s.Outcome)
				}
			}
		})
	}
}

// TestSSHSocketStepsNameTheUnit pins that the output says which unit setup
// converged or refused, since ssh and ssh.socket are two units.
func TestSSHSocketStepsNameTheUnit(t *testing.T) {
	tests := []struct {
		name        string
		check       bool
		sockEnabled string
		wantOutcome Outcome
		wantFix     string
	}{
		{"setup converges the socket", false, "disabled", OutcomeChanged, ""},
		{"setup refuses a masked socket", false, "masked", OutcomeFailed, "systemctl unmask ssh.socket, then"},
		{"check refuses a masked socket", true, "masked", OutcomeManual, "systemctl unmask ssh.socket, then"},
		{"check reports the socket as drift", true, "disabled", OutcomeChanged, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(sshSocket("disabled", "inactive", tt.sockEnabled, "inactive"))}
			s := setupFor("linux", sshOnly, r).Steps[0]
			if tt.check {
				s = checkFor("linux", sshOnly, r).Steps[1]
			}
			if s.Outcome != tt.wantOutcome || !strings.Contains(s.Message, "ssh.socket") || !strings.Contains(s.Fix, tt.wantFix) {
				t.Errorf("step = %s %q fix %q, want %s naming ssh.socket and %q", s.Outcome, s.Message, s.Fix, tt.wantOutcome, tt.wantFix)
			}
		})
	}
}

func TestSetupSSHOnlyReportsNoRDPPort(t *testing.T) {
	got := setupFor("linux", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(nil)})
	if got.RDPPort != 0 {
		t.Errorf("RDPPort = %d, want 0: an ssh-only host declares no RDP port", got.RDPPort)
	}
	if rdp := setupFor("linux", Config{Port: 3389}, &recordingRunner{}); rdp.RDPPort != 3389 {
		t.Errorf("a host with no declared services keeps the rdp default, RDPPort = %d", rdp.RDPPort)
	}
}

func TestSetupSSHMissingServerNamesTheInstall(t *testing.T) {
	r := &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(nil),
		exit: map[string]int{sshIsEnabled: 4, sshdIsEnabled: 4}}
	r.output[sshIsEnabled], r.output[sshdIsEnabled] = "not-found\n", "not-found\n"
	s := setupFor("linux", sshOnly, r).Steps[0]
	if s.Outcome != OutcomeManual || !strings.Contains(s.Fix, "openssh-server") || !strings.Contains(s.Fix, "does not install") {
		t.Errorf("step = %s %q, want manual naming the package and that ts-bridge installs nothing", s.Outcome, s.Fix)
	}
}

// TestSetupSSHKeepsTheOperatorsOwnAllow pins the one place the rdp firewall
// logic cannot be reused as is: it deletes the unscoped allow that earlier
// releases wrote for the RDP port, and the same rule for port 22 is the
// operator's, never ts-bridge's. Deleting it could close SSH from every network.
func TestSetupSSHKeepsTheOperatorsOwnAllow(t *testing.T) {
	iptables := map[string]bool{"systemctl": true, "iptables": true, "sshd": true}
	ownRule := "-A INPUT -p tcp -m tcp --dport 22 -j ACCEPT\n"
	tests := []struct {
		name    string
		runner  *recordingRunner
		wantAdd string
	}{
		{"ufw allow 22/tcp", &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(map[string]string{ufwAdded: ufwAddedHeader + "ufw allow 22/tcp\n" + ufwSSHAllow + "\n"})}, ""},
		{"two copies in INPUT", &recordingRunner{elevated: true, onPath: iptables,
			output: sshUp(map[string]string{iptRead: iptEmpty + iptScopedSSH + ownRule + ownRule, iptChain: tsInputDefault})}, ""},
		// HOST-007 review: the operator's allow alone still gets the scoped rule added beside it.
		{"ufw allow 22/tcp and no scoped rule", &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(map[string]string{ufwAdded: ufwAddedHeader + "ufw allow 22/tcp\n"})},
			"ufw allow in on tailscale0 to any port 22 proto tcp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setupFor("linux", sshOnly, tt.runner)
			for _, c := range ran(tt.runner) {
				if strings.Contains(c, "delete") || strings.Contains(c, " -D ") {
					t.Errorf("setup deleted a rule ts-bridge did not write: %q", c)
				}
			}
			if fw := got.Steps[1]; fw.Outcome != OutcomeManual || !strings.Contains(fw.Message, "22") {
				t.Errorf("firewall step = %s %q, want manual naming the rule", fw.Outcome, fw.Message)
			}
			if tt.wantAdd != "" && !slices.Contains(ran(tt.runner), tt.wantAdd) {
				t.Errorf("ran %q, want the scoped rule added: %q", ran(tt.runner), tt.wantAdd)
			}
		})
	}
}

func TestCheckSSH(t *testing.T) {
	yes := true
	tests := []struct {
		name     string
		cfg      Config
		runner   *recordingRunner
		steps    []string
		services []Service
		legacy   CheckResult
	}{
		{"ssh only, converged", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(nil)},
			[]string{"Tailscale IP=ok", "SSH server=ok", sshFwName + "=ok"},
			[]Service{{Kind: ServiceSSH, Port: 22, Enabled: true, Active: &yes, FirewallOK: true}},
			CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, FirewallOK: true}},
		{"ssh only, stopped: setup would change it", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(map[string]string{sshIsActive: "inactive\n"})},
			[]string{"Tailscale IP=ok", "SSH server=changed", sshFwName + "=ok"},
			[]Service{{Kind: ServiceSSH, Port: 22, Enabled: true, FirewallOK: true, Active: new(bool)}},
			CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, FirewallOK: true}},
		{"socket-activated and listening: converged", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(sshSocket("disabled", "inactive", "enabled", "active"))},
			[]string{"Tailscale IP=ok", "SSH server=ok", sshFwName + "=ok"},
			[]Service{{Kind: ServiceSSH, Port: 22, Enabled: true, Active: &yes, FirewallOK: true}},
			CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, FirewallOK: true}},
		{"socket off: setup would enable it", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(sshSocket("disabled", "inactive", "disabled", "inactive"))},
			[]string{"Tailscale IP=ok", "SSH server=changed", sshFwName + "=ok"},
			[]Service{{Kind: ServiceSSH, Port: 22, FirewallOK: true, Active: new(bool)}},
			CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, FirewallOK: true}},
		{"masked socket: manual, setup does not unmask", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(sshSocket("disabled", "inactive", "masked", "inactive"))},
			[]string{"Tailscale IP=ok", "SSH server=manual", sshFwName + "=ok"},
			[]Service{{Kind: ServiceSSH, Port: 22, FirewallOK: true, Active: new(bool)}},
			CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, FirewallOK: true}},
		{"rdp and ssh: the legacy keys come from rdp alone", sshAndRDP,
			&recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "ufw": true},
				output: sshUp(map[string]string{isEnabled: "enabled\n", isActive: "active\n", ufwAdded: ufwAddedHeader + ufwScopedAllow + ufwSSHAllow + "\n"})},
			[]string{"Tailscale IP=ok", "xrdp service=ok", linuxFwName + "=ok", "SSH server=ok", sshFwName + "=ok"},
			[]Service{{Kind: ServiceRDP, Port: 3389, Enabled: true, Active: &yes, FirewallOK: true}, {Kind: ServiceSSH, Port: 22, Enabled: true, Active: &yes, FirewallOK: true}},
			CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true, FirewallOK: true}},
		{"a closed ssh port keeps firewall_ok false although rdp is open", sshAndRDP,
			&recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "ufw": true},
				output: sshUp(map[string]string{isEnabled: "enabled\n", isActive: "active\n", ufwAdded: ufwAddedHeader + ufwScopedAllow})},
			[]string{"Tailscale IP=ok", "xrdp service=ok", linuxFwName + "=ok", "SSH server=ok", sshFwName + "=changed"},
			[]Service{{Kind: ServiceRDP, Port: 3389, Enabled: true, Active: &yes, FirewallOK: true}, {Kind: ServiceSSH, Port: 22, Enabled: true, Active: &yes}},
			CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true}},
		{"not root: the firewall is unverified, the service was read fine", sshOnly, &recordingRunner{onPath: sshOnPath, output: sshUp(nil)},
			[]string{"Tailscale IP=ok", "SSH server=ok", sshFwName + "=unverified"},
			[]Service{{Kind: ServiceSSH, Port: 22, Enabled: true, Active: &yes}},
			CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true}},
		{"no systemctl: unread, never claimed off", sshOnly, &recordingRunner{onPath: map[string]bool{"sshd": true}, output: map[string]string{"tailscale ip -4": "100.64.0.5\n"}},
			[]string{"Tailscale IP=ok", "SSH server=unverified", sshFwName + "=unverified"},
			[]Service{{Kind: ServiceSSH, Port: 22, Unread: true}},
			CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true}},
		// HOST-007 review: the operator's unscoped allow is reported, not owned, and the port is not tailnet-only.
		{"an operator-owned unscoped allow: firewall_ok false, manual", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(map[string]string{ufwAdded: ufwAddedHeader + "ufw allow 22/tcp\n" + ufwSSHAllow + "\n"})},
			[]string{"Tailscale IP=ok", "SSH server=ok", sshFwName + "=manual"},
			[]Service{{Kind: ServiceSSH, Port: 22, Enabled: true, Active: &yes}},
			CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true}},
		{"not installed: manual, with the install named", sshOnly, &recordingRunner{elevated: true, onPath: sshOnPath,
			output: sshUp(map[string]string{sshIsEnabled: "not-found\n", sshdIsEnabled: "not-found\n"}), exit: map[string]int{sshIsEnabled: 4, sshdIsEnabled: 4}},
			[]string{"Tailscale IP=ok", "SSH server=manual", sshFwName + "=ok"},
			[]Service{{Kind: ServiceSSH, Port: 22, FirewallOK: true}},
			CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, FirewallOK: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkFor("linux", tt.cfg, tt.runner)
			if steps := outcomesOf(got.Steps); !reflect.DeepEqual(steps, tt.steps) {
				t.Errorf("steps = %q, want %q", steps, tt.steps)
			}
			if mutated := ran(tt.runner); len(mutated) > 0 {
				t.Errorf("check ran %q", mutated)
			}
			if !reflect.DeepEqual(got.Services, tt.services) {
				t.Errorf("services = %+v, want %+v", got.Services, tt.services)
			}
			got.Steps, got.Services, got.Elevated = nil, nil, false
			if !reflect.DeepEqual(got, tt.legacy) {
				t.Errorf("legacy keys = %+v, want %+v", got, tt.legacy)
			}
		})
	}
}

// TestCheckSSHDefaultStaysRDP pins the compatibility the loop must keep: a host
// that declares nothing is still the xrdp host every earlier test describes.
func TestCheckSSHDefaultStaysRDP(t *testing.T) {
	got := checkFor("linux", Config{Port: 3389}, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(nil)})
	if len(got.Services) != 1 || got.Services[0].Kind != ServiceRDP {
		t.Errorf("services = %+v, want only rdp", got.Services)
	}
	for _, c := range got.Steps {
		if strings.Contains(c.Name, "SSH") {
			t.Errorf("an undeclared ssh produced the step %q", c.Name)
		}
	}
}

// sshdT is the effective-configuration read; the fake runner's LookPath finds
// sshd in /usr/bin.
const sshdT = "/usr/bin/sshd -T"

func stepNamed(t *testing.T, steps []SetupStep, name string) SetupStep {
	t.Helper()
	for _, s := range steps {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no step %q in %q", name, outcomesOf(steps))
	return SetupStep{}
}

// TestSSHPort pins what ts-bridge does with a declared non-default port: it
// reads the listener from sshd's effective configuration and reports a
// mismatch, and never edits sshd_config (ADR-017 §5, "verified and guided").
func TestSSHPort(t *testing.T) {
	cfg := Config{Services: []DeclaredService{{Kind: ServiceSSH, Port: 2222}}}
	tests := []struct {
		name         string
		output       map[string]string
		fail         map[string]bool
		setup, check Outcome
		message      string
	}{
		{"Port matches", map[string]string{sshdT: "port 2222\nlistenaddress 0.0.0.0\n"}, nil, OutcomeOK, OutcomeOK, ""},
		{"ListenAddress carries the port", map[string]string{sshdT: "port 22\nlistenaddress [::]:2222\n"}, nil, OutcomeOK, OutcomeOK, ""},
		{"sshd listens elsewhere", map[string]string{sshdT: "port 22\n"}, nil, OutcomeManual, OutcomeManual, "listens on TCP 22, not the declared 2222"},
		{"unreadable without root", nil, map[string]bool{sshdT: true}, OutcomeFailed, OutcomeUnverified, "Cannot read the sshd listener"},
		{"answers with no port", map[string]string{sshdT: "usepam yes\n"}, nil, OutcomeFailed, OutcomeUnverified, "Cannot read the sshd listener"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := sshUp(tt.output)
			setup := &recordingRunner{elevated: true, onPath: sshOnPath, output: out, fail: tt.fail}
			s := stepNamed(t, setupFor("linux", cfg, setup).Steps, sshPortName)
			if s.Outcome != tt.setup || !strings.Contains(s.Message, tt.message) {
				t.Errorf("setup = %s %q, want %s with %q", s.Outcome, s.Message, tt.setup, tt.message)
			}
			if got := ran(setup); slices.ContainsFunc(got, func(c string) bool { return strings.Contains(c, "sshd_config") || strings.Contains(c, "restart") }) {
				t.Errorf("setup changed sshd: %q", got)
			}
			check := &recordingRunner{elevated: true, onPath: sshOnPath, output: out, fail: tt.fail}
			got := checkFor("linux", cfg, check)
			if c := stepNamed(t, got.Steps, sshPortName); c.Outcome != tt.check || !strings.Contains(c.Message, tt.message) {
				t.Errorf("check = %s %q, want %s with %q", c.Outcome, c.Message, tt.check, tt.message)
			}
			if got.Services[0].Unread {
				t.Error("Unread = true: the service was read, only its listener port was not")
			}
		})
	}
}

func TestSSHPortDefaultAndMissingReadNothing(t *testing.T) {
	r := &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(nil)}
	steps := setupFor("linux", sshOnly, r).Steps
	if slices.Contains(r.calls, "output "+sshdT) || slices.ContainsFunc(steps, func(s SetupStep) bool { return s.Name == sshPortName }) {
		t.Errorf("the default port 22 produced a port read or step: %q", r.calls)
	}
	missing := &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(nil), exit: map[string]int{sshIsEnabled: 4, sshdIsEnabled: 4}}
	missing.output[sshIsEnabled], missing.output[sshdIsEnabled] = "not-found\n", "not-found\n"
	setupFor("linux", Config{Services: []DeclaredService{{Kind: ServiceSSH, Port: 2222}}}, missing)
	if slices.Contains(missing.calls, "output "+sshdT) {
		t.Error("sshd -T was read although no OpenSSH server is installed")
	}
}

// TestLinuxKindWithNoStepFails pins #768: the loader admits only kinds with a
// Linux step, but a kind that reaches the steps without one must fail loudly,
// never converge nothing while the summary reads ready.
func TestLinuxKindWithNoStepFails(t *testing.T) {
	cfg := Config{Port: 3389, Services: []DeclaredService{{Kind: "vnc", Port: 5900}}}
	check := checkFor("linux", cfg, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(nil)})
	setup := setupFor("linux", cfg, &recordingRunner{elevated: true, onPath: sshOnPath, output: sshUp(nil)})
	for name, steps := range map[string][]SetupStep{"check": check.Steps, "setup": setup.Steps} {
		i := slices.IndexFunc(steps, func(s SetupStep) bool { return s.Name == "vnc service" })
		if i < 0 || steps[i].Outcome != OutcomeFailed || !strings.Contains(steps[i].Message, "vnc") {
			t.Errorf("%s steps = %q, want a failed \"vnc service\" step naming the kind", name, outcomesOf(steps))
		}
	}
	if want := []Service{{Kind: "vnc", Port: 5900, Unread: true}}; !reflect.DeepEqual(check.Services, want) {
		t.Errorf("services = %+v, want %+v: an unhandled kind is listed and unread, never left out", check.Services, want)
	}
}
