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

	"ts-bridge/internal/config"
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
	initCmd.Flags().Bool("force", false, "Overwrite existing config files without prompting")

	rootCmd.AddCommand(initCmd)
}

// initFlags holds values parsed from CLI flags for the init command.
type initFlags struct {
	AuthKey   string // #nosec G117 -- CLI flag value, not a secret in source code
	Target    string
	Instance  string
	PortRange string
	Format    string
	Config    string
	Force     bool
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
	return runInitNonInteractive(cmd, f)
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
	f.Force, _ = cmd.Flags().GetBool("force")

	// Validate format.
	switch strings.ToLower(f.Format) {
	case formatYAML, formatENV:
		f.Format = strings.ToLower(f.Format)
	default:
		return f, fmt.Errorf("invalid format %q: must be yaml or env", f.Format)
	}

	return f, nil
}

// runInitInteractive prompts the user for all required values.
func runInitInteractive(cmd *cobra.Command, f initFlags) error {
	in := cmd.InOrStdin()
	reader := bufio.NewReader(in)

	// Collect all inputs.
	if err := collectInteractiveInputs(reader, &f); err != nil {
		return err
	}

	// Resolve config path based on chosen format (only if not explicitly set).
	if f.Config == "" {
		f.Config = defaultConfigPath(f.Format)
	}

	// Write config.
	if f.Format == formatYAML {
		if err := writeYAMLConfig(cmd, f); err != nil {
			return err
		}
	} else {
		if err := writeEnvConfig(cmd, f); err != nil {
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
func runInitNonInteractive(cmd *cobra.Command, f initFlags) error {
	// Resolve config path based on chosen format (only if not explicitly set).
	if f.Config == "" {
		f.Config = defaultConfigPath(f.Format)
	}

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
		if err := writeYAMLConfig(cmd, f); err != nil {
			return err
		}
	} else {
		if err := writeEnvConfig(cmd, f); err != nil {
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

	// #nosec G104,G115 -- user-provided prompt, not a credential; os.Stdin.Fd() is always valid.
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
		msg := "auth key invalid format (must start with tskey- or hskey-)"
		if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
			msg += " (did you paste a Tailscale login URL instead of the auth key? use the key value after /auth/singleusekey/ or /auth/key)"
		}
		return fmt.Errorf("%s", msg)
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
func writeYAMLConfig(cmd *cobra.Command, f initFlags) error {
	yamlPath := f.Config
	envPath := filepath.Join(filepath.Dir(yamlPath), ".env")

	if err := ensureWritable(yamlPath, f.Force, isNonInteractive(cmd), cmd); err != nil {
		return err
	}
	if err := ensureWritable(envPath, f.Force, isNonInteractive(cmd), cmd); err != nil {
		return err
	}

	yamlContent := buildYAMLContent(f)
	if err := writeFileWithPerms(yamlPath, []byte(yamlContent)); err != nil {
		return fmt.Errorf("write YAML config: %w", err)
	}

	envContent := buildEnvContent(f.AuthKey, envPath)
	if err := writeFileWithPerms(envPath, []byte(envContent)); err != nil {
		return fmt.Errorf("write .env file: %w", err)
	}

	return nil
}

// writeEnvConfig writes all settings including auth key to a .env file.
func writeEnvConfig(cmd *cobra.Command, f initFlags) error {
	envPath := f.Config

	if err := ensureWritable(envPath, f.Force, isNonInteractive(cmd), cmd); err != nil {
		return err
	}

	envContent := buildEnvConfigContent(f)
	if err := writeFileWithPerms(envPath, []byte(envContent)); err != nil {
		return fmt.Errorf("write .env file: %w", err)
	}

	return nil
}

// ensureWritable checks if a file exists and enforces overwrite protection.
// Returns an error if the file exists and overwrite is not allowed.
func ensureWritable(path string, force bool, nonInteractive bool, cmd *cobra.Command) error {
	if _, err := os.Stat(path); err == nil {
		if !force && !nonInteractive {
			if !confirmOverwrite(path, cmd) {
				return fmt.Errorf("aborted: %s already exists (use --force to overwrite)", path)
			}
		} else if !force {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
	}
	return nil
}

// buildYAMLContent builds the YAML configuration file content.
func buildYAMLContent(f initFlags) string {
	var sb strings.Builder
	sb.WriteString("# ts-bridge YAML configuration\n")
	sb.WriteString("# Non-sensitive settings only — auth key is stored in .env\n")
	sb.WriteString(fmt.Sprintf("# Generated by ts-bridge init on %s\n\n", nowFormatted()))

	sb.WriteString("version: 1\n")
	sb.WriteString(fmt.Sprintf("target: %s\n", f.Target))
	if f.Instance != "" {
		sb.WriteString(fmt.Sprintf("hostname: tsb-%s\n", config.SanitizeHostnameLabel(f.Instance)))
	}
	if f.PortRange != "" {
		sb.WriteString(fmt.Sprintf("port_range: %s\n", f.PortRange))
	}
	return sb.String()
}

// buildEnvConfigContent builds the .env file content for the env format.
func buildEnvConfigContent(f initFlags) string {
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
	return sb.String()
}

// writeFileWithPerms writes content to a file with 0600 permissions and checks permissions.
func writeFileWithPerms(path string, content []byte) error {
	// #nosec G306 -- path is user-controlled via --config flag.
	if err := os.WriteFile(path, content, 0600); err != nil {
		return err
	}
	checkPermissions(path)
	return nil
}

// buildEnvContent builds the .env file content, merging the auth key
// with any existing .env file to avoid overwriting TS_TARGET or other vars.
func buildEnvContent(authKey, envPath string) string {
	var sb strings.Builder
	sb.WriteString("# ts-bridge environment configuration\n")
	sb.WriteString("# Auth key — DO NOT commit this file\n")
	sb.WriteString(fmt.Sprintf("# Generated by ts-bridge init on %s\n\n", nowFormatted()))

	// Read existing .env and preserve known vars.
	existingVars := make(map[string]string)
	// #nosec G304 -- envPath is derived from yamlPath directory, not user-controlled.
	if data, err := os.ReadFile(envPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if idx := strings.Index(line, "="); idx > 0 {
				key := line[:idx]
				val := line[idx+1:]
				existingVars[key] = val
			}
		}
	}

	// Write TS_AUTHKEY first.
	sb.WriteString(fmt.Sprintf("TS_AUTHKEY=%s\n", authKey))

	// Write preserved vars in a stable order.
	for _, key := range []string{"TS_TARGET", "TS_INSTANCE_NAME", "TS_PORT_RANGE",
		"TS_LOCAL_ADDR", "TS_HOSTNAME", "TS_STATE_DIR", "TS_CONTROL_URL",
		"TS_HEALTH_ADDR", "TS_LOG_FORMAT", "TS_IDLE_TIMEOUT", "TS_DIAL_TIMEOUT",
		"TS_DIAL_RETRIES", "TS_DIAL_BACKOFF_BASE", "TS_DIAL_BACKOFF_MAX",
		"TS_MAX_CONNECTIONS", "TS_TIMEOUT", "TS_DRAIN_TIMEOUT", "TS_VERBOSE"} {
		if v, ok := existingVars[key]; ok {
			sb.WriteString(fmt.Sprintf("%s=%s\n", key, v))
		}
	}

	// Write any other vars we didn't explicitly list.
	for key, val := range existingVars {
		if key == "TS_AUTHKEY" {
			continue
		}
		found := false
		for _, known := range []string{"TS_TARGET", "TS_INSTANCE_NAME", "TS_PORT_RANGE",
			"TS_LOCAL_ADDR", "TS_HOSTNAME", "TS_STATE_DIR", "TS_CONTROL_URL",
			"TS_HEALTH_ADDR", "TS_LOG_FORMAT", "TS_IDLE_TIMEOUT", "TS_DIAL_TIMEOUT",
			"TS_DIAL_RETRIES", "TS_DIAL_BACKOFF_BASE", "TS_DIAL_BACKOFF_MAX",
			"TS_MAX_CONNECTIONS", "TS_TIMEOUT", "TS_DRAIN_TIMEOUT", "TS_VERBOSE"} {
			if key == known {
				found = true
				break
			}
		}
		if !found {
			sb.WriteString(fmt.Sprintf("%s=%s\n", key, val))
		}
	}

	return sb.String()
}

// isNonInteractive returns true if stdin is not a terminal (piped input, CI, etc.).
func isNonInteractive(cmd *cobra.Command) bool {
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok {
		return !term.IsTerminal(int(f.Fd()))
	}
	return true
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
	if runtime.GOOS == windowsOS {
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

// SanitizeHostnameLabel delegates to config.SanitizeHostnameLabel.
func SanitizeHostnameLabel(value string) string {
	return config.SanitizeHostnameLabel(value)
}
