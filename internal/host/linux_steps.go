package host

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	xrdpName     = "xrdp service"
	linuxFwName  = "Firewall configuration"
	xrdpFixSetup = "run ts-bridge host setup as root"
	backendUFW   = "ufw"
	backendIPT   = "iptables"
	protoTCP     = "tcp"
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
	result.Steps = append(result.Steps, xrdpCheckStep(x), firewallCheckStep(fw, port))
	svc := Service{Kind: ServiceRDP, Port: port, Enabled: x.enabled, FirewallOK: fw.state == rulePresent}
	if x.readable {
		active := x.active
		svc.Active = &active
	}
	result.Services = []Service{svc}
}

// xrdpState is the xrdp service as systemd reports it. readable is false when
// systemctl is missing or answered nothing, so "off" and "cannot tell" differ.
type xrdpState struct{ installed, readable, enabled, active bool }

func readXRDP(r Runner) xrdpState {
	if _, err := r.LookPath("xrdp"); err != nil {
		return xrdpState{}
	}
	if _, err := r.LookPath("systemctl"); err != nil {
		return xrdpState{installed: true}
	}
	// is-enabled and is-active exit non-zero when the answer is "no", and
	// still print it, so the printed word decides, not the exit status.
	enabled, _ := r.Output("systemctl", "is-enabled", "xrdp")
	active, _ := r.Output("systemctl", "is-active", "xrdp")
	e, a := strings.TrimSpace(string(enabled)), strings.TrimSpace(string(active))
	if e == "" || a == "" {
		return xrdpState{installed: true}
	}
	return xrdpState{installed: true, readable: true, enabled: e == "enabled", active: a == "active"}
}

// xrdpUnmet returns the step for an xrdp that is missing or unreadable;
// unreadable is the outcome to report when systemd cannot be asked.
func xrdpUnmet(x xrdpState, unreadable Outcome) (SetupStep, bool) {
	if !x.installed {
		return newStep(xrdpName, OutcomeFailed, "xrdp is not installed", "sudo apt install xrdp"), true
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
	return newStep(xrdpName, OutcomeChanged, fmt.Sprintf("xrdp is not enabled and running (enabled=%t, active=%t)", x.enabled, x.active), xrdpFixSetup)
}

type ruleState int

const (
	ruleUnknown ruleState = iota
	ruleAbsent
	rulePresent
)

// firewallRead is one read of the firewall rule for a port. backend names the
// tool that answered; noBackend means neither ufw nor iptables is installed.
type firewallRead struct {
	state     ruleState
	backend   string
	noBackend bool
	reason    string
}

// readLinuxFirewall reads with ufw when it is installed, otherwise iptables.
// Every read needs root, so it runs nothing unprivileged: some iptables
// backends exit 1, the "absent" answer, on permission denied.
func readLinuxFirewall(r Runner, port int) firewallRead {
	if !r.Elevated() {
		return firewallRead{reason: "reading the firewall rules needs root"}
	}
	if _, err := r.LookPath(backendUFW); err == nil {
		out, err := r.Output(backendUFW, "show", "added")
		if err != nil {
			return firewallRead{backend: backendUFW, reason: fmt.Sprintf("ufw show added: %v", err)}
		}
		if ufwAllows(string(out), port) {
			return firewallRead{state: rulePresent, backend: backendUFW}
		}
		return firewallRead{state: ruleAbsent, backend: backendUFW}
	}
	if _, err := r.LookPath(backendIPT); err != nil {
		return firewallRead{noBackend: true, reason: "neither ufw nor iptables is installed"}
	}
	out, err := r.Output(backendIPT, "-S", "INPUT")
	if err != nil {
		return firewallRead{backend: backendIPT, reason: fmt.Sprintf("iptables -S INPUT: %v", err)}
	}
	if iptablesCovers(string(out), port) {
		return firewallRead{state: rulePresent, backend: backendIPT}
	}
	return firewallRead{state: ruleAbsent, backend: backendIPT}
}

// iptComment matches a `--comment "..."` value, escaped quotes included, so a
// comment cannot pose as a flag once the line is split.
var iptComment = regexp.MustCompile(`--comment "(?:[^"\\]|\\.)*"`)

// iptablesCovers reports whether `iptables -S` output holds an ACCEPT for TCP
// whose destination port list contains port. Interface, source and match
// qualifiers are the operator's scope and do not matter; a negated port, a
// source port, a jump or a rule without a destination port never counts.
func iptablesCovers(rules string, port int) bool {
	for _, line := range strings.Split(rules, "\n") {
		f := strings.Fields(iptComment.ReplaceAllString(line, ""))
		if len(f) == 0 || f[0] != "-A" || iptFlag(f, "-j") != "ACCEPT" || iptFlag(f, "-p") != protoTCP {
			continue
		}
		for _, flag := range []string{"--dport", "--dports", "--ports"} {
			if portListHas(iptFlag(f, flag), port) {
				return true
			}
		}
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
	return []string{"-A", "INPUT", "-p", protoTCP, "--dport", strconv.Itoa(port), "-j", "ACCEPT"}
}

// ufwAllows reports whether `ufw show added` holds an inbound allow for TCP
// port. route, out, deny and limit rules do not count, and the port must match
// exactly: 33890 is not 3389.
func ufwAllows(added string, port int) bool {
	for _, line := range strings.Split(added, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || f[0] != "ufw" || f[1] != "allow" || slices.Contains(f, "out") {
			continue
		}
		if ufwRuleMatchesPort(f[2:], port) {
			return true
		}
	}
	return false
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
		return newStep(linuxFwName, OutcomeOK, fmt.Sprintf("Firewall allows TCP %d (%s)", port, read.backend), "")
	case ruleUnknown:
		return newStep(linuxFwName, OutcomeFailed, "Cannot read the firewall: "+read.reason, firewallFix(read, port, "setup"))
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

func firewallCheckStep(read firewallRead, port int) SetupStep {
	if read.state == ruleUnknown {
		return newStep(linuxFwName, OutcomeUnverified, "Cannot read the firewall: "+read.reason, firewallFix(read, port, "check"))
	}
	return firewallReadStep(linuxFwName, read.state == rulePresent, port)
}

func firewallFix(read firewallRead, port int, command string) string {
	if read.noBackend {
		return fmt.Sprintf("install ufw, or allow TCP %d inbound with your firewall", port)
	}
	return "run ts-bridge host " + command + " as root"
}
