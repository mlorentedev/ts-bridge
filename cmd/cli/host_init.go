// Package cmd provides the Cobra CLI for ts-bridge.
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"ts-bridge/internal/host"
)

// newHostInitCmd constructs the "ts-bridge host init" subcommand.
func newHostInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [flags]",
		Short: "Interactive wizard to configure the host for RDP access",
		Long: `Run ts-bridge host init to configure the host machine for RDP access
over the Tailscale mesh network.

Interactive mode (no flags): prompts for RDP port, firewall rule name, and
sleep settings. Writes TS_HOST_* variables into the existing .env file (or
creates one if it doesn't exist).

Non-interactive mode (with flags): writes configuration silently, useful for
automation or CI.

Examples:
  # Interactive wizard
  ts-bridge host init

  # Non-interactive: .env output (default)
  ts-bridge host init --port 3389 --firewall-rule "MyRule" --no-sleep

  # Custom output path
  ts-bridge host init --port 3390 --config /etc/ts-bridge/.env
`,
		RunE: runHostInit,
	}

	cmd.Flags().Int("port", 0, "RDP port (default: 3389)")
	cmd.Flags().String("firewall-rule", "", "Firewall rule name (default: Tailscale-RDP-Ingress)")
	cmd.Flags().Bool("no-sleep", false, "Disable sleep mode")
	cmd.Flags().String("config", "", "Output .env file path (default: ./.env in CWD)")
	return cmd
}

// hostInitFlags holds values parsed from CLI flags for the host init command.
type hostInitFlags struct {
	Port         int
	FirewallRule string
	NoSleep      bool
	Config       string
}

// runHostInit is the entry point for "ts-bridge host init".
func runHostInit(cmd *cobra.Command, args []string) error {
	// Parse flags.
	f, err := parseHostInitFlags(cmd)
	if err != nil {
		return err
	}

	// Determine mode.
	isInteractive := !cmd.Flags().Changed("port") &&
		!cmd.Flags().Changed("firewall-rule") &&
		!cmd.Flags().Changed("no-sleep")

	if isInteractive {
		return runHostInitInteractive(cmd, f)
	}
	return runHostInitNonInteractive(cmd, f)
}

// parseHostInitFlags reads all host-init-specific flags into a struct.
func parseHostInitFlags(cmd *cobra.Command) (hostInitFlags, error) {
	var f hostInitFlags

	f.Port, _ = cmd.Flags().GetInt("port")
	f.FirewallRule, _ = cmd.Flags().GetString("firewall-rule")
	f.NoSleep, _ = cmd.Flags().GetBool("no-sleep")
	f.Config, _ = cmd.Flags().GetString("config")

	// Validate port if provided (non-zero means explicitly set).
	if f.Port != 0 && (f.Port < 1 || f.Port > 65535) {
		return f, fmt.Errorf("port must be between 1 and 65535, got %d", f.Port)
	}

	// Validate firewall rule if provided.
	if f.FirewallRule != "" {
		if _, err := host.SanitizeFirewallRule(f.FirewallRule); err != nil {
			return f, fmt.Errorf("firewall rule: %w", err)
		}
	}

	return f, nil
}

// runHostInitInteractive prompts the user for all required values.
func runHostInitInteractive(cmd *cobra.Command, f hostInitFlags) error {
	in := cmd.InOrStdin()
	reader := bufio.NewReader(in)

	// 1. RDP port.
	if !cmd.Flags().Changed("port") {
		portStr, err := readOptionalInput(reader, fmt.Sprintf("RDP port (default: %d): ", host.DefaultRDPPort()))
		if err != nil {
			return fmt.Errorf("read RDP port: %w", err)
		}
		if portStr != "" {
			port, err := strconv.Atoi(portStr)
			if err != nil || port < 1 || port > 65535 {
				return fmt.Errorf("invalid RDP port %q (must be 1-65535)", portStr)
			}
			f.Port = port
		} else {
			f.Port = host.DefaultRDPPort()
		}
	}

	// 2. Firewall rule name.
	if !cmd.Flags().Changed("firewall-rule") {
		rule, err := readOptionalInput(reader, fmt.Sprintf("Firewall rule name (default: %s): ", host.DefaultFirewallRule()))
		if err != nil {
			return fmt.Errorf("read firewall rule: %w", err)
		}
		if rule != "" {
			if _, err := host.SanitizeFirewallRule(rule); err != nil {
				return fmt.Errorf("firewall rule: %w", err)
			}
			f.FirewallRule = rule
		} else {
			f.FirewallRule = host.DefaultFirewallRule()
		}
	}

	// 3. Disable sleep.
	if !cmd.Flags().Changed("no-sleep") {
		sleepChoice, err := readChoiceInput(reader,
			"Disable sleep mode? (default: yes) [yes/no]: ",
			[]string{"yes", "no"})
		if err != nil {
			return fmt.Errorf("read sleep setting: %w", err)
		}
		f.NoSleep = sleepChoice == "no"
	}

	// Resolve config path.
	if f.Config == "" {
		f.Config = defaultConfigPath(formatENV)
	}

	// Write .env.
	if err := writeHostEnv(f); err != nil {
		return err
	}

	printHostInitSummary(f)
	return nil
}

// runHostInitNonInteractive writes configuration silently from flags.
func runHostInitNonInteractive(_ *cobra.Command, f hostInitFlags) error {
	// Apply defaults for unset values.
	if f.Port < 1 {
		f.Port = host.DefaultRDPPort()
	}
	if f.FirewallRule == "" {
		f.FirewallRule = host.DefaultFirewallRule()
	}

	// Resolve config path.
	if f.Config == "" {
		f.Config = defaultConfigPath(formatENV)
	}

	// Write .env.
	if err := writeHostEnv(f); err != nil {
		return err
	}

	printHostInitSummary(f)
	return nil
}

// writeHostEnv writes TS_HOST_* variables into the .env file.
// It preserves existing content and adds/updates only the host variables.
func writeHostEnv(f hostInitFlags) error {
	envPath := f.Config

	// Read existing .env if it exists, preserving its (trimmed) lines so the file
	// can be rewritten with the host section replaced rather than duplicated.
	existingLines := []string{}
	// #nosec G304 -- envPath is from the --config flag (user-controlled), the user's own .env file.
	if data, err := os.ReadFile(envPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			existingLines = append(existingLines, strings.TrimSpace(line))
		}
	}

	// Build new content: existing content + host vars at the end.
	var sb strings.Builder

	// Write existing non-host lines first (the previous host section is stripped
	// so re-running `host init` replaces it instead of appending a duplicate).
	for _, line := range stripHostSection(existingLines) {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Add a blank-line separator + header only if the file had real (non-blank)
	// content, so an empty .env gets no spurious header but an existing file
	// (including one that already held a host section) keeps exactly one.
	if hasNonBlank(existingLines) {
		sb.WriteString("\n")
		sb.WriteString("# ── Host configuration (ts-bridge host init) ──────────────────────\n")
		sb.WriteString("#\n")
	}

	// Write host vars.
	sb.WriteString(fmt.Sprintf("TS_HOST_RDP_PORT=%d\n", f.Port))
	sb.WriteString(fmt.Sprintf("TS_HOST_FIREWALL_RULE=%s\n", f.FirewallRule))
	if f.NoSleep {
		sb.WriteString("TS_HOST_NO_SLEEP=true\n")
	} else {
		sb.WriteString("# TS_HOST_NO_SLEEP=false\n")
	}

	// Write with secure permissions.
	if err := os.WriteFile(envPath, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}

	// Check permissions (Unix only).
	if runtime.GOOS != windowsOS {
		info, err := os.Stat(envPath)
		if err == nil && info.Mode().Perm()&0077 != 0 {
			fmt.Fprintf(os.Stderr, "WARNING: %s has loose permissions (%04o); consider chmod 600 %s\n",
				envPath, info.Mode().Perm(), envPath)
		}
	}

	return nil
}

// stripHostSection removes a previously-written host configuration block — the
// TS_HOST_* assignments and their comment header — from the existing .env lines,
// so a re-run of `host init` replaces the section instead of duplicating it.
func stripHostSection(lines []string) []string {
	out := make([]string, 0, len(lines))
	inHostSection := false
	for _, line := range lines {
		if strings.HasPrefix(line, "TS_HOST_") ||
			strings.Contains(line, "Host configuration (ts-bridge host init)") {
			inHostSection = true
			continue
		}
		// Within an old host section, skip its trailing blank/comment lines.
		if inHostSection && (line == "" || strings.HasPrefix(line, "#")) {
			continue
		}
		inHostSection = false
		out = append(out, line)
	}
	return out
}

// hasNonBlank reports whether any line has non-whitespace content.
func hasNonBlank(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

// printHostInitSummary displays the configuration written.
func printHostInitSummary(f hostInitFlags) {
	fmt.Println()
	fmt.Println("  ---------------------------------------")
	fmt.Println("  HOST CONFIGURED")
	fmt.Println("  ---------------------------------------")
	fmt.Printf("  RDP Port:       %d\n", f.Port)
	fmt.Printf("  Firewall Rule:  %s\n", f.FirewallRule)
	fmt.Printf("  Disable Sleep:  %v\n", !f.NoSleep)
	fmt.Println()

	absPath, _ := filepath.Abs(f.Config)
	fmt.Printf("  Config: %s\n", absPath)
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("  1. Run: ts-bridge host setup")
	fmt.Println("  2. Connect from client: ts-bridge connect")
	fmt.Println()
}
