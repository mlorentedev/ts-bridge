package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ts-bridge/internal/config"
	"ts-bridge/internal/health"
	"ts-bridge/internal/telemetry"
)

// TestConnectionLimit tests that connection limits are enforced via the
// atomic TryClaimConnection helper. Replaced the old check-then-act
// simulation after PR introducing the CAS-based claim path.
func TestConnectionLimit(t *testing.T) {
	telemetry.ResetMetrics()

	cfg := config.Config{
		MaxConnections: 2,
	}

	// Try 5 claims; the helper should admit exactly MaxConnections of them
	// and reject the rest, atomically.
	for range 5 {
		if !telemetry.TryClaimConnection(cfg.MaxConnections) {
			telemetry.AddRejectedConn()
		}
	}

	m := telemetry.GetMetrics()
	if m.RejectedConns != 3 {
		t.Errorf("expected 3 rejected connections, got %d", m.RejectedConns)
	}
	if m.ActiveConnections != 2 {
		t.Errorf("expected 2 active connections, got %d", m.ActiveConnections)
	}
}

// TestEnsureStateDir tests state directory creation with permissions.
func TestEnsureStateDir(t *testing.T) {
	// Initialize logger for test
	initLogger(config.Config{LogFormat: "text"})

	dir := t.TempDir() + "/test-state"

	err := ensureStateDir(dir)
	if err != nil {
		t.Fatalf("ensureStateDir failed: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}

	// Verify permissions (Unix only).
	if runtime.GOOS != "windows" && info.Mode().Perm() != stateDirPerms {
		t.Errorf("permissions = %o, expected %o", info.Mode().Perm(), stateDirPerms)
	}

	// Test idempotency
	err = ensureStateDir(dir)
	if err != nil {
		t.Errorf("second ensureStateDir failed: %v", err)
	}
}

// TestHealthEndpoints tests health server responses.
func TestHealthEndpoints(t *testing.T) {
	// Initialize logger for test
	l := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Find free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	var ready atomic.Bool
	server := health.StartServer(addr, &ready, l)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Test /health/live
	t.Run("liveness endpoint", func(t *testing.T) {
		resp, err := http.Get("http://" + addr + "/health/live")
		if err != nil {
			t.Fatalf("failed to request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"status":"ok"`) {
			t.Errorf("expected status ok in body, got: %s", string(body))
		}
	})

	// Test /health/ready when not ready
	t.Run("readiness not ready", func(t *testing.T) {
		resp, err := http.Get("http://" + addr + "/health/ready")
		if err != nil {
			t.Fatalf("failed to request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"status":"not_ready"`) {
			t.Errorf("expected not_ready in body, got: %s", string(body))
		}
	})

	// Set ready and test again
	ready.Store(true)

	t.Run("readiness ready", func(t *testing.T) {
		resp, err := http.Get("http://" + addr + "/health/ready")
		if err != nil {
			t.Fatalf("failed to request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"status":"ok"`) {
			t.Errorf("expected status ok in body, got: %s", string(body))
		}
	})

	// Test /metrics
	t.Run("metrics endpoint", func(t *testing.T) {
		resp, err := http.Get("http://" + addr + "/metrics")
		if err != nil {
			t.Fatalf("failed to request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "active_connections") {
			t.Errorf("expected active_connections in body, got: %s", string(body))
		}
	})
}

// TestMetricsAtomicity tests that metrics updates are thread-safe.
func TestMetricsAtomicity(t *testing.T) {
	telemetry.ResetMetrics()

	const goroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				telemetry.AddTotalConnection()
				telemetry.AddBytesTx(100)
			}
		}()
	}

	wg.Wait()

	expected := int64(goroutines * iterations)
	m := telemetry.GetMetrics()
	if m.TotalConnections != expected {
		t.Errorf("TotalConnections = %d, expected %d", m.TotalConnections, expected)
	}
	if m.TotalBytesTx != expected*100 {
		t.Errorf("TotalBytesTx = %d, expected %d", m.TotalBytesTx, expected*100)
	}
}

// TestVerboseConfig tests verbose flag handling.
func TestVerboseConfig(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-test123")
	t.Cleanup(func() {
		os.Unsetenv("TS_TARGET")
		os.Unsetenv("TS_AUTHKEY")
	})

	// Test flag
	cfg, err := config.LoadConfig(true)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if !cfg.Verbose {
		t.Error("expected Verbose=true when flag is true")
	}

	// Test env var
	os.Setenv("TS_VERBOSE", "true")

	cfg, err = config.LoadConfig(false)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if !cfg.Verbose {
		t.Error("expected Verbose=true when env var is true")
	}
}

// TestMaxConnectionsConfig tests max connections configuration.
func TestMaxConnectionsConfig(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-test123")
	t.Cleanup(func() {
		os.Unsetenv("TS_TARGET")
		os.Unsetenv("TS_AUTHKEY")
	})

	// Test default
	cfg, err := config.LoadConfig(false)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if cfg.MaxConnections != 1000 {
		t.Errorf("expected default %d, got %d", 1000, cfg.MaxConnections)
	}

	// Test custom
	os.Setenv("TS_MAX_CONNECTIONS", "500")

	cfg, err = config.LoadConfig(false)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if cfg.MaxConnections != 500 {
		t.Errorf("expected 500, got %d", cfg.MaxConnections)
	}

	// Test invalid
	os.Setenv("TS_MAX_CONNECTIONS", "invalid")
	_, err = config.LoadConfig(false)
	if err == nil {
		t.Error("expected error for invalid max connections")
	}
}
