package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// newStatusCmd constructs the "ts-bridge status" subcommand.
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [flags]",
		Short: "Show bridge health and metrics summary",
		Long: `Query the bridge's health and metrics endpoints and display
a human-readable summary of the running bridge state.

Endpoints queried:
  /health/live   — liveness probe
  /health/ready  — readiness probe
  /metrics       — operational metrics

The health server is off unless connect runs with --health-addr or
TS_HEALTH_ADDR. With no answer at --addr, status exits 0.

With --json, a down bridge prints {"running":false,"addr":...} and an up
one prints the /metrics document, which has no "running" key. A script
should test for the key (jq 'has("running")'), not read its value.

Examples:
  ts-bridge status
  ts-bridge status --addr 127.0.0.1:9090
  ts-bridge status --json
  ts-bridge status --watch --interval 2s`,
		RunE: runStatus,
	}

	cmd.Flags().String("addr", "127.0.0.1:9090", "Health server address")
	cmd.Flags().Bool("json", false, "Output raw JSON from /metrics ({\"running\":false,...} when nothing answers)")
	cmd.Flags().BoolP("watch", "w", false, "Continuously watch and update status")
	cmd.Flags().DurationP("interval", "i", 5*time.Second, "Polling interval for --watch")
	return cmd
}

// metricsResponse matches the JSON shape from /metrics.
type metricsResponse struct {
	ActiveConnections   int64 `json:"active_connections"`
	TotalConnections    int64 `json:"total_connections"`
	TotalBytesTx        int64 `json:"total_bytes_tx"`
	TotalBytesRx        int64 `json:"total_bytes_rx"`
	TotalErrors         int64 `json:"total_errors"`
	RejectedConnections int64 `json:"rejected_connections"`
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
		reportNoAnswer(addr, jsonOut, w)
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

// reportNoAnswer explains an unanswered health probe. The health server is off
// unless connect runs with --health-addr or TS_HEALTH_ADDR, so silence at addr
// does not prove the bridge is down. With --json, stdout also carries
// {"running":false,"addr":...} so a pipeline can tell down from up.
func reportNoAnswer(addr string, jsonOut bool, w io.Writer) {
	fmt.Fprintf(os.Stderr, "No health endpoint answered at %s: the bridge is not running, or connect was started without --health-addr / TS_HEALTH_ADDR\n", addr)
	if jsonOut {
		data, _ := json.Marshal(struct {
			Running bool   `json:"running"`
			Addr    string `json:"addr"`
		}{false, addr})
		fmt.Fprintln(w, string(data))
	}
}

func runWatch(addr string, interval time.Duration, jsonOut bool, cmd *cobra.Command) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	return runWatchLoop(addr, interval, jsonOut, cmd.OutOrStdout(), sigCh)
}

// runWatchLoop is the signal-agnostic watch loop. Accepts a signal channel
// so tests can inject signals without relying on OS-level signal delivery.
func runWatchLoop(addr string, interval time.Duration, jsonOut bool, w io.Writer, sigCh <-chan os.Signal) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// First display immediately.
	if err := displayOnce(addr, jsonOut, w); err != nil {
		reportNoAnswer(addr, jsonOut, w)
	}

	for {
		select {
		case <-sigCh:
			fmt.Fprintln(w)
			fmt.Fprintln(os.Stderr, "Exiting watch mode")
			return nil
		case <-ticker.C:
			if err := displayOnce(addr, jsonOut, w); err != nil {
				reportNoAnswer(addr, jsonOut, w)
			}
		}
	}
}

func displayOnce(addr string, jsonOut bool, w io.Writer) error {
	// Clear screen for watch mode — skip on Windows without VT.
	// We conservatively skip ANSI clear on Windows to avoid garbage output
	// when VT processing is not enabled (which requires syscall.GetConsoleMode).
	// On Unix, ANSI codes work reliably.
	if !jsonOut && supportsClearScreen() {
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

func fetchHealth(addr string) (live, ready bool, metrics metricsResponse, err error) { //nolint:funlen // baseline #629
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

// supportsClearScreen returns true if ANSI escape codes should be emitted.
// On Windows, VT processing must be enabled for ANSI codes to work correctly.
// Without VT, escape sequences produce garbage output. We conservatively skip
// the clear on Windows (checking VT mode requires syscall.GetConsoleMode,
// which is Windows-only). On Unix, ANSI codes work reliably.
//
// Note: runtime.GOOS is a compile-time constant, so golangci-lint may flag
// this as "always true/false" on non-Windows runners. The //nolint comment
// below suppresses that false-positive lint warning.
func supportsClearScreen() bool {
	return runtime.GOOS != windowsOS
}
