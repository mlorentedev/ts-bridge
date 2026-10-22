package host

import (
	"reflect"
	"testing"
)

// The `ufw show added` lines below are verbatim from ufw 0.36.2 on Ubuntu
// 24.04 (specs/BUG-034-linux-host-idempotent, measured 2026-10-22).
const ufwAddedHeader = "Added user rules (see 'ufw status' for running firewall):\n"

func TestUFWAllows(t *testing.T) {
	tests := []struct {
		name  string
		added string
		want  bool
	}{
		{"port/tcp", "ufw allow 3389/tcp", true},
		{"bare port", "ufw allow 3389", true},
		{"interface scoped", "ufw allow in on tailscale0 to any port 3389 proto tcp", true},
		{"to any port, no proto", "ufw allow to any port 3389", true},
		{"longer port", "ufw allow 33890/tcp", false},
		{"shorter port", "ufw allow 338/tcp", false},
		{"udp only", "ufw allow 3389/udp", false},
		{"udp proto", "ufw allow in on tailscale0 to any port 3389 proto udp", false},
		{"outbound", "ufw allow out 3389/tcp", false},
		{"route", "ufw route allow in on tailscale0 to any port 3389 proto tcp", false},
		{"deny", "ufw deny 3389/tcp", false},
		{"limit", "ufw limit 3389/tcp", false},
		{"no rules", "", false},
		{"with comment", "ufw allow 3389/tcp comment 'xrdp'", true},
		{"full form with comment", "ufw allow in on tailscale0 to any port 3389 proto tcp comment 'rdp port'", true},
		{"comment naming the port", "ufw allow 22/tcp comment 'not 3389'", false},
		{"range", "ufw allow 3380:3390/tcp", true},
		{"range below", "ufw allow 3000:3010/tcp", false},
		{"list", "ufw allow 22,3389/tcp", true},
		{"full form range", "ufw allow to any port 3380:3390 proto tcp", true},
		{"source port only", "ufw allow from any port 3389 proto tcp", false},
		{"source port, other destination", "ufw allow from any port 3389 to any port 22 proto tcp", false},
		{"source and destination port", "ufw allow from any port 1234 to any port 3389 proto tcp", true},
		{"logged", "ufw allow log 3389/tcp", true},
		{"logged all, full form", "ufw allow in log-all on tailscale0 to any port 3389 proto tcp", true},
		{"app profile", "ufw allow OpenSSH", false},
		{"among others", "ufw allow 22/tcp\nufw allow 3389/tcp\nufw deny 4000/tcp", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ufwAllows(ufwAddedHeader+tt.added+"\n", 3389); got != tt.want {
				t.Errorf("ufwAllows(%q) = %v, want %v", tt.added, got, tt.want)
			}
		})
	}
}

const (
	ufwAdded   = "ufw show added"
	iptCheck   = "iptables -C INPUT -p tcp --dport 3389 -j ACCEPT"
	iptAppend  = "iptables -A INPUT -p tcp --dport 3389 -j ACCEPT"
	isEnabled  = "systemctl is-enabled xrdp"
	isActive   = "systemctl is-active xrdp"
	enableXRDP = "systemctl enable --now xrdp"
)

var xrdpReads = []string{"lookpath systemctl", "output " + isEnabled, "output " + isActive}

// TestIptablesRule pins the iptables read (AC2, AC5): only exit 0 and exit 1
// of -C are answers; any other result is an unknown state.
func TestIptablesRule(t *testing.T) {
	tests := []struct {
		name   string
		runner *recordingRunner
		want   ruleState
	}{
		{"present", &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}}, rulePresent},
		{"absent", &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, exit: map[string]int{iptCheck: 1}}, ruleAbsent},
		{"permission", &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, exit: map[string]int{iptCheck: 4}}, ruleUnknown},
		{"no exit status", &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, fail: map[string]bool{iptCheck: true}}, ruleUnknown},
		{"no backend", &recordingRunner{elevated: true}, ruleUnknown},
		{"not root", &recordingRunner{onPath: map[string]bool{"iptables": true}}, ruleUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readLinuxFirewall(tt.runner, 3389); got.state != tt.want {
				t.Errorf("state = %v, want %v (reason %q)", got.state, tt.want, got.reason)
			}
		})
	}
}

func TestXRDP(t *testing.T) {
	up := map[string]string{isEnabled: "enabled\n", isActive: "active\n"}
	tests := []struct {
		name   string
		runner *recordingRunner
		want   xrdpState
	}{
		{"enabled and active", &recordingRunner{onPath: map[string]bool{"xrdp": true, "systemctl": true}, output: up},
			xrdpState{installed: true, readable: true, enabled: true, active: true}},
		{"disabled and inactive", &recordingRunner{onPath: map[string]bool{"xrdp": true, "systemctl": true},
			output: map[string]string{isEnabled: "disabled\n", isActive: "inactive\n"},
			exit:   map[string]int{isEnabled: 1, isActive: 3}},
			xrdpState{installed: true, readable: true}},
		{"enabled, not running", &recordingRunner{onPath: map[string]bool{"xrdp": true, "systemctl": true},
			output: map[string]string{isEnabled: "enabled\n", isActive: "failed\n"}, exit: map[string]int{isActive: 3}},
			xrdpState{installed: true, readable: true, enabled: true}},
		{"not installed", &recordingRunner{onPath: map[string]bool{"systemctl": true},
			output: map[string]string{isEnabled: "not-found\n"}, exit: map[string]int{isEnabled: 4}}, xrdpState{}},
		{"sbin off PATH: systemd still finds the unit", &recordingRunner{onPath: map[string]bool{"systemctl": true}, output: up},
			xrdpState{installed: true, readable: true, enabled: true, active: true}},
		{"static unit", &recordingRunner{onPath: map[string]bool{"systemctl": true},
			output: map[string]string{isEnabled: "static\n", isActive: "active\n"}},
			xrdpState{installed: true, readable: true, enabled: true, active: true}},
		{"indirect unit", &recordingRunner{onPath: map[string]bool{"systemctl": true},
			output: map[string]string{isEnabled: "indirect\n", isActive: "active\n"}},
			xrdpState{installed: true, readable: true, enabled: true, active: true}},
		{"enabled for this boot only", &recordingRunner{onPath: map[string]bool{"systemctl": true},
			output: map[string]string{isEnabled: "enabled-runtime\n", isActive: "active\n"}},
			xrdpState{installed: true, readable: true, active: true}},
		{"masked", &recordingRunner{onPath: map[string]bool{"systemctl": true},
			output: map[string]string{isEnabled: "masked\n", isActive: "inactive\n"}, exit: map[string]int{isEnabled: 1, isActive: 3}},
			xrdpState{installed: true, readable: true, masked: true}},
		{"no systemctl", &recordingRunner{onPath: map[string]bool{"xrdp": true}}, xrdpState{installed: true}},
		{"no systemctl, no xrdp", &recordingRunner{}, xrdpState{}},
		{"systemctl prints nothing", &recordingRunner{onPath: map[string]bool{"xrdp": true, "systemctl": true},
			fail: map[string]bool{isEnabled: true, isActive: true}}, xrdpState{installed: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readXRDP(tt.runner); got != tt.want {
				t.Errorf("readXRDP = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func linuxUp(extra map[string]string) map[string]string {
	out := map[string]string{"tailscale ip -4": "100.64.0.5\n", isEnabled: "enabled\n", isActive: "active\n"}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// TestSetupForLinux pins setup as read-then-act (AC1, AC2, AC5): a converged
// host records no run call, and each mutation runs once, only on a difference.
func TestSetupForLinux(t *testing.T) {
	ufw := map[string]bool{"xrdp": true, "systemctl": true, "ufw": true}
	ipt := map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}
	tests := []struct {
		name     string
		runner   *recordingRunner
		calls    []string
		outcomes []Outcome
	}{
		{
			name:     "ufw, converged",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + "ufw allow 3389/tcp\n"})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwAdded, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeOK, OutcomeOK},
		},
		{
			name: "ufw, nothing configured",
			runner: &recordingRunner{elevated: true, onPath: ufw,
				output: linuxUp(map[string]string{isEnabled: "disabled\n", isActive: "inactive\n", ufwAdded: ufwAddedHeader + "ufw route allow in on tailscale0 to any port 3389 proto tcp\n"}),
				exit:   map[string]int{isEnabled: 1, isActive: 3}},
			calls: append(append([]string{}, xrdpReads...), "run "+enableXRDP, "lookpath ufw", "output "+ufwAdded,
				"run ufw allow 3389/tcp", "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeChanged, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "xrdp enabled, not running",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{isActive: "failed\n", ufwAdded: ufwAddedHeader + "ufw allow 3389/tcp\n"}), exit: map[string]int{isActive: 3}},
			calls:    append(append([]string{}, xrdpReads...), "run "+enableXRDP, "lookpath ufw", "output "+ufwAdded, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeChanged, OutcomeOK, OutcomeOK},
		},
		{
			name:     "xrdp masked: no enable, fix names unmask",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{isEnabled: "masked\n", isActive: "inactive\n", ufwAdded: ufwAddedHeader + "ufw allow 3389/tcp\n"}), exit: map[string]int{isEnabled: 1, isActive: 3}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwAdded, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeFailed, OutcomeOK, OutcomeOK},
		},
		{
			name:     "iptables, converged",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(nil)},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "run "+iptCheck, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeOK, OutcomeOK},
		},
		{
			name:     "iptables, rule missing",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(nil), exit: map[string]int{iptCheck: 1}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "run "+iptCheck, "run "+iptAppend, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "iptables -C fails with another status",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(nil), exit: map[string]int{iptCheck: 2}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "run "+iptCheck, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
		},
		{
			name:     "not root: no firewall command runs",
			runner:   &recordingRunner{onPath: ufw, output: linuxUp(nil)},
			calls:    append(append([]string{}, xrdpReads...), "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
		},
		{
			name:     "xrdp missing, no systemctl, no backend, Tailscale down",
			runner:   &recordingRunner{elevated: true, fail: map[string]bool{"tailscale ip -4": true}},
			calls:    []string{"lookpath systemctl", "lookpath xrdp", "lookpath ufw", "lookpath iptables", "output tailscale ip -4"},
			outcomes: []Outcome{OutcomeFailed, OutcomeFailed, OutcomeFailed},
		},
		{
			name:     "no systemctl",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "ufw": true}, output: linuxUp(map[string]string{ufwAdded: "ufw allow 3389/tcp"})},
			calls:    []string{"lookpath systemctl", "lookpath xrdp", "lookpath ufw", "output " + ufwAdded, "output tailscale ip -4"},
			outcomes: []Outcome{OutcomeFailed, OutcomeOK, OutcomeOK},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setupFor("linux", Config{Port: 3389, FirewallRule: "Rule"}, tt.runner)
			if !reflect.DeepEqual(tt.runner.calls, tt.calls) {
				t.Errorf("calls:\n got %q\nwant %q", tt.runner.calls, tt.calls)
			}
			var outcomes []Outcome
			for _, s := range got.Steps {
				outcomes = append(outcomes, s.Outcome)
				if s.Success != s.Outcome.Succeeded() {
					t.Errorf("step %q: Success = %v but outcome %q", s.Name, s.Success, s.Outcome)
				}
				if s.Outcome != OutcomeOK && s.Outcome != OutcomeChanged && s.Fix == "" {
					t.Errorf("step %q is %s with no fix", s.Name, s.Outcome)
				}
			}
			if !reflect.DeepEqual(outcomes, tt.outcomes) {
				t.Errorf("outcomes = %v, want %v", outcomes, tt.outcomes)
			}
		})
	}
}

// TestCheckForLinux pins check as the read half of setup (AC1, AC5): the same
// reads, no run call except the read-only iptables -C, and an unreadable state
// reported as unverified.
func TestCheckForLinux(t *testing.T) {
	yes, no := true, false
	ufw := map[string]bool{"xrdp": true, "systemctl": true, "ufw": true}
	tests := []struct {
		name     string
		runner   *recordingRunner
		calls    []string
		steps    []string
		services []Service
		legacy   CheckResult
	}{
		{
			name:     "converged",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + "ufw allow 3389/tcp\n"})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwAdded),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=ok"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes, FirewallOK: true}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true, FirewallOK: true},
		},
		{
			name: "xrdp disabled, only a longer port allowed",
			runner: &recordingRunner{elevated: true, onPath: ufw,
				output: linuxUp(map[string]string{isEnabled: "disabled\n", isActive: "inactive\n", ufwAdded: ufwAddedHeader + "ufw allow 33890/tcp\n"}),
				exit:   map[string]int{isEnabled: 1, isActive: 3}},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwAdded),
			steps:    []string{"Tailscale IP=ok", "xrdp service=changed", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Active: &no}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true},
		},
		{
			name:     "xrdp enabled, not running",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{isActive: "failed\n", ufwAdded: ufwAddedHeader + "ufw allow 3389/tcp\n"}), exit: map[string]int{isActive: 3}},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwAdded),
			steps:    []string{"Tailscale IP=ok", "xrdp service=changed", "Firewall configuration=ok"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &no, FirewallOK: true}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true, FirewallOK: true},
		},
		{
			name:     "not root, iptables host",
			runner:   &recordingRunner{onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(nil)},
			calls:    append([]string{"output tailscale ip -4"}, xrdpReads...),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=unverified"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
		},
		{
			name:     "no systemctl, ufw unreadable",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "ufw": true}, output: linuxUp(nil), fail: map[string]bool{ufwAdded: true}},
			calls:    []string{"output tailscale ip -4", "lookpath systemctl", "lookpath xrdp", "lookpath ufw", "output " + ufwAdded},
			steps:    []string{"Tailscale IP=ok", "xrdp service=unverified", "Firewall configuration=unverified"},
			services: []Service{{Kind: "rdp", Port: 3389}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true},
		},
		{
			name:     "xrdp not installed",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, output: linuxUp(nil), exit: map[string]int{iptCheck: 1}},
			calls:    []string{"output tailscale ip -4", "lookpath systemctl", "lookpath xrdp", "lookpath ufw", "lookpath iptables", "run " + iptCheck},
			steps:    []string{"Tailscale IP=ok", "xrdp service=failed", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkFor("linux", Config{Port: 3389}, tt.runner)
			if !reflect.DeepEqual(tt.runner.calls, tt.calls) {
				t.Errorf("calls:\n got %q\nwant %q", tt.runner.calls, tt.calls)
			}
			for _, c := range tt.runner.calls {
				if c == "run "+iptAppend || c == "run "+enableXRDP || c == "run ufw allow 3389/tcp" {
					t.Errorf("check ran a mutating command: %q", c)
				}
			}
			var steps []string
			for _, s := range got.Steps {
				steps = append(steps, s.Name+"="+string(s.Outcome))
				if s.Success != s.Outcome.Succeeded() {
					t.Errorf("step %q: Success = %v but outcome %q", s.Name, s.Success, s.Outcome)
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
