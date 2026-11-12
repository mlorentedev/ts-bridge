package host

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// Every Windows step reads before it acts, so a converged host reports "ok"
// and a second run changes nothing (ADR-017): unattended mode (W1), the RDP
// denial (W2) and the firewall rule (W3) from #455; the services, the network
// profile and the power timeouts from #604 and #603.

func setupWindows(cfg Config, r Runner) SetupResult {
	steps := []SetupStep{
		// The service first: `tailscale debug prefs` and `tailscale set` need a running daemon (#700).
		serviceStep(r, "Tailscale", "Tailscale service", "Tailscale service not found — is Tailscale installed?", "install Tailscale"),
		unattendedStep(r),
		serviceStep(r, "SSDPSRV", "SSDP Discovery", "Failed to start SSDPSRV", "start the SSDPSRV service"),
		serviceStep(r, "upnphost", "UPnP Device Host", "Failed to start upnphost", "start the upnphost service"),
		networkProfileStep(r),
	}

	// The RDP port lives in the registry; the firewall must open that exact port.
	rdpStep, rdpPort := rdpSetupStep(r)
	steps = append(steps, declaredRDPPort(rdpStep, cfg, rdpPort))

	steps = append(steps, windowsFirewallSetupStep(r, cfg.FirewallRule, rdpPort))

	if !cfg.NoSleep {
		steps = append(steps, powerStep(r))
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
	if err := r.Run("tailscale", "set", "--unattended"); err != nil {
		if _, lerr := r.LookPath("tailscale"); lerr != nil {
			return newStep(name, OutcomeFailed, "Tailscale CLI not found — is Tailscale installed?", "install Tailscale")
		}
		return newStep(name, OutcomeFailed, "Could not enable via CLI — check Tailscale GUI", "enable unattended mode in the Tailscale GUI")
	}
	return newStep(name, OutcomeChanged, "Unattended mode enabled", "")
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
	f := parseKeyValues(out)
	return rdpDeny{policy: f[denyPolicy.Key], local: f[denyLocal.Key]}, nil
}

// parseKeyValues reads the key=value lines the PowerShell reads print. The
// first occurrence of a key wins, so a repeated block reports its first entry.
func parseKeyValues(out []byte) map[string]string {
	fields := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, _ := strings.Cut(strings.TrimSpace(line), "=")
		if _, seen := fields[key]; !seen {
			fields[key] = value
		}
	}
	return fields
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
	port, err := readPortPS(r, psReadRDPPort)
	svc := Service{Kind: ServiceRDP, Unread: err != nil}
	step := actRead("RDP configuration", err, "", "Could not read the RDP port from the registry", rdpEnableFix)
	if err == nil {
		svc.Port = port
		step, svc.Enabled = rdpCheckStep(r, port)
		step = declaredRDPPort(step, cfg, port)
	}
	result.Steps = append(result.Steps, step)
	fwStep, fwOK := windowsFirewallCheckStep(r, cfg.FirewallRule, svc.Port)
	svc.FirewallOK = fwOK
	result.Steps = append(result.Steps, fwStep)
	if !cfg.NoSleep {
		result.Steps = append(result.Steps, powerCheckStep(r))
	}
	result.Services = []Service{svc}
}

// declaredRDPPort turns a converged RDP step manual when the host declares a
// port the registry does not listen on (#557). ts-bridge does not move the RDP
// listener on Windows, so the operator changes it or the declaration.
func declaredRDPPort(step SetupStep, cfg Config, port int) SetupStep {
	if !cfg.PortDeclared || port == cfg.Port || !step.Outcome.Succeeded() {
		return step
	}
	return newStep(step.Name, OutcomeManual,
		fmt.Sprintf("RDP listens on port %d (set in the registry), not the declared %d", port, cfg.Port),
		fmt.Sprintf(`declare port %d, or change PortNumber under HKLM\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp to %d and restart the TermService service`, port, cfg.Port))
}

// actRead maps a read's error to "ok" or "failed".
func actRead(name string, err error, okMsg, failMsg, fix string) SetupStep {
	if err != nil {
		return newStep(name, OutcomeFailed, failMsg, fix)
	}
	return newStep(name, OutcomeOK, okMsg, "")
}

// psField is one line of a key=value PowerShell read. The script prints
// "Key=<Expr>" and the parser looks the value up by the same Key, so the two
// cannot disagree on a name (#601).
type psField struct{ Key, Expr string }

func (f psField) print() string { return fmt.Sprintf(`Write-Output "%s=%s"`, f.Key, f.Expr) }

// printAll joins the print statements of fields into one script fragment.
func printAll(fields ...psField) string {
	stmts := make([]string, len(fields))
	for i, f := range fields {
		stmts[i] = f.print()
	}
	return strings.Join(stmts, "; ")
}

var (
	netCategory = psField{"category", "$($_.NetworkCategory)"}

	denyPolicy = psField{"policy", "$p"}
	denyLocal  = psField{"local", "$l"}

	fwCount     = psField{"count", "$($r.Count)"}
	fwEnabled   = psField{"enabled", "$($x.Enabled)"}
	fwAction    = psField{"action", "$($x.Action)"}
	fwDirection = psField{"direction", "$($x.Direction)"}
	fwProfile   = psField{"profile", "$($x.Profile)"}
	fwRemote    = psField{"remote", "$(($x | Get-NetFirewallAddressFilter).RemoteAddress -join ',')"}
	fwProtocol  = psField{"protocol", "$($f.Protocol)"}
	fwPort      = psField{keyPort, "$($f.LocalPort)"}
)

// keyPort is the key a port is read and declared under: the firewall read's
// key=value line, the host file's services[] entry and xrdp.ini's [Globals].
const keyPort = "port"

const (
	psNetworkProfilePrivate = `Get-NetConnectionProfile | Where-Object { $_.InterfaceAlias -match "Tailscale" -or $_.Name -match "Tailscale" } | ForEach-Object { $_ | Set-NetConnectionProfile -NetworkCategory Private }`
	psReadRDPPort           = `$RdpPort = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp' -ErrorAction SilentlyContinue).PortNumber; if (-not $RdpPort) { $RdpPort = 3389 }; Write-Output $RdpPort`
	psEnableRDP             = `Set-ItemProperty -Path 'HKLM:\System\CurrentControlSet\Control\Terminal Server' -Name "fDenyTSConnections" -Value 0; ` + psReadRDPPort
)

// rdGroupID is the built-in "Remote Desktop" rule group by its resource id.
// The display name is localized ("Escritorio remoto" on a Spanish install), so
// a -DisplayGroup query matches nothing there and reads as no exposure (#707).
const rdGroupID = "@FirewallAPI.dll,-28752"

// The key=value reads. psReadFirewallRule takes the rule name as its %q.
var (
	psReadNetworkProfile = `Get-NetConnectionProfile | Where-Object { $_.InterfaceAlias -match "Tailscale" -or $_.Name -match "Tailscale" } | ForEach-Object { ` + netCategory.print() + ` }`
	psReadRDPDeny        = `$p = (Get-ItemProperty -Path 'HKLM:\SOFTWARE\Policies\Microsoft\Windows NT\Terminal Services' -Name fDenyTSConnections -ErrorAction SilentlyContinue).fDenyTSConnections; $l = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server' -Name fDenyTSConnections -ErrorAction SilentlyContinue).fDenyTSConnections; ` + printAll(denyPolicy, denyLocal)
	psReadRDGroup        = `$g = @(Get-NetFirewallRule -Group '` + rdGroupID + `' -ErrorAction SilentlyContinue); Write-Output "total=$($g.Count)"; ` +
		`Write-Output "exposed=$(@($g | Where-Object { $_.Enabled -eq 'True' -and $_.Direction -eq 'Inbound' -and $_.Action -eq 'Allow' -and ($_ | Get-NetFirewallAddressFilter).RemoteAddress -eq 'Any' }).Count)"`
	psReadFirewallRule = `$r = @(Get-NetFirewallRule -DisplayName %q -ErrorAction SilentlyContinue); ` + fwCount.print() + `; foreach ($x in $r) { ` +
		printAll(fwEnabled, fwAction, fwDirection, fwProfile, fwRemote) + `; $f = $x | Get-NetFirewallPortFilter; ` + printAll(fwProtocol, fwPort) + ` }`
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

// serviceStep sets service name to start automatically and starts it, reading
// both first: "ok" when it already is, "changed" when this run acted (#604).
func serviceStep(r Runner, svc, name, failMsg, fix string) SetupStep {
	auto, running := serviceState(r, svc)
	if auto && running {
		return newStep(name, OutcomeOK, "Already running", "")
	}
	if !auto {
		if err := r.Run("sc", "config", svc, "start=auto"); err != nil {
			return newStep(name, OutcomeFailed, fmt.Sprintf("%s: sc config: %v", failMsg, err), fix)
		}
	}
	if !running {
		if err := r.Run("net", "start", svc); err != nil {
			return newStep(name, OutcomeFailed, fmt.Sprintf("%s: net start: %v", failMsg, err), fix)
		}
	}
	return newStep(name, OutcomeChanged, "Running", "")
}

// serviceState reads the start type (sc qc) and the state (sc query). A read
// that fails reports false, so the step acts rather than trusting it.
func serviceState(r Runner, svc string) (auto, running bool) {
	if out, err := r.Output("sc", "qc", svc); err == nil {
		auto = strings.Contains(string(out), "AUTO_START")
	}
	if out, err := r.Output("sc", "query", svc); err == nil {
		running = strings.Contains(string(out), "RUNNING")
	}
	return auto, running
}

// networkProfileStep sets the Tailscale connection profile to Private after
// reading it. With no Tailscale adapter there is nothing to set: "manual",
// never "changed" (#604).
func networkProfileStep(r Runner) SetupStep {
	const name = "Network profile"
	const fail, fix = "Could not set network profile to Private", "set the Tailscale network profile to Private"
	out, err := outputPS(r, psReadNetworkProfile)
	if err != nil {
		return newStep(name, OutcomeFailed, fail, fix)
	}
	count, private := parseNetworkProfiles(string(out))
	switch {
	case count == 0:
		return newStep(name, OutcomeManual, "No Tailscale network adapter", "install Tailscale and log in, then run host setup again")
	case private == count:
		return newStep(name, OutcomeOK, "Already Private", "")
	}
	if err := runPS(r, psNetworkProfilePrivate); err != nil {
		return newStep(name, OutcomeFailed, fail, fix)
	}
	return newStep(name, OutcomeChanged, "Set to Private", "")
}

// parseNetworkProfiles counts the Tailscale profiles psReadNetworkProfile
// lists and how many of them are Private.
func parseNetworkProfiles(out string) (count, private int) {
	for _, line := range strings.Split(out, "\n") {
		v, ok := strings.CutPrefix(strings.TrimSpace(line), netCategory.Key+"=")
		if !ok {
			continue
		}
		count++
		if v == "Private" {
			private++
		}
	}
	return count, private
}

// powerSettings are the AC timeouts setup sets to 0, with the powercfg
// subgroup and setting aliases that read them.
var powerSettings = []struct{ change, sub, setting string }{
	{"standby-timeout-ac", "SUB_SLEEP", "STANDBYIDLE"},
	{"hibernate-timeout-ac", "SUB_SLEEP", "HIBERNATEIDLE"},
	{"monitor-timeout-ac", "SUB_VIDEO", "VIDEOIDLE"},
}

// powerStep disables sleep on AC power. powercfg takes one /change per call
// (#603), and each timeout is read first so a converged host reports "ok".
func powerStep(r Runner) SetupStep {
	const name = "Power settings"
	changed := false
	for _, p := range powerSettings {
		if acTimeoutZero(r, p.sub, p.setting) {
			continue
		}
		if err := r.Run("powercfg", "/change", p.change, "0"); err != nil {
			return newStep(name, OutcomeFailed, "Could not modify power settings ("+p.change+")", "disable sleep on AC power")
		}
		changed = true
	}
	if !changed {
		return newStep(name, OutcomeOK, "Sleep already disabled (AC power)", "")
	}
	return newStep(name, OutcomeChanged, "Sleep disabled (AC power)", "")
}

// acTimeoutZero reads one setting of the active scheme. powercfg localizes its
// labels but not their order: the AC index is the second-to-last hex value,
// the DC index the last. An unreadable answer reports false, so the step sets
// the value rather than trusting it.
func acTimeoutZero(r Runner, sub, setting string) bool {
	zero, err := readACTimeoutZero(r, sub, setting)
	return err == nil && zero
}

// readACTimeoutZero reads whether an AC timeout is 0, telling an unreadable
// value apart from a non-zero one for check (#652).
func readACTimeoutZero(r Runner, sub, setting string) (bool, error) {
	out, err := r.Output("powercfg", "/query", "SCHEME_CURRENT", sub, setting)
	if err != nil {
		return false, err
	}
	hex := hexValue.FindAllString(string(out), -1)
	if len(hex) < 2 {
		return false, fmt.Errorf("powercfg printed no AC setting index for %s %s", sub, setting)
	}
	v, err := strconv.ParseUint(hex[len(hex)-2][2:], 16, 32)
	if err != nil {
		return false, fmt.Errorf("powercfg AC index for %s %s: %w", sub, setting, err)
	}
	return v == 0, nil
}

// powerCheckStep is the read half of powerStep: "changed" names the AC
// timeouts setup would set to 0.
func powerCheckStep(r Runner) SetupStep {
	const name = "Power settings"
	var drift []string
	for _, p := range powerSettings {
		zero, err := readACTimeoutZero(r, p.sub, p.setting)
		if err != nil {
			return newStep(name, OutcomeFailed, "Could not read "+p.change+": "+err.Error(), "run `powercfg /query SCHEME_CURRENT` to inspect the power plan")
		}
		if !zero {
			drift = append(drift, p.change)
		}
	}
	if len(drift) == 0 {
		return newStep(name, OutcomeOK, "Sleep disabled (AC power)", "")
	}
	return newStep(name, OutcomeChanged, "Sleep is enabled on AC power: "+strings.Join(drift, ", ")+" not 0", "run ts-bridge host setup as an administrator")
}

var hexValue = regexp.MustCompile(`0x[0-9a-fA-F]{8}`)

const (
	windowsFirewallFix     = "allow TCP %d inbound from the tailnet addresses in Windows Defender Firewall"
	windowsGroupFix        = `Disable-NetFirewallRule -Group "` + rdGroupID + `"`
	windowsFirewallNameFix = "set host.firewall.rule (or --firewall-rule) to a name of A-Z, a-z, 0-9, - and _"
)

// tailnetRemoteAddress is the scope of the rule (ADR-017 Amendment B): the
// Tailscale CGNAT range and the IPv6 ULA prefix a tailnet assigns from.
const tailnetRemoteAddress = "100.64.0.0/10,fd7a:115c:a1e0::/48"

// remoteIsTailnet reports whether the RemoteAddress read is exactly the
// tailnet scope. Windows may print an IPv4 range with a dotted mask and an
// IPv6 prefix in upper case, so each entry is normalized before the sets are
// compared; "Any" or any other range is not the scope.
func remoteIsTailnet(remote string) bool {
	want := remoteSet(tailnetRemoteAddress)
	got := remoteSet(remote)
	if len(got) != len(want) {
		return false
	}
	for e := range got {
		if !want[e] {
			return false
		}
	}
	return true
}

func remoteSet(list string) map[string]bool {
	set := map[string]bool{}
	for _, e := range strings.FieldsFunc(list, func(c rune) bool { return c == ',' || c == ' ' }) {
		set[normalizeRange(e)] = true
	}
	return set
}

// normalizeRange lower-cases an entry and rewrites a dotted IPv4 mask as a
// prefix length (100.64.0.0/255.192.0.0 -> 100.64.0.0/10).
func normalizeRange(e string) string {
	e = strings.ToLower(e)
	addr, mask, ok := strings.Cut(e, "/")
	if !ok || !strings.Contains(mask, ".") {
		return e
	}
	ip := net.ParseIP(mask).To4()
	if ip == nil {
		return e
	}
	ones, bits := net.IPMask(ip).Size()
	if bits == 0 {
		return e
	}
	return addr + "/" + strconv.Itoa(ones)
}

// windowsFirewallRuleOK reads the named rule (W3, #455). It holds only when
// exactly one rule by that name exists, enabled, allowing inbound TCP from the
// tailnet addresses on every profile on port, as the recreate sets it; port 0
// (unknown) skips the port comparison. found reports whether any rule has the
// name; legacy reports a rule that allows any address, as earlier releases
// created it.
func windowsFirewallRuleOK(r Runner, rule string, port int) (ok, found, legacy bool, err error) { //nolint:gocyclo // baseline #629
	out, err := outputPS(r, fmt.Sprintf(psReadFirewallRule, rule))
	if err != nil {
		return false, false, false, err
	}
	f := parseKeyValues(out)
	ok = f[fwCount.Key] == "1" && f[fwEnabled.Key] == "True" && f[fwAction.Key] == "Allow" && f[fwDirection.Key] == "Inbound" &&
		f[fwProtocol.Key] == "TCP" && f[fwProfile.Key] == "Any" && remoteIsTailnet(f[fwRemote.Key]) && (port == 0 || f[fwPort.Key] == strconv.Itoa(port))
	return ok, f[fwCount.Key] != "0", f[fwCount.Key] == "1" && strings.EqualFold(f[fwRemote.Key], "any"), nil
}

// windowsFirewallSetupStep recreates the rule only when the read does not
// show it converged. The recreate is idempotent, so an unreadable rule is
// recreated rather than reported.
func windowsFirewallSetupStep(r Runner, ruleName string, port int) SetupStep {
	const name = "Firewall rule"
	fix := fmt.Sprintf(windowsFirewallFix, port)
	rule, err := sanitizeFirewallRule(ruleName)
	if err != nil {
		return newStep(name, OutcomeFailed, fmt.Sprintf("Failed to configure firewall: validate firewall rule: %v", err), windowsFirewallNameFix)
	}
	if ok, _, _, err := windowsFirewallRuleOK(r, rule, port); err == nil && ok {
		step, _ := builtinGroupStep(r, newStep(name, OutcomeOK, fmt.Sprintf("Rule '%s' already active (port %d)", rule, port), ""))
		return step
	}
	// The scoped rule is created before the old ones go, and errors terminate,
	// so a failed create removes nothing and RDP is never left without a rule.
	err = runPS(r, fmt.Sprintf(`$ErrorActionPreference = 'Stop'; $old = @(Get-NetFirewallRule -DisplayName %q -ErrorAction SilentlyContinue); New-NetFirewallRule -DisplayName %q -Direction Inbound -Action Allow -Protocol TCP -LocalPort %d -Profile Any -RemoteAddress %s -Enabled True | Out-Null; $old | Remove-NetFirewallRule`, rule, rule, port, tailnetRemoteAddress))
	step := actStep(name, err, fmt.Sprintf("Rule '%s' active (port %d)", rule, port),
		fmt.Sprintf("Failed to configure firewall: %v", err), fix)
	if err != nil {
		return step
	}
	step, _ = builtinGroupStep(r, step)
	return step
}

// builtinGroupStep turns a converged firewall step manual while an enabled
// rule of the built-in "Remote Desktop" group allows any address: earlier
// setups enabled the group, and its rules void the tailnet scope. It does not
// disable the group (ADR-017 Amendment B); the operator runs the printed fix.
func builtinGroupStep(r Runner, step SetupStep) (SetupStep, bool) {
	out, err := outputPS(r, psReadRDGroup)
	if err != nil {
		return newStep(step.Name, OutcomeUnverified, "Could not read the built-in Remote Desktop rule group",
			"run Get-NetFirewallRule -Group \""+rdGroupID+"\" to see why the read fails"), false
	}
	kv := parseKeyValues(out)
	total, totalErr := strconv.Atoi(kv["total"])
	exposed, parseErr := strconv.Atoi(kv["exposed"])
	if totalErr != nil || parseErr != nil {
		// A count that does not parse proves nothing about the group, as a failed read does.
		return newStep(step.Name, OutcomeUnverified, fmt.Sprintf("The built-in Remote Desktop rule group read returned no rule count (%q)", strings.TrimSpace(string(out))),
			"run Get-NetFirewallRule -Group \""+rdGroupID+"\" | Where-Object Enabled -eq True to list the enabled rules"), false
	}
	if total == 0 {
		// Windows ships the group with rules, so an empty answer means the query missed it,
		// not that nothing is exposed.
		return newStep(step.Name, OutcomeUnverified, "The built-in Remote Desktop rule group query matched no rule, so its exposure is unknown",
			"run Get-NetFirewallRule -Group \""+rdGroupID+"\" and check the group exists on this install"), false
	}
	if exposed == 0 {
		return step, true
	}
	return newStep(step.Name, OutcomeManual, fmt.Sprintf("%d enabled rule(s) of the built-in Remote Desktop group allow inbound RDP from any address, which voids the tailnet scope", exposed),
		windowsGroupFix), false
}

// windowsFirewallCheckStep is the read half of windowsFirewallSetupStep.
func windowsFirewallCheckStep(r Runner, ruleName string, port int) (SetupStep, bool) {
	const name = "Firewall rule"
	rule, err := sanitizeFirewallRule(ruleName)
	if err != nil {
		return newStep(name, OutcomeFailed, fmt.Sprintf("Invalid firewall rule name: %v", err), windowsFirewallNameFix), false
	}
	ok, found, legacy, err := windowsFirewallRuleOK(r, rule, port)
	if err != nil {
		return newStep(name, OutcomeFailed, fmt.Sprintf("Could not read firewall rule '%s'", rule),
			fmt.Sprintf("run Get-NetFirewallRule -DisplayName %q to see why the read fails", rule)), false
	}
	if ok && port == 0 {
		// The RDP port read failed, so the rule's port went unchecked (#700).
		return withGroupNote(r, newStep(name, OutcomeUnverified,
			fmt.Sprintf("Rule '%s' is scoped to the tailnet, but its port is unchecked: the RDP port could not be read", rule),
			"fix the RDP configuration step, then run ts-bridge host check again")), false
	}
	if ok {
		return builtinGroupStep(r, firewallReadStep(name, true, port))
	}
	step := firewallReadStep(name, false, port)
	if legacy {
		step = newStep(name, OutcomeChanged, fmt.Sprintf("Rule '%s' is a legacy rule that allows TCP %d from any address; setup scopes it to the tailnet", rule, port),
			"run ts-bridge host setup as an administrator")
	} else if found {
		step = newStep(name, OutcomeChanged, fmt.Sprintf("Rule '%s' differs from what setup creates (inbound TCP %d, tailnet addresses only, every profile, enabled, one rule)", rule, port),
			"run ts-bridge host setup as an administrator")
	}
	return withGroupNote(r, step), false
}

// withGroupNote adds the built-in group exposure to a step that already
// fails on the named rule, so the first check on a pre-#631 host reports
// both. A failed group read adds nothing: the step is not ok either way.
func withGroupNote(r Runner, step SetupStep) SetupStep {
	group, ok := builtinGroupStep(r, step)
	if ok || group.Outcome != OutcomeManual {
		return step
	}
	step.Message += "; " + group.Message
	step.Fix += "; then " + group.Fix
	return step
}
