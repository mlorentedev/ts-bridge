//go:build windows

package host

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

func isElevatedImpl() bool {
	return exec.Command("net", "session").Run() == nil
}

func setupImpl(cfg Config, logger *slog.Logger) (SetupResult, error) {
	var steps []SetupStep

	steps = append(steps, stepTailscaleUnattended())
	steps = append(steps, stepTailscaleService())
	steps = append(steps, stepUPnPServices()...)
	steps = append(steps, stepNetworkProfile())

	// On Windows the RDP listening port lives in the registry; stepRDPConfig
	// reads the authoritative value and the firewall must open that exact port.
	rdpPort, stepRDP := stepRDPConfig()
	steps = append(steps, stepRDP)

	steps = append(steps, stepFirewall(cfg.FirewallRule, rdpPort))

	if !cfg.NoSleep {
		steps = append(steps, stepPowerSettings())
	}

	logSetupSteps(logger, steps)

	return SetupResult{RDPPort: rdpPort, Steps: steps}, nil
}

// logSetupSteps emits one structured log line per step. The logger may be nil.
func logSetupSteps(logger *slog.Logger, steps []SetupStep) {
	if logger == nil {
		return
	}
	for _, s := range steps {
		if s.Success {
			logger.Info(s.Name, "message", s.Message)
		} else {
			logger.Warn(s.Name, "message", s.Message)
		}
	}
}

// stepTailscaleUnattended enables Tailscale unattended mode.
func stepTailscaleUnattended() SetupStep {
	name := "Tailscale unattended mode"
	if err := runCmd("tailscale", "up", "--unattended"); err != nil {
		return SetupStep{Name: name, Success: false, Message: "Could not enable via CLI — check Tailscale GUI"}
	}
	return SetupStep{Name: name, Success: true, Message: "Unattended mode enabled"}
}

// stepTailscaleService starts the Tailscale service.
func stepTailscaleService() SetupStep {
	name := "Tailscale service"
	if err := startService("Tailscale"); err != nil {
		return SetupStep{Name: name, Success: false, Message: "Tailscale service not found — is Tailscale installed?"}
	}
	return SetupStep{Name: name, Success: true, Message: "Service running"}
}

// stepUPnPServices starts UPnP-related services (SSDPSRV, upnphost).
func stepUPnPServices() []SetupStep {
	var steps []SetupStep
	for _, svc := range []struct{ name, desc string }{
		{"SSDPSRV", "SSDP Discovery"},
		{"upnphost", "UPnP Device Host"},
	} {
		name := svc.desc
		if err := startService(svc.name); err != nil {
			steps = append(steps, SetupStep{Name: name, Success: false, Message: fmt.Sprintf("Failed to start: %v", err)})
		} else {
			steps = append(steps, SetupStep{Name: name, Success: true, Message: "Running"})
		}
	}
	return steps
}

// stepNetworkProfile sets the Tailscale network profile to Private.
func stepNetworkProfile() SetupStep {
	name := "Network profile"
	if err := setNetworkProfilePrivate(); err != nil {
		return SetupStep{Name: name, Success: false, Message: "Could not set network profile to Private"}
	}
	return SetupStep{Name: name, Success: true, Message: "Set to Private"}
}

// stepRDPConfig enables RDP and returns the port and a SetupStep.
func stepRDPConfig() (int, SetupStep) {
	name := "RDP configuration"
	rdpPort, err := enableRDP()
	if err != nil {
		rdpPort = 3389
		return rdpPort, SetupStep{Name: name, Success: false, Message: "Could not verify RDP settings, assuming port 3389"}
	}
	return rdpPort, SetupStep{Name: name, Success: true, Message: fmt.Sprintf("RDP enabled on port %d", rdpPort)}
}

// stepFirewall configures the firewall rule for RDP.
func stepFirewall(ruleName string, port int) SetupStep {
	name := "Firewall rule"
	if err := configureFirewall(ruleName, port); err != nil {
		return SetupStep{Name: name, Success: false, Message: fmt.Sprintf("Failed to configure firewall: %v", err)}
	}
	return SetupStep{Name: name, Success: true, Message: fmt.Sprintf("Rule '%s' active (port %d)", ruleName, port)}
}

// stepPowerSettings disables sleep on AC power.
func stepPowerSettings() SetupStep {
	name := "Power settings"
	if err := disableSleep(); err != nil {
		return SetupStep{Name: name, Success: false, Message: "Could not modify power settings"}
	}
	return SetupStep{Name: name, Success: true, Message: "Sleep disabled (AC power)"}
}

func checkImpl(cfg Config, logger *slog.Logger) (CheckResult, error) {
	result := CheckResult{}
	tsIP := TailscaleIP()
	result.TailscaleIP = tsIP
	result.TailscaleUp = tsIP != ""

	rdpPort, err := getRDPPort()
	if err == nil {
		result.RDPPort = rdpPort
		result.RDPEnabled = rdpPort > 0
	}
	result.FirewallOK = checkFirewallRule(cfg.FirewallRule)

	if logger != nil {
		logger.Info("host check",
			"tailscale_up", result.TailscaleUp,
			"rdp_port", result.RDPPort,
			"firewall_ok", result.FirewallOK,
		)
	}
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

func checkFirewallRule(ruleName string) bool {
	sanitized, err := sanitizeFirewallRule(ruleName)
	if err != nil {
		return false
	}
	ps := fmt.Sprintf(`Get-NetFirewallRule -DisplayName %q -ErrorAction SilentlyContinue`, sanitized)
	out, err := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps).Output()
	return err == nil && len(out) > 0
}
