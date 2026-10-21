package host

import (
	"fmt"
	"strings"
)

func setupLinux(cfg Config, r Runner) SetupResult {
	rdpPort := cfg.Port
	if rdpPort == 0 {
		rdpPort = defaultRDPPort
	}
	steps := []SetupStep{stepXRDP(r)}

	// The firewall command runs on every setup: L1/L2 (#454) make it read first.
	const fwName = "Firewall configuration"
	if err := configureLinuxFirewall(r, rdpPort); err != nil {
		steps = append(steps, newStep(fwName, OutcomeFailed, fmt.Sprintf("Could not configure firewall: %v", err),
			fmt.Sprintf("allow TCP %d inbound with ufw or iptables, as root", rdpPort)))
	} else {
		steps = append(steps, newStep(fwName, OutcomeChanged, fmt.Sprintf("Firewall rule '%s' active (port %d)", cfg.FirewallRule, rdpPort), ""))
	}

	steps = append(steps, tailscaleIPStep(r))
	return SetupResult{RDPPort: rdpPort, Steps: steps}
}

func stepXRDP(r Runner) SetupStep {
	if _, err := r.LookPath("xrdp"); err != nil {
		return newStep("xrdp detection", OutcomeFailed, "xrdp not installed — install with: sudo apt install xrdp", "sudo apt install xrdp")
	}
	return newStep("xrdp detection", OutcomeOK, "xrdp found", "")
}

func configureLinuxFirewall(r Runner, port int) error {
	if _, err := r.LookPath("ufw"); err == nil {
		return r.Run("ufw", "route", "allow", "in", "to", "any", "port", fmt.Sprint(port), "proto", "tcp")
	}
	return r.Run("iptables", "-A", "INPUT", "-p", "tcp", "--dport", fmt.Sprint(port), "-j", "ACCEPT")
}

func checkLinux(cfg Config, r Runner, result *CheckResult) {
	rdpPort := cfg.Port
	if rdpPort == 0 {
		rdpPort = defaultRDPPort
	}
	if _, err := r.LookPath("xrdp"); err == nil {
		result.RDPEnabled = true
		result.RDPPort = rdpPort
	}
	result.FirewallOK = linuxFirewallAllows(r, rdpPort)
}

// linuxFirewallAllows matches the port as a substring of the rule listing; L3 (#454) fixes that.
func linuxFirewallAllows(r Runner, port int) bool {
	portStr := fmt.Sprint(port)
	if out, err := r.Output("ufw", "status"); err == nil && strings.Contains(string(out), portStr) {
		return true
	}
	out, err := r.Output("iptables", "-L", "INPUT", "-n")
	return err == nil && strings.Contains(string(out), portStr)
}
