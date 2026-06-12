package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ts-bridge/internal/config"
	"ts-bridge/internal/health"
)

func TestDiagnoseTailscaleInitError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantHint    bool
		wantContain string
	}{
		{
			name:     "nil error returns empty",
			err:      nil,
			wantHint: false,
		},
		{
			name:        "API key does not exist surfaces key remediation",
			err:         errors.New("tsnet.Up: backend: invalid key: API key does not exist"),
			wantHint:    true,
			wantContain: "expired, revoked",
		},
		{
			name:        "invalid key pattern",
			err:         errors.New("control: invalid key supplied"),
			wantHint:    true,
			wantContain: "expired, revoked",
		},
		{
			name:        "key expired pattern",
			err:         errors.New("auth key expired"),
			wantHint:    true,
			wantContain: "expired, revoked",
		},
		{
			name:        "context deadline exceeded surfaces network remediation",
			err:         errors.New("context deadline exceeded"),
			wantHint:    true,
			wantContain: "control plane unreachable",
		},
		{
			name:        "i/o timeout surfaces network remediation",
			err:         errors.New("dial tcp: i/o timeout"),
			wantHint:    true,
			wantContain: "control plane unreachable",
		},
		{
			name:     "unknown error stays silent",
			err:      errors.New("something completely unrelated"),
			wantHint: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hint, remediation := diagnoseTailscaleInitError(tc.err)
			if tc.wantHint {
				if hint == "" {
					t.Fatalf("expected hint, got empty")
				}
				if remediation == "" {
					t.Fatalf("expected remediation, got empty")
				}
				if tc.wantContain != "" && !strings.Contains(hint, tc.wantContain) {
					t.Fatalf("hint %q does not contain %q", hint, tc.wantContain)
				}
				return
			}
			if hint != "" || remediation != "" {
				t.Fatalf("expected silent output, got hint=%q remediation=%q", hint, remediation)
			}
		})
	}
}

func TestHandleShutdown_ClosesListenerAndHealthServer(t *testing.T) {
	initLogger(config.Config{LogFormat: "text"})

	var ready atomic.Bool
	ready.Store(true)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	// Start a real health server.
	healthLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	healthServer := health.StartServer(listener.Addr().String(), &ready, healthLogger)

	// Create an already-cancelled context to trigger shutdown immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Run handleShutdown in goroutine.
	done := make(chan struct{})
	go func() {
		handleShutdown(ctx, &ready, listener, healthServer)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleShutdown did not complete in time")
	}

	// Listener should be closed.
	if err := listener.Close(); err == nil {
		t.Error("expected listener to be closed after handleShutdown, but Close succeeded")
	}

	// Ready flag should be false.
	if ready.Load() {
		t.Error("expected ready flag to be false after handleShutdown")
	}
}

func TestHandleShutdown_NilHealthServerDoesNotPanic(t *testing.T) {
	initLogger(config.Config{LogFormat: "text"})

	var ready atomic.Bool
	ready.Store(true)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		// Must not panic when healthServer is nil.
		handleShutdown(ctx, &ready, listener, nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleShutdown did not complete")
	}
}

func TestDrainActiveConnections_ZeroTimeoutReturnsImmediately(t *testing.T) {
	cfg := config.Config{DrainTimeout: 0}
	var wg sync.WaitGroup

	// This should return immediately without waiting.
	start := time.Now()
	drainActiveConnections(cfg, &wg)
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("zero timeout drain took %v, expected near-instant return", elapsed)
	}
}

func TestDrainActiveConnections_WaitsForActiveConnections(t *testing.T) {
	cfg := config.Config{DrainTimeout: 5 * time.Second}
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		wg.Done()
	}()

	start := time.Now()
	drainActiveConnections(cfg, &wg)
	elapsed := time.Since(start)

	if elapsed < 30*time.Millisecond {
		t.Errorf("drain returned too fast (%v), should have waited for wg.Done", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("drain took too long (%v)", elapsed)
	}
}

func TestDrainActiveConnections_TimesOut(t *testing.T) {
	cfg := config.Config{DrainTimeout: 50 * time.Millisecond}
	var wg sync.WaitGroup

	// Add work that never finishes.
	wg.Add(1)

	start := time.Now()
	drainActiveConnections(cfg, &wg)
	elapsed := time.Since(start)

	// Should have timed out after DrainTimeout, not waited forever.
	if elapsed < 30*time.Millisecond {
		t.Errorf("drain returned too fast (%v), should have waited for timeout", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("drain took too long (%v), expected ~50ms timeout", elapsed)
	}
}

func TestPrintBanner_ContainsExpectedFields(t *testing.T) {
	// Save and restore stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := config.Config{
		Hostname:  "test-host",
		LocalAddr: "127.0.0.1:33389",
		Target:    "100.64.0.1:3389",
	}
	printBanner(cfg)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	expectedParts := []string{
		"TAILSCALE BRIDGE",
		"test-host",
		"127.0.0.1:33389",
		"100.64.0.1:3389",
		"Waiting for connections",
	}
	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("expected banner to contain %q, got:\n%s", part, output)
		}
	}
}

func TestInitLogger_TextFormat(t *testing.T) {
	cfg := config.Config{LogFormat: "text", Verbose: false}
	initLogger(cfg)

	if logger == nil {
		t.Fatal("logger should not be nil after init")
	}

	// Verify level: non-verbose should be Info.
	if !logger.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected Info level enabled")
	}
	if logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected Debug level disabled in non-verbose mode")
	}
}

func TestInitLogger_JSONFormat(t *testing.T) {
	cfg := config.Config{LogFormat: "json", Verbose: false}
	initLogger(cfg)

	if logger == nil {
		t.Fatal("logger should not be nil after init")
	}
}

func TestInitLogger_VerboseEnablesDebug(t *testing.T) {
	cfg := config.Config{LogFormat: "text", Verbose: true}
	initLogger(cfg)

	if !logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected Debug level enabled in verbose mode")
	}
}

func TestIsRetryableCleanupError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error is not retryable", err: nil, want: false},
		{name: "directory not empty", err: fmt.Errorf("remove %s: directory is not empty", "/tmp/foo"), want: true},
		{name: "access is denied", err: fmt.Errorf("remove %s: access is denied", "/tmp/foo"), want: true},
		{name: "being used by another process", err: fmt.Errorf("remove %s: The process cannot access the file because it is being used by another process", "/tmp/foo"), want: true},
		{name: "resource busy", err: fmt.Errorf("remove %s: resource busy", "/tmp/foo"), want: true},
		{name: "device or resource busy", err: fmt.Errorf("remove %s: device or resource busy", "/tmp/foo"), want: true},
		{name: "permission denied is not retryable", err: fmt.Errorf("remove %s: permission denied", "/tmp/foo"), want: false},
		{name: "no such file or directory is not retryable", err: fmt.Errorf("remove %s: no such file or directory", "/tmp/foo"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableCleanupError(tt.err)
			if got != tt.want {
				t.Errorf("isRetryableCleanupError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestCleanupEphemeralStateDir_RemovesDir(t *testing.T) {
	initLogger(config.Config{LogFormat: "text"})

	dir := t.TempDir() + "/ephemeral"
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("create dir: %v", err)
	}

	// Create a file inside to test recursive removal.
	if err := os.WriteFile(dir+"/test.txt", []byte("data"), 0600); err != nil {
		t.Fatalf("create file: %v", err)
	}

	cleanupEphemeralStateDir(dir)

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected dir to be removed, stat err: %v", err)
	}
}

func TestCleanupEphemeralStateDir_NonexistentDirNoError(t *testing.T) {
	initLogger(config.Config{LogFormat: "text"})

	// Should not panic or error on nonexistent dir.
	cleanupEphemeralStateDir("/nonexistent/path/that/should/not/exist")
}

func TestControlURLForError(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty defaults to Tailscale SaaS marker",
			input:    "",
			expected: "https://controlplane.tailscale.com (default)",
		},
		{
			name:     "custom URL is returned as-is",
			input:    "https://vpn.example.com",
			expected: "https://vpn.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := controlURLForError(tc.input); got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

