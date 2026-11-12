package host

import (
	"cmp"
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	xrdpName      = "xrdp service"
	linuxFwName   = "Firewall configuration"
	xrdpFixUnmask = "sudo systemctl unmask xrdp, then run ts-bridge host setup"
	xrdpFixSetup  = "run ts-bridge host setup as root"
	backendUFW    = "ufw"
	backendIPT    = "iptables"
	backendIP6    = "ip6tables"
	protoTCP      = "tcp"
	iptAccept     = "ACCEPT"
	iptInput      = "INPUT"
	iptDport      = "--dport"
	ufwAllow      = "allow"
	ufwPort       = "port"
)

func linuxPort(cfg Config) int {
	if cfg.Port == 0 {
		return defaultRDPPort
	}
	return cfg.Port
}

// setupLinux reads each state first and acts only on a difference, so a second
// run on a converged host changes nothing (#454, ADR-017).
func setupLinux(cfg Config, r Runner) SetupResult {
	port := linuxPort(cfg)
	steps := []SetupStep{xrdpSetupStep(r)}
	// Only a declared port moves xrdp; an undeclared one is the operator's (#557).
	if cfg.PortDeclared {
		if s, ok := xrdpPortSetupStep(r, port); ok {
			steps = append(steps, s)
		}
	}
	steps = append(steps, firewallSetupStep(r, port), tailscaleIPStep(r))
	return SetupResult{RDPPort: port, Steps: steps}
}

func checkLinux(cfg Config, r Runner, result *CheckResult) {
	port := linuxPort(cfg)
	x := readXRDP(r)
	fw := readLinuxFirewall(r, port)
	xrdp := xrdpCheckStep(x)
	result.Steps = append(result.Steps, xrdp)
	if cfg.PortDeclared {
		if s, ok := xrdpPortCheckStep(r, port); ok {
			result.Steps = append(result.Steps, s)
		}
	}
	result.Steps = append(result.Steps, firewallCheckStep(fw, port))
	svc := Service{Kind: ServiceRDP, Port: port, Enabled: x.enabled, FirewallOK: linuxFirewallOK(fw), Unread: xrdp.Outcome == OutcomeUnverified}
	if x.readable {
		active := x.active
		svc.Active = &active
	}
	result.Services = []Service{svc}
}

// xrdpState is the xrdp service as systemd reports it. readable is false when
// systemctl is missing or answered nothing, so "off" and "cannot tell" differ.
// undecided holds an is-enabled answer ts-bridge does not interpret.
type xrdpState struct {
	installed, readable, enabled, active, masked bool
	undecided                                    string
}

// readXRDP asks systemd for the unit, not PATH for the binary: xrdp lives in
// /usr/sbin, which a non-root PATH often lacks. Without systemctl the binary on
// PATH is the only evidence left.
func readXRDP(r Runner) xrdpState {
	if _, err := r.LookPath("systemctl"); err != nil {
		_, err := r.LookPath("xrdp")
		return xrdpState{installed: err == nil}
	}
	// is-enabled and is-active exit non-zero when the answer is "no", and
	// still print it, so the printed word decides, not the exit status.
	enabled, _ := r.Output("systemctl", "is-enabled", "xrdp")
	e := strings.TrimSpace(string(enabled))
	if e == "not-found" {
		return xrdpState{}
	}
	active, _ := r.Output("systemctl", "is-active", "xrdp")
	a := strings.TrimSpace(string(active))
	if e == "" || a == "" {
		return xrdpState{installed: true}
	}
	x := xrdpState{installed: true, readable: true, active: a == "active"}
	switch e {
	case "enabled", "static": // static: no [Install] section, enable cannot change it
		x.enabled = true
	case "masked":
		x.masked = true
	case "disabled", "enabled-runtime": // enabled-runtime lasts until the next boot
	default:
		// indirect, generated, alias, transient, linked...: whether xrdp starts
		// at boot depends on another unit, so ts-bridge does not guess.
		return xrdpState{installed: true, active: x.active, undecided: e}
	}
	return x
}

// xrdpUnmet returns the step for an xrdp that is missing or unreadable;
// unreadable is the outcome to report when systemd cannot be asked.
func xrdpUnmet(x xrdpState, unreadable Outcome) (SetupStep, bool) {
	if !x.installed {
		return newStep(xrdpName, OutcomeFailed, "xrdp is not installed", "sudo apt install xrdp"), true
	}
	if x.undecided != "" {
		return newStep(xrdpName, unreadable, fmt.Sprintf("systemctl is-enabled answered %q, which ts-bridge does not interpret", x.undecided),
			"check systemctl status xrdp, and enable it by hand if it does not start at boot"), true
	}
	if !x.readable {
		return newStep(xrdpName, unreadable, "Cannot read the xrdp service state: ts-bridge reads it with systemctl",
			"enable and start xrdp with your init system"), true
	}
	return SetupStep{}, false
}

func xrdpSetupStep(r Runner) SetupStep {
	x := readXRDP(r)
	if step, done := xrdpUnmet(x, OutcomeFailed); done {
		return step
	}
	if x.enabled && x.active {
		return newStep(xrdpName, OutcomeOK, "xrdp is enabled and running", "")
	}
	if x.masked {
		return newStep(xrdpName, OutcomeFailed, "xrdp is masked, so it cannot be enabled", xrdpFixUnmask)
	}
	if err := r.Run("systemctl", "enable", "--now", "xrdp"); err != nil {
		return newStep(xrdpName, OutcomeFailed, fmt.Sprintf("Could not enable xrdp: %v", err), "sudo systemctl enable --now xrdp")
	}
	return newStep(xrdpName, OutcomeChanged, "xrdp enabled and started", "")
}

func xrdpCheckStep(x xrdpState) SetupStep {
	if step, done := xrdpUnmet(x, OutcomeUnverified); done {
		return step
	}
	if x.enabled && x.active {
		return newStep(xrdpName, OutcomeOK, "xrdp is enabled and running", "")
	}
	if x.masked {
		return newStep(xrdpName, OutcomeManual, "xrdp is masked, and setup does not unmask units", xrdpFixUnmask)
	}
	return newStep(xrdpName, OutcomeChanged, fmt.Sprintf("xrdp is not enabled and running (enabled=%t, active=%t)", x.enabled, x.active), xrdpFixSetup)
}

type ruleState int

const (
	ruleUnknown ruleState = iota
	ruleAbsent
	rulePresent
	ruleBlocked // an unscoped deny decides the port before any allow
)

// firewallRead is one read of the firewall rule for a port. backend names the
// tool that answered; noBackend means neither ufw nor iptables is installed.
type firewallRead struct {
	state     ruleState
	backend   string
	noBackend bool
	reason    string
	blockedBy string // the deny rule, when state is ruleBlocked
	fix       string // the fix for an unknown read, when the generic one does not apply
	copies    int    // copies of the unscoped rule setup appended to INPUT before #631 (#480), counted on ufw hosts too (#530)
	decidedBy string // the tool whose rules decided state, when not backend: iptables on a ufw host (#530)
	// copiesAdmit means the copies are the only allow: without them the port is closed (#576).
	copiesAdmit bool
	// ufwInactive means `ufw status` says inactive: ufw enforces none of its
	// rules, so its allow admits nothing and its absence closes nothing (#608).
	ufwInactive bool
	ufwAllows   bool // ufw's own rules allow the port, whatever iptables decided
	// ufwLegacy means ufw holds the exact unscoped allow setup wrote before #631,
	// copies counts iptables' (#480), and foreign lists the unscoped allows for the
	// port that setup did not write. scoped means a rule limited to part of the
	// traffic admits the port, so setup adds none (#631).
	ufwLegacy bool
	foreign   []string
	scoped    bool
	// ufwScoped is scoped counting ufw's own list only: on an inactive ufw an
	// iptables rule does not stand in for the allow ufw will need once enabled.
	ufwScoped bool
	// v6Foreign lists the unscoped allows for the port in ip6tables' INPUT, and
	// v6Note why ip6tables could not be read. The v6 read is an exposure scan,
	// not a reachability decision: the descriptor dials the IPv4 address (#526).
	v6Foreign []string
	v6Note    string
	// inputOpen means iptables' INPUT policy is ACCEPT: every port is open on
	// every interface, so the tailscale0 scope restricts nothing (#691). An
	// active ufw writes it from its incoming default (#735). See policyRead.
	inputOpen bool
	// v6Open means ip6tables' INPUT policy is ACCEPT: every port is open on
	// every IPv6 address (#733); on ufw, incoming allow or IPV6=no (#735).
	v6Open bool
	// policyUnread is why `iptables -S INPUT`, and so the v4 policy, could not
	// be read. It decides the step on an active ufw whose own rules decided the
	// state: the exposure is unknown (#735).
	policyUnread string
	// other names a firewall the read does not parse, firewalld or a native
	// nftables ruleset, that filters inbound traffic; fix says how to open the
	// port in it. Nothing else is read: iptables is not authoritative (#739).
	other string
}

// source names the tool whose rules decided the read, for messages; backend
// stays the tool setup writes through.
func (read firewallRead) source() string {
	if read.decidedBy != "" {
		return read.decidedBy
	}
	return read.backend
}

// readLinuxFirewall reads with ufw when it is installed, otherwise iptables,
// unless a firewall it does not parse filters the host (#739).
// Every read needs root, so it runs nothing unprivileged: some iptables
// backends exit 1, the "absent" answer, on permission denied.
func readLinuxFirewall(r Runner, port int) firewallRead {
	if r.Elevated() {
		if read, found := readForeignFirewall(r, port); found {
			return read
		}
	}
	read := readLinuxFirewallV4(r, port)
	if r.Elevated() {
		read.v6Foreign, read.v6Open, read.v6Note = readIP6(r, port)
	}
	return read
}

// readIP6 lists the unscoped allows for port in ip6tables' INPUT and the
// chains it jumps to (#720), and says whether INPUT's policy is ACCEPT (#733).
// setup never writes a v6 rule, so every one of them is the operator's to
// delete.
func readIP6(r Runner, port int) (foreign []string, open bool, note string) {
	if _, err := r.LookPath(backendIP6); err != nil {
		return nil, false, ""
	}
	out, err := r.Output(backendIP6, "-S", iptInput)
	if err != nil {
		return nil, false, fmt.Sprintf("%s -S %s: %v%s", backendIP6, iptInput, err, toolSaid(err))
	}
	foreign, note = ip6Walk(r, iptRules(string(out)), port)
	return foreign, iptPolicyAccept(string(out)), note
}

// ip6Walk scores INPUT and every chain it reaches through a jump that applies
// to all the port's traffic (ip6Jumps), reading each chain once, so a loop
// ends. Unlike the v4 walk it decides nothing setup writes: a qualified jump
// is not an unknown read here, only a scope the chain's rules inherit (#720).
// A target that is not a chain is skipped; a chain that cannot be read is a
// note, and what the others showed is kept.
func ip6Walk(r Runner, input string, port int) (foreign []string, note string) {
	seen := map[string]bool{iptInput: true}
	queue := []string{iptInput}
	rules := map[string]string{iptInput: input}
	var notes []string
	for len(queue) > 0 {
		chain := queue[0]
		queue = queue[1:]
		foreign = append(foreign, ip6Exposed(rules[chain], chain, port)...)
		for _, next := range ip6Jumps(rules[chain], chain, port) {
			if seen[next] {
				continue
			}
			seen[next] = true
			out, err := r.Output(backendIP6, "-S", next)
			if err != nil {
				if !chainMissing(err, iptNoChain) {
					notes = append(notes, fmt.Sprintf("%s -S %s: %v%s", backendIP6, next, err, toolSaid(err)))
				}
				continue
			}
			rules[next] = iptRules(string(out))
			queue = append(queue, next)
		}
	}
	return foreign, strings.Join(notes, "; ")
}

// ip6Verdicts are the targets that end a rule; none of them is a chain.
var ip6Verdicts = []string{iptAccept, "DROP", "REJECT", "RETURN", "QUEUE", "NFQUEUE"}

// ufw6Prefix names the chains ufw writes for IPv6. `ufw show added` already
// reports an allow there, with a fix ufw keeps: an ip6tables -D would be undone
// on ufw's next reload, so the walk leaves them to the ufw read.
const ufw6Prefix = "ufw6-"

// ip6Jumps lists the targets chain's rules jump or go to that may be chains,
// for the jumps that apply to every packet for port: the match covers TCP on
// port (iptMatchesPort), and no plain -i, -s or -d or loopback scope holds the
// jump to part of the traffic. A chain behind a narrowed jump admits only that
// scope, which is not exposure on every interface.
func ip6Jumps(input, chain string, port int) []string {
	var targets []string
	for _, line := range strings.Split(input, "\n") {
		if target := ip6Jump(strings.Fields(iptQuoted.ReplaceAllString(line, `""`)), chain, port); target != "" {
			targets = append(targets, target)
		}
	}
	return targets
}

// ip6Jump is the target of one rule of chain when the walk follows it, or "".
// A verdict ends the rule and is no chain.
func ip6Jump(f []string, chain string, port int) string {
	if len(f) < 2 || f[0] != "-A" || f[1] != chain {
		return ""
	}
	target := cmp.Or(iptFlag(f, "-j"), iptFlag(f, "-g"))
	if target == "" || slices.Contains(ip6Verdicts, target) || strings.HasPrefix(target, ufw6Prefix) {
		return ""
	}
	return iptIfTarget(iptMatchesPort(ip6Match(f), port) && !ip6Narrowed(f) && !iptLoopback(f), target)
}

// iptIfTarget is target when ok, else "".
func iptIfTarget(ok bool, target string) string {
	if ok {
		return target
	}
	return ""
}

// ip6ScopeFlags are the scopes ip6Match drops from a jump's match.
var ip6ScopeFlags = []string{"-i", "-s", "-d"}

// ip6Match is a jump rule's match without its -i, -s and -d scopes, negated or
// not, which ip6Narrowed and iptLoopback judge, and with -g read as -j, the
// target flag iptMatchesPort knows.
func ip6Match(f []string) []string {
	var out []string
	for i := 0; i < len(f); i++ {
		switch {
		case f[i] == "!" && i+1 < len(f) && slices.Contains(ip6ScopeFlags, f[i+1]):
			i += 2
		case slices.Contains(ip6ScopeFlags, f[i]):
			i++
		case f[i] == "-g":
			out = append(out, "-j")
		default:
			out = append(out, f[i])
		}
	}
	return out
}

// ip6Exposed lists the v6 allows for the port in chain that are not narrowed:
// the exposure scan, not setup's rule-shape test. A negated -i/-s/-d or an
// explicit ::/0 still opens the port off the tailnet (BUG-526 review). The
// scan ignores rule order, as iptUnscoped does: an allow behind an unscoped
// deny is still listed, which over-reports a dead rule rather than miss one.
func ip6Exposed(input, chain string, port int) []string {
	var found []string
	for _, line := range strings.Split(input, "\n") {
		f := strings.Fields(iptQuoted.ReplaceAllString(line, `""`))
		if len(f) > 1 && f[0] == "-A" && f[1] == chain && iptFlag(f, "-j") == iptAccept && iptPortCovers(f, port) && !iptLoopback(f) && !ip6Narrowed(f) {
			found = append(found, strings.TrimSpace(line))
		}
	}
	return found
}

// ip6Narrowed reports a rule held to part of the traffic by a plain -i, -s
// or -d: a negated one, or ::/0, narrows nothing.
func ip6Narrowed(f []string) bool {
	return slices.ContainsFunc([]string{"-i", "-s", "-d"}, func(flag string) bool {
		v := iptFlag(f, flag)
		return v != "" && v != "::/0" // iptFlag skips a negated flag
	})
}

// linuxFirewallOK is check's firewall_ok: the port is admitted, through a rule
// scoped to the tailnet, and nothing admits it on every interface (#631, #526).
func linuxFirewallOK(fw firewallRead) bool {
	return fw.state == rulePresent && !fw.ufwInactive && !legacyHeld(fw) && !offTailnetOnly(fw) && len(fw.v6Foreign) == 0 && !((fw.inputOpen || fw.v6Open) && policyRead(fw)) && fw.policyUnread == ""
}

// policyRead reports whether the INPUT policies decide the host's exposure:
// on raw iptables, and on an active ufw, which writes them from its incoming
// default (#735). An inactive ufw already reports itself as off.
func policyRead(fw firewallRead) bool {
	return fw.backend == backendIPT || (fw.backend == backendUFW && !fw.ufwInactive)
}

// withPolicy reports a permissive INPUT policy on an iptables host (#691) or
// an active ufw (#735): a converged, repaired or already manual step gains the
// fix and is manual (#733), since the scoped rule restricts nothing while the
// policy admits every port. Setup never changes the policy: a DROP without an
// SSH allow can lock the operator out. On ufw the fix is ufw's own command,
// since ufw rewrites a raw policy on its next reload.
func withPolicy(read firewallRead, step SetupStep) SetupStep {
	if read.policyUnread != "" { // the policy is unknown, so inputOpen is false
		return withPolicyUnread(read, step)
	}
	if !read.inputOpen || !policyRead(read) {
		return step
	}
	note := fmt.Sprintf("; the iptables INPUT policy is ACCEPT, so unless another firewall filters this host every port is open on every interface and the %s scope restricts nothing", tailnetInterface)
	if read.backend == backendUFW {
		note = fmt.Sprintf("; ufw's incoming default is allow (the INPUT policy is ACCEPT), so every port is open on every interface and the %s scope restricts nothing", tailnetInterface)
	}
	step.Message += note
	if step.Outcome == OutcomeUnverified || step.Outcome == OutcomeFailed {
		return step // an unverified or failed step keeps its own fix; only the message gains the note
	}
	fix := "as root, allow SSH first (`ufw allow <your SSH port>/tcp`) and then run `ufw default deny incoming`; ufw keeps loopback and replies open in its own rules"
	if read.backend == backendIPT {
		fix = iptPolicyFix
	}
	if step.Fix != "" {
		fix = step.Fix + "; " + fix
	}
	step.Outcome, step.Success, step.Fix = OutcomeManual, false, fix
	return step
}

// withPolicyUnread reports an active ufw whose INPUT policy could not be read:
// ufw's own allow decided the state, but whether ufw's default opens every
// port is unknown, so a converged step is unverified, never ok (#735).
func withPolicyUnread(read firewallRead, step SetupStep) SetupStep {
	if read.backend != backendUFW {
		return step // a raw iptables host whose INPUT read failed is already unverified
	}
	step.Message += fmt.Sprintf("; the INPUT policy could not be read (%s), so whether ufw's incoming default opens every port is unknown", read.policyUnread)
	if step.Outcome == OutcomeOK || step.Outcome == OutcomeChanged {
		step.Outcome, step.Success = OutcomeUnverified, false
		step.Fix = appendFix(step.Fix, "run `iptables -S INPUT` as root to see why it fails, and `ufw status verbose` to read the incoming default")
	}
	return step
}

func appendFix(fix, more string) string {
	if fix == "" {
		return more
	}
	return fix + "; " + more
}

// iptPolicyFix closes a raw iptables INPUT policy without locking out the operator.
const iptPolicyFix = "enable a host firewall (ufw), or, as root, keep loopback, replies and SSH open first " +
	"(`iptables -A INPUT -i lo -j ACCEPT`, `iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT`, " +
	"`iptables -A INPUT -p tcp --dport <your SSH port> -j ACCEPT`) and then run `iptables -P INPUT DROP`, " +
	"persisting the set with your distribution's iptables service; or confirm another firewall (firewalld, nftables) filters this host"

// withV6Policy reports a permissive v6 INPUT policy on an iptables host
// (#733), as withPolicy does for v4: a converged, repaired or already manual
// step gains the operator's fix and is manual; an unverified or failed step
// keeps its own fix and gains only the note. Setup never changes the policy.
func withV6Policy(read firewallRead, step SetupStep) SetupStep {
	if !read.v6Open || !policyRead(read) {
		return step
	}
	note := fmt.Sprintf("; the ip6tables INPUT policy is ACCEPT, so unless another firewall filters this host every port is open on every IPv6 address and the %s scope restricts nothing", tailnetInterface)
	if read.backend == backendUFW {
		note = fmt.Sprintf("; ufw's incoming default is allow, or IPV6=no (the IPv6 INPUT policy is ACCEPT), so every port is open on every IPv6 address and the %s scope restricts nothing", tailnetInterface)
	}
	step.Message += note
	if step.Outcome == OutcomeUnverified || step.Outcome == OutcomeFailed {
		return step
	}
	// IPv6 needs ICMPv6 for neighbor discovery: a DROP without it cuts the host off IPv6.
	fix := "as root, keep loopback, replies, ICMPv6 and SSH open first (`ip6tables -A INPUT -i lo -j ACCEPT`, " +
		"`ip6tables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT`, `ip6tables -A INPUT -p ipv6-icmp -j ACCEPT`, " +
		"`ip6tables -A INPUT -p tcp --dport <your SSH port> -j ACCEPT`) " +
		"and then run `ip6tables -P INPUT DROP`, persisting the set with your distribution's ip6tables service; " +
		"or confirm another firewall filters this host, or that IPv6 is disabled"
	if read.backend == backendUFW {
		// With IPV6=no ufw stored only v4 allows; `ufw allow` adds the v6 copy cleanly only while ufw is disabled.
		fix = "as root, run `ufw disable`, set `IPV6=yes` in /etc/default/ufw, run `ufw allow <your SSH port>/tcp` so ufw adds the IPv6 copy of the SSH allow, then `ufw enable`, keeping `ufw default deny incoming`; or confirm that IPv6 is disabled"
	}
	if step.Fix != "" {
		fix = step.Fix + "; " + fix
	}
	step.Outcome, step.Success, step.Fix = OutcomeManual, false, fix
	return step
}

// withIP6 adds the v6 read to a firewall step. An unscoped v6 allow turns a
// converged or repaired step manual; a v6 read that failed is only noted.
func withIP6(read firewallRead, step SetupStep, port int) SetupStep {
	if read.v6Note != "" {
		step.Message += "; IPv6 not read: " + read.v6Note
	}
	step = withV6Policy(read, step)
	if len(read.v6Foreign) == 0 {
		return step
	}
	deletes := make([]string, 0, len(read.v6Foreign))
	for _, rule := range read.v6Foreign {
		deletes = append(deletes, "`"+backendIP6+" -D"+strings.TrimPrefix(rule, "-A")+"`")
	}
	step.Message += fmt.Sprintf("; %s admits TCP %d on every interface: %s", backendIP6, port, strings.Join(read.v6Foreign, "; "))
	fix := fmt.Sprintf("run %s as root to delete the v6 rule (setup never writes one)", strings.Join(deletes, ", then "))
	if step.Fix != "" {
		fix = step.Fix + "; " + fix
	}
	step.Fix = fix
	if step.Outcome == OutcomeOK || step.Outcome == OutcomeChanged {
		step.Outcome, step.Success = OutcomeManual, false
	}
	return step
}

func readLinuxFirewallV4(r Runner, port int) firewallRead {
	if !r.Elevated() {
		return firewallRead{reason: "reading the firewall rules needs root"}
	}
	if _, err := r.LookPath(backendUFW); err == nil {
		return readUFW(r, port)
	}
	if _, err := r.LookPath(backendIPT); err != nil {
		read := firewallRead{noBackend: true, reason: "neither ufw nor iptables is installed"}
		if nft, _ := tsInputInNFT(r, port); nft.reason != "" {
			read.reason += "; " + nft.reason
			read.fix = nft.fix
		}
		return read
	}
	return readIptables(r, port)
}

// readUFW reads `ufw show added`, then iptables. tailscaled's ts-input, and any
// rule appended before ufw was installed, live in INPUT and never in ufw's
// list, so ufw alone reported a tailnet-admitted port as absent and setup
// opened it on every interface (#530). A ufw allow or deny still decides first;
// only ufw's "absent" defers to the iptables read, whose unknown answer stands.
func readUFW(r Runner, port int) firewallRead {
	read := firewallRead{backend: backendUFW}
	active, reason := readUFWStatus(r)
	read.ufwInactive = reason == "" && !active
	if reason != "" {
		read.reason = reason
	} else if out, ufwErr := r.Output(backendUFW, "show", "added"); ufwErr != nil {
		read.reason = fmt.Sprintf("ufw show added: %v", ufwErr)
	} else {
		read.state, read.blockedBy = ufwDecide(string(out), port)
		read.ufwAllows = read.state == rulePresent
		read.ufwLegacy, read.foreign, read.ufwScoped = ufwUnscopedRead(string(out), port)
		read.scoped = read.ufwScoped
	}
	if _, err := r.LookPath(backendIPT); err != nil {
		return read
	}
	// Copies are counted even when ufw is unreadable: each one admits the
	// port on every interface whatever ufw would have said.
	return withUFWIptables(read, readIptables(r, port))
}

// withUFWIptables folds the iptables read into a ufw read: the copies and
// unscoped allows it found, the policy ufw wrote (#735), and its state when ufw
// itself found nothing (#530).
func withUFWIptables(read, ipt firewallRead) firewallRead {
	read.copies = ipt.copies
	read.inputOpen = ipt.inputOpen // ufw writes the policy from its incoming default (#735)
	if !read.ufwInactive && read.state != ruleAbsent && read.reason == "" {
		read.policyUnread = ipt.policyUnread // ufw decided, but its default is unknown (#735)
	}
	read.foreign = append(read.foreign, ipt.foreign...)
	read.scoped = read.scoped || ipt.scoped
	// Only a ufw allow of its own makes the copies redundant; an unreadable ufw proves none (#576).
	read.copiesAdmit = ipt.copiesAdmit && read.state != rulePresent
	if read.state == ruleAbsent {
		read.state, read.blockedBy, read.reason, read.fix = ipt.state, ipt.blockedBy, ipt.reason, ipt.fix
		read.decidedBy = backendIPT
	}
	return read
}

// readUFWStatus reads whether ufw is active from the first line of `ufw
// status`, which execRunner runs under LC_ALL=C so the words are ufw's own.
// An answer that is neither is no read: reason says why.
func readUFWStatus(r Runner) (active bool, reason string) {
	out, err := r.Output(backendUFW, "status")
	if err != nil {
		return false, fmt.Sprintf("ufw status: %v", err)
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	switch strings.TrimSpace(first) {
	case "Status: active":
		return true, ""
	case "Status: inactive":
		return false, ""
	}
	return false, fmt.Sprintf("ufw status printed %q, neither active nor inactive", first)
}

// tsInputChain is the chain tailscaled fills with `-i tailscale0 -j ACCEPT`;
// with --netfilter-mode=on it inserts tsInputJump at the top of INPUT (#492).
const tsInputChain = "ts-input"

var tsInputJump = []string{"-A", iptInput, "-j", tsInputChain}

// readIptables reads INPUT and, behind tailscaled's unqualified jump, the
// ts-input chain. A failed read of either is an unknown state, and so is a
// qualified jump into ts-input that the walk reaches with nothing decided:
// setup never appends an open rule on a guess (#532).
func readIptables(r Runner, port int) firewallRead {
	out, err := r.Output(backendIPT, "-S", iptInput)
	if err != nil {
		reason := fmt.Sprintf("iptables -S INPUT: %v", err)
		return firewallRead{backend: backendIPT, reason: reason, policyUnread: reason}
	}
	read := iptablesRead(r, port, string(out))
	read.inputOpen = iptPolicyAccept(string(out))
	return read
}

// iptPolicyAccept reports whether `iptables -S INPUT` sets the chain policy to
// ACCEPT. Only the first line is the policy: a later line can be the tail of a
// multi-line --comment, which iptables prints raw. A listing without a policy
// line is not read as open.
func iptPolicyAccept(out string) bool {
	first, _, _ := strings.Cut(out, "\n")
	return strings.TrimSpace(first) == "-P INPUT ACCEPT"
}

func iptablesRead(r Runner, port int, out string) firewallRead { //nolint:gocyclo // baseline #629
	input := iptRules(out)
	rules, jump := iptBeforeQualifiedJump(input)
	diverted := slices.ContainsFunc(strings.Split(rules, "\n"), iptTSInputJump)
	if diverted {
		chain, err := r.Output(backendIPT, "-S", tsInputChain)
		if err != nil {
			return firewallRead{backend: backendIPT, reason: fmt.Sprintf("iptables -S %s: %v", tsInputChain, err)}
		}
		rules = iptInlineChain(rules, iptRules(string(chain)))
	}
	state, rule := iptablesDecide(rules, port)
	if state == ruleAbsent && jump == "" && !diverted {
		if unseen, found := tsInputUnreached(r, port); found {
			return unseen
		}
	}
	if state == ruleAbsent && jump != "" {
		return firewallRead{backend: backendIPT, reason: fmt.Sprintf("INPUT reaches %s through %q, which ts-bridge cannot evaluate", tsInputChain, jump),
			fix: fmt.Sprintf("allow TCP %d inbound on the Tailscale interface ahead of %q, or confirm by hand that the jump admits it", port, jump)}
	}
	copies := iptLegacyCopies(input, port)
	foreign, scoped := iptUnscopedRead(input, rules, port, state)
	return firewallRead{state: state, backend: backendIPT, blockedBy: rule, copies: copies, foreign: foreign, scoped: scoped,
		copiesAdmit: state == rulePresent && copies > 0 && !iptablesCovers(iptWithoutLegacy(rules, port), port)}
}

// iptUnscopedRead splits the unscoped allows for port in INPUT into the ones
// setup did not write and says whether a scoped rule admits the port without
// them (#631). Rules walked is what decided state: tailscaled's ts-input is
// inlined there, so its `-i tailscale0 -j ACCEPT` counts as the scoped rule.
// Unscoped allows are looked for in INPUT only: an unscoped port ACCEPT inside
// ts-input would count as the scoped rule. tailscaled writes none (its accepts
// are `-i tailscale0` and loopback, see the captures in the tests), and the
// chain is tailscaled's to rewrite, not setup's.
func iptUnscopedRead(input, walked string, port int, state ruleState) (foreign []string, scoped bool) {
	unscoped := iptUnscoped(input, port)
	legacy := iptPrinted(iptablesLegacy(port))
	foreign = slices.DeleteFunc(slices.Clone(unscoped), func(line string) bool { return slices.Equal(strings.Fields(line), legacy) })
	rest := slices.DeleteFunc(strings.Split(walked, "\n"), func(line string) bool {
		return slices.Contains(unscoped, strings.TrimSpace(line)) || iptAcceptOffTailnet(line)
	})
	return foreign, state == rulePresent && iptablesCovers(strings.Join(rest, "\n"), port)
}

// iptAcceptOffTailnet reports an ACCEPT scoped to something other than the
// tailnet: another interface, or a source or destination address. Such a rule
// admits the port, but not from the tailnet, so it is not the scoped rule and
// setup must still add `-i tailscale0` before it deletes a legacy copy. A
// negated scope (`! -i tailscale0`, `! -s ...`) counts as off the tailnet: it
// may exclude the tailnet, so it never proves the rule admits it (#631 review).
func iptAcceptOffTailnet(line string) bool {
	f := strings.Fields(iptQuoted.ReplaceAllString(line, `""`))
	if iptFlag(f, "-j") != iptAccept {
		return false
	}
	if slices.ContainsFunc([]string{"-i", "-s", "-d"}, func(flag string) bool { return iptNegated(f, flag) }) {
		return true
	}
	if in := iptFlag(f, "-i"); in != "" && in != tailnetInterface {
		return true
	}
	return iptFlag(f, "-s") != "" || iptFlag(f, "-d") != ""
}

// tsInputUnreached reads the two places tailscaled keeps ts-input where an
// INPUT without a jump to it hides it (#526), once nothing in INPUT has decided
// the port. Either is an unknown read: setup must not append a rule that opens
// every interface on a host whose tailnet ACCEPT it cannot see.
func tsInputUnreached(r Runner, port int) (firewallRead, bool) {
	out, err := r.Output(backendIPT, "-S", tsInputChain)
	switch {
	case err != nil && !chainMissing(err, iptNoChain):
		return firewallRead{backend: backendIPT, reason: fmt.Sprintf("iptables -S %s: %v", tsInputChain, err)}, true
	case err == nil && slices.Contains(strings.Split(iptRules(string(out)), "\n"), "-N "+tsInputChain):
		// --netfilter-mode=nodivert: iptables_runner.go creates the chain and skips AddHooks.
		return firewallRead{backend: backendIPT, reason: "ts-input exists but INPUT does not jump to it (tailscaled --netfilter-mode=nodivert, or a divert from another chain)",
			fix: fmt.Sprintf("jump to ts-input from INPUT, or allow TCP %d inbound on the Tailscale interface by hand", port)}, true
	}
	return tsInputInNFT(r, port)
}

// tsInputInNFT finds ts-input in nftables mode (TS_DEBUG_FIREWALL_MODE=nftables,
// or auto-detected where iptables is absent). nftables_runner.go AddChains puts
// it in the conventional `ip filter` table, which iptables-legacy never prints,
// and iptables-nft prints only when INPUT jumps to it. Without the nft binary
// there is nothing to ask.
func tsInputInNFT(r Runner, port int) (firewallRead, bool) {
	if _, err := r.LookPath("nft"); err != nil {
		return firewallRead{}, false
	}
	args := []string{"list", "chain", "ip", "filter", tsInputChain}
	out, err := r.Output("nft", args...)
	switch {
	case err != nil && !chainMissing(err, "No such file or directory"):
		return firewallRead{backend: backendIPT, reason: fmt.Sprintf("nft %s: %v", strings.Join(args, " "), err)}, true
	case err == nil && strings.Contains(string(out), "chain "+tsInputChain):
		return firewallRead{backend: backendIPT, reason: "tailscaled keeps ts-input in nftables table ip filter (nftables mode), which iptables does not read",
			fix: fmt.Sprintf("read it with `nft list chain ip filter ts-input`, or allow TCP %d inbound on the Tailscale interface by hand", port)}, true
	}
	return firewallRead{}, false
}

// toolSaid quotes the first stderr line of a failed command, so a note says
// why the read failed and not only that it did.
func toolSaid(err error) string {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return ""
	}
	if line, _, _ := strings.Cut(strings.TrimSpace(string(exit.Stderr)), "\n"); line != "" {
		return " (" + line + ")"
	}
	return ""
}

// iptNoChain is what iptables and ip6tables print for a chain that does not
// exist, and for a target that is an extension, not a chain.
const iptNoChain = "No chain/target/match by that name"

// chainMissing reports a failure that says the chain does not exist: stderr
// carries the tool's own words for it (iptables "No chain/target/match by that
// name", nft "No such file or directory"). Exit status 1 is the only failure
// code either tool has, so it cannot tell a missing chain from a refused read;
// any other failure leaves the chain unread.
func chainMissing(err error, words string) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && strings.Contains(string(exit.Stderr), words)
}

// iptWithoutLegacy is rules with every copy of the legacy unscoped rule removed.
func iptWithoutLegacy(rules string, port int) string {
	printed := iptPrinted(iptablesLegacy(port))
	kept := slices.DeleteFunc(strings.Split(rules, "\n"), func(line string) bool {
		return slices.Equal(strings.Fields(line), printed)
	})
	return strings.Join(kept, "\n")
}

// iptInlineChain puts the ts-input chain's rules in place of each unqualified
// jump to it, so the walk evaluates them where a packet meets them. A jump with
// a qualifier applies the chain to part of the traffic only; the walk stops
// before one (iptBeforeQualifiedJump).
func iptInlineChain(input, chain string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		if iptTSInputJump(line) {
			lines[i] = strings.TrimRight(chain, "\n")
		}
	}
	return strings.Join(lines, "\n")
}

// iptLegacyCopies counts the lines of rules that are the unscoped rule setup
// appended before #631, in the form `iptables -S` prints it.
func iptLegacyCopies(rules string, port int) int {
	printed := iptPrinted(iptablesLegacy(port))
	n := 0
	for _, line := range strings.Split(rules, "\n") {
		if slices.Equal(strings.Fields(line), printed) {
			n++
		}
	}
	return n
}

// iptPrinted is a rule setup writes as `iptables -S` prints it: the protocol
// after `-p` gains an explicit `-m <protocol>` match. The interface stays ahead
// of `-p`, where iptables prints it.
func iptPrinted(rule []string) []string {
	i := slices.Index(rule, "-p") + 2
	return slices.Insert(slices.Clone(rule), i, "-m", rule[i-1])
}

func iptTSInputJump(line string) bool {
	return slices.Equal(strings.Fields(line), tsInputJump)
}

// iptBeforeQualifiedJump splits INPUT at the first rule that jumps or goes to
// ts-input in any form but tailscaled's unqualified jump. It returns the rules
// before that one and the rule itself, or all of INPUT and "". Quoted values
// are blanked first, so a comment that spells a jump is not one.
func iptBeforeQualifiedJump(input string) (string, string) {
	lines := strings.Split(input, "\n")
	for n, line := range lines {
		f := strings.Fields(iptQuoted.ReplaceAllString(line, `""`))
		if iptTSInputJump(line) || len(f) < 2 || f[0] != "-A" {
			continue
		}
		for i := 2; i+1 < len(f); i++ {
			if (f[i] == "-j" || f[i] == "-g") && f[i+1] == tsInputChain {
				return strings.Join(lines[:n], "\n"), strings.TrimSpace(line)
			}
		}
	}
	return input, ""
}

// iptRules cuts `iptables -S` output into one rule per line, once, before any
// reader splits it (#540). xtables_save_string escapes only `\` and `"`, so a
// newline inside a quoted value (a `--comment`) prints raw, and the text after
// it would read as a rule of its own; here it becomes a space. An unterminated
// quote is not iptables' output: it is returned as printed, so the rules after
// it are not swallowed into one line.
func iptRules(out string) string { //nolint:gocyclo // baseline #629
	b := []byte(out)
	quoted, escaped := false, false
	for i, c := range b {
		switch {
		case escaped:
			escaped = false
			if c == '\n' {
				b[i] = ' '
			}
		case quoted && c == '\\':
			escaped = true
		case c == '"':
			quoted = !quoted
		case quoted && c == '\n':
			b[i] = ' '
		}
	}
	if quoted {
		return out
	}
	return string(b)
}

// iptQuoted matches a quoted value (a `--comment` or `--string`), escaped
// quotes included, so its words cannot pose as flags once the line is split.
var iptQuoted = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

// iptDestPortFlags name a destination port list; iptPortFlags are every flag
// that makes a rule port-bearing, negated or not.
var (
	iptDestPortFlags = []string{iptDport, "--dports", "--ports"}
	iptPortFlags     = append(slices.Clone(iptDestPortFlags), "--sport", "--sports")
)

// iptablesCovers reports whether `iptables -S` output holds an ACCEPT that
// admits TCP port before any unscoped deny decides it.
func iptablesCovers(rules string, port int) bool {
	state, _ := iptablesDecide(rules, port)
	return state == rulePresent
}

// iptablesDecide walks `iptables -S` output in chain order and returns the
// answer of the first rule that decides TCP port: an ACCEPT that admits it
// (interface and source qualifiers are the operator's scope), or an unscoped
// DROP/REJECT that would admit it as an ACCEPT, together with that rule. A
// loopback rule, a negated port, a source port and a jump never decide.
func iptablesDecide(rules string, port int) (ruleState, string) {
	for _, line := range strings.Split(rules, "\n") {
		f := strings.Fields(iptQuoted.ReplaceAllString(line, `""`))
		if len(f) < 2 || f[0] != "-A" || iptLoopback(f) || !iptMatchesPort(f, port) {
			continue
		}
		switch iptFlag(f, "-j") {
		case iptAccept:
			return rulePresent, ""
		case "DROP", "REJECT":
			if !iptHasScope(f) {
				return ruleBlocked, strings.TrimSpace(line)
			}
		}
	}
	return ruleAbsent, ""
}

// iptScopeFlags narrow a rule to part of the traffic; a deny carrying one is
// the operator's scope and never decides the port.
var iptScopeFlags = []string{"-i", "-s", "-d", "!"}

func iptHasScope(f []string) bool {
	return slices.ContainsFunc(f, func(w string) bool { return slices.Contains(iptScopeFlags, w) })
}

// iptUnscoped lists the INPUT rules that accept TCP port with no interface,
// source or destination qualifier: the legacy rules of #631. A rule with no
// port in it is the operator's posture, not a rule for the port, and is not
// listed. Another narrowing (conntrack, a comment) is not looked at, so such a
// rule is listed too and reported rather than deleted.
func iptUnscoped(input string, port int) []string {
	var found []string
	for _, line := range strings.Split(input, "\n") {
		f := strings.Fields(iptQuoted.ReplaceAllString(line, `""`))
		if len(f) > 1 && f[0] == "-A" && f[1] == iptInput && iptFlag(f, "-j") == iptAccept && iptPortCovers(f, port) && !iptLoopback(f) && !iptHasScope(f) {
			found = append(found, strings.TrimSpace(line))
		}
	}
	return found
}

// iptMatchesPort reports whether a rule's match, whatever its target, applies
// to new TCP connections on port.
func iptMatchesPort(f []string, port int) bool {
	if slices.ContainsFunc(f, func(w string) bool { return slices.Contains(iptPortFlags, w) }) {
		return iptPortCovers(f, port)
	}
	return iptPortlessCovers(f)
}

// iptPortCovers reports whether a port-bearing TCP ACCEPT names port in a
// non-negated destination port list.
func iptPortCovers(f []string, port int) bool {
	if iptFlag(f, "-p") != protoTCP {
		return false
	}
	for _, flag := range iptDestPortFlags {
		if portListHas(iptFlag(f, flag), port) {
			return true
		}
	}
	return false
}

// iptPortlessCovers reports whether a non-loopback rule with no port flag
// matches a new TCP connection on every port. Every token after the chain must
// be one this parser models; anything else (conntrack, state, limit, `-f`, a
// `--tcp-flags` set other than `--syn`, a negation that is not of loopback)
// narrows the rule in a way not modeled here, so the rule reads as absent.
func iptPortlessCovers(f []string) bool {
	for i := 2; i < len(f); {
		n := iptPortlessToken(f[i:])
		if n == 0 {
			return false
		}
		i += n
	}
	return true
}

// iptPortlessToken returns how many fields the qualifier at the start of f
// spans when it keeps a rule open to every TCP port, or 0 when it narrows it.
func iptPortlessToken(f []string) int { //nolint:gocyclo // baseline #629
	if len(f) < 2 {
		return 0
	}
	switch v := f[1]; f[0] {
	case "-j", "--reject-with": // the caller decides by target
		return 2
	case "-p", "-m":
		return iptIf(v == protoTCP || (f[0] == "-m" && v == "comment"), 2)
	case "--comment", "-i", "-s", "-d":
		return 2 // loopback scopes were rejected before this point
	case "--tcp-flags": // `--syn` prints as this exact pair
		return iptIf(len(f) > 2 && v == "FIN,SYN,RST,ACK" && f[2] == "SYN", 3)
	case "!": // only a negated loopback scope leaves every other source open
		return iptIf(len(f) > 2 && iptLoopbackScope(v, f[2]), 3)
	}
	return 0
}

func iptIf(ok bool, n int) int {
	if ok {
		return n
	}
	return 0
}

// iptLoopback reports whether a rule is scoped to loopback by a non-negated
// `-i lo` or a loopback source or destination prefix.
func iptLoopback(f []string) bool {
	for i := 0; i+1 < len(f); i++ {
		if (i == 0 || f[i-1] != "!") && iptLoopbackScope(f[i], f[i+1]) {
			return true
		}
	}
	return false
}

// iptLoopbackScope reports whether flag value names loopback: `-i lo` or its
// wildcard `-i lo+`, or `-s`/`-d` with a loopback prefix.
func iptLoopbackScope(flag, value string) bool {
	switch flag {
	case "-i":
		return value == "lo" || value == "lo+"
	case "-s", "-d":
		p, err := netip.ParsePrefix(value)
		return err == nil && p.Addr().IsLoopback()
	}
	return false
}

// iptFlag returns the value of flag in f, or "" when it is absent or negated.
func iptFlag(f []string, flag string) string {
	i := slices.Index(f, flag)
	if i < 0 || i+1 >= len(f) || (i > 0 && f[i-1] == "!") {
		return ""
	}
	return f[i+1]
}

// iptNegated reports a flag written under `!`, which iptFlag reads as absent.
func iptNegated(f []string, flag string) bool {
	i := slices.Index(f, flag)
	return i > 0 && f[i-1] == "!"
}

// tailnetInterface is the interface tailscaled creates, and the only one setup
// opens the service port on (ADR-017 Amendment B, #631).
const tailnetInterface = "tailscale0"

// iptablesAppend is the rule setup appends: the port, on the tailnet interface.
func iptablesAppend(port int) []string {
	return []string{"-A", iptInput, "-i", tailnetInterface, "-p", protoTCP, iptDport, strconv.Itoa(port), "-j", iptAccept}
}

// iptablesLegacy is the unscoped rule setup appended before #631.
func iptablesLegacy(port int) []string {
	return []string{"-A", iptInput, "-p", protoTCP, iptDport, strconv.Itoa(port), "-j", iptAccept}
}

// iptablesDelete deletes one copy of the legacy rule.
func iptablesDelete(port int) []string {
	return append([]string{"-D"}, iptablesLegacy(port)[1:]...)
}

// legacyHeld reports whether the firewall holds an unscoped allow for the
// port: setup replaces the ones it wrote and reports the others (#631).
func legacyHeld(read firewallRead) bool {
	return read.state == rulePresent && (read.ufwLegacy || read.copies > 0 || len(read.foreign) > 0)
}

// offTailnetOnly reports a port that is allowed, but only off the tailnet: every
// allow is scoped to another interface or to an address, so setup still owes
// the tailscale0 rule (#631).
func offTailnetOnly(read firewallRead) bool {
	return read.state == rulePresent && !read.scoped && !legacyHeld(read)
}

// addScopedRule writes the rule setup owns, through the backend that manages
// the host's rules.
func addScopedRule(r Runner, backend string, port int) error {
	if backend == backendUFW {
		return r.Run(backendUFW, ufwAllowArgs(port)...)
	}
	return r.Run(backendIPT, iptablesAppend(port)...)
}

// legacyDeletes are the commands that delete the rules setup wrote before #631:
// its ufw allow, then each iptables copy.
func legacyDeletes(read firewallRead, port int) [][]string {
	var cmds [][]string
	if read.ufwLegacy {
		cmds = append(cmds, append([]string{backendUFW}, ufwDeleteArgs(port)...))
	}
	for range read.copies {
		cmds = append(cmds, append([]string{backendIPT}, iptablesDelete(port)...))
	}
	return cmds
}

// ufwAllows reports whether `ufw show added` holds an inbound allow for TCP
// port before any unscoped deny decides it.
func ufwAllows(added string, port int) bool {
	state, _ := ufwDecide(added, port)
	return state == rulePresent
}

// ufwDecide walks `ufw show added`, which lists user rules in the order ufw
// evaluates them, and returns the answer of the first rule that decides TCP
// port: an inbound allow, or an unscoped deny/reject (no `from`, no `on`, no
// `to <address>`),
// together with that rule. route, out and limit rules never decide, and the
// port must match exactly: 33890 is not 3389.
func ufwDecide(added string, port int) (ruleState, string) { //nolint:gocyclo // baseline #629
	for _, line := range strings.Split(added, "\n") {
		f := strings.Fields(line)
		if i := slices.Index(f, "comment"); i >= 0 {
			f = f[:i]
		}
		if len(f) < 3 || f[0] != backendUFW || slices.Contains(f, "out") || !ufwRuleMatchesPort(f[2:], port) {
			continue
		}
		switch f[1] {
		case ufwAllow:
			return rulePresent, ""
		case "deny", "reject":
			if !ufwScoped(f) {
				return ruleBlocked, strings.TrimSpace(line)
			}
		}
	}
	return ruleAbsent, ""
}

// ufwScoped reports whether a ufw rule narrows its traffic by source,
// interface or destination address, as iptables' -s/-i/-d do. `to any` does
// not narrow it.
func ufwScoped(f []string) bool {
	return slices.Contains(f, "on") || ufwArgIsNot(f, "from", "any") || ufwArgIsNot(f, "to", "any")
}

// ufwAllowArgs is the allow setup writes: the port, on the tailnet interface.
func ufwAllowArgs(port int) []string {
	return []string{ufwAllow, "in", "on", tailnetInterface, "to", "any", ufwPort, strconv.Itoa(port), "proto", protoTCP}
}

// ufwDeleteArgs deletes the unscoped allow setup wrote before #631.
func ufwDeleteArgs(port int) []string {
	return []string{"delete", ufwAllow, ufwLegacyShort(port)}
}

func ufwLegacyShort(port int) string { return fmt.Sprintf("%d/%s", port, protoTCP) }

// ufwUnscopedRead splits the unscoped allows for port in `ufw show added` into
// the exact rule setup wrote before #631 (the only one it deletes) and the
// others, and says whether a scoped allow admits the port without them. A
// trailing comment makes a rule foreign: whether `ufw delete` matches it is not
// known. A rule with no port in it is not a rule for the port.
func ufwUnscopedRead(added string, port int) (legacy bool, foreign []string, scoped bool) {
	var rest []string
	for _, line := range strings.Split(added, "\n") {
		f := strings.Fields(line)
		if ufwAllowOffTailnet(f) {
			continue
		}
		if !ufwUnscopedAllow(f, port) {
			rest = append(rest, line)
		} else if slices.Equal(f, []string{backendUFW, ufwAllow, ufwLegacyShort(port)}) {
			legacy = true
		} else {
			foreign = append(foreign, strings.TrimSpace(line))
		}
	}
	state, _ := ufwDecide(strings.Join(rest, "\n"), port)
	return legacy, foreign, state == rulePresent
}

// ufwAllowOffTailnet reports an allow scoped to something other than the
// tailnet: another interface, or a source or destination address. Such a rule
// admits the port, but not from the tailnet, so it is not the scoped rule and
// setup must still add the tailscale0 allow before it deletes a legacy one.
func ufwAllowOffTailnet(f []string) bool {
	if i := slices.Index(f, "comment"); i >= 0 {
		f = f[:i]
	}
	if !ufwInboundAllow(f) {
		return false
	}
	return ufwArgIsNot(f, "on", tailnetInterface) || ufwArgIsNot(f, "from", "any") || ufwArgIsNot(f, "to", "any")
}

// ufwInboundAllow reports whether the fields of a `ufw show added` line are an
// inbound allow with at least one argument.
func ufwInboundAllow(f []string) bool {
	return len(f) >= 3 && f[0] == backendUFW && f[1] == ufwAllow && !slices.Contains(f, "out")
}

// ufwArgIsNot reports whether keyword appears with a value other than want.
func ufwArgIsNot(f []string, keyword, want string) bool {
	i := slices.Index(f, keyword)
	return i >= 0 && i+1 < len(f) && f[i+1] != want
}

// ufwUnscopedAllow reports whether a `ufw show added` line is an inbound allow
// for TCP port with no interface, source or destination address.
func ufwUnscopedAllow(f []string, port int) bool {
	if i := slices.Index(f, "comment"); i >= 0 {
		f = f[:i+1] // keep the keyword: a commented rule never equals the exact legacy one
	}
	if !ufwInboundAllow(f) || ufwScoped(f) {
		return false
	}
	g := slices.DeleteFunc(slices.Clone(f[2:]), func(w string) bool { return w == "log" || w == "log-all" })
	return ufwRuleMatchesPort(f[2:], port) && (len(g) <= 2 || destPortIndex(g) >= 0)
}

// ufwRuleMatchesPort matches the short form (`3389/tcp`, `3389`, `in 3389/tcp`)
// and the full form (`... port 3389 [proto tcp]`). A trailing `comment '...'` is
// ignored, as is a `log`/`log-all` modifier, and the port may sit in a list or range (`22,3389/tcp`, `3380:3390/tcp`).
func ufwRuleMatchesPort(f []string, port int) bool { //nolint:gocyclo // baseline #629
	if i := slices.Index(f, "comment"); i >= 0 {
		f = f[:i]
	}
	f = slices.DeleteFunc(slices.Clone(f), func(w string) bool { return w == "log" || w == "log-all" })
	if len(f) == 2 && f[0] == "in" {
		f = f[1:]
	}
	if len(f) == 1 {
		ports, proto, _ := strings.Cut(f[0], "/")
		return (proto == "" || proto == protoTCP) && portListHas(ports, port)
	}
	i := destPortIndex(f)
	if i < 0 {
		// No destination port: the rule allows every port, unless a source
		// port or an application profile narrows it.
		return !slices.Contains(f, "port") && !slices.Contains(f, "app") && ufwProtoTCP(f)
	}
	return i+1 < len(f) && portListHas(f[i+1], port) && ufwProtoTCP(f)
}

// ufwProtoTCP reports whether a ufw rule applies to TCP: `proto tcp` or none.
func ufwProtoTCP(f []string) bool {
	j := slices.Index(f, "proto")
	return j < 0 || (j+1 < len(f) && f[j+1] == protoTCP)
}

// destPortIndex returns the index of the destination `port` keyword: the one
// after `to`. A `port` in the `from` clause is a source port and never counts.
func destPortIndex(f []string) int {
	if t := slices.Index(f, "to"); t >= 0 {
		if i := slices.Index(f[t:], "port"); i >= 0 {
			return t + i
		}
		return -1
	}
	if slices.Contains(f, "from") {
		return -1
	}
	return slices.Index(f, "port")
}

// portListHas reports whether a ufw port list (`3389`, `22,3389`, `3380:3390`)
// contains port.
func portListHas(list string, port int) bool {
	for _, p := range strings.Split(list, ",") {
		lo, hi, isRange := strings.Cut(p, ":")
		if !isRange {
			hi = lo
		}
		a, errLo := strconv.Atoi(lo)
		b, errHi := strconv.Atoi(hi)
		if errLo == nil && errHi == nil && a <= port && port <= b {
			return true
		}
	}
	return false
}

func firewallSetupStep(r Runner, port int) SetupStep {
	read := readLinuxFirewall(r, port)
	if read.other != "" {
		return foreignStep(read, port, "setup")
	}
	return withIP6(read, withPolicy(read, firewallSetupFrom(r, read, port)), port)
}

func firewallSetupFrom(r Runner, read firewallRead, port int) SetupStep {
	if inactiveUFWDecides(read) {
		return ufwInactiveSetupStep(r, read, port)
	}
	switch read.state {
	case rulePresent:
		if legacyHeld(read) {
			return legacySetupStep(r, read, port)
		}
		if offTailnetOnly(read) {
			break
		}
		return newStep(linuxFwName, OutcomeOK, fmt.Sprintf("Firewall allows TCP %d (%s)", port, read.source()), "")
	case ruleBlocked:
		return firewallBlockedStep(read, port)
	case ruleUnknown:
		message, fix := firewallUnreadable(read, port, "setup")
		return newStep(linuxFwName, OutcomeFailed, message, fix)
	}
	if err := addScopedRule(r, read.backend, port); err != nil {
		return newStep(linuxFwName, OutcomeFailed, fmt.Sprintf("Could not add the firewall rule: %v", err),
			fmt.Sprintf("allow TCP %d inbound on %s with %s, as root", port, tailnetInterface, read.backend))
	}
	return newStep(linuxFwName, OutcomeChanged, fmt.Sprintf("Firewall rule added: TCP %d on %s (%s)", port, tailnetInterface, read.backend), "")
}

// legacySetupStep replaces the unscoped allows for the port (#631). The scoped
// rule goes in first, so the port is never closed in between, and a failed add
// deletes nothing. Only the rules setup wrote are deleted; any other unscoped
// allow is reported, since setup cannot know it wrote it.
func legacySetupStep(r Runner, read firewallRead, port int) SetupStep {
	if !read.scoped {
		if err := addScopedRule(r, read.backend, port); err != nil {
			return newStep(linuxFwName, OutcomeFailed, fmt.Sprintf("Could not add the firewall rule scoped to %s: %v", tailnetInterface, err),
				fmt.Sprintf("allow TCP %d inbound on %s with %s, as root; the unscoped rule is still in place", port, tailnetInterface, read.backend))
		}
	}
	for _, c := range legacyDeletes(read, port) {
		if err := r.Run(c[0], c[1:]...); err != nil {
			return newStep(linuxFwName, OutcomeFailed, fmt.Sprintf("Could not delete the legacy unscoped firewall rule: %v", err),
				fmt.Sprintf("run `%s`, as root, to delete it", strings.Join(c, " ")))
		}
	}
	if len(read.foreign) > 0 {
		return newStep(linuxFwName, OutcomeManual, fmt.Sprintf("Firewall allows TCP %d on %s (%s), but legacy unscoped rule: %s", port, tailnetInterface, read.source(), strings.Join(read.foreign, "; ")),
			foreignFix(read))
	}
	if read.scoped {
		return newStep(linuxFwName, OutcomeChanged, fmt.Sprintf("Legacy unscoped firewall rule deleted: TCP %d stays allowed by the scoped rule (%s)", port, read.source()), "")
	}
	return newStep(linuxFwName, OutcomeChanged, fmt.Sprintf("Firewall rule replaced: TCP %d is now allowed on %s only (%s)", port, tailnetInterface, read.backend), "")
}

// legacyCheckStep is the read half of legacySetupStep: "changed" when setup
// would act alone, "manual" when a rule it did not write is involved.
func legacyCheckStep(read firewallRead, port int) SetupStep {
	var held []string
	if read.ufwLegacy {
		held = append(held, "ufw allow "+ufwLegacyShort(port))
	}
	if read.copies > 0 {
		held = append(held, copiesHeld(read, port))
	}
	held = append(held, read.foreign...)
	message := fmt.Sprintf("Firewall allows TCP %d on every interface (%s), not only %s: legacy unscoped rule: %s", port, read.source(), tailnetInterface, strings.Join(held, "; "))
	if len(read.foreign) > 0 {
		return newStep(linuxFwName, OutcomeManual, message, foreignFix(read))
	}
	return newStep(linuxFwName, OutcomeChanged, message,
		fmt.Sprintf("run ts-bridge host setup as root: it adds the rule scoped to %s and deletes the unscoped one", tailnetInterface))
}

// foreignFix is the operator's part: setup deletes only the rules it wrote.
func foreignFix(read firewallRead) string {
	return fmt.Sprintf("scope %s to %s, or remove it; setup does not delete rules it did not write", strings.Join(read.foreign, "; "), tailnetInterface)
}

// firewallUnreadable is the message and fix for a firewall that could not be
// read, naming any copies of setup's rule it still counted. Setup and check
// share it so the two report the same state the same way (#576).
func firewallUnreadable(read firewallRead, port int, command string) (message, fix string) {
	message, fix = "Cannot read the firewall: "+read.reason, firewallFix(read, port, command)
	if read.copies > 0 {
		message += "; it also holds " + copiesHeld(read, port)
		fix += "; " + copiesFix(read, port)
	}
	return message, fix
}

// inactiveUFWDecides reports whether the read is an inactive ufw's to report.
// A deny iptables holds is live whatever ufw's state, so it stays the blocked
// step; an unknown read stays the unknown step.
func inactiveUFWDecides(read firewallRead) bool {
	if !read.ufwInactive || read.state == ruleUnknown {
		return false
	}
	return read.state != ruleBlocked || read.decidedBy == ""
}

// ufwInactiveSetupStep adds ufw's allow only when nothing else admits the
// port, so it stays open once ufw is enabled. A ufw deny or an iptables allow
// (tailscaled's ts-input, a leftover copy) means no write: an unscoped ufw
// allow would open the port on every interface (#530).
func ufwInactiveSetupStep(r Runner, read firewallRead, port int) SetupStep {
	if read.ufwLegacy {
		// The legacy allow is inert while ufw is off and unscoped once it is on.
		if step, failed := ufwInactiveReplace(r, read, port); failed {
			return step
		}
		read.ufwLegacy, read.ufwAllows, read.ufwScoped, read.scoped = false, true, true, true // the scoped allow is in place now
	}
	state, add := ufwInactiveState(read, port)
	if !add {
		return ufwInactiveStep(read, port, state)
	}
	if err := r.Run(backendUFW, ufwAllowArgs(port)...); err != nil {
		return newStep(linuxFwName, OutcomeFailed, fmt.Sprintf("ufw is inactive, and adding its allow failed: %v", err),
			fmt.Sprintf("run `ufw %s` as root, then enable ufw with `ufw enable`", strings.Join(ufwAllowArgs(port), " ")))
	}
	added := "allow added (ufw) for when ufw is enabled"
	if offTailnetOnly(read) {
		added += "; the allow it already held admits only addresses or interfaces off the tailnet"
	}
	return ufwInactiveStep(read, port, added)
}

// ufwInactiveReplace swaps ufw's legacy allow for the scoped one in ufw's own
// rule list, adding the scoped allow only when no scoped allow is there already.
func ufwInactiveReplace(r Runner, read firewallRead, port int) (SetupStep, bool) {
	if !read.ufwScoped {
		if err := r.Run(backendUFW, ufwAllowArgs(port)...); err != nil {
			return newStep(linuxFwName, OutcomeFailed, fmt.Sprintf("ufw is inactive, and adding its scoped allow failed: %v", err),
				fmt.Sprintf("run `ufw %s` as root, then enable ufw with `ufw enable`", strings.Join(ufwAllowArgs(port), " "))), true
		}
	}
	if err := r.Run(backendUFW, ufwDeleteArgs(port)...); err != nil {
		return newStep(linuxFwName, OutcomeFailed, fmt.Sprintf("ufw is inactive, and deleting its unscoped allow failed: %v", err),
			fmt.Sprintf("run `ufw %s` as root", strings.Join(ufwDeleteArgs(port), " "))), true
	}
	return SetupStep{}, false
}

// ufwInactiveState is what the rules say on an inactive ufw, and whether setup
// should add ufw's allow. Setup and check share it (#576).
func ufwInactiveState(read firewallRead, port int) (state string, add bool) {
	switch {
	case read.state == ruleBlocked:
		return fmt.Sprintf("once ufw is enabled, its rule %s blocks the port", read.blockedBy), false
	case read.ufwLegacy:
		return fmt.Sprintf("its allow for TCP %d is unscoped, which ts-bridge host setup replaces with one scoped to %s", port, tailnetInterface), false
	case offTailnetOnly(read):
		// An allow scoped to an address or another interface will not admit the tailnet once ufw is enabled (#631).
		return fmt.Sprintf("its allow for TCP %d admits only addresses or interfaces off the tailnet, so setup adds one on %s", port, tailnetInterface), true
	case read.ufwAllows:
		return "its allow is in place for when ufw is enabled", false
	case read.state == rulePresent:
		return fmt.Sprintf("%s admits TCP %d, so setup adds no ufw allow", read.source(), port), false
	}
	return fmt.Sprintf("it holds no allow for TCP %d yet, which ts-bridge host setup adds", port), true
}

// ufwInactiveStep is manual: enabling a firewall can cut the operator's own
// access, so setup never runs `ufw enable` itself. It names any copies of
// setup's iptables rule, which admit the port whatever ufw's state.
func ufwInactiveStep(read firewallRead, port int, state string) SetupStep {
	message := fmt.Sprintf("ufw is inactive, so it filters nothing: TCP %d is reachable on every interface unless another firewall stops it; %s", port, state)
	fix := "enable ufw with `ufw enable`, as root, after allowing any port you reach this host through (SSH), or confirm another firewall filters this host"
	if read.state == ruleBlocked {
		fix = fmt.Sprintf("remove %s, or put an allow for TCP %d before it, then %s", read.blockedBy, port, fix)
	}
	if read.copies > 0 {
		message += "; it also holds " + copiesHeld(read, port)
		fix += "; " + copiesFix(read, port)
	}
	if len(read.foreign) > 0 {
		message += "; legacy unscoped rule: " + strings.Join(read.foreign, "; ")
		fix += "; " + foreignFix(read)
	}
	return newStep(linuxFwName, OutcomeManual, message, fix)
}

func firewallCheckStep(read firewallRead, port int) SetupStep {
	if read.other != "" {
		return foreignStep(read, port, "check")
	}
	return withIP6(read, withPolicy(read, firewallCheckV4(read, port)), port)
}

func firewallCheckV4(read firewallRead, port int) SetupStep {
	if read.state == ruleUnknown {
		message, fix := firewallUnreadable(read, port, "check")
		return newStep(linuxFwName, OutcomeUnverified, message, fix)
	}
	if inactiveUFWDecides(read) {
		state, _ := ufwInactiveState(read, port)
		return ufwInactiveStep(read, port, state)
	}
	if read.state == ruleBlocked {
		return firewallBlockedStep(read, port)
	}
	if legacyHeld(read) {
		return legacyCheckStep(read, port)
	}
	if offTailnetOnly(read) {
		return newStep(linuxFwName, OutcomeChanged, fmt.Sprintf("Firewall allows TCP %d (%s), but not on %s: every allow is scoped to another interface or address", port, read.source(), tailnetInterface),
			fmt.Sprintf("run ts-bridge host setup as root: it adds the rule scoped to %s", tailnetInterface))
	}
	return firewallReadStep(linuxFwName, read.state == rulePresent, port)
}

// copiesHeld names how many copies of setup's iptables rule INPUT holds.
func copiesHeld(read firewallRead, port int) string {
	noun := "copies"
	if read.copies == 1 {
		noun = "copy"
	}
	return fmt.Sprintf("%d %s of %s", read.copies, noun, strings.Join(iptPrinted(iptablesLegacy(port)), " "))
}

// copiesFix is the delete command for the copies of the legacy rule, which setup
// leaves alone where it cannot tell they are redundant: ufw inactive or unreadable.
func copiesFix(read firewallRead, port int) string {
	first := ""
	if read.copiesAdmit {
		// Deleting every copy would close the port the step reports as allowed.
		first = fmt.Sprintf("run `ufw %s` first, as root, since the copies are what admits the port now; then ", strings.Join(ufwAllowArgs(port), " "))
	}
	return fmt.Sprintf("%srun `iptables %s` %d time(s), as root, so none remains: ufw manages this host's rules, and under its default deny policy each copy admits the port on every interface; setup does not delete firewall rules",
		first, strings.Join(iptablesDelete(port), " "), read.copies)
}

// firewallBlockedStep reports a deny that decides the port before any allow.
// Setup never deletes or reorders firewall rules, so this is the operator's.
func firewallBlockedStep(read firewallRead, port int) SetupStep {
	return newStep(linuxFwName, OutcomeManual,
		fmt.Sprintf("Firewall blocks TCP %d before any allow (%s): %s", port, read.source(), read.blockedBy),
		fmt.Sprintf("remove that rule, or put an allow for TCP %d before it; setup does not delete or reorder firewall rules", port))
}

func firewallFix(read firewallRead, port int, command string) string {
	if read.fix != "" {
		return read.fix
	}
	if read.noBackend {
		return fmt.Sprintf("install ufw, or allow TCP %d inbound with your firewall", port)
	}
	return "run ts-bridge host " + command + " as root"
}
