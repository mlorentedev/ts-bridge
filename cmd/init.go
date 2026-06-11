package cmd

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/spf13/cobra"
)

// initCmd is the "ts-bridge init" subcommand — interactive setup wizard.
var initCmd = &cobra.Command{
	Use:   "init [flags]",
	Short: "Interactive setup wizard to create a ts-bridge configuration file",
	Long: `Run ts-bridge init to create a configuration file.

Interactive mode (no flags): prompts for auth key (masked), target, instance
name, and config format (YAML or .env).

Non-interactive mode (with --auth-key and --target): writes config silently,
useful for automation or CI.

Security notes:
  - Auth key input is masked in interactive mode (no echo).
  - In YAML mode the auth key is written to .env, NOT to the YAML file.
  - A permission warning is shown if config files are world-readable.

Examples:
  # Interactive wizard
  ts-bridge init

  # Non-interactive: .env output (default)
  ts-bridge init --auth-key tskey-auth-xxx --target 100.64.0.1:3389

  # Non-interactive: YAML output
  ts-bridge init --auth-key tskey-auth-xxx --target 100.64.0.1:3389 --format yaml

  # Custom output path
  ts-bridge init --auth-key tskey-auth-xxx --target 100.64.0.1:3389 --config /etc/ts-bridge.yaml
`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().String("auth-key", "", "Auth key (non-interactive mode) — WARNING: visible in process list")
	initCmd.Flags().String("target", "", "Target address HOST:PORT (non-interactive mode)")
	initCmd.Flags().String("instance", "", "Instance name for auto-mode")
	initCmd.Flags().String("port-range", "", "Port range for auto mode (e.g. 33389-34388)")
	initCmd.Flags().String("format", "env", "Output format: yaml or env (default: env)")
	initCmd.Flags().String("config", "", "Output config file path (default: ./ts-bridge.yaml for yaml, ./.env for env)")

	rootCmd.AddCommand(initCmd)
}

// initFlags holds values parsed from CLI flags for the init command.
type initFlags struct {
	AuthKey   string
	Target    string
	Instance  string
	PortRange string
	Format    string
	Config    string
}

const (
	formatYAML = "yaml"
	formatENV  = "env"
)

// runInit is the entry point for "ts-bridge init".
func runInit(cmd *cobra.Command, args []string) error {
	// Parse flags.
	f, err := parseInitFlags(cmd)
	if err != nil {
		return err
	}

	// Determine mode.
	isInteractive := f.AuthKey == "" || f.Target == ""

	if isInteractive {
		return runInitInteractive(cmd, f)
	}
	return runInitNonInteractive(f)
}

// parseInitFlags reads all init-specific flags into a struct.
func parseInitFlags(cmd *cobra.Command) (initFlags, error) {
	var f initFlags

	f.AuthKey, _ = cmd.Flags().GetString("auth-key")
	f.Target, _ = cmd.Flags().GetString("target")
	f.Instance, _ = cmd.Flags().GetString("instance")
	f.PortRange, _ = cmd.Flags().GetString("port-range")
	f.Format, _ = cmd.Flags().GetString("format")
	f.Config, _ = cmd.Flags().GetString("config")

	// Validate format.
	switch strings.ToLower(f.Format) {
	case formatYAML, formatENV:
		f.Format = strings.ToLower(f.Format)
	default:
		return f, fmt.Errorf("invalid format %q: must be yaml or env", f.Format)
	}

	// Resolve default config path.
	if f.Config == "" {
		if f.Format == formatYAML {
			f.Config = "ts-bridge.yaml"
		} else {
			f.Config = ".env"
		}
	}

	return f, nil
}

// runInitInteractive prompts the user for all required values.
func runInitInteractive(_ *cobra.Command, f initFlags) error {
	reader := bufio.NewReader(os.Stdin)

	// Collect all inputs.
	if err := collectInteractiveInputs(reader, &f); err != nil {
		return err
	}

	// Resolve config path based on chosen format.
	if f.Config == "" {
		f.Config = defaultConfigPath(f.Format)
	}

	// Write config.
	if f.Format == formatYAML {
		if err := writeYAMLConfig(f); err != nil {
			return err
		}
	} else {
		if err := writeEnvConfig(f); err != nil {
			return err
		}
	}

	// Print next steps.
	printNextSteps(f)

	return nil
}

// collectInteractiveInputs prompts the user for all required values.
func collectInteractiveInputs(reader *bufio.Reader, f *initFlags) error {
	// 1. Auth key (masked).
	authKey, err := readMaskedInput("Auth key (tskey-… or hskey-…): ")
	if err != nil {
		return fmt.Errorf("read auth key: %w", err)
	}
	if err := validateAuthKey(authKey); err != nil {
		return err
	}
	f.AuthKey = authKey

	// 2. Target.
	target, err := readRequiredInput(reader, "Target host:port: ")
	if err != nil {
		return err
	}
	if err := validateTarget(target); err != nil {
		return err
	}
	f.Target = target

	// 3. Instance name (optional).
	f.Instance, err = readOptionalInput(reader, "Instance name (optional, press Enter to skip): ")
	if err != nil {
		return err
	}

	// 4. Port range (optional, shown only if no instance provided).
	if f.Instance == "" {
		portRange, err := readOptionalInput(reader, "Port range for auto mode (optional, e.g. 33389-34388): ")
		if err != nil {
			return err
		}
		if portRange != "" {
			if err := validatePortRange(portRange); err != nil {
				return err
			}
			f.PortRange = portRange
		}
	}

	// 5. Config format.
	f.Format, err = readChoiceInput(reader, "Config format [yaml/env] (default: env): ", []string{formatYAML, formatENV})
	if err != nil {
		return err
	}

	return nil
}

// runInitNonInteractive writes config silently from flags.
func runInitNonInteractive(f initFlags) error {
	// Validate inputs.
	if err := validateAuthKey(f.AuthKey); err != nil {
		return err
	}
	if err := validateTarget(f.Target); err != nil {
		return err
	}
	if f.Instance != "" {
		// Instance is freeform — just trim whitespace.
		f.Instance = strings.TrimSpace(f.Instance)
	}
	if f.PortRange != "" {
		if err := validatePortRange(f.PortRange); err != nil {
			return err
		}
	}

	// Write config.
	if f.Format == formatYAML {
		if err := writeYAMLConfig(f); err != nil {
			return err
		}
	} else {
		if err := writeEnvConfig(f); err != nil {
			return err
		}
	}

	// Print next steps (non-interactive still shows them).
	printNextSteps(f)

	return nil
}

// ─── Input helpers ───────────────────────────────────────────────

// readMaskedInput reads a line with no echo (masked) using golang.org/x/term.
func readMaskedInput(prompt string) (string, error) {
	fmt.Print(prompt)

	// #nosec G104 -- user-provided prompt, not a credential.
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println() // newline after input
	return string(password), nil
}

// readRequiredInput reads a required (non-empty) line.
func readRequiredInput(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	fmt.Println()
	value := strings.TrimSpace(line)
	if value == "" {
		return "", fmt.Errorf("value is required")
	}
	return value, nil
}

// readOptionalInput reads an optional (may be empty) line.
func readOptionalInput(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	fmt.Println()
	return strings.TrimSpace(line), nil
}

// readChoiceInput reads a line and validates it against allowed choices.
func readChoiceInput(reader *bufio.Reader, prompt string, choices []string) (string, error) {
	choiceStr := strings.Join(choices, "/")
	for {
		fmt.Printf("%s [%s]: ", prompt, choiceStr)
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		fmt.Println()
		value := strings.TrimSpace(strings.ToLower(line))
		if value == "" {
			// Default: first choice.
			return choices[0], nil
		}
		for _, c := range choices {
			if value == c {
				return c, nil
			}
		}
		fmt.Fprintf(os.Stderr, "Invalid choice. Please enter one of: %s\n", choiceStr)
	}
}

// ─── Validation ──────────────────────────────────────────────────

func validateAuthKey(key string) error {
	if key == "" {
		return fmt.Errorf("auth key is required")
	}
	if !strings.HasPrefix(key, "tskey-") && !strings.HasPrefix(key, "hskey-") {
		return fmt.Errorf("auth key invalid format (must start with tskey- or hskey-)")
	}
	return nil
}

func validateTarget(target string) error {
	if target == "" {
		return fmt.Errorf("target is required")
	}
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return fmt.Errorf("target invalid format (expected HOST:PORT): %w", err)
	}
	if host == "" {
		return fmt.Errorf("target: host cannot be empty")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("target: invalid port %q", portStr)
	}
	return nil
}

func validatePortRange(pr string) error {
	parts := strings.Split(pr, "-")
	if len(parts) != 2 {
		return fmt.Errorf("port range invalid format %q (expected START-END)", pr)
	}
	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return fmt.Errorf("port range invalid start: %w", err)
	}
	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return fmt.Errorf("port range invalid end: %w", err)
	}
	if start < 1 || end > 65535 || start > end {
		return fmt.Errorf("port range out of bounds: %d-%d", start, end)
	}
	return nil
}

// ─── Config writers ──────────────────────────────────────────────

// writeYAMLConfig writes non-sensitive settings to a YAML file and
// the auth key to a .env file (never to YAML).
func writeYAMLConfig(f initFlags) error {
	yamlPath := f.Config
	envPath := filepath.Join(filepath.Dir(yamlPath), ".env")

	// Build YAML content.
	var sb strings.Builder
	sb.WriteString("# ts-bridge YAML configuration\n")
	sb.WriteString("# Non-sensitive settings only — auth key is stored in .env\n")
	sb.WriteString(fmt.Sprintf("# Generated by ts-bridge init on %s\n\n", nowFormatted()))

	sb.WriteString("version: 1\n")
	sb.WriteString(fmt.Sprintf("target: %s\n", f.Target))
	if f.Instance != "" {
		sb.WriteString(fmt.Sprintf("hostname: tsb-%s\n", sanitizeHostnameLabel(f.Instance)))
	}
	if f.PortRange != "" {
		sb.WriteString(fmt.Sprintf("port_range: %s\n", f.PortRange))
	}

	// Write YAML file.
	// #nosec G306 -- yamlPath is user-controlled via --config flag.
	if err := os.WriteFile(yamlPath, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("write YAML config: %w", err)
	}
	checkPermissions(yamlPath)

	// Write .env with auth key.
	var envSB strings.Builder
	envSB.WriteString("# ts-bridge environment configuration\n")
	envSB.WriteString("# Auth key — DO NOT commit this file\n")
	envSB.WriteString(fmt.Sprintf("# Generated by ts-bridge init on %s\n\n", nowFormatted()))
	envSB.WriteString(fmt.Sprintf("TS_AUTHKEY=%s\n", f.AuthKey))

	// #nosec G306 -- envPath is derived from yamlPath.
	if err := os.WriteFile(envPath, []byte(envSB.String()), 0600); err != nil {
		return fmt.Errorf("write .env file: %w", err)
	}
	checkPermissions(envPath)

	return nil
}

// writeEnvConfig writes all settings including auth key to a .env file.
func writeEnvConfig(f initFlags) error {
	envPath := f.Config

	var sb strings.Builder
	sb.WriteString("# ── Required ─────────────────────────────────────────────────\n")
	sb.WriteString("#\n")
	sb.WriteString("# Auth key (tskey- for Tailscale SaaS, hskey- for Headscale)\n")
	sb.WriteString("#   Tailscale: https://login.tailscale.com/admin/settings/keys\n")
	sb.WriteString("#   Headscale: headscale preauthkeys create --user <ID> --reusable --ephemeral --expiration 8760h\n")
	sb.WriteString(fmt.Sprintf("TS_AUTHKEY=%s\n\n", f.AuthKey))
	sb.WriteString("# Target host:port on the Tailscale/Headscale network\n")
	sb.WriteString(fmt.Sprintf("TS_TARGET=%s\n\n", f.Target))
	sb.WriteString("# ── Optional (sensible defaults) ─────────────────────────────\n")
	sb.WriteString("# See README.md → Configuration Reference for all options.\n")

	if f.Instance != "" {
		sb.WriteString(fmt.Sprintf("TS_INSTANCE_NAME=%s\n", f.Instance))
	}
	if f.PortRange != "" {
		sb.WriteString(fmt.Sprintf("TS_PORT_RANGE=%s\n", f.PortRange))
	}
	sb.WriteString("#\n")
	sb.WriteString("# TS_LOCAL_ADDR=127.0.0.1:33389   # Local bind address\n")
	sb.WriteString("# TS_CONTROL_URL=                  # Custom control plane\n")
	sb.WriteString("# TS_IDLE_TIMEOUT=                 # Close idle conns after this duration\n")
	sb.WriteString("# TS_DIAL_TIMEOUT=5s               # Per-connection target dial timeout\n")
	sb.WriteString("# TS_DIAL_RETRIES=3                # Max retries for transient dial failures\n")
	sb.WriteString("# TS_DIAL_BACKOFF_BASE=1s          # Base backoff for retries\n")
	sb.WriteString("# TS_DIAL_BACKOFF_MAX=30s          # Cap on backoff duration per retry\n")

	// #nosec G306 -- envPath is user-controlled via --config flag.
	if err := os.WriteFile(envPath, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("write .env file: %w", err)
	}
	checkPermissions(envPath)

	return nil
}

// defaultConfigPath returns the default config filename for the given format.
func defaultConfigPath(format string) string {
	if format == formatYAML {
		return "ts-bridge.yaml"
	}
	return ".env"
}

// checkPermissions warns if the file is world-readable (Unix only).
func checkPermissions(path string) {
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err == nil && info.Mode().Perm()&0077 != 0 {
			fmt.Fprintf(os.Stderr, "WARNING: %s has loose permissions (%04o); consider chmod 600 %s\n",
				path, info.Mode().Perm(), path)
		}
	}
}

// ─── Output helpers ──────────────────────────────────────────────

func nowFormatted() string {
	return time.Now().Format("2006-01-02 15:04:05 MST")
}

func printNextSteps(f initFlags) {
	fmt.Println()
	fmt.Println("Configuration written successfully.")
	fmt.Printf("  Config: %s\n", f.Config)
	if f.Format == formatYAML {
		envPath := filepath.Join(filepath.Dir(f.Config), ".env")
		fmt.Printf("  Auth key: %s\n", envPath)
	}
	if f.Instance != "" {
		fmt.Printf("  Instance: %s\n", f.Instance)
	}
	fmt.Println()
	fmt.Println("Next step:")
	fmt.Println("  ts-bridge connect")
	fmt.Println()
}

// sanitizeHostnameLabel mirrors the logic in internal/config/config.go.
func sanitizeHostnameLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	previousDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			previousDash = false
			continue
		}
		if !previousDash {
			b.WriteByte('-')
			previousDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
