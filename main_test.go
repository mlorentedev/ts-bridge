package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// mockDialer implements Dialer for testing without tsnet.
type mockDialer struct {
	dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)
}

func (m *mockDialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	return m.dialFunc(ctx, network, addr)
}

// TestDialerInterfaceSatisfaction verifies that mockDialer satisfies the Dialer interface.
// This is a compile-time check: if Dialer doesn't exist or has a different signature, this fails.
var _ Dialer = (*mockDialer)(nil)

func TestHandleConnWithDialer(t *testing.T) {
	initLogger(Config{LogFormat: "text"})

	tests := []struct {
		name          string
		dialFunc      func(ctx context.Context, network, addr string) (net.Conn, error)
		wantErrors    int64
		wantTotalConn int64
	}{
		{
			name: "successful proxy via dialer",
			dialFunc: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Return a pipe that immediately closes (simulates short-lived connection)
				server, client := net.Pipe()
				go func() {
					// Echo one read then close
					buf := make([]byte, 1024)
					n, _ := server.Read(buf)
					if n > 0 {
						_, _ = server.Write(buf[:n])
					}
					server.Close()
				}()
				return client, nil
			},
			wantErrors:    0,
			wantTotalConn: 1,
		},
		{
			name: "dial failure increments errors",
			dialFunc: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return nil, errors.New("connection refused")
			},
			wantErrors:    1,
			wantTotalConn: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset metrics
			oldMetrics := metrics
			metrics = Metrics{}
			defer func() { metrics = oldMetrics }()

			dialer := &mockDialer{dialFunc: tt.dialFunc}
			cfg := Config{
				Target:         "100.64.0.1:3389",
				ConnectTimeout: 5 * time.Second,
			}

			// Create a client connection via pipe
			clientConn, proxyConn := net.Pipe()
			defer clientConn.Close()

			// Run handleConn in goroutine (it blocks until proxy finishes)
			done := make(chan struct{})
			go func() {
				handleConn(proxyConn, dialer, cfg)
				close(done)
			}()

			if tt.wantErrors == 0 {
				// Send data through the proxy
				_, _ = clientConn.Write([]byte("HELLO"))

				buf := make([]byte, 1024)
				_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
				n, err := clientConn.Read(buf)
				if err != nil && !errors.Is(err, io.EOF) {
					t.Fatalf("read from proxy failed: %v", err)
				}
				if n > 0 && string(buf[:n]) != "HELLO" {
					t.Errorf("expected echo HELLO, got %q", buf[:n])
				}
			}

			// Close client side to let handleConn finish
			clientConn.Close()

			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("handleConn did not finish in time")
			}

			gotErrors := atomic.LoadInt64(&metrics.TotalErrors)
			if gotErrors != tt.wantErrors {
				t.Errorf("TotalErrors = %d, want %d", gotErrors, tt.wantErrors)
			}

			gotTotal := atomic.LoadInt64(&metrics.TotalConnections)
			if gotTotal != tt.wantTotalConn {
				t.Errorf("TotalConnections = %d, want %d", gotTotal, tt.wantTotalConn)
			}
		})
	}
}

func TestAcceptLoopWithDialer(t *testing.T) {
	initLogger(Config{LogFormat: "text"})

	// Snapshot metrics before test to check delta after
	connsBefore := atomic.LoadInt64(&metrics.TotalConnections)

	// Mock dialer that echoes data
	dialer := &mockDialer{
		dialFunc: func(ctx context.Context, network, addr string) (net.Conn, error) {
			server, client := net.Pipe()
			go func() {
				defer server.Close()
				buf := make([]byte, 1024)
				n, _ := server.Read(buf)
				if n > 0 {
					_, _ = server.Write(buf[:n])
				}
			}()
			return client, nil
		},
	}

	cfg := Config{
		Target:         "100.64.0.1:3389",
		ConnectTimeout: 5 * time.Second,
		MaxConnections: 1000,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	// Run accept loop in background
	loopDone := make(chan error, 1)
	go func() {
		loopDone <- acceptLoop(listener, dialer, cfg)
	}()

	// Connect a client through the accept loop
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	_, _ = conn.Write([]byte("TEST"))

	buf := make([]byte, 1024)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("read failed: %v", err)
	}
	if string(buf[:n]) != "TEST" {
		t.Errorf("expected TEST, got %q", buf[:n])
	}
	conn.Close()

	// Close listener to stop accept loop
	listener.Close()

	select {
	case err := <-loopDone:
		if err != nil {
			t.Errorf("acceptLoop returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("acceptLoop did not stop")
	}

	// Check that at least one connection was handled (use atomic reads, no struct reset)
	connsAfter := atomic.LoadInt64(&metrics.TotalConnections)
	if connsAfter <= connsBefore {
		t.Errorf("expected TotalConnections to increase, before=%d after=%d", connsBefore, connsAfter)
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		verbose bool
		wantErr bool
		check   func(t *testing.T, cfg Config)
	}{
		{
			name:    "valid config with defaults",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.Target != "100.64.0.1:3389" {
					t.Errorf("expected target 100.64.0.1:3389, got %s", cfg.Target)
				}
				if !cfg.AutoInstance {
					t.Error("expected auto instance mode enabled by default")
				}
				if !strings.HasPrefix(cfg.LocalAddr, "127.0.0.1:") {
					t.Errorf("expected auto local loopback addr, got %s", cfg.LocalAddr)
				}
				if !strings.HasPrefix(cfg.Hostname, "tsb-") {
					t.Errorf("expected auto hostname with tsb- prefix, got %s", cfg.Hostname)
				}
				if !cfg.EphemeralState {
					t.Error("expected default mode to use ephemeral state")
				}
				if !strings.Contains(cfg.StateDir, "ts-bridge") {
					t.Errorf("expected auto state dir under ts-bridge path, got %s", cfg.StateDir)
				}
				if cfg.ConnectTimeout != 30*time.Second {
					t.Errorf("expected 30s timeout, got %v", cfg.ConnectTimeout)
				}
			},
		},
		{
			name: "manual mode restores legacy defaults",
			env: map[string]string{
				"TS_TARGET":      "100.64.0.1:3389",
				"TS_AUTHKEY":     "tskey-auth-test123",
				"TS_MANUAL_MODE": "true",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.AutoInstance {
					t.Error("expected manual mode to disable auto mode")
				}
				if cfg.LocalAddr != defaultLocalAddr {
					t.Errorf("expected legacy local addr, got %s", cfg.LocalAddr)
				}
				if cfg.Hostname != "ts-bridge" {
					t.Errorf("expected legacy hostname, got %s", cfg.Hostname)
				}
				if cfg.StateDir != "./ts-state" {
					t.Errorf("expected legacy state dir, got %s", cfg.StateDir)
				}
				if cfg.EphemeralState {
					t.Error("expected legacy mode to keep persistent state")
				}
			},
		},
		{
			name: "explicit auto flag false restores legacy defaults",
			env: map[string]string{
				"TS_TARGET":        "100.64.0.1:3389",
				"TS_AUTHKEY":       "tskey-auth-test123",
				"TS_AUTO_INSTANCE": "false",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.AutoInstance {
					t.Error("expected explicit false auto flag to disable auto mode")
				}
				if cfg.LocalAddr != defaultLocalAddr {
					t.Errorf("expected legacy local addr, got %s", cfg.LocalAddr)
				}
			},
		},
		{
			name:    "custom timeout",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123", "TS_TIMEOUT": "1m30s"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.ConnectTimeout != 90*time.Second {
					t.Errorf("expected timeout 1m30s, got %v", cfg.ConnectTimeout)
				}
			},
		},
		{
			name:    "control URL unset defaults to empty",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.ControlURL != "" {
					t.Errorf("expected empty ControlURL, got %q", cfg.ControlURL)
				}
			},
		},
		{
			name:    "control URL set to headscale",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123", "TS_CONTROL_URL": "https://vpn.example.com"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.ControlURL != "https://vpn.example.com" {
					t.Errorf("expected https://vpn.example.com, got %q", cfg.ControlURL)
				}
			},
		},
		{
			name: "auto instance mode derives runtime values",
			env: map[string]string{
				"TS_TARGET":        "100.64.0.1:3389",
				"TS_AUTHKEY":       "tskey-auth-test123",
				"TS_AUTO_INSTANCE": "true",
				"TS_INSTANCE_NAME": "office-laptop",
				"TS_PORT_RANGE":    "61000-61100",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if !cfg.AutoInstance {
					t.Error("expected auto instance mode to be enabled")
				}
				if !strings.HasPrefix(cfg.LocalAddr, "127.0.0.1:") {
					t.Errorf("expected loopback local addr, got %s", cfg.LocalAddr)
				}
				if !strings.HasPrefix(cfg.Hostname, "tsb-") {
					t.Errorf("expected generated hostname with tsb- prefix, got %s", cfg.Hostname)
				}
				if !cfg.EphemeralState {
					t.Error("expected generated state directory to be ephemeral")
				}
				if !strings.Contains(cfg.StateDir, "ts-bridge") {
					t.Errorf("expected state dir to include ts-bridge, got %s", cfg.StateDir)
				}
			},
		},
		{
			name: "auto instance mode keeps explicit values",
			env: map[string]string{
				"TS_TARGET":        "100.64.0.1:3389",
				"TS_AUTHKEY":       "tskey-auth-test123",
				"TS_AUTO_INSTANCE": "1",
				"TS_LOCAL_ADDR":    "127.0.0.1:40001",
				"TS_HOSTNAME":      "manual-host",
				"TS_STATE_DIR":     "./custom-state",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.LocalAddr != "127.0.0.1:40001" {
					t.Errorf("expected explicit local addr, got %s", cfg.LocalAddr)
				}
				if cfg.Hostname != "manual-host" {
					t.Errorf("expected explicit hostname, got %s", cfg.Hostname)
				}
				if cfg.StateDir != "./custom-state" {
					t.Errorf("expected explicit state dir, got %s", cfg.StateDir)
				}
				if cfg.EphemeralState {
					t.Error("expected explicit state dir to disable ephemeral cleanup")
				}
			},
		},
		{
			name: "auto instance mode invalid port range",
			env: map[string]string{
				"TS_TARGET":        "100.64.0.1:3389",
				"TS_AUTHKEY":       "tskey-auth-test123",
				"TS_AUTO_INSTANCE": "true",
				"TS_PORT_RANGE":    "bad-range",
			},
			wantErr: true,
		},
		{
			name: "manual mode overrides explicit auto mode flag",
			env: map[string]string{
				"TS_TARGET":        "100.64.0.1:3389",
				"TS_AUTHKEY":       "tskey-auth-test123",
				"TS_AUTO_INSTANCE": "true",
				"TS_MANUAL_MODE":   "true",
			},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.AutoInstance {
					t.Error("expected manual mode to take precedence over auto flag")
				}
				if cfg.LocalAddr != defaultLocalAddr {
					t.Errorf("expected legacy local addr, got %s", cfg.LocalAddr)
				}
			},
		},
		{
			name:    "missing target",
			env:     map[string]string{"TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "missing auth key",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389"},
			wantErr: true,
		},
		{
			name:    "invalid auth key format",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "invalid-key-format"},
			wantErr: true,
		},
		{
			name:    "headscale auth key accepted",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "hskey-auth-test123"},
			wantErr: false,
			check: func(t *testing.T, cfg Config) {
				if cfg.AuthKey != "hskey-auth-test123" {
					t.Errorf("expected hskey-auth-test123, got %q", cfg.AuthKey)
				}
			},
		},
		{
			name:    "invalid timeout",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:3389", "TS_AUTHKEY": "tskey-auth-test123", "TS_TIMEOUT": "invalid"},
			wantErr: true,
		},
		{
			name:    "target no port",
			env:     map[string]string{"TS_TARGET": "100.64.0.1", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "target empty host",
			env:     map[string]string{"TS_TARGET": ":3389", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "target invalid port",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:abc", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "target port too high",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:99999", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "target port zero",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:0", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
		{
			name:    "target negative port",
			env:     map[string]string{"TS_TARGET": "100.64.0.1:-1", "TS_AUTHKEY": "tskey-auth-test123"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all config env vars
			for _, key := range []string{"TS_TARGET", "TS_AUTHKEY", "TS_TIMEOUT", "TS_VERBOSE",
				"TS_LOCAL_ADDR", "TS_HOSTNAME", "TS_STATE_DIR", "TS_CONTROL_URL",
				"TS_MAX_CONNECTIONS", "TS_HEALTH_ADDR", "TS_LOG_FORMAT",
				"TS_AUTO_INSTANCE", "TS_INSTANCE_NAME", "TS_PORT_RANGE", "TS_MANUAL_MODE"} {
				os.Unsetenv(key)
			}
			// Set test-specific env vars
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			cfg, err := loadConfig(tt.verbose)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestInitLogger(t *testing.T) {
	oldLogger := logger
	defer func() { logger = oldLogger }()

	tests := []struct {
		name        string
		cfg         Config
		wantHandler string
		wantLevel   slog.Level
	}{
		{
			name:        "default text handler",
			cfg:         Config{LogFormat: "text"},
			wantHandler: "*slog.TextHandler",
			wantLevel:   slog.LevelInfo,
		},
		{
			name:        "json handler",
			cfg:         Config{LogFormat: "json"},
			wantHandler: "*slog.JSONHandler",
			wantLevel:   slog.LevelInfo,
		},
		{
			name:        "verbose enables debug level",
			cfg:         Config{LogFormat: "text", Verbose: true},
			wantHandler: "*slog.TextHandler",
			wantLevel:   slog.LevelDebug,
		},
		{
			name:        "unknown format falls back to text",
			cfg:         Config{LogFormat: "yaml"},
			wantHandler: "*slog.TextHandler",
			wantLevel:   slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initLogger(tt.cfg)

			handlerType := fmt.Sprintf("%T", logger.Handler())
			if handlerType != tt.wantHandler {
				t.Errorf("handler type = %s, want %s", handlerType, tt.wantHandler)
			}

			if !logger.Handler().Enabled(context.Background(), tt.wantLevel) {
				t.Errorf("expected level %v to be enabled", tt.wantLevel)
			}
		})
	}
}

func TestEnvOr(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envValue string
		setEnv   bool
		fallback string
		want     string
	}{
		{"fallback when unset", "TEST_VAR_NOT_SET", "", false, "default", "default"},
		{"env value when set", "TEST_VAR_SET", "custom", true, "default", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(tt.key)
			if tt.setEnv {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}
			if got := envOr(tt.key, tt.fallback); got != tt.want {
				t.Errorf("envOr(%q, %q) = %q, want %q", tt.key, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestIsRetryableCleanupError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "directory not empty", err: errors.New("The directory is not empty."), want: true},
		{name: "access denied", err: errors.New("Access is denied."), want: true},
		{name: "resource busy", err: errors.New("device or resource busy"), want: true},
		{name: "non retryable", err: errors.New("invalid argument"), want: false},
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
