package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"ts-bridge/internal/discover"
)

// newDiscoverCmd constructs the "ts-bridge discover" subcommand.
func newDiscoverCmd() *cobra.Command { //nolint:funlen // baseline #629
	cmd := &cobra.Command{
		Use:   "discover [flags]",
		Short: "Discover tailnet hosts and select one interactively",
		Long: `Discover all hosts on your Tailscale/Headscale tailnet and select one.

Queries the Tailscale API (or Headscale if TS_CONTROL_URL points to a
Headscale instance) to list all authorized devices.

The control-plane API takes an API key, not the node auth key that connect
uses (TS_AUTHKEY is never sent to it):
  Tailscale SaaS   TS_API_KEY, --api-key or --api-key-file, plus TS_TAILNET
                   or --tailnet
  Headscale        TS_HEADSCALE_API_KEY, --api-key or --api-key-file, with
                   TS_CONTROL_URL set to the Headscale server
A Headscale control URL without its API key is an error; it never falls back
to the Tailscale SaaS API.

Interactive mode (default): shows a numbered list, type a number or hostname
to select. With --profile NAME the chosen host is saved as profile NAME, as
import does; without it, discover offers to write TS_TARGET to ./.env.

Non-interactive mode (--json): outputs a JSON array of devices for scripting.

Flags:
  --json          Output devices as JSON (no interactive prompt)
  --filter TEXT   Filter by hostname or IP substring
  --auto          Pick the first authorized matching host, no prompt, and update .env (--profile saves a profile instead)
  --profile NAME  Save the chosen host as profile NAME instead of updating .env
  --port PORT     RDP port to use with --auto or --profile (default 3389)
  --api-key KEY   Control-plane API key (overrides the environment)
  --api-key-file FILE  Read the API key from FILE (preferred over --api-key)
  --tailnet NAME  Tailscale tailnet name (e.g. "mycompany.ts.net")

Without a terminal on stdin, discover does not offer to update .env; pass
--auto to do it on purpose.

Examples:
  ts-bridge discover
  ts-bridge discover --profile office
  ts-bridge discover --json
  ts-bridge discover --filter "desktop"
  ts-bridge discover --auto --port 3389
`,
		RunE: runDiscover,
	}

	cmd.Flags().Bool("json", false, "Output devices as JSON")
	cmd.Flags().String("filter", "", "Filter by hostname or IP substring")
	cmd.Flags().Bool("auto", false, "Pick the first authorized matching host, no prompt, and update .env (--profile saves a profile instead)")
	cmd.Flags().Int("port", 3389, "RDP port to use with --auto or --profile")
	cmd.Flags().String("api-key", "", "Control-plane API key (overrides TS_API_KEY / TS_HEADSCALE_API_KEY) — WARNING: visible in process list")
	cmd.Flags().String("api-key-file", "", "Read the control-plane API key from a file (secure alternative to --api-key)")
	cmd.Flags().String("auth-key", "", "Deprecated alias of --api-key — WARNING: visible in process list")
	_ = cmd.Flags().MarkDeprecated("auth-key", "use --api-key or --api-key-file: discover calls the control-plane API, which takes an API key, not a node auth key")
	cmd.Flags().String("tailnet", "", "Tailscale tailnet name (e.g. mycompany.ts.net)")
	cmd.Flags().String("profile", "", "Save the chosen host as this profile instead of updating .env")
	return cmd
}

// discoverFlags holds parsed flags for the discover command.
type discoverFlags struct {
	JSON       bool
	Filter     string
	Auto       bool
	Port       int
	APIKey     string
	APIKeyFile string
	Tailnet    string
	Profile    string
}

func runDiscover(cmd *cobra.Command, args []string) error { //nolint:funlen,gocyclo // baseline #629
	flags, err := parseDiscoverFlags(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	devices, err := discoverList(ctx, flags)
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		fmt.Println("No devices found on this tailnet.")
		fmt.Println("Make sure devices are online and authorized in your admin console.")
		return nil
	}

	// Apply filter if specified.
	if flags.Filter != "" {
		devices = filterDevices(devices, flags.Filter)
		if len(devices) == 0 {
			fmt.Printf("No devices match filter %q.\n", flags.Filter)
			return nil
		}
	}

	// JSON output: print and exit.
	if flags.JSON {
		return printDevicesJSON(devices)
	}

	selected, err := chooseHost(devices, flags.Auto)
	if err != nil {
		return err
	}

	// Determine target address.
	target := selected.Hostname
	if target == "" {
		// Fallback to first IP.
		if len(selected.Addresses) > 0 {
			// Strip CIDR suffix.
			addr := selected.Addresses[0]
			if idx := strings.Index(addr, "/"); idx > 0 {
				addr = addr[:idx]
			}
			target = addr
		} else {
			target = selected.Name
		}
	}

	fmt.Printf("\nSelected: %s\n", target)

	// Read controlURL from env so the descriptor reflects the active control plane.
	return persistSelection(cmd, flags, target, os.Getenv("TS_CONTROL_URL"))
}

// persistSelection keeps the chosen host: as a named profile with --profile,
// through import's own path, otherwise in ./.env, which stays for setups that
// still read it (#646).
func persistSelection(cmd *cobra.Command, flags discoverFlags, target, controlURL string) error {
	if flags.Profile != "" {
		return runImportDirect(flags.Profile, buildDescriptorLine(target, flags.Port, controlURL), defaultProfileStorePath, false)
	}
	if flags.Auto {
		return updateEnvFile(".env", target, flags.Port, controlURL)
	}
	if !promptEnvUpdate(cmd, target, flags.Port) {
		fmt.Println("Skipped.")
		return nil
	}
	return updateEnvFile(".env", target, flags.Port, controlURL)
}

// promptEnvUpdate asks whether to add the target to .env. Without a terminal
// on stdin it does not ask at all: a closed or piped stdin is not consent.
func promptEnvUpdate(cmd *cobra.Command, target string, port int) bool {
	if isNonInteractive(cmd) {
		fmt.Fprintf(cmd.OutOrStdout(), "stdin is not a terminal; not updating .env (pass --auto to add TS_TARGET=%s:%d)\n", target, port)
		return false
	}
	return confirmEnvUpdate(cmd.InOrStdin(), cmd.OutOrStdout(), target, port)
}

// confirmEnvUpdate prompts [Y/n] and reads one line. A bare Enter takes the
// default (yes); EOF without an answer is a no.
func confirmEnvUpdate(in io.Reader, out io.Writer, target string, port int) bool {
	fmt.Fprintf(out, "Add TS_TARGET=%s:%d to .env? [Y/n] ", target, port)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	response := strings.ToLower(strings.TrimSpace(line))
	return response != "n" && response != "no"
}

func parseDiscoverFlags(cmd *cobra.Command) (discoverFlags, error) {
	var f discoverFlags

	f.JSON, _ = cmd.Flags().GetBool("json")
	f.Filter, _ = cmd.Flags().GetString("filter")
	f.Auto, _ = cmd.Flags().GetBool("auto")
	f.Port, _ = cmd.Flags().GetInt("port")
	f.APIKey, _ = cmd.Flags().GetString("api-key")
	if f.APIKey == "" {
		f.APIKey, _ = cmd.Flags().GetString("auth-key")
	}
	f.APIKeyFile, _ = cmd.Flags().GetString("api-key-file")
	f.Tailnet, _ = cmd.Flags().GetString("tailnet")
	f.Profile, _ = cmd.Flags().GetString("profile")

	if cmd.Flags().Changed("profile") && f.Profile == "" {
		return f, fmt.Errorf("--profile needs a name")
	}
	if f.JSON && f.Profile != "" {
		return f, fmt.Errorf("--json saves nothing: drop --json to save the host as profile %q", f.Profile)
	}
	if f.Port < 1 || f.Port > 65535 {
		return f, fmt.Errorf("invalid port %d (must be 1-65535)", f.Port)
	}

	return f, nil
}

// discoverList fetches devices from the control plane TS_CONTROL_URL names:
// Headscale when it points away from tailscale.com, Tailscale SaaS otherwise.
func discoverList(ctx context.Context, flags discoverFlags) ([]discover.Device, error) {
	controlURL := os.Getenv("TS_CONTROL_URL")
	if controlURL != "" && !strings.Contains(controlURL, "tailscale.com") {
		return discoverHeadscale(ctx, flags, controlURL)
	}
	return discoverTailscale(ctx, flags)
}

func discoverHeadscale(ctx context.Context, flags discoverFlags, controlURL string) ([]discover.Device, error) {
	apiKey, err := discoverAPIKey(flags, "TS_HEADSCALE_API_KEY")
	if err != nil {
		return nil, err
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Headscale API key is required for the configured TS_CONTROL_URL (set TS_HEADSCALE_API_KEY, --api-key or --api-key-file)")
	}
	devices, err := discover.ListHeadscaleDevices(ctx, apiKey, controlURL)
	if err != nil {
		return nil, fmt.Errorf("discover from Headscale: %w", err)
	}
	return convertHeadscaleDevices(devices), nil
}

func discoverTailscale(ctx context.Context, flags discoverFlags) ([]discover.Device, error) {
	apiKey, err := discoverAPIKey(flags, "TS_API_KEY")
	if err != nil {
		return nil, err
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Tailscale API key is required (set TS_API_KEY, --api-key or --api-key-file); TS_AUTHKEY is a node key and cannot list devices")
	}
	tailnet := flags.Tailnet
	if tailnet == "" {
		tailnet = os.Getenv("TS_TAILNET")
	}
	if tailnet == "" {
		return nil, fmt.Errorf("tailnet name is required (set TS_TAILNET or --tailnet)")
	}
	devices, err := discover.ListDevices(ctx, apiKey, tailnet)
	if err != nil {
		return nil, fmt.Errorf("discover from Tailscale: %w", err)
	}
	return devices, nil
}

// discoverAPIKey resolves the control-plane API key: --api-key-file, then
// --api-key, then the environment variable for the control plane in use.
func discoverAPIKey(flags discoverFlags, envName string) (string, error) {
	if flags.APIKeyFile != "" {
		key, err := readAuthKeyFile(flags.APIKeyFile)
		if err != nil {
			return "", fmt.Errorf("--api-key-file: %w", err)
		}
		return key, nil
	}
	if flags.APIKey != "" {
		return flags.APIKey, nil
	}
	return os.Getenv(envName), nil
}

// filterDevices returns devices whose hostname or IP contains the filter string.
func filterDevices(devices []discover.Device, filter string) []discover.Device {
	filter = strings.ToLower(filter)
	result := make([]discover.Device, 0, len(devices))
	for _, d := range devices {
		if strings.Contains(strings.ToLower(d.Hostname), filter) {
			result = append(result, d)
			continue
		}
		for _, addr := range d.Addresses {
			ip := addr
			if idx := strings.Index(addr, "/"); idx > 0 {
				ip = addr[:idx]
			}
			if strings.Contains(ip, filter) {
				result = append(result, d)
				break
			}
		}
	}
	return result
}

// convertHeadscaleDevices converts HeadscaleDevice to Device.
func convertHeadscaleDevices(devices []discover.HeadscaleDevice) []discover.Device {
	result := make([]discover.Device, 0, len(devices))
	for _, d := range devices {
		result = append(result, discover.Device{
			Hostname:   d.Name,
			Addresses:  d.IPs,
			Name:       d.Name,
			User:       d.User,
			Authorized: d.Authorized,
			LastSeen:   d.LastSeen,
			Expires:    d.Expires,
		})
	}
	return result
}

// printDevicesJSON outputs devices as a JSON array.
func printDevicesJSON(devices []discover.Device) error {
	type jsonDevice struct {
		Hostname   string   `json:"hostname"`
		Addresses  []string `json:"addresses"`
		Name       string   `json:"name"`
		User       string   `json:"user"`
		OS         string   `json:"os"`
		Authorized bool     `json:"authorized"`
		LastSeen   string   `json:"lastSeen"`
	}

	jsonDevices := make([]jsonDevice, len(devices))
	for i, d := range devices {
		jsonDevices[i] = jsonDevice{
			Hostname:   d.Hostname,
			Addresses:  d.Addresses,
			Name:       d.Name,
			User:       d.User,
			OS:         d.OS,
			Authorized: d.Authorized,
			LastSeen:   d.LastSeen,
		}
	}

	data, err := json.MarshalIndent(jsonDevices, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal devices: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// updateEnvFile updates .env with the selected target and optionally prints a
// shareable tsb:// descriptor for the chosen host/port/controlURL.
func updateEnvFile(path, target string, port int, controlURL string) error {
	if err := discover.UpdateEnv(path, target, port); err != nil {
		return fmt.Errorf("update .env: %w", err)
	}
	fmt.Printf("\n✓ TS_TARGET=%s:%d added to .env\n", target, port)
	if desc := buildDescriptorLine(target, port, controlURL); desc != "" {
		fmt.Printf("  Descriptor: %s\n", desc)
		fmt.Printf("  (import: ts-bridge import <name> %q)\n", desc)
	}
	return nil
}

// chooseHost picks the host to persist. Under --auto it takes the first
// authorized host in list order without reading stdin, so a script with no
// terminal never blocks (#684); otherwise it prompts.
func chooseHost(devices []discover.Device, auto bool) (discover.Device, error) {
	if !auto {
		return selectHost(devices)
	}
	sortDevices(devices) // authorized first, then by hostname
	if len(devices) == 0 || !devices[0].Authorized {
		return discover.Device{}, fmt.Errorf("--auto found no authorized host among %d device(s); authorize one in the admin console or drop --auto to pick by hand", len(devices))
	}
	return devices[0], nil
}

// selectHost presents an interactive list and returns the selected device.
func selectHost(devices []discover.Device) (discover.Device, error) {
	sortDevices(devices)
	printHostList(devices)

	input, err := readSelection()
	if err != nil {
		return discover.Device{}, err
	}

	return findDevice(devices, input)
}

// sortDevices sorts: authorized first, then by hostname.
func sortDevices(devices []discover.Device) {
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Authorized != devices[j].Authorized {
			return devices[i].Authorized
		}
		return devices[i].Hostname < devices[j].Hostname
	})
}

// printHostList prints a numbered list of devices.
func printHostList(devices []discover.Device) {
	fmt.Println()
	fmt.Println("  TAILNET HOSTS")
	fmt.Println("  ───────────────────────────────────────────────────────")
	for i, d := range devices {
		fmt.Printf("  [%d] %-20s %-16s %-8s %s\n",
			i+1, d.Hostname, deviceIP(d), statusLabel(d), d.LastSeen)
	}
	fmt.Println()
}

// deviceIP returns the first Tailscale IP (without CIDR).
func deviceIP(d discover.Device) string {
	if len(d.Addresses) == 0 {
		return ""
	}
	ip := d.Addresses[0]
	if idx := strings.Index(ip, "/"); idx > 0 {
		return ip[:idx]
	}
	return ip
}

// statusLabel returns "online" or "offline".
func statusLabel(d discover.Device) string {
	if d.Authorized {
		return "online"
	}
	return "offline"
}

// readSelection reads user input from the terminal.
func readSelection() (string, error) {
	fmt.Print("Enter host number (or type hostname): ")
	var input string
	_, _ = fmt.Scanln(&input)
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("no selection made")
	}
	return input, nil
}

// findDevice resolves user input to a device by number, exact hostname, or
// partial match. Returns an error if no match or ambiguous.
func findDevice(devices []discover.Device, input string) (discover.Device, error) {
	// Try numeric.
	if d, ok := findByNumber(devices, input); ok {
		return d, nil
	}
	// Try exact hostname.
	if d, ok := findByExact(devices, input); ok {
		return d, nil
	}
	// Try partial.
	if d, n := findByPartial(devices, input); n == 1 {
		return d, nil
	} else if n > 1 {
		return discover.Device{}, fmt.Errorf("multiple hosts match %q — be more specific", input)
	}
	return discover.Device{}, fmt.Errorf("no host matches %q", input)
}

// findByNumber matches a numeric index.
func findByNumber(devices []discover.Device, input string) (discover.Device, bool) {
	var index int
	if _, err := fmt.Sscanf(input, "%d", &index); err == nil && index > 0 && index <= len(devices) {
		return devices[index-1], true
	}
	return discover.Device{}, false
}

// findByExact matches an exact hostname (case-insensitive).
// Returns the device and a bool indicating if found.
func findByExact(devices []discover.Device, input string) (discover.Device, bool) {
	for _, d := range devices {
		if strings.EqualFold(d.Hostname, input) {
			return d, true
		}
	}
	return discover.Device{}, false
}

// findByPartial matches a substring in hostname. Returns (device, count).
func findByPartial(devices []discover.Device, input string) (discover.Device, int) {
	var match discover.Device
	matches := 0
	inputLower := strings.ToLower(input)
	for _, d := range devices {
		if strings.Contains(strings.ToLower(d.Hostname), inputLower) {
			match = d
			matches++
		}
	}
	return match, matches
}
