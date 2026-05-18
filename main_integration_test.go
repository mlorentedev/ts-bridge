package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
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

// TestProxyBidirectionalFlow tests that data flows correctly in both directions.
func TestProxyBidirectionalFlow(t *testing.T) {
	// Create a mock "remote" server
	remoteListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create remote listener: %v", err)
	}
	defer remoteListener.Close()

	// Track data received by remote
	var remoteReceived []byte
	var remoteWg sync.WaitGroup
	remoteWg.Add(1)

	go func() {
		defer remoteWg.Done()
		conn, err := remoteListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Echo server: read data, send it back
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return
		}
		remoteReceived = buf[:n]

		// Send response
		_, _ = conn.Write([]byte("RESPONSE"))
	}()

	// Create local listener (simulating ts-bridge listener)
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create local listener: %v", err)
	}
	defer localListener.Close()

	// Start proxy handler
	go func() {
		client, err := localListener.Accept()
		if err != nil {
			return
		}

		// Connect to "remote"
		remote, err := net.Dial("tcp", remoteListener.Addr().String())
		if err != nil {
			client.Close()
			return
		}

		// Bidirectional copy (simplified version of handleConn)
		var once sync.Once
		closeAll := func() {
			once.Do(func() {
				client.Close()
				remote.Close()
			})
		}

		go func() {
			defer closeAll()
			_, _ = io.Copy(client, remote)
		}()
		func() {
			defer closeAll()
			_, _ = io.Copy(remote, client)
		}()
	}()

	// Connect as client
	conn, err := net.Dial("tcp", localListener.Addr().String())
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	// Send data
	testData := []byte("HELLO")
	_, err = conn.Write(testData)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Read response
	response := make([]byte, 1024)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(response)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("failed to read response: %v", err)
	}

	remoteWg.Wait()

	// Verify data was received by remote
	if string(remoteReceived) != "HELLO" {
		t.Errorf("remote received %q, expected HELLO", remoteReceived)
	}

	// Verify response was received by client
	if string(response[:n]) != "RESPONSE" {
		t.Errorf("client received %q, expected RESPONSE", response[:n])
	}
}

// TestConnectionClosePropagation tests that closing one side closes the other.
func TestConnectionClosePropagation(t *testing.T) {
	remoteListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create remote listener: %v", err)
	}
	defer remoteListener.Close()

	remoteClosed := make(chan struct{})

	go func() {
		conn, err := remoteListener.Accept()
		if err != nil {
			return
		}
		// Wait for close
		buf := make([]byte, 1)
		_, _ = conn.Read(buf) // Will return when connection closes
		close(remoteClosed)
		conn.Close()
	}()

	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create local listener: %v", err)
	}
	defer localListener.Close()

	go func() {
		client, err := localListener.Accept()
		if err != nil {
			return
		}

		remote, err := net.Dial("tcp", remoteListener.Addr().String())
		if err != nil {
			client.Close()
			return
		}

		var once sync.Once
		closeAll := func() {
			once.Do(func() {
				client.Close()
				remote.Close()
			})
		}

		go func() {
			defer closeAll()
			_, _ = io.Copy(client, remote)
		}()
		func() {
			defer closeAll()
			_, _ = io.Copy(remote, client)
		}()
	}()

	conn, err := net.Dial("tcp", localListener.Addr().String())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Close client side
	conn.Close()

	// Remote should close within reasonable time
	select {
	case <-remoteClosed:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("remote connection did not close after client closed")
	}
}

// TestConcurrentConnections tests multiple simultaneous connections.
func TestConcurrentConnections(t *testing.T) {
	remoteListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create remote listener: %v", err)
	}
	defer remoteListener.Close()

	// Handle multiple remote connections
	go func() {
		for {
			conn, err := remoteListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				n, _ := c.Read(buf)
				_, _ = c.Write(buf[:n]) // Echo
			}(conn)
		}
	}()

	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create local listener: %v", err)
	}
	defer localListener.Close()

	// Simple proxy
	go func() {
		for {
			client, err := localListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				remote, err := net.Dial("tcp", remoteListener.Addr().String())
				if err != nil {
					c.Close()
					return
				}

				var once sync.Once
				closeAll := func() {
					once.Do(func() {
						c.Close()
						remote.Close()
					})
				}

				go func() {
					defer closeAll()
					_, _ = io.Copy(c, remote)
				}()
				func() {
					defer closeAll()
					_, _ = io.Copy(remote, c)
				}()
			}(client)
		}
	}()

	// Launch concurrent connections
	const numConns = 50
	var wg sync.WaitGroup
	var successCount int64

	for i := 0; i < numConns; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", localListener.Addr().String())
			if err != nil {
				t.Logf("connection %d failed: %v", id, err)
				return
			}
			defer conn.Close()

			testData := []byte("PING")
			_, _ = conn.Write(testData)

			response := make([]byte, 4)
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := io.ReadFull(conn, response)
			if err != nil {
				t.Logf("connection %d read failed: %v", id, err)
				return
			}

			if string(response[:n]) == "PING" {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if successCount < numConns*90/100 {
		t.Errorf("only %d/%d connections succeeded", successCount, numConns)
	}
}

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
		resp, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer resp.Close()

		_, _ = resp.Write([]byte("GET /health/live HTTP/1.1\r\nHost: localhost\r\n\r\n"))

		buf := make([]byte, 1024)
		n, _ := resp.Read(buf)
		response := string(buf[:n])

		if !strings.Contains(response, "200 OK") {
			t.Errorf("expected 200 OK, got: %s", response)
		}
		if !strings.Contains(response, `"status":"ok"`) {
			t.Errorf("expected status ok in body, got: %s", response)
		}
	})

	// Test /health/ready when not ready
	t.Run("readiness not ready", func(t *testing.T) {
		resp, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer resp.Close()

		_, _ = resp.Write([]byte("GET /health/ready HTTP/1.1\r\nHost: localhost\r\n\r\n"))

		buf := make([]byte, 1024)
		n, _ := resp.Read(buf)
		response := string(buf[:n])

		if !strings.Contains(response, "503") {
			t.Errorf("expected 503, got: %s", response)
		}
		if !strings.Contains(response, `"status":"not_ready"`) {
			t.Errorf("expected not_ready in body, got: %s", response)
		}
	})

	// Set ready and test again
	ready.Store(true)

	t.Run("readiness ready", func(t *testing.T) {
		resp, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer resp.Close()

		_, _ = resp.Write([]byte("GET /health/ready HTTP/1.1\r\nHost: localhost\r\n\r\n"))

		buf := make([]byte, 1024)
		n, _ := resp.Read(buf)
		response := string(buf[:n])

		if !strings.Contains(response, "200 OK") {
			t.Errorf("expected 200 OK, got: %s", response)
		}
		if !strings.Contains(response, `"status":"ok"`) {
			t.Errorf("expected status ok in body, got: %s", response)
		}
	})

	// Test /metrics
	t.Run("metrics endpoint", func(t *testing.T) {
		resp, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer resp.Close()

		_, _ = resp.Write([]byte("GET /metrics HTTP/1.1\r\nHost: localhost\r\n\r\n"))

		buf := make([]byte, 1024)
		n, _ := resp.Read(buf)
		response := string(buf[:n])

		if !strings.Contains(response, "200 OK") {
			t.Errorf("expected 200 OK, got: %s", response)
		}
		if !strings.Contains(response, "active_connections") {
			t.Errorf("expected active_connections in body, got: %s", response)
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
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

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
	defer os.Unsetenv("TS_VERBOSE")

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
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

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
	defer os.Unsetenv("TS_MAX_CONNECTIONS")

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
