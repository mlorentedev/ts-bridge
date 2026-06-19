//go:build linux

package host

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"syscall"
)

func isElevatedImpl() bool {
	return syscall.Geteuid() == 0
}

func setupImpl(cfg Config, logger *slog.Logger) (SetupResult, error) {
	var steps []SetupStep

	// 1. Detect xrdp.
	steps = append(steps, SetupStep{Name: "xrdp detection"})
	if _, err := exec.LookPath("xrdp"); err == nil {
		steps[0].Success = true
		steps[0].Message = "xrdp found"
		if logger != nil {
			logger.Info("xrdp detected")
		}
	} else {
		steps[0].Success = false
		steps[0].Message = "xrdp not installed — install with: sudo apt install xrdp"
		if logger != nil {
			logger.Warn("xrdp not found", "hint", "sudo apt install xrdp")
		}
	}

	// 2. Configure firewall.
	steps = append(steps, SetupStep{Name: "Firewall configuration"})
	rdpPort := cfg.Port
	if rdpPort == 0 {
		rdpPort = defaultRDPPort
	}
	if err := configureFirewall(cfg.FirewallRule, rdpPort); err != nil {
		steps[1].Success = false
		steps[1].Message = fmt.Sprintf("Could not configure firewall: %v", err)
		if logger != nil {
			logger.Warn("failed to configure firewall", "error", err)
		}
	} else {
		steps[1].Success = true
		steps[1].Message = fmt.Sprintf("Firewall rule '%s' active (port %d)", cfg.FirewallRule, rdpPort)
		if logger != nil {
			logger.Info("configured firewall", "rule", cfg.FirewallRule, "port", rdpPort)
		}
	}

	// 3. Print Tailscale IP.
	steps = append(steps, SetupStep{Name: "Tailscale IP"})
	tsIP := TailscaleIP()
	if tsIP != "" {
		steps[2].Success = true
		steps[2].Message = fmt.Sprintf("Tailscale IP: %s", tsIP)
		if logger != nil {
			logger.Info("Tailscale IP", "ip", tsIP)
		}
	} else {
		steps[2].Success = false
		steps[2].Message = "Tailscale not connected or CLI not available"
		if logger != nil {
			logger.Warn("Tailscale not connected or CLI not available")
		}
	}

	return SetupResult{RDPPort: rdpPort, Steps: steps}, nil
}

func checkImpl(logger *slog.Logger) (CheckResult, error) {
	result := CheckResult{}
	tsIP := TailscaleIP()
	result.TailscaleIP = tsIP
	result.TailscaleUp = tsIP != ""

	if _, err := exec.LookPath("xrdp"); err == nil {
		result.RDPEnabled = true
		result.RDPPort = 3389
		if logger != nil {
			logger.Info("xrdp check", "enabled", true, "port", result.RDPPort)
		}
	} else {
		if logger != nil {
			logger.Warn("xrdp not found")
		}
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
