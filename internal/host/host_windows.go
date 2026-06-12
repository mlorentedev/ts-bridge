//go:build windows

package host

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func isElevatedImpl() bool {
	return exec.Command("net", "session").Run() == nil
}

func setupImpl(flags SetupFlags) (SetupResult, error) {
	var steps []SetupStep

	// 1. Tailscale unattended mode.
	steps = append(steps, SetupStep{Name: "Tailscale unattended mode"})
	if err := runCmd("tailscale", "up", "--unattended"); err != nil {
		steps[len(steps)-1].Success = false
		steps[len(steps)-1].Message = "Could not enable via CLI — check Tailscale GUI"
	} else {
		steps[len(steps)-1].Success = true
		steps[len(steps)-1].Message = "Unattended mode enabled"
	}

	// 2. Tailscale service.
	steps = append(steps, SetupStep{Name: "Tailscale service"})
	if err := startService("Tailscale"); err != nil {
		steps[len(steps)-1].Success = false
		steps[len(steps)-1].Message = "Tailscale service not found — is Tailscale installed?"
	} else {
		steps[len(steps)-1].Success = true
		steps[len(steps)-1].Message = "Service running"
	}

	// 3. UPnP services.
	steps = append(steps, SetupStep{Name: "UPnP services"})
	upnpOk := true
	for _, svc := range []struct{ name, desc string }{
		{"SSDPSRV", "SSDP Discovery"},
		{"upnphost", "UPnP Device Host"},
	} {
		if err := startService(svc.name); err != nil {
			steps[len(steps)-1].Message = fmt.Sprintf("%s failed to start", svc.desc)
			upnpOk = false
		}
	}
	if upnpOk {
		steps[len(steps)-1].Success = true
		steps[len(steps)-1].Message = "UPnP services active"
	} else {
		steps[len(steps)-1].Success = false
	}

	// 4. Network profile.
	steps = append(steps, SetupStep{Name: "Network profile"})
	if err := setNetworkProfilePrivate(); err != nil {
		steps[len(steps)-1].Success = false
		steps[len(steps)-1].Message = "Could not set network profile to Private"
	} else {
		steps[len(steps)-1].Success = true
		steps[len(steps)-1].Message = "Set to Private"
	}

	// 5. RDP configuration.
	steps = append(steps, SetupStep{Name: "RDP configuration"})
	rdpPort, err := enableRDP()
	if err != nil {
		rdpPort = 3389
		steps[len(steps)-1].Success = false
		steps[len(steps)-1].Message = "Could not verify RDP settings, assuming port 3389"
	} else {
		steps[len(steps)-1].Success = true
		steps[len(steps)-1].Message = fmt.Sprintf("RDP enabled on port %d", rdpPort)
	}

	// 6. Firewall rule.
	steps = append(steps, SetupStep{Name: "Firewall rule"})
	if err := configureFirewall(flags.FirewallRule, rdpPort); err != nil {
		steps[len(steps)-1].Success = false
		steps[len(steps)-1].Message = fmt.Sprintf("Failed to configure firewall: %v", err)
	} else {
		steps[len(steps)-1].Success = true
		steps[len(steps)-1].Message = fmt.Sprintf("Rule '%s' active (port %d)", flags.FirewallRule, rdpPort)
	}

	// 7. Sleep settings (optional).
	if !flags.NoSleep {
		steps = append(steps, SetupStep{Name: "Power settings"})
		if err := disableSleep(); err != nil {
			steps[len(steps)-1].Success = false
			steps[len(steps)-1].Message = "Could not modify power settings"
		} else {
			steps[len(steps)-1].Success = true
			steps[len(steps)-1].Message = "Sleep disabled (AC power)"
		}
	}

	return SetupResult{RDPPort: rdpPort, Steps: steps}, nil
}

func checkImpl() (CheckResult, error) {
	result := CheckResult{}
	tsIP := TailscaleIP()
	result.TailscaleIP = tsIP
	result.TailscaleUp = tsIP != ""

	rdpPort, err := getRDPPort()
	if err == nil {
		result.RDPPort = rdpPort
		result.RDPEnabled = rdpPort > 0
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

// ─── Windows helpers ─────────────────────────────────────────────

func runCmd(name string, args ...string) error {
	// #nosec G204 -- name is a hardcoded binary path, args are controlled values.
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func startService(name string) error {
	// #nosec G204 -- service name is controlled (Tailscale, SSDPSRV, upnphost).
	if err := runCmd("sc", "config", name, "start=auto"); err != nil {
		return err
	}
	// #nosec G204 -- service name is controlled.
	out, err := exec.Command("sc", "query", name).Output()
	if err != nil {
		return err
	}
	if !strings.Contains(string(out), "RUNNING") {
		return runCmd("net", "start", name)
	}
	return nil
}

func setNetworkProfilePrivate() error {
	ps := `Get-NetConnectionProfile | Where-Object { $_.InterfaceAlias -match "Tailscale" -or $_.Name -match "Tailscale" } | ForEach-Object { $_ | Set-NetConnectionProfile -NetworkCategory Private }`
	return runCmd("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps)
}

func enableRDP() (int, error) {
	ps := `Set-ItemProperty -Path 'HKLM:\System\CurrentControlSet\Control\Terminal Server' -Name "fDenyTSConnections" -Value 0; $RdpPort = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp' -ErrorAction SilentlyContinue).PortNumber; if (-not $RdpPort) { $RdpPort = 3389 }; Write-Output $RdpPort`
	out, err := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps).Output()
	if err != nil {
		return 3389, err
	}
	port, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	if port < 1 {
		port = 3389
	}
	return port, nil
}

func configureFirewall(ruleName string, port int) error {
	sanitized, err := sanitizeFirewallRule(ruleName)
	if err != nil {
		return fmt.Errorf("validate firewall rule: %w", err)
	}
	ps := fmt.Sprintf(`Remove-NetFirewallRule -DisplayName %q -ErrorAction SilentlyContinue; New-NetFirewallRule -DisplayName %q -Direction Inbound -Action Allow -Protocol TCP -LocalPort %d -Profile Any -Enabled True | Out-Null; Enable-NetFirewallRule -DisplayGroup "Remote Desktop" -ErrorAction SilentlyContinue`, sanitized, sanitized, port)
	return runCmd("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps)
}

func disableSleep() error {
	return runCmd("powercfg", "/change", "standby-timeout-ac", "0", "/change", "hibernate-timeout-ac", "0", "/change", "monitor-timeout-ac", "0")
}

func getRDPPort() (int, error) {
	ps := `$RdpPort = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp' -ErrorAction SilentlyContinue).PortNumber; if (-not $RdpPort) { $RdpPort = 3389 }; Write-Output $RdpPort`
	out, err := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps).Output()
	if err != nil {
		return 0, err
	}
	port, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	if port < 1 {
		port = 3389
	}
	return port, nil
}

func checkFirewallRule() bool {
	out, err := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", `Get-NetFirewallRule -DisplayName "Tailscale-RDP-Ingress" -ErrorAction SilentlyContinue`).Output()
	return err == nil && len(out) > 0
}
