package host

import (
	"fmt"
	"strconv"
	"strings"
)

// Every Windows step acts on each run, so success is "changed"; #455 adds the
// reads that let a converged host report "ok" (W1-W3).

func setupWindows(cfg Config, r Runner) SetupResult {
	steps := []SetupStep{
		actStep("Tailscale unattended mode", r.Run("tailscale", "up", "--unattended"),
			"Unattended mode enabled", "Could not enable via CLI — check Tailscale GUI", "enable unattended mode in the Tailscale GUI"),
		actStep("Tailscale service", startService(r, "Tailscale"),
			"Service running", "Tailscale service not found — is Tailscale installed?", "install Tailscale"),
	}
	for _, svc := range []struct{ name, desc string }{{"SSDPSRV", "SSDP Discovery"}, {"upnphost", "UPnP Device Host"}} {
		err := startService(r, svc.name)
		steps = append(steps, actStep(svc.desc, err, "Running", fmt.Sprintf("Failed to start: %v", err), "start the "+svc.name+" service"))
	}
	steps = append(steps, actStep("Network profile", runPS(r, psNetworkProfilePrivate),
		"Set to Private", "Could not set network profile to Private", "set the Tailscale network profile to Private"))

	// The RDP port lives in the registry; the firewall must open that exact port.
	rdpPort, err := readPortPS(r, psEnableRDP)
	if err != nil {
		rdpPort = defaultRDPPort
	}
	steps = append(steps, actStep("RDP configuration", err, fmt.Sprintf("RDP enabled on port %d", rdpPort),
		"Could not verify RDP settings, assuming port 3389", "enable Remote Desktop in Settings > System > Remote Desktop"))

	err = configureWindowsFirewall(r, cfg.FirewallRule, rdpPort)
	steps = append(steps, actStep("Firewall rule", err, fmt.Sprintf("Rule '%s' active (port %d)", cfg.FirewallRule, rdpPort),
		fmt.Sprintf("Failed to configure firewall: %v", err), fmt.Sprintf("allow TCP %d inbound in Windows Defender Firewall", rdpPort)))

	if !cfg.NoSleep {
		err = r.Run("powercfg", "/change", "standby-timeout-ac", "0", "/change", "hibernate-timeout-ac", "0", "/change", "monitor-timeout-ac", "0")
		steps = append(steps, actStep("Power settings", err, "Sleep disabled (AC power)", "Could not modify power settings", "disable sleep on AC power"))
	}
	return SetupResult{RDPPort: rdpPort, Steps: steps}
}

// actStep maps an action's error to "changed" or "failed".
func actStep(name string, err error, okMsg, failMsg, fix string) SetupStep {
	if err != nil {
		return newStep(name, OutcomeFailed, failMsg, fix)
	}
	return newStep(name, OutcomeChanged, okMsg, "")
}

func checkWindows(cfg Config, r Runner, result *CheckResult) {
	svc := Service{Kind: ServiceRDP}
	port, err := readPortPS(r, psReadRDPPort)
	if err == nil {
		svc.Port, svc.Enabled = port, true // W2 (#455): fDenyTSConnections is never read
	}
	result.Steps = append(result.Steps, actRead("RDP configuration", err, fmt.Sprintf("RDP port %d", port),
		"Could not read the RDP port from the registry", "enable Remote Desktop in Settings > System > Remote Desktop"))
	svc.FirewallOK = windowsFirewallRuleExists(r, cfg.FirewallRule)
	result.Steps = append(result.Steps, firewallReadStep("Firewall rule", svc.FirewallOK, svc.Port))
	result.Services = []Service{svc}
}

// actRead maps a read's error to "ok" or "failed".
func actRead(name string, err error, okMsg, failMsg, fix string) SetupStep {
	if err != nil {
		return newStep(name, OutcomeFailed, failMsg, fix)
	}
	return newStep(name, OutcomeOK, okMsg, "")
}

const (
	psNetworkProfilePrivate = `Get-NetConnectionProfile | Where-Object { $_.InterfaceAlias -match "Tailscale" -or $_.Name -match "Tailscale" } | ForEach-Object { $_ | Set-NetConnectionProfile -NetworkCategory Private }`
	psReadRDPPort           = `$RdpPort = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp' -ErrorAction SilentlyContinue).PortNumber; if (-not $RdpPort) { $RdpPort = 3389 }; Write-Output $RdpPort`
	psEnableRDP             = `Set-ItemProperty -Path 'HKLM:\System\CurrentControlSet\Control\Terminal Server' -Name "fDenyTSConnections" -Value 0; ` + psReadRDPPort
)

func runPS(r Runner, script string) error {
	return r.Run("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
}

func outputPS(r Runner, script string) ([]byte, error) {
	return r.Output("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
}

func readPortPS(r Runner, script string) (int, error) {
	out, err := outputPS(r, script)
	if err != nil {
		return 0, err
	}
	port, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	if port < 1 {
		port = defaultRDPPort
	}
	return port, nil
}

func startService(r Runner, name string) error {
	if err := r.Run("sc", "config", name, "start=auto"); err != nil {
		return err
	}
	out, err := r.Output("sc", "query", name)
	if err != nil {
		return err
	}
	if !strings.Contains(string(out), "RUNNING") {
		return r.Run("net", "start", name)
	}
	return nil
}

// configureWindowsFirewall deletes and recreates the rule on every run (W3, #455).
func configureWindowsFirewall(r Runner, ruleName string, port int) error {
	sanitized, err := sanitizeFirewallRule(ruleName)
	if err != nil {
		return fmt.Errorf("validate firewall rule: %w", err)
	}
	return runPS(r, fmt.Sprintf(`Remove-NetFirewallRule -DisplayName %q -ErrorAction SilentlyContinue; New-NetFirewallRule -DisplayName %q -Direction Inbound -Action Allow -Protocol TCP -LocalPort %d -Profile Any -Enabled True | Out-Null; Enable-NetFirewallRule -DisplayGroup "Remote Desktop" -ErrorAction SilentlyContinue`, sanitized, sanitized, port))
}

func windowsFirewallRuleExists(r Runner, ruleName string) bool {
	sanitized, err := sanitizeFirewallRule(ruleName)
	if err != nil {
		return false
	}
	out, err := outputPS(r, fmt.Sprintf(`Get-NetFirewallRule -DisplayName %q -ErrorAction SilentlyContinue`, sanitized))
	return err == nil && len(out) > 0
}
