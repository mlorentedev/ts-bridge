//go:build linux

package host

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

func isElevatedImpl() bool {
	return syscall.Geteuid() == 0
}

func setupImpl(flags SetupFlags) (SetupResult, error) {
	var steps []SetupStep

	// 1. Detect xrdp.
	steps = append(steps, SetupStep{Name: "xrdp detection"})
	if _, err := exec.LookPath("xrdp"); err == nil {
		steps[0].Success = true
		steps[0].Message = "xrdp found"
	} else {
		steps[0].Success = false
		steps[0].Message = "xrdp not installed — install with: sudo apt install xrdp"
	}

	// 2. Configure firewall.
	steps = append(steps, SetupStep{Name: "Firewall configuration"})
	rdpPort := 3389
	if err := configureFirewall(flags.FirewallRule, rdpPort); err != nil {
		steps[1].Success = false
		steps[1].Message = fmt.Sprintf("Could not configure firewall: %v", err)
	} else {
		steps[1].Success = true
		steps[1].Message = fmt.Sprintf("Firewall rule '%s' active (port %d)", flags.FirewallRule, rdpPort)
	}

	// 3. Print Tailscale IP.
	steps = append(steps, SetupStep{Name: "Tailscale IP"})
	tsIP := TailscaleIP()
	if tsIP != "" {
		steps[2].Success = true
		steps[2].Message = fmt.Sprintf("Tailscale IP: %s", tsIP)
	} else {
		steps[2].Success = false
		steps[2].Message = "Tailscale not connected or CLI not available"
	}

	return SetupResult{RDPPort: rdpPort, Steps: steps}, nil
}

func checkImpl() (CheckResult, error) {
	result := CheckResult{}
	tsIP := TailscaleIP()
	result.TailscaleIP = tsIP
	result.TailscaleUp = tsIP != ""

	if _, err := exec.LookPath("xrdp"); err == nil {
		result.RDPEnabled = true
		result.RDPPort = 3389
	}
	result.FirewallOK = checkFirewallRule()
	return result, nil
}

func tailscaleIPImpl() string {
	out, err := exec.Command("tailscale", "ip", "-4").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ─── Linux helpers ───────────────────────────────────────────────

func configureFirewall(ruleName string, port int) error {
	if _, err := exec.LookPath("ufw"); err == nil {
		return runUFW(port)
	}
	return runIPTables(port)
}

func runUFW(port int) error {
	// #nosec G204 -- ufw is a hardcoded system binary, args are controlled port numbers.
	return exec.Command("ufw", "route", "allow", "in", "to", "any", "port", fmt.Sprintf("%d", port), "proto", "tcp").Run()
}

func runIPTables(port int) error {
	// #nosec G204 -- iptables is a hardcoded system binary, args are controlled port numbers.
	return exec.Command("iptables", "-A", "INPUT", "-p", "tcp", "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT").Run()
}

func checkFirewallRule() bool {
	// #nosec G204 -- ufw/iptables are hardcoded system binaries.
	if out, err := exec.Command("ufw", "status").Output(); err == nil && strings.Contains(string(out), "3389") {
		return true
	}
	// #nosec G204 -- iptables is a hardcoded system binary.
	if out, err := exec.Command("iptables", "-L", "INPUT", "-n").Output(); err == nil && strings.Contains(string(out), "3389") {
		return true
	}
	return false
}
