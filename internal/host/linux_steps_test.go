package host

import (
	"reflect"
	"slices"
	"strings"
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
		{"source only", "ufw allow from 100.64.0.0/10", true},
		{"interface only", "ufw allow in on tailscale0", true},
		{"source only, tcp", "ufw allow from 100.64.0.0/10 proto tcp", true},
		{"source only, udp", "ufw allow from 100.64.0.0/10 proto udp", false},
		{"source port, no destination", "ufw allow from any port 1234", false},
		{"source app profile", "ufw allow from 100.64.0.0/10 app OpenSSH", false},
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
	iptRead    = "iptables -S INPUT"
	iptChain   = "iptables -S ts-input"
	iptAppend  = "iptables -A INPUT -p tcp --dport 3389 -j ACCEPT"
	isEnabled  = "systemctl is-enabled xrdp"
	isActive   = "systemctl is-active xrdp"
	enableXRDP = "systemctl enable --now xrdp"
)

// Verbatim `iptables -S INPUT` output: a Tailscale-scoped allow, a chain
// holding only tailscaled's jump, a port-less tailnet allow, and a hardened
// chain whose only allows are loopback and established traffic.
const (
	iptScoped       = "-P INPUT ACCEPT\n-A INPUT -j ts-input\n-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT\n"
	iptEmpty        = "-P INPUT ACCEPT\n-A INPUT -j ts-input\n"
	iptQualified    = "-P INPUT ACCEPT\n-A INPUT -i tailscale0 -j ts-input\n"
	iptTailnet      = "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -i lo -j ACCEPT\n-A INPUT -i tailscale0 -j ACCEPT\n"
	iptDropFirst    = "-P INPUT ACCEPT\n-A INPUT -j ts-input\n-A INPUT -p tcp -m tcp --dport 3389 -j DROP\n"
	iptAckOnly      = "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK ACK -j ACCEPT\n"
	iptLoopbackOnly = "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -i lo -j ACCEPT\n-A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT\n"
)

// tsInputDefault is tailscaled's ts-input chain as `iptables -S ts-input`
// prints it: tailscale.com v1.104.2's AddChains, addBase4, AddExternalCGNATRules,
// AddLoopbackRule and AddMagicsockPortRule replayed in that order with iptables
// 1.8.10 (nf_tables) in ubuntu:24.04 (#492). tsInputNoAccept is the same chain
// without the tailnet ACCEPT, so only rules that never decide remain.
const (
	tsInputDefault  = "-N ts-input\n-A ts-input -s 100.64.0.5/32 -i lo -j ACCEPT\n-A ts-input -i tailscale0 -j ACCEPT\n-A ts-input -s 100.115.92.0/23 ! -i tailscale0 -j RETURN\n-A ts-input -s 100.64.0.0/10 ! -i tailscale0 -j DROP\n-A ts-input -p udp -m udp --dport 41641 -j ACCEPT\n"
	tsInputNoAccept = "-N ts-input\n-A ts-input -s 100.64.0.5/32 -i lo -j ACCEPT\n-A ts-input -s 100.115.92.0/23 ! -i tailscale0 -j RETURN\n-A ts-input -s 100.64.0.0/10 ! -i tailscale0 -j DROP\n-A ts-input -p udp -m udp --dport 41641 -j ACCEPT\n"
)

var xrdpReads = []string{"lookpath systemctl", "output " + isEnabled, "output " + isActive}

// TestIptablesCovers feeds verbatim `iptables -S INPUT` lines (iptables 1.8.10,
// nf_tables) to the parser.
func TestIptablesCovers(t *testing.T) {
	tests := []struct {
		name string
		rule string
		want bool
	}{
		{"exact", "-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT", true},
		{"interface scoped", "-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT", true},
		{"source scoped", "-A INPUT -s 100.64.0.0/10 -p tcp -m tcp --dport 3389 -j ACCEPT", true},
		{"negated interface", "-A INPUT ! -i lo -p tcp -m tcp --dport 3389 -j ACCEPT", true},
		{"multiport list", "-A INPUT -p tcp -m multiport --dports 22,3389 -j ACCEPT", true},
		{"multiport range in list", "-A INPUT -p tcp -m multiport --dports 3380:3390,22 -j ACCEPT", true},
		{"range", "-A INPUT -p tcp -m tcp --dport 3380:3390 -j ACCEPT", true},
		{"source or destination ports", "-A INPUT -p tcp -m multiport --ports 3389 -j ACCEPT", true},
		{"conntrack before the port", "-A INPUT -p tcp -m conntrack --ctstate NEW -m tcp --dport 3389 -j ACCEPT", true},
		{"comment", `-A INPUT -p tcp -m tcp --dport 3389 -m comment --comment "rdp port" -j ACCEPT`, true},
		{"longer port", "-A INPUT -p tcp -m tcp --dport 33890 -j ACCEPT", false},
		{"udp", "-A INPUT -p udp -m udp --dport 3389 -j ACCEPT", false},
		{"source port", "-A INPUT -p tcp -m tcp --sport 3389 -j ACCEPT", false},
		{"negated port", "-A INPUT -p tcp -m tcp ! --dport 3389 -j ACCEPT", false},
		{"negated multiport", "-A INPUT -p tcp -m multiport ! --dports 22,3389 -j ACCEPT", false},
		{"drop", "-A INPUT -p tcp -m tcp --dport 3389 -j DROP", false},
		{"jump", "-A INPUT -j ts-input", false},
		{"no destination port", "-A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", false},
		{"comment holding -j ACCEPT", `-A INPUT -p tcp -m tcp --dport 3389 -m comment --comment "x -j ACCEPT" -j ACCEPT`, true},
		{"comment port never counts", `-A INPUT -p tcp -m tcp --dport 22 -m comment --comment "say \"hi\" --dport 3389 -j ACCEPT" -j ACCEPT`, false},
		{"quoted string holding -j DROP", `-A INPUT -p tcp -m tcp --dport 3389 -m string --string "marker -j DROP here" --algo bm --to 65535 -j ACCEPT`, true},
		{"quoted string holding -j ACCEPT on a drop", `-A INPUT -p tcp -m tcp --dport 3389 -m string --string "x -j ACCEPT" --algo bm --to 65535 -j DROP`, false},
		{"policy only", "-P INPUT ACCEPT", false},
		// A rule with no port flag covers every TCP port, unless it is loopback,
		// non-TCP, or narrowed by a match the parser does not model.
		{"port-less blanket", "-A INPUT -j ACCEPT", true},
		{"port-less interface", "-A INPUT -i tailscale0 -j ACCEPT", true},
		{"port-less source", "-A INPUT -s 100.64.0.0/10 -j ACCEPT", true},
		{"port-less tcp", "-A INPUT -p tcp -j ACCEPT", true},
		{"port-less negated loopback interface", "-A INPUT ! -i lo -j ACCEPT", true},
		{"port-less negated loopback source", "-A INPUT ! -s 127.0.0.0/8 -j ACCEPT", true},
		{"port-less tcp flags", "-A INPUT -p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK SYN -j ACCEPT", true},
		{"port-less comment", "-A INPUT -i tailscale0 -m comment --comment tailnet -j ACCEPT", true},
		{"port-less loopback interface", "-A INPUT -i lo -j ACCEPT", false},
		{"port-less loopback interface wildcard", "-A INPUT -i lo+ -j ACCEPT", false},
		{"port-less loopback source", "-A INPUT -s 127.0.0.0/8 -j ACCEPT", false},
		{"port-less loopback destination", "-A INPUT -d 127.0.0.1/32 -j ACCEPT", false},
		{"port-less ipv6 loopback source", "-A INPUT -s ::1/128 -j ACCEPT", false},
		{"port-less udp", "-A INPUT -p udp -j ACCEPT", false},
		{"port-less icmp", "-A INPUT -p icmp -j ACCEPT", false},
		{"port-less negated tcp", "-A INPUT ! -p tcp -j ACCEPT", false},
		{"port-less state", "-A INPUT -m state --state NEW -j ACCEPT", false},
		{"port-less limit", "-A INPUT -p tcp -m limit --limit 5/min -j ACCEPT", false},
		{"port-less recent", "-A INPUT -m recent --rcheck --name x --mask 255.255.255.255 --rsource -j ACCEPT", false},
		{"port-less drop", "-A INPUT -i tailscale0 -j DROP", false},
		// Narrowing the parser does not model reads as absent, the visible failure.
		{"port-less ack-only tcp flags", "-A INPUT -p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK ACK -j ACCEPT", false},
		{"port-less syn+ack tcp flags", "-A INPUT -p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK SYN,ACK -j ACCEPT", false},
		{"port-less other tcp flags mask", "-A INPUT -p tcp -m tcp --tcp-flags SYN,RST SYN -j ACCEPT", false},
		{"port-less fragment", "-A INPUT -f -j ACCEPT", false},
		{"port-less negated tailnet source", "-A INPUT ! -s 100.64.0.0/10 -j ACCEPT", false},
		{"port-less negated tailnet interface", "-A INPUT ! -i tailscale0 -j ACCEPT", false},
		{"loopback interface with the port", "-A INPUT -i lo -p tcp -m tcp --dport 3389 -j ACCEPT", false},
		{"loopback source with the port", "-A INPUT -s 127.0.0.0/8 -p tcp -m tcp --dport 3389 -j ACCEPT", false},
		{"among others", "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -p tcp -m tcp --dport 22 -j ACCEPT\n-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := iptablesCovers(tt.rule+"\n", 3389); got != tt.want {
				t.Errorf("iptablesCovers(%q) = %v, want %v", tt.rule, got, tt.want)
			}
		})
	}
}

// TestIptablesRule pins the iptables read (AC2, AC5): `iptables -S INPUT` either
// prints the rules or fails, and a failure is an unknown state.
func TestIptablesRule(t *testing.T) {
	tests := []struct {
		name   string
		runner *recordingRunner
		want   ruleState
		reason string // a substring an unknown read's reason must carry
	}{
		{"present", &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, output: map[string]string{iptRead: iptScoped}}, rulePresent, ""},
		{"absent behind a ts-input jump", &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, output: map[string]string{iptRead: iptEmpty}}, ruleAbsent, ""},
		{"present inside ts-input", &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, output: map[string]string{iptRead: iptEmpty, iptChain: tsInputDefault}}, rulePresent, ""},
		{"a deny after the jump is shadowed by ts-input", &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, output: map[string]string{iptRead: iptDropFirst, iptChain: tsInputDefault}}, rulePresent, ""},
		{"ts-input unreadable", &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, output: map[string]string{iptRead: iptEmpty}, exit: map[string]int{iptChain: 1}}, ruleUnknown, iptChain},
		{"permission", &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, exit: map[string]int{iptRead: 4}}, ruleUnknown, ""},
		{"no exit status", &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, fail: map[string]bool{iptRead: true}}, ruleUnknown, ""},
		{"no backend", &recordingRunner{elevated: true}, ruleUnknown, ""},
		{"not root", &recordingRunner{onPath: map[string]bool{"iptables": true}}, ruleUnknown, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readLinuxFirewall(tt.runner, 3389)
			if got.state != tt.want {
				t.Errorf("state = %v, want %v (reason %q)", got.state, tt.want, got.reason)
			}
			if !strings.Contains(got.reason, tt.reason) {
				t.Errorf("reason %q does not name %q", got.reason, tt.reason)
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
		{"indirect unit: an answer ts-bridge does not decide on", &recordingRunner{onPath: map[string]bool{"systemctl": true},
			output: map[string]string{isEnabled: "indirect\n", isActive: "active\n"}},
			xrdpState{installed: true, active: true, undecided: "indirect"}},
		{"generated unit: an answer ts-bridge does not decide on", &recordingRunner{onPath: map[string]bool{"systemctl": true},
			output: map[string]string{isEnabled: "generated\n", isActive: "active\n"}},
			xrdpState{installed: true, active: true, undecided: "generated"}},
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
		blocked  string // the rule a manual firewall step must name
		message  string // a substring the firewall step's message must carry
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
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptScoped})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeOK, OutcomeOK},
		},
		{
			name:     "iptables, rule missing",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptEmpty})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptAppend, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "iptables, port-less tailnet allow",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptTailnet})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeOK, OutcomeOK},
		},
		{
			name:     "iptables, only loopback and established allowed",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptLoopbackOnly})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptAppend, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "iptables, only an established-traffic tcp-flags allow",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptAckOnly})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptAppend, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "ufw, a deny decides the port first",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + "ufw deny 3389\nufw allow 3389/tcp\n"})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwAdded, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeManual, OutcomeOK},
			blocked:  "ufw deny 3389",
		},
		{
			name:     "iptables, a drop and no allow: nothing appended after it",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptDropFirst})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeManual, OutcomeOK},
			blocked:  "-A INPUT -p tcp -m tcp --dport 3389 -j DROP",
		},
		{
			name:     "iptables, tailscaled's ts-input accepts the tailnet",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptEmpty, iptChain: tsInputDefault})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeOK, OutcomeOK},
		},
		{
			name:     "iptables, ts-input unreadable: nothing appended",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptEmpty}), exit: map[string]int{iptChain: 1}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
			message:  iptChain,
		},
		{
			name:     "iptables, a deny after the jump is shadowed by ts-input",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptDropFirst, iptChain: tsInputDefault})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeOK, OutcomeOK},
		},
		{
			name:     "iptables, no ts-input jump: one read",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: "-P INPUT ACCEPT\n"})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "run "+iptAppend, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "iptables, a qualified ts-input jump: nothing appended",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptQualified, iptChain: tsInputDefault})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
			message:  "-A INPUT -i tailscale0 -j ts-input",
		},
		{
			name:     "iptables -S fails",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(nil), exit: map[string]int{iptRead: 2}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output tailscale ip -4"),
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
				assertNamesBlockedRule(t, s, tt.blocked)
				assertFirewallMessage(t, s, tt.message)
			}
			if !reflect.DeepEqual(outcomes, tt.outcomes) {
				t.Errorf("outcomes = %v, want %v", outcomes, tt.outcomes)
			}
		})
	}
}

// TestXRDPStepOutcomes pins the outcomes for xrdp states setup cannot act on:
// a masked unit needs a person to unmask it, and an is-enabled answer ts-bridge
// does not interpret is reported as such, never guessed.
func TestXRDPStepOutcomes(t *testing.T) {
	masked := map[string]string{isEnabled: "masked\n", isActive: "inactive\n"}
	indirect := map[string]string{isEnabled: "indirect\n", isActive: "active\n"}
	tests := []struct {
		name         string
		output       map[string]string
		setup, check Outcome
		fixHas       string
	}{
		{"masked", masked, OutcomeFailed, OutcomeManual, "unmask"},
		{"indirect", indirect, OutcomeFailed, OutcomeUnverified, "systemctl status xrdp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recordingRunner{elevated: true, onPath: map[string]bool{"systemctl": true}, output: tt.output}
			s := xrdpSetupStep(r)
			if s.Outcome != tt.setup || !strings.Contains(s.Fix, tt.fixHas) {
				t.Errorf("setup = %s (fix %q), want %s with %q", s.Outcome, s.Fix, tt.setup, tt.fixHas)
			}
			if slices.Contains(r.calls, "run "+enableXRDP) {
				t.Errorf("setup ran enable on a unit it cannot act on: %v", r.calls)
			}
			c := xrdpCheckStep(readXRDP(r))
			if c.Outcome != tt.check || !strings.Contains(c.Fix, tt.fixHas) {
				t.Errorf("check = %s (fix %q), want %s with %q", c.Outcome, c.Fix, tt.check, tt.fixHas)
			}
		})
	}
}

// TestCheckForLinux pins check as the read half of setup (AC1, AC5): the same
// reads, no run call (the iptables read is `-S` through Output), and an unreadable state
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
		blocked  string // the rule a manual firewall step must name
		message  string // a substring the firewall step's message must carry
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
			name:     "iptables, scoped allow",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptScoped})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=ok"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes, FirewallOK: true}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true, FirewallOK: true},
		},
		{
			name:     "iptables, port-less tailnet allow",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptTailnet})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=ok"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes, FirewallOK: true}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true, FirewallOK: true},
		},
		{
			name:     "iptables, only loopback and established allowed",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptLoopbackOnly})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
		},
		{
			name:     "iptables, a drop decides the port first",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptDropFirst + "-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT\n"})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=manual"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			blocked:  "-A INPUT -p tcp -m tcp --dport 3389 -j DROP",
		},
		{
			name:     "iptables, tailscaled's ts-input accepts the tailnet",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptEmpty, iptChain: tsInputDefault})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=ok"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes, FirewallOK: true}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true, FirewallOK: true},
		},
		{
			name:     "iptables, ts-input unreadable",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptEmpty}), exit: map[string]int{iptChain: 1}},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=unverified"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  iptChain,
		},
		{
			name:     "iptables, no ts-input jump: one read",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: "-P INPUT ACCEPT\n"})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
		},
		{
			name:     "iptables, a qualified ts-input jump",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptQualified, iptChain: tsInputDefault})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=unverified"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "-A INPUT -i tailscale0 -j ts-input",
		},
		{
			name:     "iptables -S fails",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(nil), exit: map[string]int{iptRead: 2}},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=unverified"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
		},
		{
			name:     "xrdp not installed",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, output: linuxUp(map[string]string{iptRead: iptEmpty})},
			calls:    []string{"output tailscale ip -4", "lookpath systemctl", "lookpath xrdp", "lookpath ufw", "lookpath iptables", "output " + iptRead, "output " + iptChain},
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
				assertNamesBlockedRule(t, s, tt.blocked)
				assertFirewallMessage(t, s, tt.message)
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

// TestFirewallDecideOrder feeds each ruleset to both backends in their verbatim
// syntax (ufw 0.36.2 `show added`, iptables 1.8.10 `-S INPUT`) and expects the
// same answer: the first rule that decides the port wins, and only an unscoped
// deny decides against it (#481).
func TestFirewallDecideOrder(t *testing.T) {
	tests := []struct {
		name string
		ufw  []string
		ipt  []string
		want ruleState
	}{
		{"deny before allow",
			[]string{"ufw deny 3389", "ufw allow 3389/tcp"},
			[]string{"-A INPUT -p tcp -m tcp --dport 3389 -j DROP", "-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"},
			ruleBlocked},
		{"reject before allow",
			[]string{"ufw reject 3380:3390/tcp", "ufw allow 3389/tcp"},
			[]string{"-A INPUT -p tcp -m tcp --dport 3380:3390 -j REJECT --reject-with icmp-port-unreachable", "-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"},
			ruleBlocked},
		{"deny and no allow",
			[]string{"ufw deny 3389/tcp"},
			[]string{"-A INPUT -p tcp -m tcp --dport 3389 -j DROP"},
			ruleBlocked},
		{"hardened: scoped deny, then scoped allow",
			[]string{"ufw deny from 192.168.0.0/16 to any port 3389 proto tcp", "ufw allow in on tailscale0 to any port 3389 proto tcp"},
			[]string{"-A INPUT -s 192.168.0.0/16 -p tcp -m tcp --dport 3389 -j DROP", "-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT"},
			rulePresent},
		{"interface-scoped deny, then allow",
			[]string{"ufw deny in on eth0 to any port 3389 proto tcp", "ufw allow 3389/tcp"},
			[]string{"-A INPUT -i eth0 -p tcp -m tcp --dport 3389 -j DROP", "-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"},
			rulePresent},
		{"destination-scoped deny, then allow",
			[]string{"ufw deny to 10.0.0.5 port 3389 proto tcp", "ufw allow 3389/tcp"},
			[]string{"-A INPUT -d 10.0.0.5/32 -p tcp -m tcp --dport 3389 -j DROP", "-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"},
			rulePresent},
		{"deny to any, then allow",
			[]string{"ufw deny to any port 3389 proto tcp", "ufw allow 3389/tcp"},
			[]string{"-A INPUT -p tcp -m tcp --dport 3389 -j DROP", "-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"},
			ruleBlocked},
		{"allow before deny",
			[]string{"ufw allow 3389/tcp", "ufw deny 3389"},
			[]string{"-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT", "-A INPUT -p tcp -m tcp --dport 3389 -j DROP"},
			rulePresent},
		{"deny on another port",
			[]string{"ufw deny 33890/tcp", "ufw allow 3389/tcp"},
			[]string{"-A INPUT -p tcp -m tcp --dport 33890 -j DROP", "-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"},
			rulePresent},
		{"udp deny",
			[]string{"ufw deny 3389/udp", "ufw allow 3389/tcp"},
			[]string{"-A INPUT -p udp -m udp --dport 3389 -j DROP", "-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"},
			rulePresent},
		{"outbound deny",
			[]string{"ufw deny out 3389/tcp", "ufw allow 3389/tcp"},
			[]string{"-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"},
			rulePresent},
		{"deny with a comment naming an interface",
			[]string{"ufw deny 3389/tcp comment 'blocked on purpose'", "ufw allow in on tailscale0"},
			[]string{`-A INPUT -p tcp -m tcp --dport 3389 -m comment --comment "blocked on purpose" -j DROP`, "-A INPUT -i tailscale0 -j ACCEPT"},
			ruleBlocked},
		{"nothing",
			[]string{"ufw allow 22/tcp"},
			[]string{"-A INPUT -p tcp -m tcp --dport 22 -j ACCEPT"},
			ruleAbsent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := ufwDecide(ufwAddedHeader+strings.Join(tt.ufw, "\n")+"\n", 3389); got != tt.want {
				t.Errorf("ufw: got %v, want %v", got, tt.want)
			}
			if got, _ := iptablesDecide("-P INPUT DROP\n-A INPUT -j ts-input\n"+strings.Join(tt.ipt, "\n")+"\n", 3389); got != tt.want {
				t.Errorf("iptables: got %v, want %v", got, tt.want)
			}
		})
	}
}

// assertNamesBlockedRule checks that a manual firewall step names the deny
// that decided the port, so the operator knows which rule to remove (#481).
func assertNamesBlockedRule(t *testing.T, s SetupStep, rule string) {
	t.Helper()
	if s.Name != linuxFwName || s.Outcome != OutcomeManual {
		return
	}
	if rule == "" || !strings.Contains(s.Message, rule) {
		t.Errorf("step %q message %q does not name the blocking rule %q", s.Name, s.Message, rule)
	}
}

// assertFirewallMessage checks that the firewall step's message carries want,
// such as the command whose read failed (#492).
func assertFirewallMessage(t *testing.T, s SetupStep, want string) {
	t.Helper()
	if s.Name == linuxFwName && !strings.Contains(s.Message, want) {
		t.Errorf("step %q message %q does not contain %q", s.Name, s.Message, want)
	}
}

// TestIptablesDecideOnly pins the iptables shapes ufw cannot express.
func TestIptablesDecideOnly(t *testing.T) {
	tests := []struct {
		name string
		rule string
		want ruleState
	}{
		{"blanket drop before the allow", "-A INPUT -j DROP\n-A INPUT -i tailscale0 -j ACCEPT", ruleBlocked},
		{"tcp reset before the allow", "-A INPUT -p tcp -j REJECT --reject-with tcp-reset\n-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT", ruleBlocked},
		{"conntrack drop does not decide", "-A INPUT -m conntrack --ctstate INVALID -j DROP\n-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT", rulePresent},
		// An unmodeled match on a port-bearing rule is ignored for a deny as for an
		// allow (iptPortCovers), so this deny decides: parity, not precision.
		{"port-bearing conntrack drop decides", "-A INPUT -p tcp -m tcp --dport 3389 -m conntrack --ctstate INVALID -j DROP\n-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT", ruleBlocked},
		{"negated-scope drop does not decide", "-A INPUT ! -i tailscale0 -p tcp -m tcp --dport 3389 -j DROP\n-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT", rulePresent},
		{"loopback drop does not decide", "-A INPUT -i lo -j DROP\n-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT", rulePresent},
		{"jump does not decide", "-A INPUT -j ts-input\n-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT", rulePresent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rule := iptablesDecide(tt.rule+"\n", 3389)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
			if (got == ruleBlocked) != (rule != "") {
				t.Errorf("blocking rule = %q for state %v", rule, got)
			}
		})
	}
}

// TestIptablesTSInput pins how tailscaled's ts-input chain decides (#492): its
// rules count at the position of an unqualified `-A INPUT -j ts-input`. A
// qualified jump never reaches this walk (TestIptQualifiedTSInputJump).
// Chains are in printed form (tsInputDefault).
func TestIptablesTSInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		chain string
		want  ruleState
	}{
		{"tailscaled default", iptEmpty, tsInputDefault, rulePresent},
		{"deny before the jump decides first", "-P INPUT ACCEPT\n-A INPUT -p tcp -m tcp --dport 3389 -j DROP\n-A INPUT -j ts-input\n", tsInputDefault, ruleBlocked},
		{"deny after the jump is shadowed", iptDropFirst, tsInputDefault, rulePresent},
		{"tailscaled's other rules decide nothing", iptEmpty, tsInputNoAccept, ruleAbsent},
		{"and an INPUT allow after the jump still counts", iptScoped, tsInputNoAccept, rulePresent},
		{"no jump, nothing to follow", "-P INPUT ACCEPT\n", tsInputDefault, ruleAbsent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := iptablesDecide(iptInlineChain(tt.input, tt.chain), 3389); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIptQualifiedTSInputJump pins which INPUT rules make the read unknown
// (#532): any jump or goto into ts-input other than tailscaled's unqualified
// one. A quoted comment that spells a jump is not one.
func TestIptQualifiedTSInputJump(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"tailscaled's unqualified jump", iptEmpty, ""},
		{"no jump", "-P INPUT ACCEPT\n", ""},
		{"interface-scoped jump", iptQualified, "-A INPUT -i tailscale0 -j ts-input"},
		{"source-scoped jump", "-A INPUT -s 100.64.0.0/10 -j ts-input\n", "-A INPUT -s 100.64.0.0/10 -j ts-input"},
		{"goto into ts-input", "-A INPUT -g ts-input\n", "-A INPUT -g ts-input"},
		{"a qualified jump next to the exact one", "-A INPUT -j ts-input\n-A INPUT -i eth0 -j ts-input\n", "-A INPUT -i eth0 -j ts-input"},
		{"a comment spelling the jump", `-A INPUT -p tcp -m comment --comment "see -j ts-input above" -j DROP` + "\n", ""},
		{"another chain", "-A INPUT -i eth0 -j ts-input-extra\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := iptQualifiedTSInputJump(tt.input); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
