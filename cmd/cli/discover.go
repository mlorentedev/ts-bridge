package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"ts-bridge/internal/discover"
)

// discoverCmd is the "ts-bridge discover" subcommand.
var discoverCmd = &cobra.Command{
	Use:   "discover [flags]",
	Short: "Discover tailnet hosts and select one interactively",
	Long: `Discover all hosts on your Tailscale/Headscale tailnet and select one.

Queries the Tailscale API (or Headscale if TS_CONTROL_URL points to a
Headscale instance) to list all authorized devices.

Interactive mode (default): shows a numbered list, type a number or hostname
to select, then optionally update .env with the chosen target.

Non-interactive mode (--json): outputs a JSON array of devices for scripting.

Flags:
  --json          Output devices as JSON (no interactive prompt)
  --filter TEXT   Filter by hostname or IP substring
  --auto          Auto-select first matching host and update .env (no prompt)
  --port PORT     RDP port to use with --auto (default 3389)
  --auth-key KEY  Tailscale auth key (overrides TS_AUTHKEY)
  --tailnet NAME  Tailscale tailnet name (e.g. "mycompany.ts.net")

Examples:
  ts-bridge discover
  ts-bridge discover --json
  ts-bridge discover --filter "desktop"
  ts-bridge discover --auto --port 3389
`,
	RunE: runDiscover,
}

func init() {
	discoverCmd.Flags().Bool("json", false, "Output devices as JSON")
	discoverCmd.Flags().String("filter", "", "Filter by hostname or IP substring")
	discoverCmd.Flags().Bool("auto", false, "Auto-select first matching host and update .env")
	discoverCmd.Flags().Int("port", 3389, "RDP port to use with --auto")
	discoverCmd.Flags().String("auth-key", "", "Tailscale auth key (overrides TS_AUTHKEY)")
	discoverCmd.Flags().String("tailnet", "", "Tailscale tailnet name (e.g. mycompany.ts.net)")

	rootCmd.AddCommand(discoverCmd)
}

// discoverFlags holds parsed flags for the discover command.
type discoverFlags struct {
	JSON    bool
	Filter  string
	Auto    bool
	Port    int
	AuthKey string
	Tailnet string
}

func runDiscover(cmd *cobra.Command, args []string) error {
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

	// Interactive selection.
	selected, err := selectHost(devices)
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

	if flags.Auto {
		return updateEnvFile(".env", target, flags.Port)
	}

	// Ask user to update .env.
	fmt.Printf("Add TS_TARGET=%s:%d to .env? [Y/n] ", target, flags.Port)
	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	if response == "n" || response == "no" {
		fmt.Println("Skipped.")
		return nil
	}

	return updateEnvFile(".env", target, flags.Port)
}

func parseDiscoverFlags(cmd *cobra.Command) (discoverFlags, error) {
	var f discoverFlags

	f.JSON, _ = cmd.Flags().GetBool("json")
	f.Filter, _ = cmd.Flags().GetString("filter")
	f.Auto, _ = cmd.Flags().GetBool("auto")
	f.Port, _ = cmd.Flags().GetInt("port")
	f.AuthKey, _ = cmd.Flags().GetString("auth-key")
	f.Tailnet, _ = cmd.Flags().GetString("tailnet")

	if f.Port < 1 || f.Port > 65535 {
		return f, fmt.Errorf("invalid port %d (must be 1-65535)", f.Port)
	}

	return f, nil
}

// discoverList fetches devices using auto-detection (Tailscale or Headscale).
func discoverList(ctx context.Context, flags discoverFlags) ([]discover.Device, error) {
	authKey := flags.AuthKey
	if authKey == "" {
		authKey = os.Getenv("TS_AUTHKEY")
	}
	tailnet := flags.Tailnet
	if tailnet == "" {
		tailnet = os.Getenv("TS_TAILNET")
	}
	controlURL := os.Getenv("TS_CONTROL_URL")
	headscaleAPIKey := os.Getenv("TS_HEADSCALE_API_KEY")

	// Auto-detect: if TS_CONTROL_URL looks like Headscale, use Headscale.
	isHeadscale := controlURL != "" && !strings.Contains(controlURL, "tailscale.com")

	if isHeadscale && headscaleAPIKey != "" {
		devices, err := discover.ListHeadscaleDevices(ctx, headscaleAPIKey, controlURL)
		if err != nil {
			return nil, fmt.Errorf("discover from Headscale: %w", err)
		}
		return convertHeadscaleDevices(devices), nil
	}

	// Default: try Tailscale.
	if authKey == "" {
		return nil, fmt.Errorf("auth key is required (set TS_AUTHKEY or --auth-key)")
	}
	if tailnet == "" {
		return nil, fmt.Errorf("tailnet name is required (set TS_TAILNET or --tailnet)")
	}

	devices, err := discover.ListDevices(ctx, authKey, tailnet)
	if err != nil {
		return nil, fmt.Errorf("discover from Tailscale: %w", err)
	}
	return devices, nil
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

// updateEnvFile updates .env with the selected target.
func updateEnvFile(path, target string, port int) error {
	if err := discover.UpdateEnv(path, target, port); err != nil {
		return fmt.Errorf("update .env: %w", err)
	}
	fmt.Printf("\n✓ TS_TARGET=%s:%d added to .env\n", target, port)
	return nil
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
