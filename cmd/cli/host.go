// Package cmd provides the Cobra CLI for ts-bridge.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"ts-bridge/internal/config/envfile"
	"ts-bridge/internal/host"
	"ts-bridge/internal/profile"
)

// newHostCmd constructs the "ts-bridge host" command tree.
func newHostCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host",
		Short: "Host setup and verification commands",
		Long: `Manage host machine configuration for RDP access over Tailscale.

The host commands read the ADR-017 host file named by --config, which host init
writes.

Examples:
  ts-bridge host init --config /etc/ts-bridge/host.yaml
  ts-bridge host setup --config /etc/ts-bridge/host.yaml
  ts-bridge host check --config /etc/ts-bridge/host.yaml --json
  ts-bridge host serve --config /etc/ts-bridge/host.yaml
`,
	}
	cmd.AddCommand(newHostSetupCmd(), newHostCheckCmd(), newHostInitCmd(), newHostServeCmd())
	return cmd
}

func newHostSetupCmd() *cobra.Command { //nolint:funlen // baseline #629
	cmd := &cobra.Command{
		Use:   "setup [flags]",
		Short: "Configure the host for RDP access over Tailscale",
		Long: `Configure the host machine for RDP access over the Tailscale mesh network.

On Windows:
  - Enables RDP via registry; a declared port the registry does not listen
    on is reported for you to change (ts-bridge does not move the listener)
  - Enables Tailscale unattended mode
  - Configures Windows firewall for RDP on Tailscale interface
  - Enables UPnP services
  - Sets Tailscale network profile to Private
  - Optionally disables sleep mode

On Linux:
  - Enables and starts xrdp (systemctl enable --now) when it is not already
  - Points xrdp at a declared non-default RDP port (port= in [Globals] of
    /etc/xrdp/xrdp.ini) and restarts it, when it listens elsewhere
  - Allows the RDP port inbound with ufw (or iptables) when no rule allows it yet
  - Prints Tailscale IP

On macOS:
  - Not applicable — use ts-bridge connect to connect to a remote host.

Declared state:
  --config names a YAML host configuration file (version, host.services,
  host.firewall.rule, host.power.prevent_sleep, host.control_url). Ports,
  the firewall rule and sleep resolve flags > env (TS_HOST_*, including
  ./.env) > file > per-OS defaults; host.control_url is read from the file
  only. An invalid file, or an entry this OS has no step for, exits 1
  before any step runs.

Exit status:
  0  every step is ok, changed or manual (a manual step prints what is left to you)
  2  setup ran and at least one step failed
  1  setup could not run (for example, without admin privileges)
Run ts-bridge host check afterwards to confirm the host converged.

Examples:
  ts-bridge host setup
  ts-bridge host setup --config /etc/ts-bridge/host.yaml
  ts-bridge host setup --no-sleep
  ts-bridge host setup --firewall-rule "MyRDPRule"
`,
		RunE: runHostSetup,
	}
	cmd.Flags().String("config", "", "Host configuration file (YAML) declaring the desired state")
	cmd.Flags().Bool("no-sleep", false, "Skip disabling sleep mode")
	cmd.Flags().String("firewall-rule", "", `Custom firewall rule name (default "Tailscale-RDP-Ingress")`)
	cmd.Flags().Int("port", 0, "RDP port (default: 3389)")
	cmd.Flags().Bool("json", false, "Output in JSON format")
	cmd.Flags().Bool("verbose", false, "Enable verbose (debug) logging")
	cmd.Flags().String("log-format", "", "Log format (text|json)")
	return cmd
}

func newHostCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [flags]",
		Short: "Verify host readiness (read-only)",
		Long: `Verify that the host is ready for RDP access over Tailscale.

Read-only — no administrative changes are made.

Outputs:
  - Tailscale IP address
  - RDP listening port
  - Firewall rule status

Use --json for machine-readable output.

--config names the same YAML host configuration file as host setup, so the
check diffs the declared state (flags > env > file > per-OS defaults).

Exit status:
  0  every step is ok
  1  the check could not run
  2  the check ran and at least one step is not ok (its fix is printed)

Examples:
  ts-bridge host check
  ts-bridge host check --config /etc/ts-bridge/host.yaml
  ts-bridge host check --json
`,
		RunE: runHostCheck,
	}
	cmd.Flags().String("config", "", "Host configuration file (YAML) declaring the desired state")
	cmd.Flags().Bool("json", false, "Output in JSON format")
	cmd.Flags().Bool("verbose", false, "Enable verbose (debug) logging")
	cmd.Flags().String("log-format", "", "Log format (text|json)")
	return cmd
}

// ─── Setup ───────────────────────────────────────────────────────

func runHostSetup(cmd *cobra.Command, args []string) error { //nolint:funlen,gocyclo // baseline #629
	// The file is validated on every OS, so an invalid --config is an error
	// even where setup has no steps.
	file, err := loadHostFile(cmd)
	if err != nil {
		return err
	}

	// macOS: not applicable.
	if hostGOOS == "darwin" {
		return printSetupNotApplicable(cmd)
	}

	// Admin check.
	if !hostIsElevated() {
		printElevationError(cmd.ErrOrStderr())
		return fmt.Errorf("host setup requires elevated privileges")
	}

	cfg, err := resolveHostSetupConfig(cmd, file)
	if err != nil {
		return err
	}

	result, err := runHostSetupSteps(cfg)
	if err != nil {
		return err
	}

	return printSetupResult(cmd, result)
}

// runHostSetupSteps applies the resolved configuration, logging through a logger
// built from it so --log-format / --verbose (and their TS_HOST_* env
// equivalents) actually take effect.
func runHostSetupSteps(cfg host.Config) (host.SetupResult, error) {
	logger := hostInitLogger(cfg)

	logger.Info("starting host setup",
		"firewall_rule", cfg.FirewallRule,
		"rdp_port", cfg.Port,
		"no_sleep", cfg.NoSleep,
	)

	result, err := hostSetup(cfg, logger)
	if err != nil {
		logger.Error("host setup failed", "error", err)
		return host.SetupResult{}, fmt.Errorf("host setup failed: %w", err)
	}
	return result, nil
}

// resolveHostSetupConfig merges the .env file, the CLI flags, the host file and
// the per-OS defaults into the setup configuration, and rejects an invalid
// firewall rule before host setup touches the machine. Loading the .env file is
// the one effect that precedes the check: it sets the process environment.
func resolveHostSetupConfig(cmd *cobra.Command, file host.File) (host.Config, error) {
	// Auto-load .env from CWD so the TS_HOST_* values written by
	// "ts-bridge host init" are honored here, mirroring "ts-bridge connect".
	// Explicit env vars and CLI flags still win over .env (merge precedence).
	if err := envfile.Load(".env"); err != nil {
		return host.Config{}, fmt.Errorf("load .env: %w", err)
	}

	// Collect CLI flags. The *Set fields record whether each bool flag was
	// explicitly passed, so an unset flag does not clobber the env layer.
	flags := host.Flags{
		NoSleep:      mustBool(cmd, "no-sleep"),
		NoSleepSet:   cmd.Flags().Changed("no-sleep"),
		FirewallRule: mustString(cmd, "firewall-rule"),
		Port:         mustInt(cmd, "port"),
		Verbose:      mustBool(cmd, "verbose"),
		VerboseSet:   cmd.Flags().Changed("verbose"),
		LogFormat:    mustString(cmd, "log-format"),
	}

	cfg := host.Resolve(hostGOOS, file, flags)

	// Validate firewall rule early (before any side effects).
	if _, err := host.SanitizeFirewallRule(cfg.FirewallRule); err != nil {
		return host.Config{}, fmt.Errorf("invalid firewall rule: %w", err)
	}
	return cfg, nil
}

// printSetupResult prints the setup outcome, as JSON with --json or step by
// step, and returns the convergence verdict (the exit status).
func printSetupResult(cmd *cobra.Command, result host.SetupResult) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		if err := printSetupJSON(cmd.OutOrStdout(), hostTailscaleIP(), result); err != nil {
			return err
		}
		return setupConvergence(result.Steps)
	}

	// Print step-by-step results.
	for i, step := range result.Steps {
		printStep(cmd.OutOrStdout(), i+1, len(result.Steps), step.Name)
		if step.Success {
			printOk(cmd.OutOrStdout(), step.Message)
		} else {
			printWarn(cmd.ErrOrStderr(), step.Message)
		}
	}

	// Report the port that was actually configured (result.RDPPort), which on
	// Windows comes from the registry and may differ from the requested --port.
	printSetupSummary(cmd.OutOrStdout(), hostTailscaleIP(), result)
	return setupConvergence(result.Steps)
}

// printSetupNotApplicable answers "host setup" on macOS. With --json it still
// prints the JSON document (no steps, the reason in not_applicable), so a
// script decoding stdout can tell "not applicable" from "converged".
func printSetupNotApplicable(cmd *cobra.Command) error {
	const reason = "Host setup is not applicable on macOS."
	const hint = "Use the client mode (ts-bridge connect) to connect to a remote host."
	out := cmd.OutOrStdout()
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		data, err := json.MarshalIndent(setupJSONOutput{
			Steps:         []setupStepJSON{},
			Warnings:      []string{},
			Errors:        []string{},
			NotApplicable: reason + " " + hint,
		}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal setup result: %w", err)
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	fmt.Fprintln(out, reason)
	fmt.Fprintln(out, hint)
	return nil
}

// loadHostFile loads and validates the --config host file for hostGOOS,
// warning on the command's stderr. No --config is no file.
func loadHostFile(cmd *cobra.Command) (host.File, error) {
	path := mustString(cmd, "config")
	if path == "" {
		return host.File{}, nil
	}
	return host.LoadFile(path, hostGOOS, cmd.ErrOrStderr())
}

// setupConvergence returns an ExitError with exitNotConverged when any step
// failed (ADR-017 §2). Unlike check, changed and manual exit 0: setup did its
// part, and printed the action a manual step leaves to the user.
func setupConvergence(steps []host.SetupStep) error {
	failed := failedSteps(steps)
	if len(failed) == 0 {
		return nil
	}
	return &ExitError{Code: exitNotConverged, Err: fmt.Errorf("host setup did not converge: %d of %d steps failed: %s",
		len(failed), len(steps), strings.Join(failed, ", "))}
}

// failedSteps names the steps that failed, in order.
func failedSteps(steps []host.SetupStep) []string {
	var failed []string
	for _, s := range steps {
		if s.Outcome == host.OutcomeFailed {
			failed = append(failed, s.Name)
		}
	}
	return failed
}

// ─── Check ───────────────────────────────────────────────────────

func runHostCheck(cmd *cobra.Command, args []string) error {
	// Auto-load .env so the check reflects the configured firewall rule / port
	// written by "ts-bridge host init", mirroring "ts-bridge connect".
	if err := envfile.Load(".env"); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}
	file, err := loadHostFile(cmd)
	if err != nil {
		return err
	}

	// check has no firewall-rule/port flags; those resolve from env, file and defaults.
	cfg := host.Resolve(hostGOOS, file, host.Flags{
		Verbose:    mustBool(cmd, "verbose"),
		VerboseSet: cmd.Flags().Changed("verbose"),
		LogFormat:  mustString(cmd, "log-format"),
	})
	logger := hostInitLogger(cfg)
	logger.Debug("resolved host config (flags > env > file > defaults)",
		"rdp_port", cfg.Port, "firewall_rule", cfg.FirewallRule, "no_sleep", cfg.NoSleep,
		"control_url", cfg.ControlURL, "services", cfg.Services)

	jsonOut, _ := cmd.Flags().GetBool("json")

	result, err := hostCheck(cfg, logger)
	if err != nil {
		logger.Error("host check failed", "error", err)
		return fmt.Errorf("host check failed: %w", err)
	}
	// Only --json prints published, so only it spends the probe's timeout.
	if jsonOut {
		result.Published = probeServePublished(file, cfg.Services)
	}

	return reportCheck(cmd.OutOrStdout(), result, jsonOut)
}

// reportCheck prints the check result to w and returns its exit status as an
// error.
func reportCheck(w io.Writer, result host.CheckResult, jsonOut bool) error {
	if jsonOut {
		if err := printCheckJSON(w, result); err != nil {
			return err
		}
	} else {
		printCheckText(w, result)
	}
	return checkConvergence(result.Steps)
}

// hostCheck, hostSetup, hostIsElevated, hostTailscaleIP and hostGOOS are what the host commands
// run on; tests replace them to drive the commands end to end without the
// machine's real state. hostGOOS steers only the command layer: host.Setup and
// host.Check read runtime.GOOS themselves, so a test that changes hostGOOS
// must also replace hostSetup or hostCheck.
var (
	hostGOOS       = runtime.GOOS
	hostCheck      = host.Check
	hostSetup      = host.Setup
	hostIsElevated = host.IsElevated
	// hostTailscaleIP is the IP setup's descriptor names, in text and JSON.
	hostTailscaleIP = host.TailscaleIP
)

// exitNotConverged is the exit status of a host command that ran but did not
// find (check) or reach (setup) the declared state, as `terraform plan
// -detailed-exitcode` uses 2 for "changes present".
const exitNotConverged = 2

// checkConvergence returns an ExitError with exitNotConverged unless every
// step is ok (ADR-017 §2). In check, changed means setup would change it.
func checkConvergence(steps []host.SetupStep) error {
	var pending []string
	for _, s := range steps {
		if s.Outcome != host.OutcomeOK {
			pending = append(pending, fmt.Sprintf("%s (%s)", s.Name, s.Outcome))
		}
	}
	if len(pending) == 0 {
		return nil
	}
	return &ExitError{Code: exitNotConverged, Err: fmt.Errorf("host not converged: %d of %d steps not ok: %s",
		len(pending), len(steps), strings.Join(pending, ", "))}
}

// ─── Logger initialization ──────────────────────────────────────

// hostInitLogger creates a lightweight structured logger for host commands
// from the resolved config. Host commands run independently (not after
// connect), so they need their own logger. File logging is not used for
// setup/check — only console. The output format honors cfg.LogFormat, and the
// level honors cfg.Verbose (note: the separate --json flag controls the
// command's result output, not the logger).
//
// Logs go to stderr, not stdout: the --json result is machine-readable data and
// must be the only thing on stdout, or `host check --json | jq .` fails on the
// interleaved log line. Routing rather than silencing keeps --verbose useful
// alongside --json.
func hostInitLogger(cfg host.Config) *slog.Logger {
	return newHostLogger(cfg, os.Stderr)
}

// newHostLogger builds the host command logger over an arbitrary writer. Split
// out from hostInitLogger so tests can assert format and level against a buffer
// without swapping process streams.
func newHostLogger(cfg host.Config, w io.Writer) *slog.Logger {
	level := slog.LevelInfo
	if cfg.Verbose {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	return slog.New(handler)
}

// ─── Print helpers ───────────────────────────────────────────────

func printStep(w io.Writer, num, total int, msg string) {
	fmt.Fprintf(w, "  [%d/%d] %s\n", num, total, msg)
}

func printOk(w io.Writer, msg string) {
	fmt.Fprintf(w, "       %s\n", msg)
}

func printWarn(w io.Writer, msg string) {
	fmt.Fprintf(w, "       WARNING: %s\n", msg)
}

// mustBool, mustString, mustInt are helper getters that ignore errors
// since Cobra always succeeds for registered flags.
func mustBool(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)
	return v
}

func mustString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func mustInt(cmd *cobra.Command, name string) int {
	v, _ := cmd.Flags().GetInt(name)
	return v
}

// ─── Output helpers ──────────────────────────────────────────────

func printElevationError(w io.Writer) {
	switch runtime.GOOS {
	case windowsOS:
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "ERROR: This operation requires Administrator privileges.")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "To run as Administrator:")
		fmt.Fprintln(w, "  1. Open PowerShell as Administrator (right-click → Run as Administrator)")
		fmt.Fprintln(w, "  2. Re-run: ts-bridge host setup")
		fmt.Fprintln(w, "")
	case "linux":
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "ERROR: This operation requires root privileges.")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "To run as root:")
		fmt.Fprintln(w, "  sudo ts-bridge host setup")
		fmt.Fprintln(w, "")
	}
}

func printSetupSummary(w io.Writer, tsIP string, result host.SetupResult) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  ---------------------------------------")
	if failed := failedSteps(result.Steps); len(failed) > 0 {
		fmt.Fprintln(w, "  HOST NOT READY")
		fmt.Fprintf(w, "  Failed steps: %s\n", strings.Join(failed, ", "))
	} else {
		fmt.Fprintln(w, "  HOST READY")
	}
	fmt.Fprintln(w, "  ---------------------------------------")

	if tsIP != "" {
		fmt.Fprintf(w, "  Tailscale IP: %s\n", tsIP)
	}
	fmt.Fprintf(w, "  RDP Port:     %d\n", result.RDPPort)
	fmt.Fprintln(w)
	printClientTarget(w, tsIP, result)
	fmt.Fprintln(w)
}

// printClientTarget prints what a client needs to reach the host. Without a
// Tailscale IP there is no address to name, so it says when one appears.
func printClientTarget(w io.Writer, tsIP string, result host.SetupResult) {
	if tsIP == "" {
		fmt.Fprintln(w, "  The client target appears once Tailscale is up.")
		return
	}
	fmt.Fprintln(w, "  For a client (ts-bridge connect, .env):")
	fmt.Fprintf(w, "  TS_TARGET=%s:%d\n", tsIP, result.RDPPort)
	if desc := hostDescriptorLine(tsIP, result.RDPPort, result.ControlURL); desc != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Shareable descriptor:")
		fmt.Fprintf(w, "  %s\n", desc)
		fmt.Fprintf(w, "  (import with: ts-bridge import <name> %q)\n", desc)
	} else if result.RDPPort != 0 {
		fmt.Fprintf(w, "\n  %s\n", descriptorOmittedNote)
	}
}

// buildDescriptorLine returns a tsb:// descriptor string for the given host IP,
// port, and control URL, or empty string if either ip or port is missing.
// controlURL="" produces cp=saas (Tailscale SaaS).
func buildDescriptorLine(ip string, port int, controlURL string) string {
	if ip == "" || port == 0 {
		return ""
	}
	d := profile.Descriptor{Host: ip, Port: port, ControlURL: controlURL, Version: 1}
	return d.String()
}

// descriptorOmittedNote is the one line host setup and check print in place of
// a descriptor whose control plane is neither declared nor detected.
const descriptorOmittedNote = "Descriptor omitted: control plane unknown; declare host.control_url"

// hostDescriptorLine is buildDescriptorLine for a host result: an unknown
// control plane ("") yields no descriptor instead of a SaaS one (#453).
func hostDescriptorLine(ip string, port int, controlURL string) string {
	switch controlURL {
	case "":
		return ""
	case host.ControlPlaneSaaS:
		controlURL = ""
	}
	return buildDescriptorLine(ip, port, controlURL)
}

// setupJSONOutput is the JSON structure for host setup --json output.
type setupJSONOutput struct {
	Steps    []setupStepJSON `json:"steps"`
	RDPPort  int             `json:"rdp_port"`
	Warnings []string        `json:"warnings"`
	Errors   []string        `json:"errors"`
	// Descriptor is the tsb:// line of the text summary; absent without an IP.
	Descriptor string `json:"descriptor,omitempty"`
	// NotApplicable is why setup ran no steps; absent where setup applies.
	NotApplicable string `json:"not_applicable,omitempty"`
}

type setupStepJSON struct {
	Name    string `json:"name"`
	Outcome string `json:"outcome"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Fix     string `json:"fix"`
}

// serviceJSON is one entry of check's services[] (ADR-017 §3). Active is null
// until a step reads the service state.
type serviceJSON struct {
	Kind       string `json:"kind"`
	Port       int    `json:"port"`
	Enabled    bool   `json:"enabled"`
	Active     *bool  `json:"active"`
	FirewallOK bool   `json:"firewall_ok"`
	// Unread is true when a step could not read the service: enabled and
	// firewall_ok keep their zero value and active stays null, none of them a
	// reading of "off". The steps name the cause.
	Unread bool `json:"unread"`
	// Published is whether host serve publishes it; absent without health_addr.
	Published *bool `json:"published,omitempty"`
}

func stepsJSON(steps []host.SetupStep) []setupStepJSON {
	out := make([]setupStepJSON, 0, len(steps))
	for _, s := range steps {
		out = append(out, setupStepJSON{Name: s.Name, Outcome: string(s.Outcome), Success: s.Success, Message: s.Message, Fix: s.Fix})
	}
	return out
}

func setupJSON(tsIP string, result host.SetupResult) ([]byte, error) {
	output := setupJSONOutput{
		RDPPort:    result.RDPPort,
		Steps:      stepsJSON(result.Steps),
		Warnings:   []string{},
		Errors:     []string{},
		Descriptor: hostDescriptorLine(tsIP, result.RDPPort, result.ControlURL),
	}
	for _, step := range result.Steps {
		if !step.Success {
			output.Warnings = append(output.Warnings, step.Message)
		}
	}
	return json.MarshalIndent(output, "", "  ")
}

func printSetupJSON(w io.Writer, tsIP string, result host.SetupResult) error {
	data, err := setupJSON(tsIP, result)
	if err != nil {
		return fmt.Errorf("marshal setup result: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
}

func printCheckText(w io.Writer, r host.CheckResult) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  HOST CHECK")
	fmt.Fprintln(w, "  ---------------------------------------")
	fmt.Fprintf(w, "  Platform:      %s\n", runtime.GOOS)
	fmt.Fprintf(w, "  Tailscale:     %s\n", statusStr(r.TailscaleUp))
	fmt.Fprintf(w, "  Tailscale IP:  %s\n", r.TailscaleIP)
	fmt.Fprintf(w, "  RDP:           %s\n", rdpLine(r))
	fmt.Fprintf(w, "  Firewall:      %s\n", statusStr(r.FirewallOK))
	for _, step := range r.Steps {
		if step.Outcome != host.OutcomeOK {
			fmt.Fprintf(w, "  ! %s (%s): %s\n", step.Name, step.Outcome, step.Fix)
		}
	}
	if desc := hostDescriptorLine(r.TailscaleIP, r.RDPPort, r.ControlURL); desc != "" {
		fmt.Fprintf(w, "  Descriptor:    %s\n", desc)
	} else if r.TailscaleIP != "" && r.RDPPort != 0 {
		fmt.Fprintf(w, "  %s\n", descriptorOmittedNote)
	}
	fmt.Fprintln(w, "  ---------------------------------------")
	fmt.Fprintln(w)
}

// rdpLine is the RDP line of the check. The legacy RDPPort is set only by an
// enabled service, so a declared service that is off or unread is named from
// Services rather than reported as undeclared. It claims "unread" only when a
// step said so, and blames elevation only when the process lacks it: a service
// the check read and found off is NOT OK.
func rdpLine(r host.CheckResult) string {
	if r.RDPPort > 0 {
		return fmt.Sprintf("%s (port %d)", statusStr(r.RDPEnabled), r.RDPPort)
	}
	i := slices.IndexFunc(r.Services, func(s host.Service) bool { return s.Kind == host.ServiceRDP })
	if i < 0 {
		return "no service declared"
	}
	port := "unknown"
	if p := r.Services[i].Port; p > 0 {
		port = strconv.Itoa(p)
	}
	if svc := r.Services[i]; svc.Unread {
		// The cause can be a missing systemctl or a failed port read, which
		// elevation does not fix; it is only offered as one possibility.
		line := fmt.Sprintf("unread (rdp, port %s): see the steps below for the cause", port)
		if !r.Elevated {
			line += "; running as root/administrator may help"
		}
		return line
	}
	return fmt.Sprintf("%s (rdp, port %s)", statusStr(false), port)
}

func checkJSON(r host.CheckResult) ([]byte, error) {
	type jsonOutput struct {
		TailscaleIP string          `json:"tailscale_ip"`
		RDPPort     int             `json:"rdp_port"`
		RDPEnabled  bool            `json:"rdp_enabled"`
		FirewallOK  bool            `json:"firewall_ok"`
		TailscaleUp bool            `json:"tailscale_up"`
		Platform    string          `json:"platform"`
		Descriptor  string          `json:"descriptor,omitempty"`
		Services    []serviceJSON   `json:"services"`
		Steps       []setupStepJSON `json:"steps"`
	}

	out := jsonOutput{
		TailscaleIP: r.TailscaleIP,
		RDPPort:     r.RDPPort,
		RDPEnabled:  r.RDPEnabled,
		FirewallOK:  r.FirewallOK,
		TailscaleUp: r.TailscaleUp,
		Platform:    runtime.GOOS,
		Descriptor:  hostDescriptorLine(r.TailscaleIP, r.RDPPort, r.ControlURL),
		Services:    make([]serviceJSON, 0, len(r.Services)),
		Steps:       stepsJSON(r.Steps),
	}
	for _, svc := range r.Services {
		sj := serviceJSON{Kind: svc.Kind, Port: svc.Port, Enabled: svc.Enabled, Active: svc.Active, FirewallOK: svc.FirewallOK, Unread: svc.Unread}
		if r.Published != nil {
			p := r.Published[serviceKey(svc.Kind, svc.Port)]
			sj.Published = &p
		}
		out.Services = append(out.Services, sj)
	}
	return json.MarshalIndent(out, "", "  ")
}

func printCheckJSON(w io.Writer, r host.CheckResult) error {
	data, err := checkJSON(r)
	if err != nil {
		return fmt.Errorf("marshal check result: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
}

func statusStr(ok bool) string {
	if ok {
		return "✓ OK"
	}
	return "✗ NOT OK"
}
