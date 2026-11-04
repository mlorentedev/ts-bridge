package host

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Most Windows steps act on each run, so success is "changed". #455 adds the
// reads that let a converged host report "ok": unattended mode (W1) and the
// RDP denial (W2) read first; the firewall read (W3) follows.

func setupWindows(cfg Config, r Runner) SetupResult {
	steps := []SetupStep{
		unattendedStep(r),
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
	rdpStep, rdpPort := rdpSetupStep(r)
	steps = append(steps, rdpStep)

	err := configureWindowsFirewall(r, cfg.FirewallRule, rdpPort)
	steps = append(steps, actStep("Firewall rule", err, fmt.Sprintf("Rule '%s' active (port %d)", cfg.FirewallRule, rdpPort),
		fmt.Sprintf("Failed to configure firewall: %v", err), fmt.Sprintf("allow TCP %d inbound in Windows Defender Firewall", rdpPort)))

	if !cfg.NoSleep {
		err = r.Run("powercfg", "/change", "standby-timeout-ac", "0", "/change", "hibernate-timeout-ac", "0", "/change", "monitor-timeout-ac", "0")
		steps = append(steps, actStep("Power settings", err, "Sleep disabled (AC power)", "Could not modify power settings", "disable sleep on AC power"))
	}
	return SetupResult{RDPPort: rdpPort, Steps: steps}
}

// unattendedStep sets unattended mode only when the node lacks it (W1, #455).
// `tailscale set` changes that one pref; `tailscale up` would refuse on a
// node whose other prefs (a Headscale login server) it is not given again.
// The read is a debug subcommand, so any output that does not parse to
// ForceDaemon true falls through to the idempotent set rather than skipping it.
func unattendedStep(r Runner) SetupStep {
	const name = "Tailscale unattended mode"
	if out, err := r.Output("tailscale", "debug", "prefs"); err == nil {
		var prefs struct{ ForceDaemon bool }
		if json.Unmarshal(out, &prefs) == nil && prefs.ForceDaemon {
			return newStep(name, OutcomeOK, "Unattended mode already enabled", "")
		}
	}
	return actStep(name, r.Run("tailscale", "set", "--unattended"),
		"Unattended mode enabled", "Could not enable via CLI — check Tailscale GUI", "enable unattended mode in the Tailscale GUI")
}

// actStep maps an action's error to "changed" or "failed".
func actStep(name string, err error, okMsg, failMsg, fix string) SetupStep {
	if err != nil {
		return newStep(name, OutcomeFailed, failMsg, fix)
	}
	return newStep(name, OutcomeChanged, okMsg, "")
}

const (
	rdpEnableFix = "enable Remote Desktop in Settings > System > Remote Desktop"
	rdpPolicyFix = `Group Policy denies Remote Desktop: set HKLM\SOFTWARE\Policies\Microsoft\Windows NT\Terminal Services\fDenyTSConnections to 0 ` +
		`(Computer Configuration > Administrative Templates > Windows Components > Remote Desktop Services > Remote Desktop Session Host > Connections)`
)

// rdpDeny holds the two fDenyTSConnections values; "" means the value is absent.
type rdpDeny struct{ policy, local string }

// effective reports whether RDP is denied and whether Group Policy decides it.
// The policy value wins when present; with neither set Windows denies.
func (d rdpDeny) effective() (denied, byPolicy bool) {
	if d.policy != "" {
		return d.policy != "0", true
	}
	return d.local != "0", false
}

func readRDPDeny(r Runner) (rdpDeny, error) {
	out, err := outputPS(r, psReadRDPDeny)
	if err != nil {
		return rdpDeny{}, err
	}
	var d rdpDeny
	for _, line := range strings.Split(string(out), "\n") {
		key, value, _ := strings.Cut(strings.TrimSpace(line), "=")
		switch key {
		case "policy":
			d.policy = value
		case "local":
			d.local = value
		}
	}
	return d, nil
}

// portOrDefault reads the RDP port, falling back to 3389 when unreadable.
func portOrDefault(r Runner) int {
	port, err := readPortPS(r, psReadRDPPort)
	if err != nil {
		return defaultRDPPort
	}
	return port
}

// rdpSetupStep enables RDP only when the local value is what denies it (W2,
// #455). A policy denial is left to the user: a local write would be reverted
// on the next policy refresh.
func rdpSetupStep(r Runner) (SetupStep, int) {
	const name = "RDP configuration"
	deny, err := readRDPDeny(r)
	denied, byPolicy := deny.effective()
	switch {
	case err != nil:
		port := portOrDefault(r)
		return newStep(name, OutcomeFailed, fmt.Sprintf("Could not read fDenyTSConnections, assuming port %d", port), rdpEnableFix), port
	case denied && byPolicy:
		return newStep(name, OutcomeManual, "Remote Desktop is denied by Group Policy", rdpPolicyFix), portOrDefault(r)
	case !denied:
		port, err := readPortPS(r, psReadRDPPort)
		if err != nil {
			port = defaultRDPPort
		}
		return actRead(name, err, fmt.Sprintf("RDP already enabled on port %d", port),
			"Could not read the RDP port, assuming port 3389", rdpEnableFix), port
	}
	port, err := readPortPS(r, psEnableRDP)
	if err != nil {
		port = defaultRDPPort
	}
	return actStep(name, err, fmt.Sprintf("RDP enabled on port %d", port),
		"Could not verify RDP settings, assuming port 3389", rdpEnableFix), port
}

// rdpCheckStep is the read half of rdpSetupStep: "changed" means setup would
// enable RDP, "manual" that Group Policy keeps it off.
func rdpCheckStep(r Runner, port int) (SetupStep, bool) {
	const name = "RDP configuration"
	deny, err := readRDPDeny(r)
	denied, byPolicy := deny.effective()
	switch {
	case err != nil:
		return newStep(name, OutcomeFailed, "Could not read fDenyTSConnections from the registry", rdpEnableFix), false
	case denied && byPolicy:
		return newStep(name, OutcomeManual, "Remote Desktop is denied by Group Policy", rdpPolicyFix), false
	case denied:
		return newStep(name, OutcomeChanged, "Remote Desktop is disabled (fDenyTSConnections)", "run ts-bridge host setup as an administrator"), false
	}
	return newStep(name, OutcomeOK, fmt.Sprintf("RDP enabled on port %d", port), ""), true
}

func checkWindows(cfg Config, r Runner, result *CheckResult) {
	svc := Service{Kind: ServiceRDP}
	port, err := readPortPS(r, psReadRDPPort)
	step := actRead("RDP configuration", err, "", "Could not read the RDP port from the registry", rdpEnableFix)
	if err == nil {
		svc.Port = port
		step, svc.Enabled = rdpCheckStep(r, port)
	}
	result.Steps = append(result.Steps, step)
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
	psReadRDPDeny           = `$p = (Get-ItemProperty -Path 'HKLM:\SOFTWARE\Policies\Microsoft\Windows NT\Terminal Services' -Name fDenyTSConnections -ErrorAction SilentlyContinue).fDenyTSConnections; $l = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server' -Name fDenyTSConnections -ErrorAction SilentlyContinue).fDenyTSConnections; Write-Output "policy=$p"; Write-Output "local=$l"`
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
