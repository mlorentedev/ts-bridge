package host

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const (
	sshName     = "SSH server"
	sshPortName = "SSH port"
	sshFwName   = "Firewall configuration (ssh)"
	// sshMissingFix names the packages; ts-bridge installs none (ADR-017 §5).
	sshMissingFix = "install the OpenSSH server with your package manager (apt install openssh-server, " +
		"dnf install openssh-server, pacman -S openssh); ts-bridge does not install packages"
)

// sshUnits are the names OpenSSH's unit goes by: Debian and Ubuntu ship ssh,
// Fedora, RHEL and Arch ship sshd. ssh is asked first because Debian's
// ssh.service carries sshd as an alias, which systemctl answers as "alias",
// an answer ts-bridge does not interpret.
var sshUnits = []string{"ssh", "sshd"}

// readSSH finds which unit the host has and reads it. The first result is the
// unit setup converges: the service, or its socket where systemd socket-activates
// sshd (Ubuntu since 22.10). It is "" when the host has no ssh service unit.
func readSSH(r Runner) (string, xrdpState) {
	for _, unit := range sshUnits {
		if x := readUnit(r, unit, "sshd"); x.installed {
			return withSocket(r, unit, x)
		}
	}
	return "", xrdpState{}
}

// withSocket folds the socket into the reading where the socket is the host's
// model for sshd. Then the service sits idle until the first connection while the
// socket listens, so reading only the service would report drift on a host that
// answers, and starting the service would leave it running outside the socket's
// activation: the socket is the unit to converge and the service is left alone.
// A socket exists when systemctl reads it (is-enabled answers something other
// than not-found, and a word ts-bridge interprets); one it cannot read is
// ignored, as if absent. A masked service stays masked. The socket is read even
// beside an enabled, running service: when both are enabled the host is outside
// either model, which is dual state, reported for the operator to resolve (#777).
// Dual needs is-enabled to say enabled: a static socket has no [Install], so
// disabling the service beside it would leave nothing to start sshd at boot.
func withSocket(r Runner, unit string, x xrdpState) (string, xrdpState) {
	if !x.readable || x.masked {
		return unit, x
	}
	sock := readUnit(r, unit+".socket", "sshd")
	if !sock.readable {
		return unit, x
	}
	return foldSocket(unit, x, sock)
}

// foldSocket is withSocket once the socket has been read.
func foldSocket(unit string, x, sock xrdpState) (string, xrdpState) {
	if x.enabled && sock.enabled && !sock.static {
		// The socket may be the one listening: Active says whether sshd answers.
		x.dual, x.active = true, x.active || sock.active
		return unit, x
	}
	if !socketIsModel(x, sock) {
		return unit, x
	}
	x.enabled, x.active = x.enabled || sock.enabled, x.active || sock.active
	return unit + ".socket", x
}

// socketIsModel is whether the socket, not the service, is how the operator runs
// sshd. Debian 12 and Ubuntu 22.04 and earlier ship ssh.socket disabled beside an
// enabled ssh.service. Whether the socket Conflicts= with the service depends on
// the release: Debian 11, Ubuntu 20.04 and 22.04 say so, Debian 12 and 13 and
// Ubuntu 23.04 and later (24.04's socket is Before= and RequiredBy= the service)
// do not. Where it does, starting the socket stops an sshd the operator started
// by hand; where it does not, the socket cannot bind a port that sshd already
// holds, and enabling it leaves both units enabled (#777). Either way the
// socket leads only when it is enabled (static counts, unless the service is
// enabled too: a static socket has no [Install], so nothing starts it at boot and
// it is neither the model nor dual), or when neither unit is enabled and the
// service is not running, where there is no model to preserve
// and the socket is the choice (#767). A masked socket never leads: masking it
// is how an operator opts out of socket activation, so the service is converged
// and nothing asks to unmask the socket. With both enabled the host is dual, led
// by neither (withSocket).
func socketIsModel(svc, sock xrdpState) bool {
	staticBesideService := sock.static && svc.enabled
	return (sock.enabled && !staticBesideService) || (!svc.enabled && !svc.active && !sock.masked)
}

// sshDualFix is the fix for a host whose service and socket are both enabled.
// Disabling the service alone is the safe form: the socket starts sshd from the
// next boot, and the running sshd and its sessions are not touched. The
// immediate form stops the listener before the socket takes over, so a remote
// operator who runs it can lose the connection they ran it from.
const sshDualFix = "run `sudo systemctl disable %[1]s.service`: the socket starts sshd from the next boot " +
	"and the running sshd is left alone. To switch now, from a console and not over SSH, run " +
	"`sudo systemctl disable --now %[1]s.service && sudo systemctl start %[1]s.socket`"

// sshDualStep is the step for a host with the service and its socket both
// enabled. It is manual in setup and in check: disabling a unit is the
// operator's call, never an automatic one. state says what sshd is doing.
func sshDualStep(unit, state string) SetupStep {
	return newStep(sshName, OutcomeManual,
		fmt.Sprintf("%[1]s.service and %[1]s.socket are both enabled, so sshd also starts outside the socket's activation%s", unit, state),
		fmt.Sprintf(sshDualFix, unit))
}

// sshDualSetupStep leaves sshd running where it can: with both units enabled and
// nothing listening it starts the socket, the model on these releases, and
// enables nothing and leaves the service alone. The step stays manual for the
// dual enable either way.
func sshDualSetupStep(r Runner, unit string, x xrdpState) SetupStep {
	if x.active {
		return sshDualStep(unit, "")
	}
	if err := r.Run("systemctl", "start", unit+".socket"); err != nil {
		step := sshDualStep(unit, "")
		step.Outcome = OutcomeFailed
		step.Message += fmt.Sprintf(", and sshd is not running: could not start %s.socket: %v", unit, err)
		step.Fix = fmt.Sprintf("sudo systemctl start %s.socket, then %s", unit, step.Fix)
		return step
	}
	return sshDualStep(unit, fmt.Sprintf("; sshd was not running, so %s.socket was started", unit))
}

// sshUnit is the step text for the unit read; label names the unit so a host
// that has ssh.service is told so, not "sshd".
func sshUnit(unit string) linuxUnit {
	label := "OpenSSH"
	if unit != "" {
		label += " (" + unit + ")"
	}
	return linuxUnit{name: sshName, label: label, unit: unit,
		missing: newStep(sshName, OutcomeManual, "The OpenSSH server is not installed", sshMissingFix)}
}

func sshSetupSteps(r Runner, port int) []SetupStep {
	unit, x := readSSH(r)
	steps := []SetupStep{sshServerSetupStep(r, unit, x)}
	if s, ok := sshPortStep(r, x, port, OutcomeFailed); ok {
		steps = append(steps, s)
	}
	return append(steps, sshFirewallSetupStep(r, port))
}

func sshServerSetupStep(r Runner, unit string, x xrdpState) SetupStep {
	if x.dual {
		return sshDualSetupStep(r, unit, x)
	}
	return sshUnit(unit).setupStep(r, x)
}

func sshServerCheckStep(unit string, x xrdpState) SetupStep {
	if x.dual {
		state := ""
		if !x.active {
			state = "; sshd is not running"
		}
		return sshDualStep(unit, state)
	}
	return sshUnit(unit).checkStep(x)
}

func checkSSH(r Runner, port int, result *CheckResult) {
	unit, x := readSSH(r)
	fw := sshFirewallRead(r, port)
	svcStep := sshServerCheckStep(unit, x)
	result.Steps = append(result.Steps, svcStep)
	if s, ok := sshPortStep(r, x, port, OutcomeUnverified); ok {
		result.Steps = append(result.Steps, s)
	}
	result.Steps = append(result.Steps, sshNamed(firewallCheckStep(fw, port)))
	// Unread is about the service, as for xrdp: an unreadable firewall or listener
	// port is its own unverified step and leaves enabled and active meaningful.
	svc := Service{Kind: ServiceSSH, Port: port, Enabled: x.enabled, FirewallOK: linuxFirewallOK(fw), Unread: svcStep.Outcome == OutcomeUnverified}
	if x.readable {
		active := x.active
		svc.Active = &active
	}
	result.Services = append(result.Services, svc)
}

func sshFirewallSetupStep(r Runner, port int) SetupStep {
	return sshNamed(firewallSetupFor(r, sshFirewallRead(r, port), port))
}

// sshNamed renames a firewall step so it does not shadow the rdp one.
func sshNamed(s SetupStep) SetupStep {
	s.Name = sshFwName
	return s
}

// sshFirewallRead is the firewall read for the ssh port with the unscoped
// allows reclassified. The rdp steps delete the unscoped rule earlier releases
// wrote for the RDP port, since only ts-bridge ever wrote it there. For port 22
// the same rule is the operator's, and deleting it could close SSH from every
// network: it is reported as foreign, never deleted.
func sshFirewallRead(r Runner, port int) firewallRead {
	read := readLinuxFirewall(r, port)
	if read.ufwLegacy {
		read.foreign = append(read.foreign, "ufw allow "+ufwLegacyShort(port))
	}
	if read.copies > 0 {
		read.foreign = append(read.foreign, strings.Join(iptPrinted(iptablesLegacy(port)), " "))
	}
	read.ufwLegacy, read.copies = false, 0
	return read
}

// sshPortStep reports whether sshd listens on a declared port other than 22.
// Unlike xrdp's, the listener is never moved: sshd_config is the operator's,
// and a wrong edit locks them out of the host. The default port is not read,
// since the host file cannot tell a declared 22 from an omitted one.
// unreadable is the outcome when sshd's configuration cannot be read.
func sshPortStep(r Runner, x xrdpState, declared int, unreadable Outcome) (SetupStep, bool) {
	if !x.installed || declared == defaultPorts[ServiceSSH] {
		return SetupStep{}, false
	}
	ports, err := readSSHPorts(r)
	switch {
	case err != nil:
		return newStep(sshPortName, unreadable, "Cannot read the sshd listener: "+err.Error(),
			fmt.Sprintf("run `sshd -T` as root and check that sshd listens on TCP %d", declared)), true
	case slices.Contains(ports, declared):
		return newStep(sshPortName, OutcomeOK, fmt.Sprintf("sshd listens on TCP %d", declared), ""), true
	}
	return newStep(sshPortName, OutcomeManual, fmt.Sprintf("sshd listens on TCP %s, not the declared %d", portList(ports), declared),
		fmt.Sprintf("set Port %d in /etc/ssh/sshd_config and reload sshd; ts-bridge does not edit it", declared)), true
}

// readSSHPorts reads the ports sshd listens on from its effective
// configuration: the port lines, and the port a listenaddress names. sshd is
// in /usr/sbin, which a non-root PATH often lacks.
func readSSHPorts(r Runner) ([]int, error) {
	bin := "/usr/sbin/sshd"
	if p, err := r.LookPath("sshd"); err == nil {
		bin = p
	}
	out, err := r.Output(bin, "-T")
	if err != nil {
		return nil, fmt.Errorf("sshd -T: %w", err)
	}
	var ports []int
	for _, line := range strings.Split(string(out), "\n") {
		if n, ok := sshdPort(line); ok {
			ports = append(ports, n)
		}
	}
	if len(ports) == 0 {
		return nil, errors.New("sshd -T printed no port")
	}
	return ports, nil
}

func portList(ports []int) string {
	words := make([]string, len(ports))
	for i, p := range ports {
		words[i] = strconv.Itoa(p)
	}
	return strings.Join(words, ", ")
}

// sshdPort is the port a `port N` or `listenaddress HOST:N` line of sshd -T names.
func sshdPort(line string) (int, bool) {
	f := strings.Fields(line)
	if len(f) != 2 || (f[0] != "port" && f[0] != "listenaddress") {
		return 0, false
	}
	raw := f[1]
	if f[0] == "listenaddress" {
		raw = raw[strings.LastIndex(raw, ":")+1:]
	}
	n, err := strconv.Atoi(raw)
	return n, err == nil
}
