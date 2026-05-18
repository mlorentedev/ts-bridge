package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"ts-bridge/internal/config"
	"ts-bridge/internal/telemetry"
)

// mockDialer implements Dialer for testing without tsnet.
type mockDialer struct {
	dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)
}

func (m *mockDialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	return m.dialFunc(ctx, network, addr)
}

var _ Dialer = (*mockDialer)(nil)

func TestHandleConnWithDialer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name          string
		dialFunc      func(ctx context.Context, network, addr string) (net.Conn, error)
		wantErrors    int64
		wantTotalConn int64
	}{
		{
			name: "successful proxy via dialer",
			dialFunc: func(ctx context.Context, network, addr string) (net.Conn, error) {
				server, client := net.Pipe()
				go func() {
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
			telemetry.ResetMetrics()

			dialer := &mockDialer{dialFunc: tt.dialFunc}
			cfg := config.Config{
				Target:         "100.64.0.1:3389",
				ConnectTimeout: 5 * time.Second,
			}

			clientConn, proxyConn := net.Pipe()
			defer clientConn.Close()

			done := make(chan struct{})
			go func() {
				handleConn(proxyConn, dialer, cfg, logger)
				close(done)
			}()

			if tt.wantErrors == 0 {
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

			clientConn.Close()

			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("handleConn did not finish in time")
			}

			m := telemetry.GetMetrics()
			if m.TotalErrors != tt.wantErrors {
				t.Errorf("TotalErrors = %d, want %d", m.TotalErrors, tt.wantErrors)
			}

			if m.TotalConnections != tt.wantTotalConn {
				t.Errorf("TotalConnections = %d, want %d", m.TotalConnections, tt.wantTotalConn)
			}
		})
	}
}

func TestAcceptLoopWithDialer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	telemetry.ResetMetrics()

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

	cfg := config.Config{
		Target:         "100.64.0.1:3389",
		ConnectTimeout: 5 * time.Second,
		MaxConnections: 1000,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	var wg sync.WaitGroup
	loopDone := make(chan error, 1)
	go func() {
		loopDone <- AcceptLoop(listener, dialer, cfg, &wg, logger)
	}()

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
	listener.Close()

	select {
	case err := <-loopDone:
		if err != nil {
			t.Errorf("acceptLoop returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("acceptLoop did not stop")
	}

	m := telemetry.GetMetrics()
	if m.TotalConnections <= 0 {
		t.Errorf("expected TotalConnections to increase")
	}
}

func TestIsExpectedCloseError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: true},
		{name: "EOF", err: io.EOF, want: true},
		{name: "net.ErrClosed", err: net.ErrClosed, want: true},
		{name: "random error", err: errors.New("some error"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsExpectedCloseError(tt.err)
			if got != tt.want {
				t.Errorf("IsExpectedCloseError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestWithIdleTimeoutDisabled(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	wrapped := withIdleTimeout(a, 0)
	if wrapped != a {
		t.Fatal("withIdleTimeout(c, 0) should return the original conn unchanged")
	}
}

func TestWithIdleTimeoutEnabled(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	wrapped := withIdleTimeout(a, 100*time.Millisecond)
	if _, ok := wrapped.(*idleConn); !ok {
		t.Fatalf("expected *idleConn, got %T", wrapped)
	}
}

func TestIdleConnReadTimesOutWithoutActivity(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	idle := 80 * time.Millisecond
	wrapped := withIdleTimeout(a, idle)

	start := time.Now()
	buf := make([]byte, 16)
	_, err := wrapped.Read(buf)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("expected net.Error with Timeout()==true, got %T %v", err, err)
	}
	if elapsed < idle {
		t.Errorf("Read returned in %v, expected at least %v", elapsed, idle)
	}
	if elapsed > 3*idle {
		t.Errorf("Read took too long: %v (expected ~%v)", elapsed, idle)
	}
}

func TestIdleConnReadSucceedsWithTraffic(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	wrapped := withIdleTimeout(a, 200*time.Millisecond)

	want := []byte("ping")
	go func() {
		// Write before the deadline fires.
		time.Sleep(20 * time.Millisecond)
		_, _ = b.Write(want)
	}()

	buf := make([]byte, 16)
	n, err := wrapped.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(buf[:n]) != string(want) {
		t.Errorf("read %q, want %q", buf[:n], want)
	}
}

func TestIsIdleTimeoutErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "random error", err: errors.New("boom"), want: false},
		{name: "EOF is not timeout", err: io.EOF, want: false},
		{name: "real deadline error from net.Pipe", err: pipeDeadlineErr(t), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isIdleTimeoutErr(tt.err)
			if got != tt.want {
				t.Errorf("isIdleTimeoutErr(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// pipeDeadlineErr produces a real timeout error by reading from a net.Pipe
// with an expired deadline. Tests rely on real errors rather than synthetic
// stand-ins so the matcher stays honest against stdlib behavior.
func pipeDeadlineErr(t *testing.T) error {
	t.Helper()
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	_ = a.SetReadDeadline(time.Now().Add(-1 * time.Second))
	_, err := a.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("expected deadline error from net.Pipe")
	}
	return err
}

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

