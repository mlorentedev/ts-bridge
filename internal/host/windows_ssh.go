package host

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const (
	sshWinCapName = "OpenSSH Server capability"
	sshWinFwName  = "Firewall rule (ssh)"
	sshWinService = "sshd"
	// sshFirewallRule is the ts-bridge-owned rule for the ssh port, beside
	// Tailscale-RDP-Ingress. sanitizeFirewallRule admits no space, and
	// host.firewall.rule names the RDP rule, so the name is not configurable.
	sshFirewallRule = "Tailscale-SSH-Ingress"
	// sshInstallerRule is the rule the OpenSSH Server capability installs. It
	// allows any address, so it is reported, never disabled (Amendment B).
	sshInstallerRule = "OpenSSH-Server-In-TCP"

	sshWinCapability = "OpenSSH.Server~~~~0.0.1.0"
	sshWinMissingFix = "run `Add-WindowsCapability -Online -Name " + sshWinCapability + "` in an elevated PowerShell " +
		"(a managed host needs Windows Update, WSUS or a Features-on-Demand source for it); ts-bridge does not install the capability"
	sshWinInstallerFix = "Disable-NetFirewallRule -Name " + sshInstallerRule
)

// sshCapState is the one line the capability read prints: Installed or NotPresent.
var sshCapState = psField{"state", "$($c.State)"}

var (
	// psReadSSHCapability stops on any error, so a shell that is not elevated
	// (Get-WindowsCapability needs it) exits non-zero instead of printing an empty state.
	psReadSSHCapability = `$ErrorActionPreference = 'Stop'; $c = Get-WindowsCapability -Online -Name 'OpenSSH.Server*' | Select-Object -First 1; ` + sshCapState.print()
	// psReadSSHPorts runs the installed sshd, which prints the effective
	// configuration in lower case (`port 22`, `listenaddress 0.0.0.0:22`) and needs the host keys.
	psReadSSHPorts = `& "$env:WINDIR\System32\OpenSSH\sshd.exe" -T; exit $LASTEXITCODE`
	// psReadSSHInstallerRule takes the declared port as its %d: the rule is
	// exposed only when it is enabled, inbound, allows any address and opens that port.
	psReadSSHInstallerRule = `$g = @(Get-NetFirewallRule -Name '` + sshInstallerRule + `' -ErrorAction SilentlyContinue); Write-Output "total=$($g.Count)"; ` +
		`Write-Output "exposed=$(@($g | Where-Object { $_.Enabled -eq 'True' -and $_.Direction -eq 'Inbound' -and $_.Action -eq 'Allow' -and ($_ | Get-NetFirewallAddressFilter).RemoteAddress -eq 'Any' -and ` +
		`(@(($_ | Get-NetFirewallPortFilter).LocalPort) -contains '%d' -or @(($_ | Get-NetFirewallPortFilter).LocalPort) -contains 'Any') }).Count)"`
)

// readSSHCapability reads whether the OpenSSH Server capability is installed.
// Only Installed and NotPresent are interpreted: Staged, a pending install, or
// no capability of that name at all is an answer ts-bridge cannot act on.
func readSSHCapability(r Runner) (bool, error) {
	out, err := outputPS(r, psReadSSHCapability)
	if err != nil {
		return false, err
	}
	switch state := parseKeyValues(out)[sshCapState.Key]; state {
	case "Installed":
		return true, nil
	case "NotPresent":
		return false, nil
	default:
		return false, fmt.Errorf("Get-WindowsCapability printed state %q, not Installed or NotPresent", state)
	}
}

// sshCapabilityStep is the capability step. unreadable is the outcome of a
// read that failed: failed in setup, unverified in check (ADR-017 §2).
func sshCapabilityStep(installed bool, err error, unreadable Outcome) SetupStep {
	switch {
	case err != nil:
		return newStep(sshWinCapName, unreadable, "Cannot read the OpenSSH Server capability: "+err.Error(),
			"run `Get-WindowsCapability -Online -Name 'OpenSSH.Server*'` in an elevated PowerShell")
	case !installed:
		return newStep(sshWinCapName, OutcomeManual, "The OpenSSH Server is not installed", sshWinMissingFix)
	}
	return newStep(sshWinCapName, OutcomeOK, "The OpenSSH Server is installed", "")
}

func sshWindowsSetupSteps(r Runner, port int) []SetupStep {
	installed, err := readSSHCapability(r)
	steps := []SetupStep{sshCapabilityStep(installed, err, OutcomeFailed)}
	// Without the capability there is no sshd service to converge or to read.
	if installed {
		steps = append(steps, serviceStep(r, sshWinService, sshName, "Failed to start sshd", "start the sshd service"))
		if s, ok := sshWindowsPortStep(r, port, OutcomeFailed); ok {
			steps = append(steps, s)
		}
	}
	return append(steps, windowsFirewallSetupFor(r, sshWinFwName, sshFirewallRule, port, sshInstallerRuleStep(port)))
}

func sshWindowsCheck(r Runner, port int, result *CheckResult) {
	installed, err := readSSHCapability(r)
	capStep := sshCapabilityStep(installed, err, OutcomeUnverified)
	result.Steps = append(result.Steps, capStep)
	svc := Service{Kind: ServiceSSH, Port: port, Unread: capStep.Outcome == OutcomeUnverified}
	if installed {
		step, auto, running, readable := serviceCheckStep(r, sshWinService, sshName)
		result.Steps = append(result.Steps, step)
		svc.Enabled, svc.Unread = auto, step.Outcome == OutcomeUnverified
		if readable {
			svc.Active = &running
		}
		if s, ok := sshWindowsPortStep(r, port, OutcomeUnverified); ok {
			result.Steps = append(result.Steps, s)
		}
	}
	fw, fwOK := windowsFirewallCheckFor(r, sshWinFwName, sshFirewallRule, port, sshInstallerRuleStep(port))
	svc.FirewallOK = fwOK
	result.Steps = append(result.Steps, fw)
	result.Services = append(result.Services, svc)
}

// sshInstallerRuleStep is the fwGuard for the capability's own rule. Unlike the
// Remote Desktop group, no rule at all is a normal host (not installed, or the
// operator removed it), so an empty answer is not an unknown exposure.
func sshInstallerRuleStep(port int) fwGuard {
	return func(r Runner, step SetupStep) (SetupStep, bool) {
		out, err := outputPS(r, fmt.Sprintf(psReadSSHInstallerRule, port))
		if err != nil {
			return newStep(step.Name, OutcomeUnverified, "Could not read the OpenSSH installer's firewall rule "+sshInstallerRule,
				"run Get-NetFirewallRule -Name "+sshInstallerRule+" to see why the read fails"), false
		}
		kv := parseKeyValues(out)
		_, totalErr := strconv.Atoi(kv["total"])
		exposed, parseErr := strconv.Atoi(kv["exposed"])
		if totalErr != nil || parseErr != nil {
			return newStep(step.Name, OutcomeUnverified, fmt.Sprintf("The OpenSSH installer's rule %s read returned no count (%q)", sshInstallerRule, strings.TrimSpace(string(out))),
				"run Get-NetFirewallRule -Name "+sshInstallerRule+" to read it"), false
		}
		if exposed == 0 {
			return step, true
		}
		return newStep(step.Name, OutcomeManual, fmt.Sprintf("The OpenSSH installer's rule %s allows inbound TCP %d from any address, which voids the tailnet scope", sshInstallerRule, port),
			sshWinInstallerFix), false
	}
}

// sshWindowsPortStep reports whether sshd listens on a declared port other
// than 22, as the Linux step does: the listener is never moved, since
// sshd_config is the operator's. unreadable is the outcome when `sshd -T`
// cannot be read (it needs an elevated shell and the host keys).
func sshWindowsPortStep(r Runner, declared int, unreadable Outcome) (SetupStep, bool) {
	if declared == defaultPorts[ServiceSSH] {
		return SetupStep{}, false
	}
	ports, err := readSSHWindowsPorts(r)
	switch {
	case err != nil:
		return newStep(sshPortName, unreadable, "Cannot read the sshd listener: "+err.Error(),
			fmt.Sprintf(`run `+"`"+`& "$env:WINDIR\System32\OpenSSH\sshd.exe" -T`+"`"+` in an elevated PowerShell and check that sshd listens on TCP %d`, declared)), true
	case slices.Contains(ports, declared):
		return newStep(sshPortName, OutcomeOK, fmt.Sprintf("sshd listens on TCP %d", declared), ""), true
	}
	return newStep(sshPortName, OutcomeManual, fmt.Sprintf("sshd listens on TCP %s, not the declared %d", portList(ports), declared),
		fmt.Sprintf(`set Port %d in %%ProgramData%%\ssh\sshd_config and run Restart-Service sshd; ts-bridge does not edit it`, declared)), true
}

func readSSHWindowsPorts(r Runner) ([]int, error) {
	out, err := outputPS(r, psReadSSHPorts)
	if err != nil {
		return nil, fmt.Errorf("sshd -T: %w", err)
	}
	return sshdPorts(out)
}
