// Package cmd provides the Cobra CLI for ts-bridge.
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"ts-bridge/internal/host"
)

// hostFileMode lets host check, which runs unelevated, read the file setup
// and serve read elevated. The file holds no secret.
const hostFileMode = 0o644

// newHostInitCmd constructs the "ts-bridge host init" subcommand.
func newHostInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init --config <path> [flags]",
		Short: "Write the host configuration file that host setup and check read",
		Long: `Write the host's declared state (ADR-017) to the YAML file named by
--config, which host setup, host check and host serve then read with the same
--config. No .env is written.

Interactive mode (no value flags): prompts for the RDP port and, on Windows, the
firewall rule name and sleep setting. Non-interactive mode (any of --port,
--firewall-rule, --no-sleep): writes the file from the flags and defaults.

An existing file is replaced only after confirmation, or with --force. A path
that is a directory, or a file that cannot be read, is refused even with
--force. On macOS host init is not applicable yet (#459) and writes nothing.

Choose a path whose parent directories an unelevated user can traverse, since
host check runs unelevated: /etc/ts-bridge/host.yaml on Linux,
C:\ProgramData\ts-bridge\host.yaml on Windows.

Examples:
  # Interactive wizard
  ts-bridge host init --config /etc/ts-bridge/host.yaml

  # Non-interactive
  ts-bridge host init --config C:\ProgramData\ts-bridge\host.yaml --port 3390 --firewall-rule "MyRule" --no-sleep
`,
		RunE: runHostInit,
	}

	cmd.Flags().String("config", "", "Host configuration file (YAML) to write (required)")
	cmd.Flags().Int("port", 0, "RDP port (default: 3389)")
	cmd.Flags().String("firewall-rule", "", "Firewall rule name, Windows only (default: Tailscale-RDP-Ingress)")
	cmd.Flags().Bool("no-sleep", false, "Leave sleep settings alone, Windows only")
	cmd.Flags().Bool("force", false, "Replace an existing file without asking")
	return cmd
}

// hostInitFlags holds values parsed from CLI flags for the host init command.
type hostInitFlags struct {
	Port         int
	FirewallRule string
	NoSleep      bool
	Config       string
	Force        bool
}

// runHostInit is the entry point for "ts-bridge host init".
func runHostInit(cmd *cobra.Command, _ []string) error {
	f, err := parseHostInitFlags(cmd)
	if err != nil {
		return err
	}
	if hostGOOS == "darwin" {
		fmt.Fprintln(cmd.OutOrStdout(), "Host init is not applicable on macOS yet (#459); nothing was written.")
		return nil
	}
	interactive := !cmd.Flags().Changed("port") &&
		!cmd.Flags().Changed("firewall-rule") &&
		!cmd.Flags().Changed("no-sleep")
	reader := bufio.NewReader(cmd.InOrStdin())
	if err := ensureHostFileWritable(f.Config, f.Force, interactive, reader, cmd.OutOrStdout()); err != nil {
		return err
	}
	if interactive {
		if err := promptHostInit(reader, &f); err != nil {
			return err
		}
	}
	applyHostInitDefaults(&f)
	if err := writeHostFile(f, cmd.ErrOrStderr()); err != nil {
		return err
	}
	printHostInitSummary(cmd.OutOrStdout(), f)
	return nil
}

// parseHostInitFlags reads and validates the host init flags. --config is
// required and checked before anything is prompted or written.
func parseHostInitFlags(cmd *cobra.Command) (hostInitFlags, error) {
	var f hostInitFlags
	f.Port, _ = cmd.Flags().GetInt("port")
	f.FirewallRule, _ = cmd.Flags().GetString("firewall-rule")
	f.NoSleep, _ = cmd.Flags().GetBool("no-sleep")
	f.Config, _ = cmd.Flags().GetString("config")
	f.Force, _ = cmd.Flags().GetBool("force")

	if f.Config == "" {
		return f, errors.New("host init: --config is required; it names the host file to write " +
			"(for example /etc/ts-bridge/host.yaml or C:\\ProgramData\\ts-bridge\\host.yaml)")
	}
	if f.Port != 0 && (f.Port < 1 || f.Port > 65535) {
		return f, fmt.Errorf("port must be between 1 and 65535, got %d", f.Port)
	}
	if f.FirewallRule != "" {
		if _, err := host.SanitizeFirewallRule(f.FirewallRule); err != nil {
			return f, fmt.Errorf("firewall rule: %w", err)
		}
	}
	if hostGOOS != windowsOS && (cmd.Flags().Changed("firewall-rule") || cmd.Flags().Changed("no-sleep")) {
		return f, fmt.Errorf("--firewall-rule and --no-sleep apply to Windows only; %s has no such step (ADR-017 §5)", hostGOOS)
	}
	return f, nil
}

// ensureHostFileWritable refuses a path that cannot be safely replaced: a
// directory, or an existing file that cannot be read (#408), even with
// --force. An existing readable file needs --force, or a "y" when interactive.
func ensureHostFileWritable(path string, force, interactive bool, reader *bufio.Reader, out io.Writer) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("host init: %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("host init: %s is a directory", path)
	}
	// #nosec G304 -- path is the operator's explicit --config.
	probe, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("host init: %s exists but cannot be read; refusing to overwrite it: %w", path, err)
	}
	_ = probe.Close()
	switch {
	case force:
		return nil
	case !interactive:
		return fmt.Errorf("%s already exists (use --force to overwrite)", path)
	case confirmHostOverwrite(reader, out, path):
		return nil
	default:
		return fmt.Errorf("aborted: %s already exists (use --force to overwrite)", path)
	}
}

// confirmHostOverwrite asks on the wizard's own reader, so the answer is not
// lost to a second buffer over the same stdin.
func confirmHostOverwrite(reader *bufio.Reader, out io.Writer, path string) bool {
	fmt.Fprintf(out, "%s already exists. Overwrite? [y/N]: ", path)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == answerYes
}

// promptHostInit asks for the RDP port and, on Windows, the firewall rule and
// sleep setting.
func promptHostInit(reader *bufio.Reader, f *hostInitFlags) error {
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
	}
	if hostGOOS != windowsOS {
		return nil
	}
	rule, err := readOptionalInput(reader, fmt.Sprintf("Firewall rule name (default: %s): ", host.DefaultFirewallRule()))
	if err != nil {
		return fmt.Errorf("read firewall rule: %w", err)
	}
	if rule != "" {
		if _, err := host.SanitizeFirewallRule(rule); err != nil {
			return fmt.Errorf("firewall rule: %w", err)
		}
		f.FirewallRule = rule
	}
	choice, err := readChoiceInput(reader, "Disable sleep mode? (default: yes)", []string{answerYes, "no"})
	if err != nil {
		return fmt.Errorf("read sleep setting: %w", err)
	}
	f.NoSleep = choice == "no"
	return nil
}

func applyHostInitDefaults(f *hostInitFlags) {
	if f.Port == 0 {
		f.Port = host.DefaultRDPPort()
	}
	if f.FirewallRule == "" && hostGOOS == windowsOS {
		f.FirewallRule = host.DefaultFirewallRule()
	}
}

// hostFileFor is the declared state for f on hostGOOS: firewall and power are
// Windows-only keys.
func hostFileFor(f hostInitFlags) host.File {
	port := f.Port
	file := host.File{Version: 1, Host: host.FileHost{
		Services: []host.FileService{{Kind: host.ServiceRDP, Port: &port}},
	}}
	if hostGOOS == windowsOS {
		rule := f.FirewallRule
		prevent := !f.NoSleep
		file.Host.Firewall = &host.FileFirewall{Rule: &rule}
		file.Host.Power = &host.FilePower{PreventSleep: &prevent}
	}
	return file
}

// writeHostFile writes the YAML host file and loads it back, so a file host
// setup would refuse is never left behind as a success.
func writeHostFile(f hostInitFlags, warn io.Writer) error {
	body, err := yaml.Marshal(hostFileFor(f))
	if err != nil {
		return fmt.Errorf("encode host file: %w", err)
	}
	content := "# ts-bridge host configuration (ADR-017), written by ts-bridge host init.\n" +
		"# Read by: ts-bridge host setup | check | serve --config " + f.Config + "\n" + string(body)
	// #nosec G306 -- the host file holds no secret; host check, unelevated, must read it (ADR-017).
	if err := os.WriteFile(f.Config, []byte(content), hostFileMode); err != nil {
		return fmt.Errorf("write host file: %w", err)
	}
	if hostGOOS != windowsOS {
		// WriteFile keeps an existing file's mode and applies the umask.
		// #nosec G302 -- as above: readable by host check, writable by its owner only.
		if err := os.Chmod(f.Config, hostFileMode); err != nil {
			return fmt.Errorf("set host file mode: %w", err)
		}
	}
	if _, err := host.LoadFile(f.Config, hostGOOS, warn); err != nil {
		return fmt.Errorf("host init wrote a file it cannot read back: %w", err)
	}
	return nil
}

// printHostInitSummary displays the configuration written.
func printHostInitSummary(w io.Writer, f hostInitFlags) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  ---------------------------------------")
	fmt.Fprintln(w, "  HOST CONFIGURED")
	fmt.Fprintln(w, "  ---------------------------------------")
	fmt.Fprintf(w, "  RDP Port:       %d\n", f.Port)
	if hostGOOS == windowsOS {
		fmt.Fprintf(w, "  Firewall Rule:  %s\n", f.FirewallRule)
		fmt.Fprintf(w, "  Disable Sleep:  %v\n", !f.NoSleep)
	}
	fmt.Fprintln(w)
	absPath, _ := filepath.Abs(f.Config)
	fmt.Fprintf(w, "  Config: %s\n", absPath)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Next steps:")
	fmt.Fprintf(w, "  1. Run: ts-bridge host setup --config %s\n", absPath)
	fmt.Fprintf(w, "  2. Verify: ts-bridge host check --config %s\n", absPath)
	fmt.Fprintln(w)
}
