package host

import (
	"fmt"
	"net/netip"
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
	protoTCP      = "tcp"
	iptAccept     = "ACCEPT"
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
	steps := []SetupStep{xrdpSetupStep(r), firewallSetupStep(r, port), tailscaleIPStep(r)}
	return SetupResult{RDPPort: port, Steps: steps}
}

func checkLinux(cfg Config, r Runner, result *CheckResult) {
	port := linuxPort(cfg)
	x := readXRDP(r)
	fw := readLinuxFirewall(r, port)
	xrdp := xrdpCheckStep(x)
	result.Steps = append(result.Steps, xrdp, firewallCheckStep(fw, port))
	svc := Service{Kind: ServiceRDP, Port: port, Enabled: x.enabled, FirewallOK: fw.state == rulePresent, Unread: xrdp.Outcome == OutcomeUnverified}
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
	copies    int    // copies of the rule setup appends to INPUT (#480), counted on ufw hosts too (#530)
	decidedBy string // the tool whose rules decided state, when not backend: iptables on a ufw host (#530)
	// copiesAdmit means the copies are the only allow: without them the port is closed (#576).
	copiesAdmit bool
}

// source names the tool whose rules decided the read, for messages; backend
// stays the tool setup writes through.
func (read firewallRead) source() string {
	if read.decidedBy != "" {
		return read.decidedBy
	}
	return read.backend
}

// readLinuxFirewall reads with ufw when it is installed, otherwise iptables.
// Every read needs root, so it runs nothing unprivileged: some iptables
// backends exit 1, the "absent" answer, on permission denied.
func readLinuxFirewall(r Runner, port int) firewallRead {
	if !r.Elevated() {
		return firewallRead{reason: "reading the firewall rules needs root"}
	}
	if _, err := r.LookPath(backendUFW); err == nil {
		return readUFW(r, port)
	}
	if _, err := r.LookPath(backendIPT); err != nil {
		return firewallRead{noBackend: true, reason: "neither ufw nor iptables is installed"}
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
	out, ufwErr := r.Output(backendUFW, "show", "added")
	if ufwErr != nil {
		read.reason = fmt.Sprintf("ufw show added: %v", ufwErr)
	} else {
		read.state, read.blockedBy = ufwDecide(string(out), port)
	}
	if _, err := r.LookPath(backendIPT); err != nil {
		return read
	}
	// Copies are counted even when ufw is unreadable: each one admits the
	// port on every interface whatever ufw would have said.
	ipt := readIptables(r, port)
	read.copies = ipt.copies
	// Only a ufw allow of its own makes the copies redundant; an unreadable ufw proves none (#576).
	read.copiesAdmit = ipt.copiesAdmit && read.state != rulePresent
	if read.state == ruleAbsent {
		read.state, read.blockedBy, read.reason, read.fix = ipt.state, ipt.blockedBy, ipt.reason, ipt.fix
		read.decidedBy = backendIPT
	}
	return read
}

// tsInputChain is the chain tailscaled fills with `-i tailscale0 -j ACCEPT`;
// with --netfilter-mode=on it inserts tsInputJump at the top of INPUT (#492).
const tsInputChain = "ts-input"

var tsInputJump = []string{"-A", "INPUT", "-j", tsInputChain}

// readIptables reads INPUT and, behind tailscaled's unqualified jump, the
// ts-input chain. A failed read of either is an unknown state, and so is a
// qualified jump into ts-input that the walk reaches with nothing decided:
// setup never appends an open rule on a guess (#532).
func readIptables(r Runner, port int) firewallRead {
	out, err := r.Output(backendIPT, "-S", "INPUT")
	if err != nil {
		return firewallRead{backend: backendIPT, reason: fmt.Sprintf("iptables -S INPUT: %v", err)}
	}
	input := iptRules(string(out))
	rules, jump := iptBeforeQualifiedJump(input)
	if slices.ContainsFunc(strings.Split(rules, "\n"), iptTSInputJump) {
		chain, err := r.Output(backendIPT, "-S", tsInputChain)
		if err != nil {
			return firewallRead{backend: backendIPT, reason: fmt.Sprintf("iptables -S %s: %v", tsInputChain, err)}
		}
		rules = iptInlineChain(rules, iptRules(string(chain)))
	}
	state, rule := iptablesDecide(rules, port)
	if state == ruleAbsent && jump != "" {
		return firewallRead{backend: backendIPT, reason: fmt.Sprintf("INPUT reaches %s through %q, which ts-bridge cannot evaluate", tsInputChain, jump),
			fix: fmt.Sprintf("allow TCP %d inbound on the Tailscale interface ahead of %q, or confirm by hand that the jump admits it", port, jump)}
	}
	read := firewallRead{state: state, backend: backendIPT, blockedBy: rule, copies: iptSetupCopies(input, port)}
	if state == rulePresent && read.copies > 0 {
		without, _ := iptablesDecide(iptWithoutSetup(rules, port), port)
		read.copiesAdmit = without != rulePresent
	}
	return read
}

// iptWithoutSetup is rules with every copy of setup's rule removed.
func iptWithoutSetup(rules string, port int) string {
	printed := iptSetupPrinted(port)
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

// iptSetupCopies counts the lines of rules that are the rule setup appends,
// in the form `iptables -S` prints it.
func iptSetupCopies(rules string, port int) int {
	printed := iptSetupPrinted(port)
	n := 0
	for _, line := range strings.Split(rules, "\n") {
		if slices.Equal(strings.Fields(line), printed) {
			n++
		}
	}
	return n
}

// iptSetupPrinted is iptablesAppend's rule as `iptables -S` prints it: the
// protocol after `-p` gains an explicit `-m <protocol>` match.
func iptSetupPrinted(port int) []string {
	rule := iptablesAppend(port)
	i := slices.Index(rule, "-p") + 2
	return slices.Insert(rule, i, "-m", rule[i-1])
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
func iptRules(out string) string {
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
	iptDestPortFlags = []string{"--dport", "--dports", "--ports"}
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
			if !slices.ContainsFunc(f, func(w string) bool { return slices.Contains(iptScopeFlags, w) }) {
				return ruleBlocked, strings.TrimSpace(line)
			}
		}
	}
	return ruleAbsent, ""
}

// iptScopeFlags narrow a rule to part of the traffic; a deny carrying one is
// the operator's scope and never decides the port.
var iptScopeFlags = []string{"-i", "-s", "-d", "!"}

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
func iptPortlessToken(f []string) int {
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

func iptablesAppend(port int) []string {
	return []string{"-A", "INPUT", "-p", protoTCP, "--dport", strconv.Itoa(port), "-j", iptAccept}
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
func ufwDecide(added string, port int) (ruleState, string) {
	for _, line := range strings.Split(added, "\n") {
		f := strings.Fields(line)
		if i := slices.Index(f, "comment"); i >= 0 {
			f = f[:i]
		}
		if len(f) < 3 || f[0] != "ufw" || slices.Contains(f, "out") || !ufwRuleMatchesPort(f[2:], port) {
			continue
		}
		switch f[1] {
		case "allow":
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
	if slices.Contains(f, "from") || slices.Contains(f, "on") {
		return true
	}
	t := slices.Index(f, "to")
	return t >= 0 && t+1 < len(f) && f[t+1] != "any"
}

// ufwRuleMatchesPort matches the short form (`3389/tcp`, `3389`, `in 3389/tcp`)
// and the full form (`... port 3389 [proto tcp]`). A trailing `comment '...'` is
// ignored, as is a `log`/`log-all` modifier, and the port may sit in a list or range (`22,3389/tcp`, `3380:3390/tcp`).
func ufwRuleMatchesPort(f []string, port int) bool {
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
	switch read.state {
	case rulePresent:
		// Setup read the same copies check reports, so it reports them the same way (#576).
		if extraCopies(read) > 0 {
			return firewallCopiesStep(read, port)
		}
		return newStep(linuxFwName, OutcomeOK, fmt.Sprintf("Firewall allows TCP %d (%s)", port, read.source()), "")
	case ruleBlocked:
		return firewallBlockedStep(read, port)
	case ruleUnknown:
		message, fix := firewallUnreadable(read, port, "setup")
		return newStep(linuxFwName, OutcomeFailed, message, fix)
	}
	var err error
	if read.backend == backendUFW {
		err = r.Run(backendUFW, "allow", fmt.Sprintf("%d/tcp", port))
	} else {
		err = r.Run(backendIPT, iptablesAppend(port)...)
	}
	if err != nil {
		return newStep(linuxFwName, OutcomeFailed, fmt.Sprintf("Could not add the firewall rule: %v", err),
			fmt.Sprintf("allow TCP %d inbound with %s, as root", port, read.backend))
	}
	return newStep(linuxFwName, OutcomeChanged, fmt.Sprintf("Firewall rule added: TCP %d (%s)", port, read.backend), "")
}

// firewallUnreadable is the message and fix for a firewall that could not be
// read, naming any copies of setup's rule it still counted. Setup and check
// share it so the two report the same state the same way (#576).
func firewallUnreadable(read firewallRead, port int, command string) (message, fix string) {
	message, fix = "Cannot read the firewall: "+read.reason, firewallFix(read, port, command)
	if extraCopies(read) > 0 {
		message += "; it also holds " + copiesHeld(read, port)
		fix += "; " + copiesFix(read, port)
	}
	return message, fix
}

func firewallCheckStep(read firewallRead, port int) SetupStep {
	if read.state == ruleUnknown {
		message, fix := firewallUnreadable(read, port, "check")
		return newStep(linuxFwName, OutcomeUnverified, message, fix)
	}
	if read.state == ruleBlocked {
		return firewallBlockedStep(read, port)
	}
	if extraCopies(read) > 0 {
		return firewallCopiesStep(read, port)
	}
	return firewallReadStep(linuxFwName, read.state == rulePresent, port)
}

// extraCopies is how many copies of setup's iptables rule the operator should
// delete: all but one on an iptables host, where one is setup's own rule, and
// every one on a ufw host, where setup writes through ufw and each copy is a
// leftover that admits the port on every interface (#530).
func extraCopies(read firewallRead) int {
	if read.backend == backendUFW {
		return read.copies
	}
	return max(read.copies-1, 0)
}

// firewallCopiesStep reports the copies of its own rule that setup appended on
// every run before #477. The port is allowed; setup never deletes rules, so
// removing the extras is the operator's.
func firewallCopiesStep(read firewallRead, port int) SetupStep {
	return newStep(linuxFwName, OutcomeManual,
		fmt.Sprintf("Firewall allows TCP %d (%s), but holds %s", port, read.source(), copiesHeld(read, port)),
		copiesFix(read, port))
}

// copiesHeld names how many copies of setup's iptables rule INPUT holds.
func copiesHeld(read firewallRead, port int) string {
	noun := "copies"
	if read.copies == 1 {
		noun = "copy"
	}
	return fmt.Sprintf("%d %s of %s", read.copies, noun, strings.Join(iptSetupPrinted(port), " "))
}

// copiesFix is the delete command for the copies extraCopies counts.
func copiesFix(read firewallRead, port int) string {
	remains, first := "so one copy remains", ""
	if read.backend == backendUFW {
		remains = "so none remains: ufw manages this host's rules, and under its default deny policy each copy admits the port on every interface"
		if read.copiesAdmit {
			// Deleting every copy would close the port the step reports as allowed.
			first = fmt.Sprintf("run `ufw allow %d/tcp` first, as root, since the copies are what admits the port now; then ", port)
		}
	}
	return fmt.Sprintf("%srun `iptables -D %s` %d time(s), as root, %s; setup does not delete firewall rules",
		first, strings.Join(iptablesAppend(port)[1:], " "), extraCopies(read), remains)
}

// firewallBlockedStep reports a deny that decides the port before any allow.
// Setup never deletes or reorders firewall rules, so this is the operator's.
func firewallBlockedStep(read firewallRead, port int) SetupStep {
	return newStep(linuxFwName, OutcomeManual,
		fmt.Sprintf("Firewall blocks TCP %d before any allow (%s): %s", port, read.source(), read.blockedBy),
		fmt.Sprintf("remove that rule, or put an allow for TCP %d before it; setup does not delete or reorder firewall rules", port))
}

func firewallFix(read firewallRead, port int, command string) string {
	if read.noBackend {
		return fmt.Sprintf("install ufw, or allow TCP %d inbound with your firewall", port)
	}
	if read.fix != "" {
		return read.fix
	}
	return "run ts-bridge host " + command + " as root"
}
