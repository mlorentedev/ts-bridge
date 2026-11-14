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

// withSocket folds the socket into the reading. Where the socket exists, the
// service sits idle until the first connection while the socket listens, so
// reading only the service would report drift on a host that answers, and
// starting the service would leave it running outside the socket's activation.
// The socket is then the unit to converge and the service is left alone. A
// socket exists when systemctl reads it (is-enabled answers something other than
// not-found, and a word ts-bridge interprets); one it cannot read is ignored, as
// if absent, so those hosts keep the service behavior. A masked service stays
// masked, and a masked socket is reported, never unmasked.
func withSocket(r Runner, unit string, x xrdpState) (string, xrdpState) {
	if !x.readable || x.masked || (x.enabled && x.active) {
		return unit, x
	}
	sock := readUnit(r, unit+".socket", "sshd")
	if !sock.readable {
		return unit, x
	}
	x.enabled, x.active = x.enabled || sock.enabled, x.active || sock.active
	x.masked = sock.masked
	return unit + ".socket", x
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
	steps := []SetupStep{sshUnit(unit).setupStep(r, x)}
	if s, ok := sshPortStep(r, x, port, OutcomeFailed); ok {
		steps = append(steps, s)
	}
	return append(steps, sshFirewallSetupStep(r, port))
}

func checkSSH(r Runner, port int, result *CheckResult) {
	unit, x := readSSH(r)
	fw := sshFirewallRead(r, port)
	svcStep := sshUnit(unit).checkStep(x)
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
