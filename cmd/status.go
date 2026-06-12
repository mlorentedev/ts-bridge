package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// statusCmd is the "ts-bridge status" subcommand.
var statusCmd = &cobra.Command{
	Use:   "status [flags]",
	Short: "Show bridge health and metrics summary",
	Long: `Query the bridge's health and metrics endpoints and display
a human-readable summary of the running bridge state.

Endpoints queried:
  /health/live   — liveness probe
  /health/ready  — readiness probe
  /metrics       — operational metrics

Examples:
  ts-bridge status
  ts-bridge status --addr 127.0.0.1:9090
  ts-bridge status --json
  ts-bridge status --watch --interval 2s`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().String("addr", "127.0.0.1:9090", "Health server address")
	statusCmd.Flags().Bool("json", false, "Output raw JSON from /metrics")
	statusCmd.Flags().BoolP("watch", "w", false, "Continuously watch and update status")
	statusCmd.Flags().DurationP("interval", "i", 5*time.Second, "Polling interval for --watch")

	rootCmd.AddCommand(statusCmd)
}

// metricsResponse matches the JSON shape from /metrics.
type metricsResponse struct {
	ActiveConnections    int64 `json:"active_connections"`
	TotalConnections     int64 `json:"total_connections"`
	TotalBytesTx         int64 `json:"total_bytes_tx"`
	TotalBytesRx         int64 `json:"total_bytes_rx"`
	TotalErrors          int64 `json:"total_errors"`
	RejectedConnections  int64 `json:"rejected_connections"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	addr, _ := cmd.Flags().GetString("addr")
	jsonOut, _ := cmd.Flags().GetBool("json")
	watch, _ := cmd.Flags().GetBool("watch")
	interval, _ := cmd.Flags().GetDuration("interval")

	if watch {
		return runWatch(addr, interval, jsonOut, cmd)
	}

	return runOnce(addr, jsonOut)
}

func runOnce(addr string, jsonOut bool) error {
	return runStatusWithWriter(addr, jsonOut, os.Stdout)
}

func runStatusWithWriter(addr string, jsonOut bool, w io.Writer) error {
	live, ready, metrics, err := fetchHealth(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Bridge not running at %s\n", addr)
		return nil
	}

	if jsonOut {
		data, _ := json.MarshalIndent(metrics, "", "  ")
		fmt.Fprintln(w, string(data))
		return nil
	}

	printSummary(live, ready, metrics, w)
	return nil
}

func runWatch(addr string, interval time.Duration, jsonOut bool, cmd *cobra.Command) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	w := cmd.OutOrStdout()

	// First display immediately.
	if err := displayOnce(addr, jsonOut, w); err != nil {
		fmt.Fprintf(os.Stderr, "Bridge not running at %s\n", addr)
	}

	for {
		select {
		case <-sigCh:
			fmt.Fprintln(w)
			fmt.Fprintln(os.Stderr, "Exiting watch mode")
			return nil
		case <-ticker.C:
			if err := displayOnce(addr, jsonOut, w); err != nil {
				fmt.Fprintf(os.Stderr, "Bridge not running at %s\n", addr)
			}
		}
	}
}

func displayOnce(addr string, jsonOut bool, w io.Writer) error {
	// Clear screen for watch mode (simple approach).
	if !jsonOut {
		fmt.Fprint(w, "\033[2J\033[H")
	}

	live, ready, metrics, err := fetchHealth(addr)
	if err != nil {
		return err
	}
	if jsonOut {
		data, _ := json.MarshalIndent(metrics, "", "  ")
		fmt.Fprintln(w, string(data))
		return nil
	}

	printSummary(live, ready, metrics, w)
	return nil
}

func fetchHealth(addr string) (live, ready bool, metrics metricsResponse, err error) {
	client := &http.Client{Timeout: 3 * time.Second}

	// Check liveness.
	if err := func() error {
		resp, err := client.Get(fmt.Sprintf("http://%s/health/live", addr))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		live = resp.StatusCode == http.StatusOK
		return nil
	}(); err != nil {
		return false, false, metrics, err
	}

	// Check readiness.
	if err := func() error {
		resp, err := client.Get(fmt.Sprintf("http://%s/health/ready", addr))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		ready = resp.StatusCode == http.StatusOK
		return nil
	}(); err != nil {
		return live, false, metrics, err
	}

	// Fetch metrics.
	resp, err := client.Get(fmt.Sprintf("http://%s/metrics", addr))
	if err != nil {
		return live, ready, metrics, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return live, ready, metrics, err
	}
	if err := json.Unmarshal(body, &metrics); err != nil {
		return live, ready, metrics, err
	}

	return live, ready, metrics, nil
}

func printSummary(live, ready bool, m metricsResponse, w io.Writer) {
	fmt.Fprintln(w, "  +---------------------------------------+")
	fmt.Fprintf(w, "  |         TS-BRIDGE STATUS %-12s  |\n", "")
	fmt.Fprintln(w, "  +---------------------------------------+")

	// Status line.
	if live && ready {
		fmt.Fprintln(w, "  |  Status:  RUNNING                    |")
	} else if live {
		fmt.Fprintln(w, "  |  Status:  RUNNING (not ready)        |")
	} else {
		fmt.Fprintln(w, "  |  Status:  NOT RUNNING                |")
	}

	fmt.Fprintln(w, "  +---------------------------------------+")
	fmt.Fprintf(w, "  |  Active connections:  %-18d  |\n", m.ActiveConnections)
	fmt.Fprintf(w, "  |  Total connections:   %-18d  |\n", m.TotalConnections)
	fmt.Fprintf(w, "  |  Bytes sent (tx):     %-18s  |\n", formatBytes(m.TotalBytesTx))
	fmt.Fprintf(w, "  |  Bytes received (rx): %-18s  |\n", formatBytes(m.TotalBytesRx))
	fmt.Fprintf(w, "  |  Errors:              %-18d  |\n", m.TotalErrors)
	fmt.Fprintf(w, "  |  Rejected:            %-18d  |\n", m.RejectedConnections)
	fmt.Fprintln(w, "  +---------------------------------------+")
}

// formatBytes formats bytes into human-readable KiB/MiB/GiB.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	if n < unit*unit {
		return fmt.Sprintf("%.1f KiB", float64(n)/unit)
	}
	if n < unit*unit*unit {
		return fmt.Sprintf("%.1f MiB", float64(n)/(unit*unit))
	}
	return fmt.Sprintf("%.1f GiB", float64(n)/(unit*unit*unit))
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	return d.String()
}
