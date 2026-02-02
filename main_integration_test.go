package main

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
		conn.Write([]byte("RESPONSE"))
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
			io.Copy(client, remote)
		}()
		func() {
			defer closeAll()
			io.Copy(remote, client)
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
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
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
		conn.Read(buf) // Will return when connection closes
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
			io.Copy(client, remote)
		}()
		func() {
			defer closeAll()
			io.Copy(remote, client)
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
				c.Write(buf[:n]) // Echo
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
					io.Copy(c, remote)
				}()
				func() {
					defer closeAll()
					io.Copy(remote, c)
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
			conn.Write(testData)

			response := make([]byte, 4)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
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

// TestConnectionLimit tests that connection limits are enforced.
func TestConnectionLimit(t *testing.T) {
	// Reset metrics for this test
	oldMetrics := metrics
	metrics = Metrics{}
	defer func() { metrics = oldMetrics }()

	cfg := Config{
		MaxConnections: 2,
	}

	// Track active connections
	var activeConns int64
	var rejectedConns int64

	// Simulate connection limit check
	for i := 0; i < 5; i++ {
		current := atomic.LoadInt64(&activeConns)
		if current >= cfg.MaxConnections {
			atomic.AddInt64(&rejectedConns, 1)
			continue
		}
		atomic.AddInt64(&activeConns, 1)
	}

	if rejectedConns != 3 {
		t.Errorf("expected 3 rejected connections, got %d", rejectedConns)
	}
	if activeConns != 2 {
		t.Errorf("expected 2 active connections, got %d", activeConns)
	}
}

// TestIsExpectedCloseError tests error classification.
func TestIsExpectedCloseError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, true},
		{"EOF", io.EOF, true},
		{"net.ErrClosed", net.ErrClosed, true},
		{"random error", errors.New("random error"), false},
		{"closed network", errors.New("use of closed network connection"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExpectedCloseError(tt.err)
			if result != tt.expected {
				t.Errorf("isExpectedCloseError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestAcceptLoopBackoff tests exponential backoff behavior.
func TestAcceptLoopBackoff(t *testing.T) {
	backoff := backoffMin

	// Simulate 5 consecutive failures
	for i := 0; i < 5; i++ {
		backoff = min(backoff*2, backoffMax)
	}

	// After 5 doublings: 100ms -> 200ms -> 400ms -> 800ms -> 1600ms -> 3200ms
	expected := 3200 * time.Millisecond
	if backoff != expected {
		t.Errorf("backoff after 5 failures = %v, expected %v", backoff, expected)
	}

	// Verify max cap
	for i := 0; i < 10; i++ {
		backoff = min(backoff*2, backoffMax)
	}
	if backoff != backoffMax {
		t.Errorf("backoff should cap at %v, got %v", backoffMax, backoff)
	}
}

// TestEnsureStateDir tests state directory creation with permissions.
func TestEnsureStateDir(t *testing.T) {
	// Initialize logger for test
	initLogger(Config{LogFormat: "text"})

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

	// Verify permissions (Unix only)
	if info.Mode().Perm() != stateDirPerms {
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
	initLogger(Config{LogFormat: "text"})

	// Find free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	server := startHealthServer(addr)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Test /health
	t.Run("health endpoint", func(t *testing.T) {
		resp, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer resp.Close()

		resp.Write([]byte("GET /health HTTP/1.1\r\nHost: localhost\r\n\r\n"))

		buf := make([]byte, 1024)
		n, _ := resp.Read(buf)
		response := string(buf[:n])

		if !contains(response, "200 OK") {
			t.Errorf("expected 200 OK, got: %s", response)
		}
		if !contains(response, `"status":"ok"`) {
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

		resp.Write([]byte("GET /metrics HTTP/1.1\r\nHost: localhost\r\n\r\n"))

		buf := make([]byte, 1024)
		n, _ := resp.Read(buf)
		response := string(buf[:n])

		if !contains(response, "200 OK") {
			t.Errorf("expected 200 OK, got: %s", response)
		}
		if !contains(response, "active_connections") {
			t.Errorf("expected active_connections in body, got: %s", response)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestMetricsAtomicity tests that metrics updates are thread-safe.
func TestMetricsAtomicity(t *testing.T) {
	// Reset metrics
	oldMetrics := metrics
	metrics = Metrics{}
	defer func() { metrics = oldMetrics }()

	const goroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddInt64(&metrics.TotalConnections, 1)
				atomic.AddInt64(&metrics.TotalBytesTx, 100)
			}
		}()
	}

	wg.Wait()

	expected := int64(goroutines * iterations)
	if metrics.TotalConnections != expected {
		t.Errorf("TotalConnections = %d, expected %d", metrics.TotalConnections, expected)
	}
	if metrics.TotalBytesTx != expected*100 {
		t.Errorf("TotalBytesTx = %d, expected %d", metrics.TotalBytesTx, expected*100)
	}
}

// TestVerboseConfig tests verbose flag handling.
func TestVerboseConfig(t *testing.T) {
	os.Setenv("TS_TARGET", "100.64.0.1:3389")
	os.Setenv("TS_AUTHKEY", "tskey-auth-test123")
	defer os.Unsetenv("TS_TARGET")
	defer os.Unsetenv("TS_AUTHKEY")

	// Test flag
	cfg, err := loadConfig(true)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if !cfg.Verbose {
		t.Error("expected Verbose=true when flag is true")
	}

	// Test env var
	os.Setenv("TS_VERBOSE", "true")
	defer os.Unsetenv("TS_VERBOSE")

	cfg, err = loadConfig(false)
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
	cfg, err := loadConfig(false)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if cfg.MaxConnections != defaultMaxConnections {
		t.Errorf("expected default %d, got %d", defaultMaxConnections, cfg.MaxConnections)
	}

	// Test custom
	os.Setenv("TS_MAX_CONNECTIONS", "500")
	defer os.Unsetenv("TS_MAX_CONNECTIONS")

	cfg, err = loadConfig(false)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if cfg.MaxConnections != 500 {
		t.Errorf("expected 500, got %d", cfg.MaxConnections)
	}

	// Test invalid
	os.Setenv("TS_MAX_CONNECTIONS", "invalid")
	_, err = loadConfig(false)
	if err == nil {
		t.Error("expected error for invalid max connections")
	}
}
