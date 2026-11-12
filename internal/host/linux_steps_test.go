package host

import (
	"cmp"
	"errors"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The `ufw show added` lines below are verbatim from ufw 0.36.2 on Ubuntu
// 24.04 (specs/BUG-034-linux-host-idempotent, measured 2026-10-22).
const ufwAddedHeader = "Added user rules (see 'ufw status' for running firewall):\n"

// ufwScopedAllow is the rule setup writes, as `ufw show added` prints it;
// ufwLegacyAllow is the unscoped one earlier releases wrote (#631).
const (
	ufwScopedAllow = "ufw allow in on tailscale0 to any port 3389 proto tcp\n"
	ufwLegacyAllow = "ufw allow 3389/tcp\n"
	ufwAllowCmd    = "ufw allow in on tailscale0 to any port 3389 proto tcp"
	ufwDeleteCmd   = "ufw delete allow 3389/tcp"
)

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
	ufwStatus  = "ufw status"
	ufwActive  = "Status: active\n"
	iptRead    = "iptables -S INPUT"
	iptChain   = "iptables -S ts-input"
	iptAppend  = "iptables -A INPUT -i tailscale0 -p tcp --dport 3389 -j ACCEPT"
	iptDelete  = "iptables -D INPUT -p tcp --dport 3389 -j ACCEPT"
	isEnabled  = "systemctl is-enabled xrdp"
	isActive   = "systemctl is-active xrdp"
	enableXRDP = "systemctl enable --now xrdp"
)

// `iptables -S INPUT` output: a Tailscale-scoped allow, a chain holding only
// tailscaled's jump, a port-less tailnet allow, and a hardened chain whose only
// allows are loopback and established traffic. The policy is DROP: under ACCEPT
// every step is manual (#691), which TestPermissiveInputPolicy covers.
const (
	iptScoped       = "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT\n"
	iptEmpty        = "-P INPUT DROP\n-A INPUT -j ts-input\n"
	iptQualified    = "-P INPUT DROP\n-A INPUT -i tailscale0 -j ts-input\n"
	iptTailnet      = "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -i lo -j ACCEPT\n-A INPUT -i tailscale0 -j ACCEPT\n"
	iptDropFirst    = "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -p tcp -m tcp --dport 3389 -j DROP\n"
	iptAckOnly      = "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK ACK -j ACCEPT\n"
	iptLoopbackOnly = "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -i lo -j ACCEPT\n-A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT\n"
)

// iptSetupRule is the rule `host setup` appends, as `iptables -S INPUT` prints
// it (iptables 1.8.10 adds `-m tcp`; recorded end to end in BUG-492).
// iptLegacyRule is the unscoped rule setup appended before #631, and on every
// run before #477, so old hosts carry copies (#480).
const (
	iptSetupRule  = "-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT\n"
	iptLegacyRule = "-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT\n"
	// iptForeignRule is an unscoped allow for the port that setup never wrote.
	iptForeignRule    = "-A INPUT -p tcp -m multiport --dports 22,3389 -j ACCEPT\n"
	iptForeignPrinted = "-A INPUT -p tcp -m multiport --dports 22,3389 -j ACCEPT"
	iptTwoCopies      = iptEmpty + iptLegacyRule + iptLegacyRule
	iptCopiesFix      = "iptables -D INPUT -p tcp --dport 3389 -j ACCEPT` 1 time"
	iptCopiesNote     = "2 copies of -A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"
)

// tsInputDefault is tailscaled's ts-input chain as `iptables -S ts-input`
// prints it, verbatim from tailscaled 1.104.4 (the tailscale.com of go.mod) in
// netfilter mode on, iptables 1.8.10 (nf_tables), Ubuntu 24.04, enrolled against
// a local Headscale (scripts/e2e/capture-tsfw.sh, #526). tsInputNoAccept is the
// same chain without the tailnet ACCEPT, so only rules that never decide remain.
// tsInputNFTMode is the chain tailscaled builds in nftables mode
// (TS_DEBUG_FIREWALL_MODE=nftables), as iptables-nft prints it: the interface
// is the wildcard tailscale0+, and INPUT jumps to it as in mode on.
const (
	tsInputDefault   = "-N ts-input\n-A ts-input -s 100.64.0.1/32 -i lo -j ACCEPT\n-A ts-input -i tailscale0 -j ACCEPT\n-A ts-input -s 100.115.92.0/23 ! -i tailscale0 -j RETURN\n-A ts-input -s 100.64.0.0/10 ! -i tailscale0 -j DROP\n"
	tsInputNoAccept  = "-N ts-input\n-A ts-input -s 100.64.0.1/32 -i lo -j ACCEPT\n-A ts-input -s 100.115.92.0/23 ! -i tailscale0 -j RETURN\n-A ts-input -s 100.64.0.0/10 ! -i tailscale0 -j DROP\n"
	tsInputNFTMode   = "-N ts-input\n-A ts-input -s 100.64.0.1/32 -i lo+ -j ACCEPT\n-A ts-input -i tailscale0+ -j ACCEPT\n-A ts-input -s 100.115.92.0/23 ! -i tailscale0+ -j RETURN\n-A ts-input -s 100.64.0.0/10 ! -i tailscale0+ -j DROP\n"
	iptTSInputJumped = "-P INPUT DROP\n-A INPUT -j ts-input\n"
)

// A ufw host's `iptables -S INPUT` with tailscaled's jump inserted, verbatim
// from ufw 0.36.2 and iptables 1.8.10 (nf_tables) in alpine:3.20, ufw enabled
// with `ufw allow 22/tcp` (specs/BUG-530-ufw-reads-ts-input/verification.md).
// ufw lists none of it in `ufw show added` (#530). ufwHostCopies adds setup's
// own rule twice, as on a host set up before #477 that installed ufw later.
const (
	ufwHostAdded  = ufwAddedHeader + "ufw allow 22/tcp\n"
	ufwHostInput  = "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -j ufw-before-logging-input\n-A INPUT -j ufw-before-input\n-A INPUT -j ufw-after-input\n-A INPUT -j ufw-after-logging-input\n-A INPUT -j ufw-reject-input\n-A INPUT -j ufw-track-input\n"
	ufwHostCopies = ufwHostInput + iptLegacyRule + iptLegacyRule
)

// ufwIpt is a ufw host: ufw manages the rules, iptables is the binary under it.
var ufwIpt = map[string]bool{"ufw": true, "iptables": true}

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
		// #530: ufw does not list tailscaled's rules, so a ufw host reads iptables too.
		{"ufw host, present inside ts-input", &recordingRunner{elevated: true, onPath: ufwIpt, output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwHostAdded, iptRead: ufwHostInput, iptChain: tsInputDefault}}, rulePresent, ""},
		{"ufw host, nothing covers", &recordingRunner{elevated: true, onPath: ufwIpt, output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwHostAdded, iptRead: ufwHostInput, iptChain: tsInputNoAccept}}, ruleAbsent, ""},
		{"ufw host, iptables unreadable", &recordingRunner{elevated: true, onPath: ufwIpt, output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwHostAdded}, exit: map[string]int{iptRead: 4}}, ruleUnknown, iptRead},
		{"ufw host, a qualified jump", &recordingRunner{elevated: true, onPath: ufwIpt, output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwHostAdded, iptRead: iptQualified}}, ruleUnknown, "ts-input"},
		{"ufw host, a ufw allow decides first", &recordingRunner{elevated: true, onPath: ufwIpt, output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwAddedHeader + ufwScopedAllow}, exit: map[string]int{iptRead: 4}}, rulePresent, ""},
		{"ufw host, a ufw deny decides first", &recordingRunner{elevated: true, onPath: ufwIpt, output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwAddedHeader + "ufw deny 3389/tcp\n", iptRead: ufwHostInput, iptChain: tsInputDefault}}, ruleBlocked, ""},
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
	out := map[string]string{"tailscale ip -4": "100.64.0.5\n", isEnabled: "enabled\n", isActive: "active\n", ufwStatus: ufwActive}
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
	ufwHost := map[string]bool{"xrdp": true, "systemctl": true, "ufw": true, "iptables": true}
	tests := []struct {
		name     string
		runner   *recordingRunner
		calls    []string
		outcomes []Outcome
		blocked  string // the rule a manual firewall step must name
		message  string // a substring the firewall step's message must carry
		fix      string // a substring the firewall step's fix must carry
	}{
		{
			name:     "ufw, converged",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwScopedAllow})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeOK, OutcomeOK},
		},
		{
			name:     "ufw, tailscaled's ts-input admits the tailnet: no ufw allow (#530)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwHostAdded, iptRead: ufwHostInput, iptChain: tsInputDefault})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeOK, OutcomeOK},
			message:  "Firewall allows TCP 3389 (iptables)",
		},
		{
			name:     "ufw unreadable, a leftover copy: setup names it as check does, no write (#576)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{iptRead: ufwHostInput + iptLegacyRule, iptChain: tsInputDefault}), fail: map[string]bool{ufwAdded: true}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
			message:  "it also holds 1 copy of -A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT",
			fix:      "iptables -D INPUT -p tcp --dport 3389 -j ACCEPT` 1 time",
		},
		{
			name:     "ufw, a leftover iptables copy while ts-input admits the tailnet: copies deleted, nothing added (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwHostAdded, iptRead: ufwHostCopies, iptChain: tsInputDefault})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptDelete, "run "+iptDelete, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "ufw, one leftover copy beside a scoped ufw allow: deleted (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwScopedAllow, iptRead: ufwHostInput + iptLegacyRule, iptChain: tsInputDefault})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptDelete, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "ufw, a legacy ufw allow and nothing scoped: scoped allow first, then the legacy one deleted (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwLegacyAllow, iptRead: ufwHostInput, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+ufwAllowCmd, "run "+ufwDeleteCmd, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
			message:  "replaced",
		},
		{
			name:     "ufw, a legacy ufw allow beside a scoped one: deleted only (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwScopedAllow + ufwLegacyAllow, iptRead: ufwHostInput, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+ufwDeleteCmd, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "ufw, a legacy ufw allow while ts-input admits the tailnet: deleted, nothing added (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwLegacyAllow, iptRead: ufwHostInput, iptChain: tsInputDefault})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+ufwDeleteCmd, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "ufw, an unscoped ufw allow setup did not write: scoped allow added, nothing deleted, manual (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + "ufw allow 22,3389/tcp\n", iptRead: ufwHostInput, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+ufwAllowCmd, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeManual, OutcomeOK},
			message:  "legacy unscoped rule: ufw allow 22,3389/tcp",
			fix:      "setup does not delete",
		},
		{
			name:     "ufw, adding the scoped allow fails: the legacy allow stays (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwLegacyAllow, iptRead: ufwHostInput, iptChain: tsInputNoAccept}), fail: map[string]bool{ufwAllowCmd: true}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+ufwAllowCmd, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
			fix:      "tailscale0",
		},
		{
			name:     "ufw, deleting the legacy allow fails (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwLegacyAllow, iptRead: ufwHostInput, iptChain: tsInputNoAccept}), fail: map[string]bool{ufwDeleteCmd: true}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+ufwAllowCmd, "run "+ufwDeleteCmd, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
			fix:      ufwDeleteCmd,
		},
		{
			name:     "ufw, only the leftover copies admit the port: scoped ufw allow first, then the copies deleted (#576, #631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwHostAdded, iptRead: ufwHostCopies, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+ufwAllowCmd, "run "+iptDelete, "run "+iptDelete, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "ufw, nothing covers in either: ufw allow as before",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwHostAdded, iptRead: ufwHostInput, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+ufwAllowCmd, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "ufw, iptables unreadable: no write on a guess",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwHostAdded}), exit: map[string]int{iptRead: 4}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
			message:  iptRead,
		},
		{
			name:     "ufw, a qualified ts-input jump: no write on a guess",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwHostAdded, iptRead: iptQualified})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
			message:  "-A INPUT -i tailscale0 -j ts-input",
		},
		{
			name: "ufw, nothing configured",
			runner: &recordingRunner{elevated: true, onPath: ufw,
				output: linuxUp(map[string]string{isEnabled: "disabled\n", isActive: "inactive\n", ufwAdded: ufwAddedHeader + "ufw route allow in on tailscale0 to any port 3389 proto tcp\n"}),
				exit:   map[string]int{isEnabled: 1, isActive: 3}},
			calls: append(append([]string{}, xrdpReads...), "run "+enableXRDP, "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables",
				"run "+ufwAllowCmd, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeChanged, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "xrdp enabled, not running",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{isActive: "failed\n", ufwAdded: ufwAddedHeader + ufwScopedAllow}), exit: map[string]int{isActive: 3}},
			calls:    append(append([]string{}, xrdpReads...), "run "+enableXRDP, "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeChanged, OutcomeOK, OutcomeOK},
		},
		{
			name:     "xrdp masked: no enable, fix names unmask",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{isEnabled: "masked\n", isActive: "inactive\n", ufwAdded: ufwAddedHeader + ufwScopedAllow}), exit: map[string]int{isEnabled: 1, isActive: 3}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output tailscale ip -4"),
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
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output tailscale ip -4"),
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
			name:     "iptables, two legacy copies and nothing else admits: scoped rule first, then both deleted (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptTwoCopies, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptAppend, "run "+iptDelete, "run "+iptDelete, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
			message:  "replaced",
		},
		{
			name:     "iptables, a legacy copy beside a scoped allow: deleted, nothing added (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptScoped + iptLegacyRule, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptDelete, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "iptables, a legacy copy while ts-input admits the tailnet: deleted, nothing added (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptTwoCopies, iptChain: tsInputDefault})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptDelete, "run "+iptDelete, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "iptables, an unscoped rule setup did not write: scoped rule added, nothing deleted, manual (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptEmpty + iptForeignRule, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptAppend, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeManual, OutcomeOK},
			message:  "legacy unscoped rule: " + iptForeignPrinted,
			fix:      "setup does not delete",
		},
		{
			name:     "iptables, a legacy copy and a foreign rule: the copy replaced, the foreign one reported (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptEmpty + iptLegacyRule + iptForeignRule, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptAppend, "run "+iptDelete, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeManual, OutcomeOK},
			message:  "legacy unscoped rule: " + iptForeignPrinted,
			fix:      "setup does not delete",
		},
		{
			name:     "iptables, adding the scoped rule fails: the legacy copy stays (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptEmpty + iptLegacyRule, iptChain: tsInputNoAccept}), fail: map[string]bool{iptAppend: true}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptAppend, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
			fix:      "tailscale0",
		},
		{
			name:     "iptables, deleting the legacy copy fails (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptEmpty + iptLegacyRule, iptChain: tsInputNoAccept}), fail: map[string]bool{iptDelete: true}},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "run "+iptAppend, "run "+iptDelete, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
			message:  "legacy",
			fix:      iptDelete,
		},
		{
			name:     "iptables, no ts-input jump and no chain: appended",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: "-P INPUT DROP\n"}), exit: map[string]int{iptChain: 1}, stderr: noIptChain},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "lookpath nft", "run "+iptAppend, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeChanged, OutcomeOK},
		},
		{
			name:     "iptables, a qualified ts-input jump: nothing appended",
			runner:   &recordingRunner{elevated: true, onPath: ipt, output: linuxUp(map[string]string{iptRead: iptQualified, iptChain: tsInputDefault})},
			calls:    append(append([]string{}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output tailscale ip -4"),
			outcomes: []Outcome{OutcomeOK, OutcomeFailed, OutcomeOK},
			message:  "-A INPUT -i tailscale0 -j ts-input",
			fix:      "-A INPUT -i tailscale0 -j ts-input",
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
			calls:    []string{"lookpath systemctl", "lookpath xrdp", "lookpath ufw", "lookpath iptables", "lookpath nft", "output tailscale ip -4"},
			outcomes: []Outcome{OutcomeFailed, OutcomeFailed, OutcomeFailed},
		},
		{
			name:     "no systemctl",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "ufw": true}, output: linuxUp(map[string]string{ufwAdded: ufwScopedAllow})},
			calls:    []string{"lookpath systemctl", "lookpath xrdp", "lookpath ufw", "output " + ufwStatus, "output " + ufwAdded, "lookpath iptables", "output tailscale ip -4"},
			outcomes: []Outcome{OutcomeFailed, OutcomeOK, OutcomeOK},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setupFor("linux", Config{Port: 3389, FirewallRule: "Rule"}, tt.runner)
			if calls := withoutProbes(tt.runner.calls); !reflect.DeepEqual(calls, tt.calls) {
				t.Errorf("calls:\n got %q\nwant %q", calls, tt.calls)
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
				assertFirewallFix(t, s, tt.fix, "")
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
	ufwHost := map[string]bool{"xrdp": true, "systemctl": true, "ufw": true, "iptables": true}
	tests := []struct {
		name     string
		runner   *recordingRunner
		calls    []string
		steps    []string
		services []Service
		legacy   CheckResult
		blocked  string // the rule a manual firewall step must name
		message  string // a substring the firewall step's message must carry
		fix      string // a substring the firewall step's fix must carry
		absent   string // a substring the firewall step's message must not carry
	}{
		{
			name:     "converged",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwScopedAllow})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables"),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=ok"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes, FirewallOK: true}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true, FirewallOK: true},
		},
		{
			name: "firewalld running: unverified, not firewall_ok, ufw never read (#739 review)",
			runner: &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "ufw": true, "firewall-cmd": true},
				output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwScopedAllow, "firewall-cmd --state": "running\n"})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "output firewall-cmd --state"),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=unverified"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "firewalld is running",
		},
		{
			name:     "ufw host, iptables unreadable: unverified (#530 review)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwHostAdded}), exit: map[string]int{iptRead: 4}},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=unverified"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  iptRead,
		},
		{
			name:     "ufw host, an iptables DROP decides: credited to iptables (#530 review)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwHostAdded, iptRead: ufwHostInput + "-A INPUT -p tcp -m tcp --dport 3389 -j DROP\n", iptChain: tsInputNoAccept})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=manual"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			blocked:  "-A INPUT -p tcp -m tcp --dport 3389 -j DROP",
			message:  "before any allow (iptables)",
		},
		{
			name:     "ufw host, tailscaled's ts-input admits the tailnet (#530)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwHostAdded, iptRead: ufwHostInput, iptChain: tsInputDefault})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=ok"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes, FirewallOK: true}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true, FirewallOK: true},
		},
		{
			name:     "ufw host, two leftover copies in INPUT: setup would delete them (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwHostAdded, iptRead: ufwHostCopies, iptChain: tsInputDefault})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "legacy unscoped rule: " + iptCopiesNote,
			fix:      "ts-bridge host setup",
		},
		{
			name:     "ufw host, one leftover copy beside a scoped ufw allow (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwScopedAllow, iptRead: ufwHostInput + iptLegacyRule, iptChain: tsInputDefault})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "1 copy of -A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT",
			fix:      "ts-bridge host setup",
		},
		{
			name:     "ufw host, a legacy ufw allow (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + ufwLegacyAllow, iptRead: ufwHostInput, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "legacy unscoped rule: ufw allow 3389/tcp",
			fix:      "ts-bridge host setup",
		},
		{
			name:     "ufw host, an unscoped ufw allow setup did not write (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + "ufw allow 22,3389/tcp\n", iptRead: ufwHostInput, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=manual"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "legacy unscoped rule: ufw allow 22,3389/tcp",
			fix:      "setup does not delete",
		},
		{
			name:     "ufw host, the only allow is scoped off the tailnet (#631)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwAddedHeader + "ufw allow from 10.0.0.0/8 to any port 3389 proto tcp\n", iptRead: ufwHostInput, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "not on tailscale0",
			fix:      "ts-bridge host setup",
		},
		{
			name:     "ufw host, a qualified ts-input jump: unverified",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{ufwAdded: ufwHostAdded, iptRead: iptQualified})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=unverified"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "-A INPUT -i tailscale0 -j ts-input",
		},
		{
			name: "xrdp disabled, only a longer port allowed",
			runner: &recordingRunner{elevated: true, onPath: ufw,
				output: linuxUp(map[string]string{isEnabled: "disabled\n", isActive: "inactive\n", ufwAdded: ufwAddedHeader + "ufw allow 33890/tcp\n"}),
				exit:   map[string]int{isEnabled: 1, isActive: 3}},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables"),
			steps:    []string{"Tailscale IP=ok", "xrdp service=changed", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Active: &no}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true},
		},
		{
			name:     "xrdp enabled, not running",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{isActive: "failed\n", ufwAdded: ufwAddedHeader + ufwScopedAllow}), exit: map[string]int{isActive: 3}},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables"),
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
			calls:    []string{"output tailscale ip -4", "lookpath systemctl", "lookpath xrdp", "lookpath ufw", "output " + ufwStatus, "output " + ufwAdded, "lookpath iptables"},
			steps:    []string{"Tailscale IP=ok", "xrdp service=unverified", "Firewall configuration=unverified"},
			services: []Service{{Kind: "rdp", Port: 3389, Unread: true}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true},
		},
		{
			// What the e2e shim does: a systemctl that exits 1 and prints nothing is unread, not off (#638).
			name:     "systemctl present, answers nothing, ufw unreadable",
			runner:   &recordingRunner{elevated: true, onPath: ufw, output: linuxUp(map[string]string{isEnabled: "", isActive: ""}), exit: map[string]int{isEnabled: 1, isActive: 1}, fail: map[string]bool{ufwAdded: true}},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables"),
			steps:    []string{"Tailscale IP=ok", "xrdp service=unverified", "Firewall configuration=unverified"},
			services: []Service{{Kind: "rdp", Port: 3389, Unread: true}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true},
		},
		{
			name:     "ufw host, ufw unreadable, a leftover copy: still reported (#530)",
			runner:   &recordingRunner{elevated: true, onPath: ufwHost, output: linuxUp(map[string]string{iptRead: ufwHostInput + iptLegacyRule, iptChain: tsInputDefault}), fail: map[string]bool{ufwAdded: true}},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "output "+ufwStatus, "output "+ufwAdded, "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=unverified"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "it also holds 1 copy of -A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT",
			fix:      "iptables -D INPUT -p tcp --dport 3389 -j ACCEPT` 1 time",
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
			name:     "iptables, no ts-input jump and no chain",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: "-P INPUT DROP\n"}), exit: map[string]int{iptChain: 1}, stderr: noIptChain},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain, "lookpath nft"),
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
			fix:      "-A INPUT -i tailscale0 -j ts-input",
		},
		{
			name:     "iptables, two legacy copies: setup would replace them (#631)",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptTwoCopies, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "legacy unscoped rule: " + iptCopiesNote,
			fix:      "ts-bridge host setup",
		},
		{
			name:     "iptables, two legacy copies behind tailscaled's ts-input (#631)",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptTwoCopies, iptChain: tsInputDefault})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  iptCopiesNote,
			fix:      "ts-bridge host setup",
		},
		{
			name:     "iptables, three legacy copies (#631)",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptTwoCopies + iptLegacyRule, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "3 copies",
			fix:      "ts-bridge host setup",
		},
		{
			name:     "iptables, a legacy copy beside a scoped allow (#631)",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptScoped + iptLegacyRule, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=changed"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "1 copy of ",
			fix:      "ts-bridge host setup",
		},
		{
			name:     "iptables, an unscoped rule setup did not write: manual (#631)",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptScoped + iptForeignRule, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=manual"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			message:  "legacy unscoped rule: " + iptForeignPrinted,
			fix:      "setup does not delete",
		},
		{
			name:     "iptables, a drop before two copies",
			runner:   &recordingRunner{elevated: true, onPath: map[string]bool{"xrdp": true, "systemctl": true, "iptables": true}, output: linuxUp(map[string]string{iptRead: iptDropFirst + iptLegacyRule + iptLegacyRule, iptChain: tsInputNoAccept})},
			calls:    append(append([]string{"output tailscale ip -4"}, xrdpReads...), "lookpath ufw", "lookpath iptables", "output "+iptRead, "output "+iptChain),
			steps:    []string{"Tailscale IP=ok", "xrdp service=ok", "Firewall configuration=manual"},
			services: []Service{{Kind: "rdp", Port: 3389, Enabled: true, Active: &yes}},
			legacy:   CheckResult{TailscaleIP: "100.64.0.5", TailscaleUp: true, RDPPort: 3389, RDPEnabled: true},
			blocked:  "-A INPUT -p tcp -m tcp --dport 3389 -j DROP",
			absent:   "copies",
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
			if calls := withoutProbes(tt.runner.calls); !reflect.DeepEqual(calls, tt.calls) {
				t.Errorf("calls:\n got %q\nwant %q", calls, tt.calls)
			}
			for _, c := range tt.runner.calls {
				if c == "run "+iptAppend || c == "run "+enableXRDP || c == "run "+ufwAllowCmd || c == "run "+ufwDeleteCmd || c == "run "+iptDelete {
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
				assertFirewallFix(t, s, tt.fix, tt.absent)
			}
			if !reflect.DeepEqual(steps, tt.steps) {
				t.Errorf("steps = %q, want %q", steps, tt.steps)
			}
			if !reflect.DeepEqual(got.Services, tt.services) {
				t.Errorf("services = %+v, want %+v", got.Services, tt.services)
			}
			got.Steps, got.Services, got.Elevated = nil, nil, false // Elevated is the runner's, not a legacy key
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
	if rule == "" {
		if strings.Contains(s.Message, "blocks") {
			t.Errorf("step %q message %q reports a block the row does not name", s.Name, s.Message)
		}
		return
	}
	if !strings.Contains(s.Message, rule) {
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

// assertFirewallFix checks the firewall step's fix carries fix and its message
// does not carry absent (#480).
func assertFirewallFix(t *testing.T, s SetupStep, fix, absent string) {
	t.Helper()
	if s.Name != linuxFwName {
		return
	}
	if !strings.Contains(s.Fix, fix) {
		t.Errorf("step %q fix %q does not contain %q", s.Name, s.Fix, fix)
	}
	if absent != "" && strings.Contains(s.Message, absent) {
		t.Errorf("step %q message %q contains %q", s.Name, s.Message, absent)
	}
}

// TestIptablesSetupCopies pins which lines count as copies of the rule setup
// appends (#480): only its exact printed form, in INPUT, for this port.
func TestIptablesSetupCopies(t *testing.T) {
	tests := []struct {
		name  string
		rules string
		want  int
	}{
		{"none", iptEmpty, 0},
		{"one", iptEmpty + iptLegacyRule, 1},
		{"two", iptTwoCopies, 2},
		{"three, with a rule between", iptTwoCopies + "-A INPUT -i lo -j ACCEPT\n" + iptLegacyRule, 3},
		{"a scoped allow is not a copy", iptScoped + iptLegacyRule, 1},
		{"another port is not a copy", iptEmpty + "-A INPUT -p tcp -m tcp --dport 33890 -j ACCEPT\n" + iptLegacyRule, 1},
		{"a drop on the port is not a copy", iptDropFirst + iptLegacyRule, 1},
		{"ts-input rules are not copies", iptInlineChain(iptTwoCopies, tsInputDefault), 2},
		{"the same rule inside ts-input is not a copy", iptInlineChain(iptEmpty+iptLegacyRule, tsInputDefault+"-A ts-input -p tcp -m tcp --dport 3389 -j ACCEPT\n"), 1},
		{"setup's own command line is not the printed form", iptEmpty + "-A INPUT -p tcp --dport 3389 -j ACCEPT\n", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := iptLegacyCopies(tt.rules, 3389); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
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

// TestIptQualifiedTSInputJump pins which INPUT rules the walk cannot pass
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
			if _, got := iptBeforeQualifiedJump(tt.input); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestReadIptablesQualifiedJump pins the walk up to a qualified jump into
// ts-input (#532): what the rules before it decide stands, and when they
// decide nothing the read is unknown, with a fix that names the jump.
func TestReadIptablesQualifiedJump(t *testing.T) {
	const jump = "-A INPUT -i tailscale0 -j ts-input"
	tests := []struct {
		name    string
		input   string
		want    ruleState // when the read decides
		unknown bool
	}{
		{"nothing before the jump", iptQualified, 0, true},
		{"a deny before the jump decides", "-A INPUT -p tcp -m tcp --dport 3389 -j DROP\n" + jump + "\n", ruleBlocked, false},
		{"an allow before the jump decides", "-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT\n" + jump + "\n", rulePresent, false},
		{"tailscaled's jump before it decides", "-A INPUT -j ts-input\n" + jump + "\n", rulePresent, false},
		{"a deny after the jump does not", jump + "\n-A INPUT -p tcp -m tcp --dport 3389 -j DROP\n", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recordingRunner{output: map[string]string{iptRead: tt.input, iptChain: tsInputDefault}}
			got := readIptables(r, 3389)
			if (got.reason != "") != tt.unknown || (!tt.unknown && got.state != tt.want) {
				t.Fatalf("got state %v reason %q, want state %v unknown %v", got.state, got.reason, tt.want, tt.unknown)
			}
			if fix := firewallFix(got, 3389, "setup"); tt.unknown && !strings.Contains(fix, jump) {
				t.Errorf("fix %q does not name the jump %q", fix, jump)
			}
		})
	}
}

// TestReadIptablesQualifiedJumpCopies pins that the cut at a qualified jump
// limits what decides, not what is counted: copies of setup's rule behind the
// jump are still copies (#480, #532).
func TestReadIptablesQualifiedJumpCopies(t *testing.T) {
	input := iptLegacyRule + "-A INPUT -i tailscale0 -j ts-input\n" + iptLegacyRule
	r := &recordingRunner{output: map[string]string{iptRead: input, iptChain: tsInputDefault}}
	if got := readIptables(r, 3389); got.state != rulePresent || got.copies != 2 {
		t.Fatalf("got state %v copies %d, want present with 2 copies", got.state, got.copies)
	}
}

// TestIptRules pins how `iptables -S` output is cut into rules (#540): a
// newline inside a quoted value does not end a rule, and an unterminated quote
// leaves the output as printed rather than swallowing the rules after it.
func TestIptRules(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"plain rules", "-P INPUT ACCEPT\n-A INPUT -j ts-input\n", "-P INPUT ACCEPT\n-A INPUT -j ts-input\n"},
		{"a newline in a comment", "-A INPUT -m comment --comment \"a\nb\" -j DROP\n-A INPUT -j ts-input\n", "-A INPUT -m comment --comment \"a b\" -j DROP\n-A INPUT -j ts-input\n"},
		{"an escaped quote before the newline", "-A INPUT -m comment --comment \"a\\\"\nb\" -j DROP\n", "-A INPUT -m comment --comment \"a\\\" b\" -j DROP\n"},
		{"an escaped backslash ends no quote", "-A INPUT -m comment --comment \"a\\\\\" -j DROP\n-A INPUT -m comment --comment \"c\nd\" -j LOG\n", "-A INPUT -m comment --comment \"a\\\\\" -j DROP\n-A INPUT -m comment --comment \"c d\" -j LOG\n"},
		{"an escaped backslash before the newline", "-A INPUT -m comment --comment \"a\\\\\nb\" -j DROP\n", "-A INPUT -m comment --comment \"a\\\\ b\" -j DROP\n"},
		{"an unterminated quote", "-A INPUT -m comment --comment \"a\n-A INPUT -j DROP\n", "-A INPUT -m comment --comment \"a\n-A INPUT -j DROP\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := iptRules(tt.input); got != tt.want {
				t.Errorf("iptRules(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestReadIptablesMultilineComment pins that text after a newline in a
// comment is never read as a rule (#540): not as an allow, which would fail
// open, and not as a qualified jump into ts-input, which would fail closed.
func TestReadIptablesMultilineComment(t *testing.T) {
	tests := []struct {
		name, input, chain string
	}{
		{"a comment that spells an allow", "-P INPUT ACCEPT\n-A INPUT -m comment --comment \"see\n-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT \" -j LOG\n", ""},
		{"a comment that spells a qualified jump", "-P INPUT ACCEPT\n-A INPUT -m comment --comment \"see\n-A INPUT -i eth0 -j ts-input \" -j LOG\n-A INPUT -j ts-input\n", tsInputNoAccept},
		{"a comment in ts-input that spells an allow", "-P INPUT ACCEPT\n-A INPUT -j ts-input\n", tsInputNoAccept + "-A ts-input -m comment --comment \"see\n-A ts-input -p tcp -m tcp --dport 3389 -j ACCEPT \" -j LOG\n"},
		// Verbatim from `iptables -S INPUT` (iptables v1.8.10 nf_tables and legacy, Ubuntu 24.04),
		// after `iptables -A INPUT ... -m comment --comment "$(printf 'x\n-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT\ny')" -j DROP`:
		// the newline prints raw, so the middle line is a whole allow of its own (#526).
		{"captured: a comment that prints a whole allow line", "-P INPUT ACCEPT\n-A INPUT -s 192.0.2.1/32 -p tcp -m tcp --dport 9 -m comment --comment \"x\n-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT\ny\" -j DROP\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recordingRunner{output: map[string]string{iptRead: tt.input, iptChain: tt.chain}}
			if got := readIptables(r, 3389); got.reason != "" || got.state != ruleAbsent {
				t.Fatalf("got state %v reason %q, want absent with no reason", got.state, got.reason)
			}
		})
	}
}

// TestCopiesFixKeepsThePortOpen pins that the delete advice never closes the
// port it reports as allowed: on a ufw host where the copies are what admits
// it, the operator adds a ufw allow first, and only then (#576).
func TestCopiesFixKeepsThePortOpen(t *testing.T) {
	cases := []struct {
		name string
		read firewallRead
		want bool
	}{
		{"ufw, the copies alone admit the port", firewallRead{backend: backendUFW, copies: 2, copiesAdmit: true}, true},
		{"ufw, ts-input admits the port too", firewallRead{backend: backendUFW, copies: 2}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Contains(copiesFix(tc.read, 3389), "run `"+ufwAllowCmd+"` first")
			if got != tc.want {
				t.Errorf("copiesFix = %q, asks for a ufw allow first: %v, want %v", copiesFix(tc.read, 3389), got, tc.want)
			}
		})
	}
}

// TestReadIptablesCopiesAdmit pins when the copies are the only allow: the
// walk with them removed no longer finds the port allowed (#576).
func TestReadIptablesCopiesAdmit(t *testing.T) {
	cases := []struct {
		name  string
		chain string
		want  bool
	}{
		{"ts-input accepts the tailnet", tsInputDefault, false},
		{"ts-input accepts nothing", tsInputNoAccept, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &recordingRunner{elevated: true, output: map[string]string{iptRead: ufwHostCopies, iptChain: tc.chain}}
			if got := readIptables(r, 3389).copiesAdmit; got != tc.want {
				t.Errorf("copiesAdmit = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestReadUFWCopiesAdmit pins that the guard holds when ufw itself cannot be
// read: nothing then proves a ufw allow exists, so iptables' answer stands,
// and a ufw allow of its own makes the copies redundant (#576).
func TestReadUFWCopiesAdmit(t *testing.T) {
	ufwIpt := map[string]bool{"ufw": true, "iptables": true}
	cases := []struct {
		name   string
		runner *recordingRunner
		want   bool
	}{
		{"ufw unreadable, the copies alone admit", &recordingRunner{elevated: true, onPath: ufwIpt, output: map[string]string{ufwStatus: ufwActive, iptRead: ufwHostCopies, iptChain: tsInputNoAccept}, fail: map[string]bool{ufwAdded: true}}, true},
		{"ufw status unreadable, the copies alone admit", &recordingRunner{elevated: true, onPath: ufwIpt, output: map[string]string{iptRead: ufwHostCopies, iptChain: tsInputNoAccept}, fail: map[string]bool{ufwStatus: true}}, true},
		{"ufw absent, the copies alone admit", &recordingRunner{elevated: true, onPath: ufwIpt, output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwHostAdded, iptRead: ufwHostCopies, iptChain: tsInputNoAccept}}, true},
		{"ufw allows the port itself", &recordingRunner{elevated: true, onPath: ufwIpt, output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwAddedHeader + ufwScopedAllow, iptRead: ufwHostCopies, iptChain: tsInputNoAccept}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := readUFW(tc.runner, 3389).copiesAdmit; got != tc.want {
				t.Errorf("copiesAdmit = %v, want %v", got, tc.want)
			}
		})
	}
}

// nftTSInputRead is the command that finds tailscaled's ts-input in the nftables
// table it shares with iptables-nft; nftTSInput is what it prints, verbatim from
// tailscaled 1.104.4 in nftables mode with nftables 1.0.9 (capture-tsfw.sh, #526).
// On that host iptables-nft prints the chain and INPUT's jump to it as well, so
// the nft read matters only where the iptables binary is iptables-legacy.
const (
	nftTSInputRead = "nft list chain ip filter ts-input"
	nftTSInput     = "table ip filter {\n\tchain ts-input {\n\t\tiifname \"lo*\" ip saddr 100.64.0.1 counter packets 0 bytes 0 accept\n\t\tiifname \"tailscale0*\" counter packets 0 bytes 0 accept\n\t\tiifname != \"tailscale0*\" ip saddr 100.115.92.0/23 counter packets 0 bytes 0 return\n\t\tiifname != \"tailscale0*\" ip saddr 100.64.0.0/10 counter packets 0 bytes 0 drop\n\t}\n}\n"
	iptNoJump      = "-P INPUT ACCEPT\n"
)

// What iptables and nft print for a chain that does not exist. Exit status 1 is
// their only failure code, so these words, not the status, tell a missing chain
// from a refused read (iptables: xtables "No chain/target/match by that name";
// nft: the kernel's ENOENT).
var (
	noIptChain = map[string]string{iptChain: "iptables: No chain/target/match by that name.\n"}
	noChains   = map[string]string{iptChain: noIptChain[iptChain], nftTSInputRead: "Error: No such file or directory\nlist chain ip filter ts-input\n                      ^^^^^^^^^\n"}
)

// TestReadIptablesTailscaledModes pins the two tailscaled rule sets the
// iptables read cannot see (#526). nodivert (iptables_runner.go AddHooks is
// skipped) leaves ts-input without a jump, and nftables mode keeps it where
// `iptables -S INPUT` never prints it. Either is an unknown read, never an
// absent one, so setup does not append a rule that opens every interface.
func TestReadIptablesTailscaledModes(t *testing.T) {
	ipt, nft := map[string]bool{"iptables": true}, map[string]bool{"iptables": true, "nft": true}
	tests := []struct {
		name   string
		runner *recordingRunner
		want   ruleState
		reason string
	}{
		{"nodivert: ts-input exists, INPUT does not reach it", &recordingRunner{elevated: true, onPath: ipt, output: map[string]string{iptRead: iptNoJump, iptChain: tsInputDefault}}, ruleUnknown, "ts-input exists but INPUT does not jump to it"},
		{"nodivert: an allow in INPUT still decides", &recordingRunner{elevated: true, onPath: ipt, output: map[string]string{iptRead: iptNoJump + "-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT\n", iptChain: tsInputDefault}}, rulePresent, ""},
		{"nodivert: a deny in INPUT still decides", &recordingRunner{elevated: true, onPath: ipt, output: map[string]string{iptRead: iptNoJump + "-A INPUT -p tcp -m tcp --dport 3389 -j DROP\n", iptChain: tsInputDefault}}, ruleBlocked, ""},
		{"no ts-input chain, nft not installed: absent", &recordingRunner{elevated: true, onPath: ipt, output: map[string]string{iptRead: iptNoJump}, exit: map[string]int{iptChain: 1}, stderr: noIptChain}, ruleAbsent, ""},
		{"ts-input unreadable for another reason", &recordingRunner{elevated: true, onPath: ipt, output: map[string]string{iptRead: iptNoJump}, exit: map[string]int{iptChain: 4}}, ruleUnknown, iptChain},
		{"nftables mode: ts-input only in nft", &recordingRunner{elevated: true, onPath: nft, output: map[string]string{iptRead: iptNoJump, nftTSInputRead: nftTSInput}, exit: map[string]int{iptChain: 1}, stderr: noIptChain}, ruleUnknown, "nftables"},
		{"nft installed, no ts-input in it: absent", &recordingRunner{elevated: true, onPath: nft, output: map[string]string{iptRead: iptNoJump}, exit: map[string]int{iptChain: 1, nftTSInputRead: 1}, stderr: noChains}, ruleAbsent, ""},
		{"nft unreadable for another reason", &recordingRunner{elevated: true, onPath: nft, output: map[string]string{iptRead: iptNoJump}, exit: map[string]int{iptChain: 1, nftTSInputRead: 4}, stderr: noIptChain}, ruleUnknown, nftTSInputRead},
		{"iptables exit 1 without the missing-chain text", &recordingRunner{elevated: true, onPath: ipt, output: map[string]string{iptRead: iptNoJump}, exit: map[string]int{iptChain: 1}, stderr: map[string]string{iptChain: "iptables: Permission denied (you must be root).\n"}}, ruleUnknown, iptChain},
		{"nft exit 1 for a refused read", &recordingRunner{elevated: true, onPath: nft, output: map[string]string{iptRead: iptNoJump}, exit: map[string]int{iptChain: 1, nftTSInputRead: 1}, stderr: map[string]string{iptChain: noIptChain[iptChain], nftTSInputRead: "Error: Operation not permitted\n"}}, ruleUnknown, nftTSInputRead},
		{"nft exit 1 with no message", &recordingRunner{elevated: true, onPath: nft, output: map[string]string{iptRead: iptNoJump}, exit: map[string]int{iptChain: 1, nftTSInputRead: 1}, stderr: map[string]string{iptChain: noIptChain[iptChain], nftTSInputRead: ""}}, ruleUnknown, nftTSInputRead},
		{"no iptables, nft holds ts-input", &recordingRunner{elevated: true, onPath: map[string]bool{"nft": true}, output: map[string]string{nftTSInputRead: nftTSInput}}, ruleUnknown, "nftables"},
		{"ufw host, nodivert", &recordingRunner{elevated: true, onPath: map[string]bool{"ufw": true, "iptables": true}, output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwHostAdded, iptRead: iptNoJump, iptChain: tsInputDefault}}, ruleUnknown, "ts-input exists but INPUT does not jump to it"},
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

// TestFirewallStepsTailscaledModes pins that neither step writes on a rule set
// it cannot read (#526): setup fails and check is unverified, both naming the mode.
func TestFirewallStepsTailscaledModes(t *testing.T) {
	newRunner := func(out map[string]string, exit map[string]int) *recordingRunner {
		return &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true, "nft": true}, output: out, exit: exit, stderr: noIptChain}
	}
	tests := []struct {
		name   string
		runner func() *recordingRunner
		mode   string
	}{
		{"nodivert", func() *recordingRunner {
			return newRunner(map[string]string{iptRead: iptNoJump, iptChain: tsInputDefault}, nil)
		}, "ts-input exists but INPUT does not jump to it"},
		{"nftables", func() *recordingRunner {
			return newRunner(map[string]string{iptRead: iptNoJump, nftTSInputRead: nftTSInput}, map[string]int{iptChain: 1})
		}, "nftables mode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.runner()
			setup := firewallSetupStep(r, 3389)
			if slices.ContainsFunc(r.calls, func(c string) bool { return strings.HasPrefix(c, "run ") }) {
				t.Errorf("setup wrote on an unreadable rule set: %v", r.calls)
			}
			check := firewallCheckStep(readLinuxFirewall(tt.runner(), 3389), 3389)
			if setup.Outcome != OutcomeFailed || check.Outcome != OutcomeUnverified {
				t.Errorf("outcomes = %v, %v, want failed, unverified", setup.Outcome, check.Outcome)
			}
			for _, step := range []SetupStep{setup, check} {
				if !strings.Contains(step.Message, tt.mode) || !strings.Contains(step.Fix, "3389") {
					t.Errorf("step %q / fix %q does not name %q and the port", step.Message, step.Fix, tt.mode)
				}
			}
		})
	}
}

// TestNoBackendKeepsTheNFTFix pins that the fix for tailscaled's nftables mode
// survives a host with neither ufw nor iptables (#526): the generic "install
// ufw" advice would not tell the operator where the ts-input chain is.
func TestNoBackendKeepsTheNFTFix(t *testing.T) {
	r := &recordingRunner{elevated: true, onPath: map[string]bool{"nft": true}, output: map[string]string{nftTSInputRead: nftTSInput}}
	read := readLinuxFirewall(r, 3389)
	if !read.noBackend || !strings.Contains(read.reason, "nftables") {
		t.Fatalf("read = %+v, want noBackend naming nftables", read)
	}
	if fix := firewallFix(read, 3389, "setup"); !strings.Contains(fix, nftTSInputRead) {
		t.Errorf("fix = %q, want the nft read hint", fix)
	}
}

// An inactive ufw enforces none of its rules, so the port is open because no
// firewall runs, not because of an allow (#608). `ufw status` is verbatim from
// ufw 0.36.2 on Ubuntu 24.04 (Lima), read under LC_ALL=C.
func TestUFWInactive(t *testing.T) {
	ufwOnly := map[string]bool{"xrdp": true, "systemctl": true, "ufw": true}
	withIPT := map[string]bool{"xrdp": true, "systemctl": true, "ufw": true, "iptables": true}
	inactive := func(added, input, chain string) *recordingRunner {
		out := map[string]string{ufwStatus: "Status: inactive\n", ufwAdded: ufwAddedHeader + added}
		path := ufwOnly
		if input != "" {
			out[iptRead], out[iptChain], path = input, cmp.Or(chain, tsInputDefault), withIPT
		}
		return &recordingRunner{elevated: true, onPath: path, output: linuxUp(out)}
	}
	tests := []struct {
		name     string
		added    string
		input    string // iptables INPUT; empty means iptables is not installed
		chain    string // ts-input; empty means tailscaled's default, which admits the tailnet
		writes   bool   // setup adds the allow, so it holds once ufw is enabled
		replaces bool   // setup also deletes the legacy allow
		message  string
		check    string // what check's message names, when not just "inactive"
		fix      string // what both fixes must also name
	}{
		{name: "no allow yet", writes: true, message: "allow added"},
		{name: "allow already added", added: ufwScopedAllow, message: "allow is in place"},
		// The legacy allow is inert while ufw is off, and still unscoped once it is on: setup replaces it.
		{name: "a legacy allow is replaced", added: ufwLegacyAllow, replaces: true, message: "allow is in place"},
		// An unscoped allow setup did not write opens the port on every interface
		// once ufw is enabled: both commands name it, and setup leaves it (#631).
		{name: "a foreign unscoped allow", added: "ufw allow 22,3389/tcp\n", message: "legacy unscoped rule: ufw allow 22,3389/tcp", check: "legacy unscoped rule: ufw allow 22,3389/tcp", fix: "setup does not delete rules it did not write"},
		// ts-input's scoped iptables rule is not a ufw allow: the legacy ufw
		// allow is still replaced by a scoped ufw one, never just deleted (#631).
		{name: "a legacy allow beside ts-input is replaced", added: ufwLegacyAllow, input: ufwHostInput, replaces: true, message: "allow is in place", check: "unscoped"},
		// An allow scoped to an address or another interface will not admit the
		// tailnet once ufw is enabled: setup still adds the tailscale0 allow (#631 review).
		{name: "an allow scoped off the tailnet", added: "ufw allow from 10.0.0.0/8 to any port 3389 proto tcp\n", writes: true, message: "off the tailnet", check: "off the tailnet"},
		// A ufw deny decides the port once ufw is enabled: name it, add nothing.
		{name: "a ufw deny", added: "ufw deny 3389/tcp\n", message: "ufw deny 3389/tcp", check: "ufw deny 3389/tcp"},
		// tailscaled's ts-input admits the tailnet whatever ufw's state: an
		// unscoped ufw allow would open the port on every interface (#530).
		{name: "ts-input admits the tailnet", input: ufwHostInput, message: "iptables admits TCP 3389", check: "iptables admits TCP 3389"},
		// The copies are live iptables rules, so both commands name them (#576).
		{name: "leftover copies", input: ufwHostCopies, message: "2 copies of", check: "2 copies of"},
		// With no ts-input accept the copies are the only allow: deleting them
		// before adding ufw's would close the port once ufw is enabled.
		{name: "only the copies admit the port", input: ufwHostCopies, chain: tsInputNoAccept, message: "2 copies of", check: "2 copies of", fix: "run `" + ufwAllowCmd + "` first"},
	}
	for _, tt := range tests {
		t.Run("setup/"+tt.name, func(t *testing.T) {
			r := inactive(tt.added, tt.input, tt.chain)
			s := firewallSetupStep(r, 3389)
			assertUFWInactiveStep(t, s, tt.message)
			if !strings.Contains(s.Fix, tt.fix) {
				t.Errorf("fix %q does not name %q", s.Fix, tt.fix)
			}
			if got := slices.Contains(r.calls, "run "+ufwAllowCmd); got != (tt.writes || tt.replaces) {
				t.Errorf("ufw allow ran = %v, want %v: %v", got, tt.writes || tt.replaces, r.calls)
			}
			if got := slices.Contains(r.calls, "run "+ufwDeleteCmd); got != tt.replaces {
				t.Errorf("ufw delete ran = %v, want %v: %v", got, tt.replaces, r.calls)
			}
		})
		t.Run("check/"+tt.name, func(t *testing.T) {
			r := inactive(tt.added, tt.input, tt.chain)
			var result CheckResult
			checkLinux(Config{Port: 3389}, r, &result)
			assertUFWInactiveStep(t, result.Steps[1], cmp.Or(tt.check, "inactive"))
			if !strings.Contains(result.Steps[1].Fix, tt.fix) {
				t.Errorf("fix %q does not name %q", result.Steps[1].Fix, tt.fix)
			}
			deriveLegacy(&result)
			if result.Services[0].FirewallOK || result.FirewallOK {
				t.Error("check reports firewall_ok for a port no firewall filters")
			}
			if slices.ContainsFunc(r.calls, func(c string) bool { return strings.HasPrefix(c, "run ") }) {
				t.Errorf("check ran a mutating command: %v", r.calls)
			}
		})
	}
}

func assertUFWInactiveStep(t *testing.T, s SetupStep, message string) {
	t.Helper()
	if s.Outcome != OutcomeManual || !strings.Contains(s.Message, "ufw is inactive") || !strings.Contains(s.Message, message) {
		t.Errorf("step = %s %q, want manual saying ufw is inactive and %q", s.Outcome, s.Message, message)
	}
	if strings.Contains(s.Message, "Firewall allows") || strings.Contains(s.Message, "rule added") {
		t.Errorf("message %q claims a rule admits the port", s.Message)
	}
	if !strings.Contains(s.Fix, "ufw enable") {
		t.Errorf("fix %q does not name `ufw enable`", s.Fix)
	}
}

// A `ufw status` that says neither active nor inactive is not a read: setup
// must not write on it, and check must not report the port as allowed.
func TestUFWStatusUnrecognized(t *testing.T) {
	for name, runner := range map[string]*recordingRunner{
		"unrecognized": {elevated: true, onPath: map[string]bool{"ufw": true}, output: map[string]string{ufwStatus: "ERROR: problem running\n", ufwAdded: ufwAddedHeader}},
		"failed":       {elevated: true, onPath: map[string]bool{"ufw": true}, output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwAddedHeader}, fail: map[string]bool{ufwStatus: true}},
	} {
		t.Run(name, func(t *testing.T) {
			s := firewallSetupStep(runner, 3389)
			if s.Outcome != OutcomeFailed || !strings.Contains(s.Message, "ufw status") {
				t.Errorf("setup = %s %q, want failed naming `ufw status`", s.Outcome, s.Message)
			}
			if slices.Contains(runner.calls, "run "+ufwAllowCmd) {
				t.Errorf("setup wrote on an unread status: %v", runner.calls)
			}
		})
	}
}

// TestIptablesPrintedForm pins that what setup appends is what `iptables -S`
// prints, for the scoped rule and the legacy one it replaces. A disagreement
// makes every setup read its own rule as absent and append it again (#480).
func TestIptablesPrintedForm(t *testing.T) {
	tests := []struct {
		name string
		rule []string
		want string
	}{
		{"scoped rule", iptablesAppend(3389), iptSetupRule},
		{"legacy rule", iptablesLegacy(3389), iptLegacyRule},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(iptPrinted(tt.rule), " "); got != strings.TrimSpace(tt.want) {
				t.Errorf("printed %q, want %q", got, strings.TrimSpace(tt.want))
			}
		})
	}
	if got := strings.Join(iptablesAppend(3389), " "); got != strings.TrimPrefix(iptAppend, "iptables ") {
		t.Errorf("append = %q, want %q", got, iptAppend)
	}
}

// TestReadIptablesUnscoped pins which allows for the port are legacy: those with
// no interface, source or destination, and a port in them (#631).
func TestReadIptablesUnscoped(t *testing.T) {
	tests := []struct {
		name    string
		rules   string
		copies  int
		foreign int
		scoped  bool
	}{
		{"scoped rule only", iptScoped, 0, 0, true},
		{"legacy copy only", iptEmpty + iptLegacyRule, 1, 0, false},
		{"legacy copy beside a scoped rule", iptScoped + iptLegacyRule, 1, 0, true},
		{"two copies", iptTwoCopies, 2, 0, false},
		{"multiport allow", iptEmpty + iptForeignRule, 0, 1, false},
		{"a comment on the legacy rule makes it foreign", iptEmpty + `-A INPUT -p tcp -m tcp --dport 3389 -m comment --comment "rdp" -j ACCEPT` + "\n", 0, 1, false},
		{"a source address does not admit the tailnet", iptEmpty + "-A INPUT -s 10.0.0.0/8 -p tcp -m tcp --dport 3389 -j ACCEPT\n", 0, 0, false},
		{"another interface does not admit the tailnet", iptEmpty + "-A INPUT -i eth1 -p tcp -m tcp --dport 3389 -j ACCEPT\n", 0, 0, false},
		{"a negated tailnet interface does not admit the tailnet", iptEmpty + "-A INPUT ! -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT\n", 0, 0, false},
		{"a negated source does not admit the tailnet", iptEmpty + "-A INPUT ! -s 100.64.0.0/10 -p tcp -m tcp --dport 3389 -j ACCEPT\n", 0, 0, false},
		{"a legacy copy beside an eth1 rule is not scoped", iptEmpty + "-A INPUT -i eth1 -p tcp -m tcp --dport 3389 -j ACCEPT\n" + iptLegacyRule, 1, 0, false},
		{"a port-less accept is the operator's posture", iptEmpty + "-A INPUT -p tcp -j ACCEPT\n", 0, 0, true},
		{"another port", iptEmpty + "-A INPUT -p tcp -m tcp --dport 33890 -j ACCEPT\n", 0, 0, false},
		{"a drop is not an allow", iptDropFirst, 0, 0, false},
		{"ts-input admits the tailnet", iptEmpty + iptLegacyRule, 1, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := tsInputNoAccept
			if tt.name == "ts-input admits the tailnet" {
				chain = tsInputDefault
			}
			r := &recordingRunner{elevated: true, output: map[string]string{iptRead: tt.rules, iptChain: chain}}
			got := readIptables(r, 3389)
			if got.copies != tt.copies || len(got.foreign) != tt.foreign || got.scoped != tt.scoped {
				t.Errorf("copies %d foreign %q scoped %v, want %d, %d, %v", got.copies, got.foreign, got.scoped, tt.copies, tt.foreign, tt.scoped)
			}
		})
	}
}

// TestUFWAllowArgs pins the one place the ufw write is spelled (#631).
func TestUFWAllowArgs(t *testing.T) {
	if got := "ufw " + strings.Join(ufwAllowArgs(3389), " "); got != ufwAllowCmd {
		t.Errorf("ufwAllowArgs = %q, want %q", got, ufwAllowCmd)
	}
	if got := "ufw " + strings.Join(ufwDeleteArgs(3389), " "); got != ufwDeleteCmd {
		t.Errorf("ufwDeleteArgs = %q, want %q", got, ufwDeleteCmd)
	}
	if !ufwAllows(ufwAddedHeader+ufwScopedAllow, 3389) {
		t.Error("the rule setup writes is not read back as an allow: every setup would write it again")
	}
}

// TestReadUFWUnscoped pins which ufw allows for the port are legacy: those with
// no interface, source or destination, and a port in them. Only the exact rule
// earlier releases wrote is setup's to delete (#631).
func TestReadUFWUnscoped(t *testing.T) {
	tests := []struct {
		name    string
		added   string
		legacy  bool
		foreign int
		scoped  bool
	}{
		{"scoped allow only", ufwScopedAllow, false, 0, true},
		{"legacy allow only", ufwLegacyAllow, true, 0, false},
		{"legacy beside scoped", ufwScopedAllow + ufwLegacyAllow, true, 0, true},
		{"port list", "ufw allow 22,3389/tcp\n", false, 1, false},
		{"bare port", "ufw allow 3389\n", false, 1, false},
		{"explicit inbound", "ufw allow in 3389/tcp\n", false, 1, false},
		{"a comment makes it foreign", "ufw allow 3389/tcp comment 'rdp'\n", false, 1, false},
		{"a source address does not admit the tailnet", "ufw allow from 10.0.0.0/8 to any port 3389 proto tcp\n", false, 0, false},
		{"another interface does not admit the tailnet", "ufw allow in on eth1 to any port 3389 proto tcp\n", false, 0, false},
		{"a destination address does not admit the tailnet", "ufw allow to 192.168.1.5 port 3389 proto tcp\n", false, 0, false},
		{"a legacy allow beside an eth1 rule is not scoped", "ufw allow in on eth1 to any port 3389 proto tcp\n" + ufwLegacyAllow, true, 0, false},
		{"from any is not a scope", "ufw allow from any to any port 3389 proto tcp\n", false, 1, false},
		{"from any beside a legacy allow is not scoped", "ufw allow from any to any port 3389 proto tcp\n" + ufwLegacyAllow, true, 1, false},
		{"a tailnet allow from any source is scoped", "ufw allow in on tailscale0 from any to any port 3389 proto tcp\n", false, 0, true},
		{"another port", "ufw allow 33890/tcp\n", false, 0, false},
		{"udp", "ufw allow 3389/udp\n", false, 0, false},
		{"a deny is not an allow", "ufw deny 3389/tcp\n", false, 0, false},
		{"outbound", "ufw allow out 3389/tcp\n", false, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			legacy, foreign, scoped := ufwUnscopedRead(ufwAddedHeader+tt.added, 3389)
			if legacy != tt.legacy || len(foreign) != tt.foreign || scoped != tt.scoped {
				t.Errorf("legacy %v foreign %q scoped %v, want %v, %d, %v", legacy, foreign, scoped, tt.legacy, tt.foreign, tt.scoped)
			}
		})
	}
}

// The RDP line of check points at "the steps below for the cause" when a
// service is unread (#638), so the fix of every unread step names that cause
// and never falls back to blaming privilege.
func TestUnreadStepFixesDoNotBlamePrivilege(t *testing.T) {
	privilege := regexp.MustCompile(`(?i)\b(root|sudo|administrator|elevat)`)
	steps := map[string]SetupStep{}
	for name, x := range map[string]xrdpState{
		"no systemd reading": {installed: true},
		"undecided answer":   {installed: true, readable: true, undecided: "indirect"},
	} {
		step, _ := xrdpUnmet(x, OutcomeUnverified)
		steps["xrdp: "+name] = step
	}
	steps["windows: RDP port unread"] = actRead("RDP configuration", errors.New("read failed"), "", "Could not read the RDP port from the registry", rdpEnableFix)
	for name, step := range steps {
		if step.Fix == "" || privilege.MatchString(step.Fix) {
			t.Errorf("%s: fix %q should name the cause, not privilege", name, step.Fix)
		}
	}
}

// TestReadIptablesCapturedModes reads what tailscaled 1.104.4 leaves in
// iptables in each netfilter mode, captured live (capture-tsfw.sh, #526):
// mode on and nftables mode admit the port through ts-input, and nodivert
// leaves the chain unreached, which is unknown, never absent.
func TestReadIptablesCapturedModes(t *testing.T) {
	tests := []struct {
		name, input, chain string
		want               ruleState
		reason             string
	}{
		{"netfilter-mode on", iptTSInputJumped, tsInputDefault, rulePresent, ""},
		{"netfilter-mode nodivert", iptNoJump, tsInputDefault, ruleUnknown, "nodivert"},
		{"nftables mode, read through iptables-nft", iptTSInputJumped, tsInputNFTMode, rulePresent, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recordingRunner{output: map[string]string{iptRead: tt.input, iptChain: tt.chain}}
			got := readIptables(r, 3389)
			if got.state != tt.want || !strings.Contains(got.reason, tt.reason) {
				t.Fatalf("got state %v reason %q, want %v with %q", got.state, got.reason, tt.want, tt.reason)
			}
		})
	}
}

// ip6tables INPUT as tailscaled 1.104.4 leaves it in netfilter mode on
// (captured, #526), and the same chain with an unscoped v6 allow added.
const (
	ip6Read     = "ip6tables -S INPUT"
	ip6Captured = "-P INPUT ACCEPT\n-A INPUT -j ts-input\n"
	// ip6Closed is the captured chain under a DROP policy, so the exposure rows
	// below read only the rules, not the policy (#733).
	ip6Closed    = "-P INPUT DROP\n-A INPUT -j ts-input\n"
	ip6Unscoped  = "-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"
	ip6DeleteFix = "ip6tables -D INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"
	// ip6TSInput is tailscaled's v6 ts-input as `ip6tables -S ts-input` printed
	// it in --netfilter-mode=on, captured with scripts/e2e/capture-tsfw.sh on
	// 2026-11-12 (tailscale 1.104.4); ip6TSInputNFT is the same chain under
	// iptables-nft, which prints the interfaces as wildcards (#720).
	ip6ChainRead  = "ip6tables -S ts-input"
	ip6TSInput    = "-N ts-input\n-A ts-input -s fd7a:115c:a1e0::1/128 -i lo -j ACCEPT\n-A ts-input -i tailscale0 -j ACCEPT\n"
	ip6TSInputNFT = "-N ts-input\n-A ts-input -s fd7a:115c:a1e0::1/128 -i lo+ -j ACCEPT\n-A ts-input -i tailscale0+ -j ACCEPT\n"
	ip6InChain    = "-A ts-input -p tcp -m tcp --dport 3389 -j ACCEPT"
)

// TestIP6Exposure pins the v6 read as an exposure scan (#526): an unscoped v6
// allow for the port is reported and never deleted; anything else in v6, or no
// v6 read at all, leaves the IPv4 outcome as it was.
func TestIP6Exposure(t *testing.T) {
	tests := []struct {
		name    string
		onPath  bool
		v6      string
		v6Fails bool
		want    Outcome
		message string
		fix     string // the -D fix a manual step names; ip6DeleteFix when empty
		chain   string // what ip6tables -S ts-input prints; ip6TSInput when empty
	}{
		{name: "captured chain", onPath: true, v6: ip6Closed, want: OutcomeOK},
		{name: "captured chain, iptables-nft", onPath: true, v6: ip6Closed, chain: ip6TSInputNFT, want: OutcomeOK},
		// An allow inside the chain INPUT jumps to opens the port as one in INPUT does (#720).
		{name: "an unscoped v6 allow inside ts-input", onPath: true, v6: ip6Closed, chain: ip6TSInput + ip6InChain + "\n", want: OutcomeManual,
			message: ip6InChain, fix: "ip6tables -D ts-input -p tcp -m tcp --dport 3389 -j ACCEPT"},
		{name: "a tailnet-scoped v6 allow", onPath: true, v6: ip6Closed + "-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT\n", want: OutcomeOK},
		{name: "a port-less v6 accept is the operator's posture", onPath: true, v6: ip6Closed + "-A INPUT -p tcp -j ACCEPT\n", want: OutcomeOK},
		{name: "an unscoped v6 allow", onPath: true, v6: ip6Closed + ip6Unscoped + "\n", want: OutcomeManual, message: ip6Unscoped},
		// A negated scope or an explicit any prefix still opens the port off
		// the tailnet (BUG-526 review, finding 1).
		{name: "a v6 allow on every interface but tailscale0", onPath: true, v6: ip6Closed + "-A INPUT ! -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT\n", want: OutcomeManual,
			message: "! -i tailscale0", fix: "ip6tables -D INPUT ! -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT"},
		{name: "a v6 allow from any source", onPath: true, v6: ip6Closed + "-A INPUT -s ::/0 -p tcp -m tcp --dport 3389 -j ACCEPT\n", want: OutcomeManual,
			message: "-s ::/0", fix: "ip6tables -D INPUT -s ::/0 -p tcp -m tcp --dport 3389 -j ACCEPT"},
		{name: "a v6 allow to any destination", onPath: true, v6: ip6Closed + "-A INPUT -d ::/0 -p tcp -m tcp --dport 3389 -j ACCEPT\n", want: OutcomeManual,
			message: "-d ::/0", fix: "ip6tables -D INPUT -d ::/0 -p tcp -m tcp --dport 3389 -j ACCEPT"},
		{name: "a v6 allow for one LAN prefix is the operator's scope", onPath: true, v6: ip6Closed + "-A INPUT -s 2001:db8::/32 -p tcp -m tcp --dport 3389 -j ACCEPT\n", want: OutcomeOK},
		{name: "v6 read fails", onPath: true, v6Fails: true, want: OutcomeOK, message: ip6Read},
		{name: "no ip6tables", want: OutcomeOK},
	}
	unprivileged := &recordingRunner{onPath: map[string]bool{"iptables": true, "ip6tables": true}}
	readLinuxFirewall(unprivileged, 3389)
	if len(unprivileged.calls) != 0 {
		t.Errorf("an unprivileged read ran %v; every firewall read needs root", unprivileged.calls)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newRunner := func() *recordingRunner {
				return &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true, "ip6tables": tt.onPath},
					output: map[string]string{iptRead: iptScoped, iptChain: tsInputNoAccept, ip6Read: tt.v6, ip6ChainRead: cmp.Or(tt.chain, ip6TSInput)},
					fail:   map[string]bool{ip6Read: tt.v6Fails}}
			}
			r := newRunner()
			setup := firewallSetupStep(r, 3389)
			if slices.ContainsFunc(r.calls, func(c string) bool { return strings.HasPrefix(c, "run ") }) {
				t.Errorf("setup wrote: %v", r.calls)
			}
			want := []string{"lookpath " + firewalldCmd, "lookpath nft", "lookpath ufw", "lookpath iptables", "output " + iptRead, "output " + iptChain, "lookpath " + backendIP6}
			if tt.onPath {
				want = append(want, "output "+ip6Read)
			}
			if tt.onPath && !tt.v6Fails {
				want = append(want, "output "+ip6ChainRead)
			}
			if !reflect.DeepEqual(r.calls, want) {
				t.Errorf("calls:\n got %q\nwant %q", r.calls, want)
			}
			cr := newRunner()
			read := readLinuxFirewall(cr, 3389)
			if !reflect.DeepEqual(cr.calls, want) {
				t.Errorf("check calls:\n got %q\nwant %q", cr.calls, want)
			}
			check := firewallCheckStep(read, 3389)
			for _, step := range []SetupStep{setup, check} {
				if step.Outcome != tt.want || !strings.Contains(step.Message, tt.message) {
					t.Errorf("outcome %v message %q, want %v naming %q", step.Outcome, step.Message, tt.want, tt.message)
				}
				// A row with no expected v6 text must stay silent about v6 (AC4: no
				// ip6tables, or nothing to report, reads as before).
				if tt.message == "" && (strings.Contains(step.Message, "ip6tables") || strings.Contains(step.Message, "IPv6")) {
					t.Errorf("message %q mentions IPv6, want the IPv4 report alone", step.Message)
				}
				fix := cmp.Or(tt.fix, ip6DeleteFix)
				if tt.want == OutcomeManual && !strings.Contains(step.Fix, fix) {
					t.Errorf("fix %q lacks %q", step.Fix, fix)
				}
			}
			if got := linuxFirewallOK(read); got != (tt.want == OutcomeOK) {
				t.Errorf("linuxFirewallOK = %v, want %v", got, tt.want == OutcomeOK)
			}
		})
	}
}

// TestOffTailnetOnlyAllow pins that an allow scoped to something other than the
// tailnet does not converge the host: the port is open, but not from the
// tailnet, so setup adds the tailscale0 rule and check reports it (#631).
func TestOffTailnetOnlyAllow(t *testing.T) {
	ufwRunner := func(added string) *recordingRunner {
		return &recordingRunner{elevated: true, onPath: map[string]bool{"ufw": true, "iptables": true}, output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwAddedHeader + added, iptRead: ufwHostInput, iptChain: tsInputNoAccept}}
	}
	iptRunner := func(rule string) *recordingRunner {
		return &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, output: map[string]string{iptRead: iptEmpty + rule, iptChain: tsInputNoAccept}}
	}
	tests := []struct {
		name   string
		runner func() *recordingRunner
		write  string
	}{
		{"iptables, a source address", func() *recordingRunner {
			return iptRunner("-A INPUT -s 10.0.0.0/8 -p tcp -m tcp --dport 3389 -j ACCEPT\n")
		}, iptAppend},
		{"iptables, another interface", func() *recordingRunner {
			return iptRunner("-A INPUT -i eth1 -p tcp -m tcp --dport 3389 -j ACCEPT\n")
		}, iptAppend},
		{"ufw, a source address", func() *recordingRunner {
			return ufwRunner("ufw allow from 10.0.0.0/8 to any port 3389 proto tcp\n")
		}, ufwAllowCmd},
		{"ufw, another interface", func() *recordingRunner {
			return ufwRunner("ufw allow in on eth1 to any port 3389 proto tcp\n")
		}, ufwAllowCmd},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.runner()
			setup := firewallSetupStep(r, 3389)
			if setup.Outcome != OutcomeChanged || !slices.Contains(r.calls, "run "+tt.write) {
				t.Errorf("setup = %v %q, calls %v; want changed with %q", setup.Outcome, setup.Message, r.calls, tt.write)
			}
			if !strings.Contains(setup.Message, tailnetInterface) {
				t.Errorf("setup message %q does not name %s", setup.Message, tailnetInterface)
			}
			check := firewallCheckStep(readLinuxFirewall(tt.runner(), 3389), 3389)
			if check.Outcome != OutcomeChanged || !strings.Contains(check.Message, tailnetInterface) {
				t.Errorf("check = %v %q, want changed naming %s", check.Outcome, check.Message, tailnetInterface)
			}
		})
	}
}

// TestIP6Jumps pins the walk into the chains INPUT jumps to (#720): a chain is
// followed when the jump applies to every packet for the port, once, however
// deep, and its unscoped allows are reported under the chain's own name. A
// target that is no chain is skipped; a chain that cannot be read is a note.
func TestIP6Jumps(t *testing.T) {
	const (
		open   = "-A mine -p tcp -m tcp --dport 3389 -j ACCEPT"
		noMine = "ip6tables v1.8.10 (nf_tables): chain `mine' in table `filter' is incompatible, use 'nft' tool.\n"
	)
	missing := "ip6tables: No chain/target/match by that name.\n"
	tests := []struct {
		name   string
		input  string
		chains map[string]string // chain -> what ip6tables -S <chain> prints
		fail   map[string]string // chain -> stderr of a failed read
		want   []string
		note   string
		said   string   // the tool's words the note must quote
		reads  []string // the chains read, in order
	}{
		{name: "tailscaled's ts-input", input: "-A INPUT -j ts-input", chains: map[string]string{"ts-input": ip6TSInput}, reads: []string{"ts-input"}},
		{name: "an allow in ts-input", input: "-A INPUT -j ts-input", chains: map[string]string{"ts-input": ip6TSInput + ip6InChain}, want: []string{ip6InChain}, reads: []string{"ts-input"}},
		{name: "a jump through goto", input: "-A INPUT -g mine", chains: map[string]string{"mine": open}, want: []string{open}, reads: []string{"mine"}},
		{name: "a jump for the port", input: "-A INPUT -p tcp -m tcp --dport 3389 -j mine", chains: map[string]string{"mine": open}, want: []string{open}, reads: []string{"mine"}},
		{name: "a jump for another port is not followed", input: "-A INPUT -p tcp -m tcp --dport 22 -j mine", chains: map[string]string{"mine": open}},
		{name: "a jump held to one interface is not followed", input: "-A INPUT -i eth0 -j mine", chains: map[string]string{"mine": open}},
		{name: "a loopback jump is not followed", input: "-A INPUT -i lo -j mine", chains: map[string]string{"mine": open}},
		{name: "a jump on every interface but one is followed", input: "-A INPUT ! -i tailscale0 -j mine", chains: map[string]string{"mine": open}, want: []string{open}, reads: []string{"mine"}},
		{name: "two jumps deep", input: "-A INPUT -j ts-input", chains: map[string]string{"ts-input": ip6TSInput + "-A ts-input -j mine", "mine": open}, want: []string{open}, reads: []string{"ts-input", "mine"}},
		{name: "a loop ends", input: "-A INPUT -j mine", chains: map[string]string{"mine": open + "\n-A mine -j other", "other": "-A other -j mine\n-A other -j INPUT"}, want: []string{open}, reads: []string{"mine", "other"}},
		{name: "a chain jumped to twice is read once", input: "-A INPUT -j mine\n-A INPUT -p tcp -j mine", chains: map[string]string{"mine": open}, want: []string{open}, reads: []string{"mine"}},
		{name: "verdicts are not chains", input: "-A INPUT -p tcp -j DROP\n-A INPUT -j RETURN\n-A INPUT -p tcp -j REJECT --reject-with tcp-reset"},
		{name: "a target that is no chain is skipped without a note", input: "-A INPUT -j LOG", fail: map[string]string{"LOG": missing}, reads: []string{"LOG"}},
		// ufw writes its v6 chains and `ufw show added` reports an allow there
		// with a fix ufw keeps; an ip6tables -D would be undone on its reload.
		{name: "ufw's chains are left to the ufw read", input: "-A INPUT -j ufw6-before-input", chains: map[string]string{"ufw6-before-input": "-A ufw6-before-input -p tcp -m tcp --dport 3389 -j ACCEPT"}},
		{name: "a chain that cannot be read is a note, INPUT still read", input: ip6Unscoped + "\n-A INPUT -j mine", fail: map[string]string{"mine": noMine}, want: []string{ip6Unscoped},
			note: "ip6tables -S mine: ", said: "(" + strings.TrimSuffix(noMine, "\n") + ")", reads: []string{"mine"}},
		// A port-less accept is the operator's posture, in a chain as in INPUT
		// (the INPUT row in TestIP6Exposure): the scan reports allows for the port.
		{name: "a port-less accept in a chain is the operator's posture", input: "-A INPUT -j ts-input", chains: map[string]string{"ts-input": ip6TSInput + "-A ts-input -j ACCEPT"}, reads: []string{"ts-input"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recordingRunner{elevated: true, onPath: map[string]bool{backendIP6: true}, output: map[string]string{ip6Read: "-P INPUT DROP\n" + tt.input + "\n"},
				exit: map[string]int{}, stderr: map[string]string{}}
			for chain, out := range tt.chains {
				r.output["ip6tables -S "+chain] = out + "\n"
			}
			for chain, said := range tt.fail {
				r.exit["ip6tables -S "+chain], r.stderr["ip6tables -S "+chain] = 1, said
			}
			foreign, _, note := readIP6(r, 3389)
			if !slices.Equal(foreign, tt.want) {
				t.Errorf("foreign = %q, want %q", foreign, tt.want)
			}
			if (tt.note == "") != (note == "") || !strings.Contains(note, tt.note) || !strings.Contains(note, tt.said) {
				t.Errorf("note = %q, want %q", note, tt.note)
			}
			var reads []string
			for _, c := range r.calls {
				if chain, ok := strings.CutPrefix(c, "output ip6tables -S "); ok && chain != iptInput {
					reads = append(reads, chain)
				}
			}
			if !slices.Equal(reads, tt.reads) {
				t.Errorf("chains read = %q, want %q", reads, tt.reads)
			}
		})
	}
}

// TestIP6JumpsUFWHost pins the ufw6 exclusion end to end (#720 review): on a
// ufw host, `ufw allow 3389` also writes an unscoped v6 allow into ufw's own
// chains. It is reported once, by the ufw read, with ufw's fix: the v6 walk
// never reads a ufw6 chain, so no ip6tables -D that ufw would undo is named.
func TestIP6JumpsUFWHost(t *testing.T) {
	const ufw6 = "ip6tables -S ufw6-user-input"
	r := &recordingRunner{elevated: true, onPath: map[string]bool{"ufw": true, "iptables": true, backendIP6: true}, output: map[string]string{
		ufwStatus: ufwActive, ufwAdded: ufwAddedHeader + "ufw allow 3389\n", iptRead: ufwHostInput, iptChain: tsInputNoAccept,
		ip6Read: "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -j ufw6-user-input\n", ip6ChainRead: ip6TSInput,
		ufw6: "-N ufw6-user-input\n-A ufw6-user-input -p tcp -m tcp --dport 3389 -j ACCEPT\n"}}
	check := firewallCheckStep(readLinuxFirewall(r, 3389), 3389)
	if slices.Contains(r.calls, "output "+ufw6) {
		t.Errorf("the walk read %s; ufw's chains are the ufw read's", ufw6)
	}
	if check.Outcome != OutcomeManual || strings.Contains(check.Fix, "ip6tables -D") || !strings.Contains(check.Fix, "ufw") {
		t.Errorf("check = %v, fix %q; want manual with ufw's fix and no ip6tables -D", check.Outcome, check.Fix)
	}
}

// withoutProbes drops the lookups that every elevated read makes: ip6tables
// (#526), and the foreign-firewall probe's firewall-cmd and nft (#739), which
// open the read. These tables pin the IPv4 sequence on hosts with neither;
// TestIP6Exposure pins the order, TestForeignFirewall what the probe finds.
func withoutProbes(calls []string) []string {
	out := slices.Clone(calls)
	if i := slices.Index(out, "lookpath "+firewalldCmd); i >= 0 {
		end := i + 1
		if end < len(out) && out[end] == "lookpath nft" {
			end++
		}
		out = slices.Delete(out, i, end)
	}
	return slices.DeleteFunc(out, func(c string) bool { return c == "lookpath "+backendIP6 })
}

// TestReadIP6NoteQuotesTheTool pins that a failed v6 read says why, in the
// tool's own words, as on a host booted with ipv6.disable=1 (BUG-526 review).
func TestReadIP6NoteQuotesTheTool(t *testing.T) {
	const said = "ip6tables v1.8.10 (nf_tables): Could not fetch rule set generation id: Address family not supported by protocol"
	r := &recordingRunner{onPath: map[string]bool{backendIP6: true}, exit: map[string]int{"ip6tables -S INPUT": 1}, stderr: map[string]string{"ip6tables -S INPUT": said + "\n"}}
	_, _, note := readIP6(r, 3389)
	if !strings.Contains(note, "Address family not supported by protocol") {
		t.Errorf("note = %q, want it to quote the tool's stderr", note)
	}
}

// TestPermissiveInputPolicy: on an iptables host with no ufw, an INPUT policy
// of ACCEPT opens every port on every interface, so the tailscale0 scope
// restricts nothing. A step that would be ok or changed is manual instead,
// firewall_ok is false, and setup never touches the policy (#691).
func TestPermissiveInputPolicy(t *testing.T) {
	const scopedRule = "-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT\n"
	tests := []struct {
		name    string
		input   string
		check   Outcome
		setup   Outcome
		appends bool
	}{
		{"ACCEPT, rule present", "-P INPUT ACCEPT\n-A INPUT -j ts-input\n" + scopedRule, OutcomeManual, OutcomeManual, false},
		{"ACCEPT, rule absent: setup adds it and still reports manual", "-P INPUT ACCEPT\n-A INPUT -j ts-input\n", OutcomeManual, OutcomeManual, true},
		{"DROP, rule present", "-P INPUT DROP\n-A INPUT -j ts-input\n" + scopedRule, OutcomeOK, OutcomeOK, false},
		{"DROP, rule absent", "-P INPUT DROP\n-A INPUT -j ts-input\n", OutcomeChanged, OutcomeChanged, true},
		{"ACCEPT, a jump the read cannot follow stays unverified", "-P INPUT ACCEPT\n-A INPUT -i eth0 -j ts-input\n", OutcomeUnverified, OutcomeFailed, false},
		{"ACCEPT, setup's legacy rule: the replace is still promoted to manual", "-P INPUT ACCEPT\n-A INPUT -j ts-input\n-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT\n", OutcomeManual, OutcomeManual, true},
		{"DROP, a multi-line comment cannot forge the policy line", "-P INPUT DROP\n-A INPUT -j ts-input\n-A INPUT -m comment --comment \"x\n-P INPUT ACCEPT\ny\" -j LOG\n" + scopedRule, OutcomeOK, OutcomeOK, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newRunner := func() *recordingRunner {
				return &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true},
					output: map[string]string{iptRead: tt.input, iptChain: tsInputNoAccept}}
			}
			r := newRunner()
			setup := firewallSetupStep(r, 3389)
			if slices.ContainsFunc(r.calls, func(c string) bool { return strings.Contains(c, " -P ") }) {
				t.Errorf("setup changed the policy: %v", r.calls)
			}
			if got := slices.Contains(r.calls, "run "+iptAppend); got != tt.appends {
				t.Errorf("setup appended the scoped rule = %v, want %v (calls %v)", got, tt.appends, r.calls)
			}
			read := readLinuxFirewall(newRunner(), 3389)
			check := firewallCheckStep(read, 3389)
			for _, s := range []struct {
				name string
				step SetupStep
				want Outcome
			}{{"setup", setup, tt.setup}, {"check", check, tt.check}} {
				if s.step.Outcome != s.want {
					t.Errorf("%s = %v %q, want %v", s.name, s.step.Outcome, s.step.Message, s.want)
				}
				permissive := strings.HasPrefix(tt.input, "-P INPUT ACCEPT")
				if names := strings.Contains(s.step.Message, "INPUT policy is ACCEPT"); permissive && s.want == OutcomeManual && !names {
					t.Errorf("%s message %q does not name the ACCEPT policy", s.name, s.step.Message)
				} else if !permissive && names {
					t.Errorf("%s message %q names a policy the host does not have", s.name, s.step.Message)
				}
				if s.want == OutcomeManual && !strings.Contains(s.step.Fix, "-P INPUT DROP") {
					t.Errorf("%s fix %q does not name a DROP policy", s.name, s.step.Fix)
				} else if s.want != OutcomeManual && strings.Contains(s.step.Fix, "-P INPUT DROP") {
					t.Errorf("%s fix %q names the policy on a %v step, whose own fix comes first", s.name, s.step.Fix, s.want)
				}
				// Closing the policy without loopback and replies cuts local DNS and
				// tailscaled's own control connection, and with it the SSH it kept.
				for _, keep := range []string{"-i lo -j ACCEPT", "ESTABLISHED,RELATED -j ACCEPT", "--dport <your SSH port> -j ACCEPT", "another firewall"} {
					if s.want == OutcomeManual && !strings.Contains(s.step.Fix, keep) {
						t.Errorf("%s fix %q closes the policy without keeping %q open", s.name, s.step.Fix, keep)
					}
				}
			}
			if got, want := linuxFirewallOK(read), tt.check == OutcomeOK; got != want {
				t.Errorf("linuxFirewallOK = %v, want %v", got, want)
			}
		})
	}
}

// TestIP6Policy pins the v6 INPUT policy on an iptables host (#733): ACCEPT
// opens every port on every IPv6 address, so a converged or repaired step turns
// manual, an unverified or failed one gains only the note, and setup never
// changes the policy.
func TestIP6Policy(t *testing.T) {
	const scopedRule = "-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT\n"
	const v4Closed = "-P INPUT DROP\n-A INPUT -j ts-input\n"
	tests := []struct {
		name         string
		v4, v6       string
		check, setup Outcome
		v6Fix        bool // the fix names `ip6tables -P INPUT DROP`
		v4Fix        bool // the fix names `iptables -P INPUT DROP` too
	}{
		{"v4 DROP, v6 ACCEPT, rule present", v4Closed + scopedRule, ip6Captured, OutcomeManual, OutcomeManual, true, false},
		{"v4 DROP, v6 ACCEPT, rule absent: setup adds it and reports manual", v4Closed, ip6Captured, OutcomeManual, OutcomeManual, true, false},
		{"v4 DROP, v6 DROP", v4Closed + scopedRule, ip6Closed, OutcomeOK, OutcomeOK, false, false},
		{"both ACCEPT: one manual step naming both policies", "-P INPUT ACCEPT\n-A INPUT -j ts-input\n" + scopedRule, ip6Captured, OutcomeManual, OutcomeManual, true, true},
		{"an unfollowable v4 jump keeps its outcome", "-P INPUT DROP\n-A INPUT -i eth0 -j ts-input\n", ip6Captured, OutcomeUnverified, OutcomeFailed, false, false},
		{"a multi-line comment cannot forge the v6 policy", v4Closed + scopedRule, ip6Closed + "-A INPUT -m comment --comment \"x\n-P INPUT ACCEPT\ny\" -j LOG\n", OutcomeOK, OutcomeOK, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newRunner := func() *recordingRunner {
				return &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true, "ip6tables": true},
					output: map[string]string{iptRead: tt.v4, iptChain: tsInputNoAccept, ip6Read: tt.v6}}
			}
			r := newRunner()
			setup := firewallSetupStep(r, 3389)
			if slices.ContainsFunc(r.calls, func(c string) bool { return strings.Contains(c, " -P ") }) {
				t.Errorf("setup changed a policy: %v", r.calls)
			}
			read := readLinuxFirewall(newRunner(), 3389)
			check := firewallCheckStep(read, 3389)
			v6Open := strings.HasPrefix(tt.v6, "-P INPUT ACCEPT")
			for _, s := range []struct {
				name string
				step SetupStep
				want Outcome
			}{{"setup", setup, tt.setup}, {"check", check, tt.check}} {
				if s.step.Outcome != s.want {
					t.Errorf("%s = %v %q, want %v", s.name, s.step.Outcome, s.step.Message, s.want)
				}
				if names := strings.Contains(s.step.Message, "ip6tables INPUT policy is ACCEPT"); names != v6Open {
					t.Errorf("%s message %q: names the v6 policy = %v, want %v", s.name, s.step.Message, names, v6Open)
				}
				if got := strings.Contains(s.step.Fix, "`ip6tables -P INPUT DROP`"); got != tt.v6Fix {
					t.Errorf("%s fix %q: names the v6 DROP = %v, want %v", s.name, s.step.Fix, got, tt.v6Fix)
				}
				if got := strings.Contains(s.step.Fix, "`iptables -P INPUT DROP`"); got != tt.v4Fix {
					t.Errorf("%s fix %q: names the v4 DROP = %v, want %v", s.name, s.step.Fix, got, tt.v4Fix)
				}
				for _, keep := range []string{"`ip6tables -A INPUT -i lo -j ACCEPT`", "`ip6tables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT`", "`ip6tables -A INPUT -p tcp --dport <your SSH port> -j ACCEPT`", "`ip6tables -A INPUT -p ipv6-icmp -j ACCEPT`", "IPv6 is disabled"} {
					if tt.v6Fix && !strings.Contains(s.step.Fix, keep) {
						t.Errorf("%s fix %q closes the v6 policy without %q", s.name, s.step.Fix, keep)
					}
				}
			}
			if got, want := linuxFirewallOK(read), tt.check == OutcomeOK; got != want {
				t.Errorf("linuxFirewallOK = %v, want %v", got, want)
			}
		})
	}
	// An inactive ufw already reports itself as off: its open policy is that same fact (#735).
	ufw := withIP6(firewallRead{backend: backendUFW, ufwInactive: true, v6Open: true}, SetupStep{Outcome: OutcomeOK}, 3389)
	if ufw.Outcome != OutcomeOK || ufw.Fix != "" || ufw.Message != "" {
		t.Errorf("an inactive ufw host's step changed: %+v", ufw)
	}
}

// TestPolicyFixOnManualStep pins that a step already manual for another reason
// (a foreign unscoped allow) still gains the policy fix, for v4 as for v6.
func TestPolicyFixOnManualStep(t *testing.T) {
	read := firewallRead{inputOpen: true, v6Open: true, backend: backendIPT}
	in := SetupStep{Outcome: OutcomeManual, Message: "foreign allow", Fix: "delete the foreign rule"}
	got := withIP6(read, withPolicy(read, in), 3389)
	if got.Outcome != OutcomeManual {
		t.Fatalf("outcome = %s, want manual", got.Outcome)
	}
	for _, want := range []string{"delete the foreign rule", "`iptables -P INPUT DROP`", "`ip6tables -P INPUT DROP`"} {
		if !strings.Contains(got.Fix, want) {
			t.Errorf("fix lacks %q: %s", want, got.Fix)
		}
	}
}

// TestManualRewriteClearsSuccess pins that every rewrite of an ok step to
// manual also clears the legacy Success, so setup warns and its JSON agrees.
func TestManualRewriteClearsSuccess(t *testing.T) {
	ok := newStep("Firewall configuration", OutcomeOK, "Firewall allows TCP 3389", "")
	cases := map[string]SetupStep{
		"v4 policy":   withPolicy(firewallRead{inputOpen: true, backend: backendIPT}, ok),
		"v6 policy":   withV6Policy(firewallRead{v6Open: true, backend: backendIPT}, ok),
		"v6 exposure": withIP6(firewallRead{v6Foreign: []string{"-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT"}, backend: backendIPT}, ok, 3389),
	}
	for name, got := range cases {
		if got.Outcome != OutcomeManual || got.Success {
			t.Errorf("%s: outcome %s success %v, want manual and false", name, got.Outcome, got.Success)
		}
	}
}

// TestUFWPolicy pins #735: an active ufw writes the kernel's INPUT policies
// from its incoming default, so an ACCEPT policy there restricts nothing, as
// on an iptables host. The fix names ufw's own commands, never a raw -P.
func TestUFWPolicy(t *testing.T) {
	// With IPV6=no, ufw stored only the v4 copy of every allow: SSH over IPv6 needs
	// its v6 copy before the v6 deny applies, and `ufw allow` adds it cleanly only
	// while ufw is disabled (probed on ufw 0.36.2; review of #748).
	const v6UFWSteps = "run `ufw disable`, set `IPV6=yes` in /etc/default/ufw, run `ufw allow <your SSH port>/tcp` so ufw adds the IPv6 copy of the SSH allow, then `ufw enable`"
	policy := func(p string) string { return strings.Replace(ufwHostInput, "-P INPUT DROP", "-P INPUT "+p, 1) }
	runner := func(status, v4, v6 string, fail bool) *recordingRunner {
		r := &recordingRunner{elevated: true, onPath: map[string]bool{"ufw": true, "iptables": true, "ip6tables": true},
			output: map[string]string{ufwStatus: status, ufwAdded: ufwAddedHeader + ufwScopedAllow, iptRead: v4, iptChain: tsInputNoAccept, ip6Read: "-P INPUT " + v6 + "\n-A INPUT -j ts-input\n"}}
		if fail {
			r.fail = map[string]bool{ufwStatus: true}
		}
		return r
	}
	steps := func(status, v4, v6 string, fail bool) (setup, check SetupStep, ok bool) {
		read := readLinuxFirewall(runner(status, v4, v6, fail), 3389)
		return firewallSetupStep(runner(status, v4, v6, fail), 3389), firewallCheckStep(read, 3389), linuxFirewallOK(read)
	}
	tests := []struct {
		name     string
		v4, v6   string
		manual   bool
		fixHas   []string
		fixLacks []string
	}{
		{"incoming allow: both policies ACCEPT", "ACCEPT", "ACCEPT", true, []string{"`ufw allow <your SSH port>/tcp`", "`ufw default deny incoming`", v6UFWSteps}, []string{"-P INPUT DROP", "`ufw reload`"}},
		{"IPV6=no after a boot: v6 ACCEPT only", "DROP", "ACCEPT", true, []string{v6UFWSteps}, []string{"-P INPUT DROP", "`ufw reload`"}},
		{"incoming deny: both DROP", "DROP", "DROP", false, nil, []string{"ufw default deny incoming", "IPV6=yes"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup, check, ok := steps(ufwActive, policy(tt.v4), tt.v6, false)
			for _, s := range []SetupStep{setup, check} {
				if (s.Outcome == OutcomeManual) != tt.manual || s.Success == tt.manual {
					t.Errorf("outcome %v success %v, want manual=%v: %q", s.Outcome, s.Success, tt.manual, s.Message)
				}
				for _, want := range tt.fixHas {
					if !strings.Contains(s.Fix, want) {
						t.Errorf("fix lacks %q: %s", want, s.Fix)
					}
				}
				for _, bad := range tt.fixLacks {
					if strings.Contains(s.Fix, bad) {
						t.Errorf("fix names %q: %s", bad, s.Fix)
					}
				}
			}
			if ok == tt.manual {
				t.Errorf("linuxFirewallOK = %v, want %v", ok, !tt.manual)
			}
		})
	}
	// An inactive or unreadable ufw keeps what it reported with closed policies.
	for _, c := range []struct {
		name   string
		status string
		fail   bool
	}{{"inactive", "Status: inactive\n", false}, {"unreadable", "", true}} {
		openSetup, openCheck, _ := steps(c.status, policy("ACCEPT"), "ACCEPT", c.fail)
		shutSetup, shutCheck, _ := steps(c.status, policy("DROP"), "DROP", c.fail)
		if openSetup.Outcome != shutSetup.Outcome || openCheck.Outcome != shutCheck.Outcome || openSetup.Fix != shutSetup.Fix || openCheck.Fix != shutCheck.Fix {
			t.Errorf("%s ufw changed with open policies: setup %v -> %v, check %v -> %v", c.name, shutSetup.Outcome, openSetup.Outcome, shutCheck.Outcome, openCheck.Outcome)
		}
	}
}

// TestUFWPolicyUnread pins review F3 of #735: on an active ufw whose own allow
// decides, a failed iptables read leaves the policy unknown, so check and setup
// must not report the host as closed.
func TestUFWPolicyUnread(t *testing.T) {
	newRunner := func() *recordingRunner {
		return &recordingRunner{elevated: true, onPath: map[string]bool{"ufw": true, "iptables": true},
			output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwAddedHeader + ufwScopedAllow}, exit: map[string]int{iptRead: 4}}
	}
	read := readLinuxFirewall(newRunner(), 3389)
	for name, s := range map[string]SetupStep{"setup": firewallSetupStep(newRunner(), 3389), "check": firewallCheckStep(read, 3389)} {
		if s.Outcome != OutcomeUnverified || s.Success {
			t.Errorf("%s = %v (success %v) %q, want unverified", name, s.Outcome, s.Success, s.Message)
		}
		if !strings.Contains(s.Message, "INPUT policy could not be read") {
			t.Errorf("%s message does not say the policy is unread: %q", name, s.Message)
		}
	}
	if linuxFirewallOK(read) {
		t.Error("linuxFirewallOK = true with the policy unread")
	}
}

// TestUFWPolicyMessage pins review F4 of #735: on a ufw host the message names
// ufw's incoming default, not a raw iptables policy another firewall may filter.
func TestUFWPolicyMessage(t *testing.T) {
	ok := newStep(linuxFwName, OutcomeOK, "Firewall allows TCP 3389 (ufw)", "")
	got := withPolicy(firewallRead{backend: backendUFW, inputOpen: true}, ok)
	if !strings.Contains(got.Message, "ufw's incoming default is allow") || strings.Contains(got.Message, "unless another firewall") {
		t.Errorf("v4 message: %q", got.Message)
	}
	got6 := withV6Policy(firewallRead{backend: backendUFW, v6Open: true}, ok)
	if !strings.Contains(got6.Message, "IPV6=no") || strings.Contains(got6.Message, "unless another firewall") {
		t.Errorf("v6 message: %q", got6.Message)
	}
	// The runbook keys its raw-iptables rows on these phrases, and their fix is
	// the `-P` that ufw rewrites: a ufw message must not match those rows.
	for _, key := range []string{"the iptables INPUT policy is ACCEPT", "the ip6tables INPUT policy is ACCEPT"} {
		for _, m := range []string{got.Message, got6.Message} {
			if strings.Contains(m, key) {
				t.Errorf("ufw message %q carries the iptables runbook key %q", m, key)
			}
		}
	}
}

// TestUFWPolicyReadButChainUnread pins review round 2 of #735: a readable
// ACCEPT policy stays manual with ufw's fix when only the ts-input walk fails.
func TestUFWPolicyReadButChainUnread(t *testing.T) {
	r := &recordingRunner{elevated: true, onPath: map[string]bool{"ufw": true, "iptables": true},
		output: map[string]string{ufwStatus: ufwActive, ufwAdded: ufwAddedHeader + ufwScopedAllow, iptRead: strings.Replace(ufwHostInput, "-P INPUT DROP", "-P INPUT ACCEPT", 1)},
		exit:   map[string]int{iptChain: 4}}
	read := readLinuxFirewall(r, 3389)
	check := firewallCheckStep(read, 3389)
	if read.policyUnread != "" || check.Outcome != OutcomeManual || !strings.Contains(check.Fix, "`ufw default deny incoming`") {
		t.Errorf("policyUnread %q, check %v %q fix %q; want manual naming ufw default deny incoming", read.policyUnread, check.Outcome, check.Message, check.Fix)
	}
	// A raw iptables host whose INPUT read fails says so once, without ufw's words.
	ipt := &recordingRunner{elevated: true, onPath: map[string]bool{"iptables": true}, exit: map[string]int{iptRead: 4}}
	if c := firewallCheckStep(readLinuxFirewall(ipt, 3389), 3389); strings.Contains(c.Message, "ufw") {
		t.Errorf("an iptables host's unread step names ufw: %q", c.Message)
	}
}

// `nft list chains` output for TestForeignFirewall, in the shape nft 1.0
// prints it (probed on Ubuntu 24.04 for #739).
const (
	nftInetFilter = "table inet filter {\n\tchain input {\n\t\ttype filter hook input priority filter; policy accept;\n\t}\n\tchain output {\n\t\ttype filter hook output priority filter; policy accept;\n\t}\n}\n"
	nftFirewalld  = "table inet firewalld {\n\tchain filter_INPUT {\n\t\ttype filter hook input priority filter + 10; policy accept;\n\t}\n}\n"
	nftIptOnly    = "table ip filter {\n\tchain INPUT {\n\t\ttype filter hook input priority filter; policy accept;\n\t}\n\tchain ts-input {\n\t}\n}\n" +
		"table ip6 filter {\n\tchain INPUT {\n\t\ttype filter hook input priority filter; policy accept;\n\t}\n}\n" +
		"table ip nat {\n\tchain POSTROUTING {\n\t\ttype nat hook postrouting priority srcnat; policy accept;\n\t}\n}\n"
	nftNoInput = "table inet natonly {\n\tchain post {\n\t\ttype nat hook postrouting priority srcnat; policy accept;\n\t}\n}\n"
)

// TestForeignFirewall pins #739: where firewalld or a native nftables ruleset
// filters inbound traffic, iptables is not authoritative. check is unverified
// and setup manual, both naming that firewall; setup writes nothing, and
// neither the v4 nor the v6 policy is read. A failed probe is an unknown read.
func TestForeignFirewall(t *testing.T) {
	const fwState, nftChains = "firewall-cmd --state", "nft list chains"
	tests := []struct {
		name          string
		firewalld     bool // firewall-cmd on PATH
		fwOut         string
		fwExit        int
		nft           bool // nft on PATH
		nftOut        string
		nftFails      bool
		want, fixWant string // "" in want: no foreign firewall, the iptables read decides
		unknown       bool
	}{
		{name: "firewalld running", firewalld: true, fwOut: "running\n", want: "firewalld", fixWant: "--change-interface=tailscale0"},
		{name: "firewalld running wins over its nft table", firewalld: true, fwOut: "running\n", nft: true, nftOut: nftFirewalld, want: "firewalld is running", fixWant: "firewall-cmd --reload"},
		{name: "firewalld not running", firewalld: true, fwOut: "not running\n", fwExit: 252},
		{name: "firewalld not running, exit 0", firewalld: true, fwOut: "not running\n"},
		{name: "firewalld state fails", firewalld: true, fwExit: 1, want: fwState, unknown: true},
		{name: "native inet filter", nft: true, nftOut: nftInetFilter, want: "table inet filter", fixWant: "`nft insert rule inet filter input iifname \"tailscale0\" tcp dport 3389 accept`"},
		{name: "firewalld table, firewall-cmd absent", nft: true, nftOut: nftFirewalld, want: "table inet firewalld", fixWant: "nft insert rule inet firewalld filter_INPUT"},
		{name: "iptables-nft and tailscaled tables only", nft: true, nftOut: nftIptOnly},
		{name: "table without an input hook", nft: true, nftOut: nftNoInput},
		{name: "nft listing fails", nft: true, nftFails: true, want: nftChains, unknown: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newRunner := func() *recordingRunner {
				r := &recordingRunner{elevated: true,
					onPath: map[string]bool{"iptables": true, "ip6tables": true, "firewall-cmd": tt.firewalld, "nft": tt.nft},
					output: map[string]string{iptRead: "-P INPUT ACCEPT\n", ip6Read: "-P INPUT ACCEPT\n", fwState: tt.fwOut, nftChains: tt.nftOut},
					exit:   map[string]int{}, fail: map[string]bool{nftChains: tt.nftFails}}
				if tt.fwExit != 0 {
					r.exit[fwState] = tt.fwExit
				}
				return r
			}
			r := newRunner()
			setup := firewallSetupStep(r, 3389)
			wrote := slices.ContainsFunc(r.calls, func(c string) bool { return strings.HasPrefix(c, "run ") })
			cr := newRunner()
			read := readLinuxFirewall(cr, 3389)
			check := firewallCheckStep(read, 3389)
			if tt.want == "" {
				if !wrote || !strings.Contains(check.Message, "the iptables INPUT policy is ACCEPT") {
					t.Errorf("no foreign firewall: setup wrote = %v, check %q; want the iptables read to decide", wrote, check.Message)
				}
				return
			}
			setupWant := OutcomeManual
			if tt.unknown {
				setupWant = OutcomeFailed
			}
			for _, s := range []struct {
				name string
				step SetupStep
				want Outcome
			}{{"setup", setup, setupWant}, {"check", check, OutcomeUnverified}} {
				if s.step.Outcome != s.want || s.step.Success {
					t.Errorf("%s = %v (success %v) %q, want %v", s.name, s.step.Outcome, s.step.Success, s.step.Message, s.want)
				}
				if !strings.Contains(s.step.Message, tt.want) {
					t.Errorf("%s message %q does not name %q", s.name, s.step.Message, tt.want)
				}
				if !strings.Contains(s.step.Fix, tt.fixWant) {
					t.Errorf("%s fix %q lacks %q", s.name, s.step.Fix, tt.fixWant)
				}
				if strings.Contains(s.step.Message, "INPUT policy is ACCEPT") {
					t.Errorf("%s message %q reads a policy iptables does not decide", s.name, s.step.Message)
				}
			}
			if wrote {
				t.Errorf("setup wrote a rule: %v", r.calls)
			}
			if slices.Contains(cr.calls, "output "+ip6Read) {
				t.Errorf("the v6 read ran: %v", cr.calls)
			}
			if linuxFirewallOK(read) {
				t.Error("linuxFirewallOK = true")
			}
		})
	}
}

// TestNFTForeignInput pins which `nft list chains` tables the #739 probe
// counts: an input hook outside iptables-nft's own ip/ip6 tables.
func TestNFTForeignInput(t *testing.T) {
	hook := func(family, table, chain, h string) string {
		return "table " + family + " " + table + " {\n\tchain " + chain + " {\n\t\ttype filter hook " + h + " priority filter; policy accept;\n\t}\n}\n"
	}
	tests := []struct{ name, out, want string }{
		{"inet table", hook("inet", "filter", "input", "input"), "inet filter input"},
		{"ip table of another tool", hook("ip", "myfw", "in", "input"), "ip myfw in"},
		{"ip6 filter is iptables-nft's", hook("ip6", "filter", "INPUT", "input"), ""},
		{"ip security is iptables-nft's", hook("ip", "security", "INPUT", "input"), ""},
		{"ip mangle is iptables-nft's", hook("ip", "mangle", "INPUT", "input"), ""},
		{"ip6 raw is iptables-nft's", hook("ip6", "raw", "PREROUTING", "input"), ""},
		{"inet output hook only", hook("inet", "filter", "output", "output"), ""},
		{"bridge family", hook("bridge", "filter", "input", "input"), ""},
		{"regular chain, no hook", "table inet filter {\n\tchain allow {\n\t}\n}\n", ""},
		{"second table", hook("ip", "filter", "INPUT", "input") + hook("inet", "fw", "in", "input"), "inet fw in"},
		{"empty listing", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			family, table, chain := nftForeignInput(tt.out)
			if got := strings.TrimSpace(strings.Join([]string{family, table, chain}, " ")); got != tt.want {
				t.Errorf("nftForeignInput = %q, want %q", got, tt.want)
			}
		})
	}
}
