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
	// exposed only when it is enabled, inbound, allows any address and opens that
	// port, alone, as Any or inside a low-high range. Only a missing rule is
	// absent: any other error, the filter reads included, stops the read, so an
	// unreadable rule is never counted as one that exposes nothing.
	psReadSSHInstallerRule = `$ErrorActionPreference = 'Stop'; $port = %d; $g = @(); ` +
		`try { $g = @(Get-NetFirewallRule -Name '` + sshInstallerRule + `') } catch { if ($_.CategoryInfo.Category -ne 'ObjectNotFound') { throw } }; ` +
		`$opens = { param($ports) foreach ($e in @($ports)) { if ($e -eq 'Any' -or $e -eq "$port") { return $true }; ` +
		`if ($e -match '^(\d+)-(\d+)$' -and [int]$Matches[1] -le $port -and $port -le [int]$Matches[2]) { return $true } }; return $false }; ` +
		`Write-Output "total=$($g.Count)"; ` +
		`Write-Output "exposed=$(@($g | Where-Object { $_.Enabled -eq 'True' -and $_.Direction -eq 'Inbound' -and $_.Action -eq 'Allow' -and ` +
		`@(($_ | Get-NetFirewallAddressFilter).RemoteAddress) -contains 'Any' -and (& $opens ($_ | Get-NetFirewallPortFilter).LocalPort) }).Count)"`
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

// sshInstall is what the host has of the OpenSSH Server. external is an
// install that is not the Windows capability (the Win32-OpenSSH MSI), found by
// its service; bin is the sshd.exe the service runs, "" for the default path.
type sshInstall struct {
	installed, external bool
	bin                 string
	// args is what the service passes sshd after bin (-f another file, say).
	args string
}

// readSSHInstall reads the capability and, since the MSI installs no capability,
// the sshd service: NotPresent with no service is a missing server, and
// NotPresent with a service is one installed outside Windows capabilities.
// A capability read that fails is not retried through the service: the host
// state is unknown, as for any unreadable step.
func readSSHInstall(r Runner) (sshInstall, error) {
	capInstalled, err := readSSHCapability(r)
	if err != nil {
		return sshInstall{}, err
	}
	out, qcErr := r.Output("sc", "qc", sshWinService)
	in := sshInstall{installed: capInstalled || qcErr == nil, external: !capInstalled && qcErr == nil}
	if qcErr == nil {
		in.bin, in.args = serviceBinary(string(out))
	}
	return in, nil
}

// serviceBinary is the executable in the BINARY_PATH_NAME line of `sc qc`,
// quoted or not, and the arguments after it; "" when there is none. sc localizes
// no key names, and the path is the value after the first colon.
func serviceBinary(qc string) (bin, args string) {
	for _, line := range strings.Split(qc, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "BINARY_PATH_NAME" {
			continue
		}
		value = strings.TrimSpace(value)
		if rest, quoted := strings.CutPrefix(value, `"`); quoted {
			path, after, _ := strings.Cut(rest, `"`)
			return path, strings.TrimSpace(after)
		}
		if i := strings.Index(strings.ToLower(value), ".exe"); i >= 0 {
			return value[:i+len(".exe")], strings.TrimSpace(value[i+len(".exe"):])
		}
	}
	return "", ""
}

// sshPortsScript runs sshd -T from bin, or from the capability's path when the
// service names none. The path comes from the service configuration, which only
// an administrator writes; it is single-quoted so it is a literal, not code.
func sshPortsScript(bin string) string {
	if bin == "" {
		return psReadSSHPorts
	}
	return `& '` + strings.ReplaceAll(bin, "'", "''") + `' -T; exit $LASTEXITCODE`
}

// sshCapabilityStep is the capability step. unreadable is the outcome of a
// read that failed: failed in setup, unverified in check (ADR-017 §2).
func sshCapabilityStep(in sshInstall, err error, unreadable Outcome) SetupStep {
	switch {
	case err != nil:
		return newStep(sshWinCapName, unreadable, "Cannot read the OpenSSH Server capability: "+err.Error(),
			"run `Get-WindowsCapability -Online -Name 'OpenSSH.Server*'` in an elevated PowerShell")
	case in.external:
		return newStep(sshWinCapName, OutcomeOK, "The OpenSSH Server is installed outside Windows capabilities (the "+sshWinService+" service exists)", "")
	case !in.installed:
		return newStep(sshWinCapName, OutcomeManual, "The OpenSSH Server is not installed", sshWinMissingFix)
	}
	return newStep(sshWinCapName, OutcomeOK, "The OpenSSH Server is installed", "")
}

func sshWindowsSetupSteps(r Runner, port int) []SetupStep {
	in, err := readSSHInstall(r)
	steps := []SetupStep{sshCapabilityStep(in, err, OutcomeFailed)}
	// Without the capability or a service there is no sshd to converge or to read.
	if in.installed {
		steps = append(steps, serviceStep(r, sshWinService, sshName, "Failed to start sshd", "start the sshd service"))
		if s, ok := sshWindowsPortStep(r, port, in, OutcomeFailed); ok {
			steps = append(steps, s)
		}
	}
	return append(steps, windowsFirewallSetupFor(r, sshWinFwName, sshFirewallRule, port, sshInstallerRuleStep(port)))
}

func sshWindowsCheck(r Runner, port int, result *CheckResult) {
	in, err := readSSHInstall(r)
	capStep := sshCapabilityStep(in, err, OutcomeUnverified)
	result.Steps = append(result.Steps, capStep)
	svc := Service{Kind: ServiceSSH, Port: port, Unread: capStep.Outcome == OutcomeUnverified}
	if in.installed {
		step, auto, running, readable := serviceCheckStep(r, sshWinService, sshName)
		result.Steps = append(result.Steps, step)
		svc.Enabled, svc.Unread = auto, step.Outcome == OutcomeUnverified
		if readable {
			svc.Active = &running
		}
		if s, ok := sshWindowsPortStep(r, port, in, OutcomeUnverified); ok {
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
// than 22 (from the sshd.exe in, the default path when it names none), as the Linux step does: the listener is never moved, since
// sshd_config is the operator's. unreadable is the outcome when `sshd -T`
// cannot be read (it needs an elevated shell and the host keys).
func sshWindowsPortStep(r Runner, declared int, in sshInstall, unreadable Outcome) (SetupStep, bool) {
	if declared == defaultPorts[ServiceSSH] {
		return SetupStep{}, false
	}
	ports, err := readSSHWindowsPorts(r, in)
	switch {
	case err != nil:
		return newStep(sshPortName, unreadable, "Cannot read the sshd listener: "+err.Error(),
			fmt.Sprintf("run `sshd.exe -T` (the one `sc qc sshd` names; %%WINDIR%%\\System32\\OpenSSH\\sshd.exe for the capability) in an elevated PowerShell and check that sshd listens on TCP %d", declared)), true
	case slices.Contains(ports, declared):
		return newStep(sshPortName, OutcomeOK, fmt.Sprintf("sshd listens on TCP %d", declared), ""), true
	}
	return newStep(sshPortName, OutcomeManual, fmt.Sprintf("sshd listens on TCP %s, not the declared %d", portList(ports), declared),
		fmt.Sprintf(`set Port %d in %%ProgramData%%\ssh\sshd_config and run Restart-Service sshd; ts-bridge does not edit it`, declared)), true
}

// readSSHWindowsPorts runs sshd -T as the service does. A service that passes
// sshd arguments (-f another file, say) is not probed: sshd -T without them
// would read the default configuration and answer for another listener.
func readSSHWindowsPorts(r Runner, in sshInstall) ([]int, error) {
	if in.args != "" {
		return nil, fmt.Errorf("the %s service starts sshd with arguments (%s), which ts-bridge does not interpret", sshWinService, in.args)
	}
	out, err := outputPS(r, sshPortsScript(in.bin))
	if err != nil {
		return nil, fmt.Errorf("sshd -T: %w", err)
	}
	return sshdPorts(out)
}
