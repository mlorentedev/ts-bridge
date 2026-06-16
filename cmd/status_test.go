package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// newStatusCommand creates an isolated status command for testing.
func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show bridge health and metrics summary",
		RunE:  runStatus,
	}
	cmd.Flags().String("addr", "127.0.0.1:9090", "Health server address")
	cmd.Flags().Bool("json", false, "Output raw JSON from /metrics")
	cmd.Flags().BoolP("watch", "w", false, "Continuously watch and update status")
	cmd.Flags().DurationP("interval", "i", 5*time.Second, "Polling interval for --watch")
	return cmd
}

func TestStatusCommandNotRunning(t *testing.T) {
	// When the bridge is not running, status should print a message to stderr
	// and return nil (not an error exit).
	var buf bytes.Buffer
	err := runStatusWithWriter("127.0.0.1:1", false, &buf)
	if err != nil {
		t.Fatalf("expected nil error when bridge not running, got: %v", err)
	}
	// Output should be empty (message went to stderr).
	if buf.Len() != 0 {
		t.Errorf("expected empty stdout when bridge not running, got:\n%s", buf.String())
	}
}

func TestStatusCommandJSONDirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{
			"active_connections":  2,
			"total_connections":   10,
			"total_bytes_tx":      1024,
			"total_bytes_rx":      2048,
			"total_errors":        0,
			"rejected_connections": 1,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	var buf bytes.Buffer
	if err := runStatusWithWriter(addr, true, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "active_connections") {
		t.Errorf("expected JSON output with 'active_connections', got:\n%s", output)
	}
}

func TestStatusCommandHumanReadable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{
			"active_connections":  3,
			"total_connections":   42,
			"total_bytes_tx":      1048576,
			"total_bytes_rx":      524288,
			"total_errors":        1,
			"rejected_connections": 0,
		})
	})
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	var buf bytes.Buffer
	if err := runStatusWithWriter(addr, false, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	expected := []string{
		"Active connections",
		"Total connections",
		"Bytes sent",
		"Bytes received",
		"Errors",
	}
	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestStatusCommandHealthCheck(t *testing.T) {
	ready := atomic.Bool{}
	ready.Store(true)

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{})
	})
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_ready"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	var buf bytes.Buffer
	if err := runStatusWithWriter(addr, false, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "RUNNING") {
		t.Errorf("expected 'RUNNING' status, got:\n%s", output)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{1024, "1.0 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
		{1536, "1.5 KiB"},
		{1023, "1023 B"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "0s"},
		{5 * time.Second, "5s"},
		{90 * time.Second, "1m30s"},
		{2*time.Hour + 30*time.Minute, "2h30m0s"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.input)
		if got != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestStatusWatchMode(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{
			"active_connections":  int64(callCount),
			"total_connections":   10,
			"total_bytes_tx":      1024,
			"total_bytes_rx":      2048,
			"total_errors":        0,
			"rejected_connections": 0,
		})
	})
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	cmd := newStatusCommand()
	cmd.SetArgs([]string{
		"--addr", addr,
		"--watch",
		"--interval", "200ms",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Send SIGINT after a short delay to stop the watch loop.
	stop := make(chan struct{})
	go func() {
		time.Sleep(600 * time.Millisecond)
		// Send SIGINT via syscall to trigger the signal handler in runWatch.
		pid := os.Getpid()
		syscall.Kill(pid, syscall.SIGINT)
		close(stop)
	}()

	err := cmd.Execute()

	// Watch mode returns nil on graceful shutdown (SIGINT).
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Assert: multiple poll cycles occurred (at least 2 ticks + initial).
	// Each non-JSON tick produces a status block with "RUNNING".
	lines := strings.Split(output, "\n")
	runningCount := 0
	for _, line := range lines {
		if strings.Contains(line, "RUNNING") {
			runningCount++
		}
	}
	if runningCount < 2 {
		t.Errorf("expected at least 2 RUNNING status lines (multiple polls), got %d\noutput:\n%s", runningCount, output)
	}

	// Assert: callCount incremented across multiple polls.
	if callCount < 2 {
		t.Errorf("expected at least 2 metric fetches across watch polls, got %d", callCount)
	}

	// Assert: output contains expected metrics fields.
	if !strings.Contains(output, "Active connections") {
		t.Errorf("expected output to contain 'Active connections', got:\n%s", output)
	}
	if !strings.Contains(output, "Bytes sent") {
		t.Errorf("expected output to contain 'Bytes sent', got:\n%s", output)
	}

	// Assert: graceful shutdown message appears on stderr (we can't capture
	// stderr in this test, but we verify the function returned nil — the
	// signal handler path — confirming clean exit without panic).
	<-stop
}
