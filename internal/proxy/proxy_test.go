package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
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
				DialTimeout:    5 * time.Second,
			}

			clientConn, proxyConn := net.Pipe()
			defer clientConn.Close()

			done := make(chan struct{})
			go func() {
				handleConn(proxyConn, dialer, cfg, nil, logger)
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
		DialTimeout:    5 * time.Second,
		MaxConnections: 1000,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	var wg sync.WaitGroup
	loopDone := make(chan error, 1)
	go func() {
		loopDone <- AcceptLoop(listener, dialer, cfg, &wg, nil, logger)
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

// halfCloseConn wraps a net.Pipe end and records whether CloseWrite was
// invoked. Used to verify proxyConnections half-closes on graceful EOF.
type halfCloseConn struct {
	net.Conn
	closeWriteCalled atomic.Bool
}

func (h *halfCloseConn) CloseWrite() error {
	h.closeWriteCalled.Store(true)
	// Mirror real TCPConn semantics: after CloseWrite the peer sees EOF
	// on its read. With net.Pipe we approximate by fully closing this end
	// — the opposite-direction copy will then see EOF on its own Read.
	return h.Conn.Close()
}

func TestHalfCloseWrite_UnwrapsIdleConn(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	hc := &halfCloseConn{Conn: a}
	wrapped := withIdleTimeout(hc, 100*time.Millisecond)

	if !halfCloseWrite(wrapped) {
		t.Fatal("halfCloseWrite should reach the inner halfCloser through idleConn")
	}
	if !hc.closeWriteCalled.Load() {
		t.Fatal("CloseWrite was not invoked on the underlying conn")
	}
}

func TestHalfCloseWrite_ReturnsFalseForNonHalfCloser(t *testing.T) {
	// net.Pipe ends do NOT implement halfCloser. Verify the fallback path.
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	if halfCloseWrite(a) {
		t.Fatal("halfCloseWrite should return false for non-halfCloser conns")
	}
}

func TestHandleConn_TerminalDialError_CallsCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		Target:      "host:3389",
		DialTimeout: 2 * time.Second,
	}

	termErr := &TerminalDialError{Cause: errors.New("tsnet: backend in state Stopped")}
	dialer := &mockDialer{
		dialFunc: func(_ context.Context, _, _ string) (net.Conn, error) {
			return nil, termErr
		},
	}

	var cancelCalled atomic.Bool
	var cancelledWith error
	cancel := func(cause error) {
		cancelCalled.Store(true)
		cancelledWith = cause
	}

	clientConn, proxyConn := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		handleConn(proxyConn, dialer, cfg, cancel, logger)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleConn did not return")
	}

	if !cancelCalled.Load() {
		t.Error("expected cancel to be called on TerminalDialError, but it was not")
	}
	var got *TerminalDialError
	if !errors.As(cancelledWith, &got) {
		t.Errorf("expected cancel called with *TerminalDialError, got %T: %v", cancelledWith, cancelledWith)
	}
}

func TestHandleConn_TransientDialError_DoesNotCallCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		Target:      "host:3389",
		DialTimeout: 2 * time.Second,
	}

	dialer := &mockDialer{
		dialFunc: func(_ context.Context, _, _ string) (net.Conn, error) {
			return nil, io.EOF // transient, not a TerminalDialError
		},
	}

	var cancelCalled atomic.Bool
	cancel := func(_ error) { cancelCalled.Store(true) }

	_, proxyConn := net.Pipe()

	done := make(chan struct{})
	go func() {
		handleConn(proxyConn, dialer, cfg, cancel, logger)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleConn did not return")
	}

	if cancelCalled.Load() {
		t.Error("cancel should NOT be called for transient dial errors")
	}
}

func TestProxyConnections_HalfClosesOnEOF(t *testing.T) {
	// Server pair: what the proxy sees as "client" (a) and what tests
	// drive (aPeer). Same for remote.
	a, aPeer := net.Pipe()
	r, rPeer := net.Pipe()
	defer aPeer.Close()
	defer rPeer.Close()

	clientHC := &halfCloseConn{Conn: a}
	remoteHC := &halfCloseConn{Conn: r}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	done := make(chan struct{})
	go func() {
		_, _ = proxyConnections(clientHC, remoteHC, "test", logger)
		close(done)
	}()

	// Send one byte from the remote side, then EOF that direction.
	go func() {
		_, _ = rPeer.Write([]byte("X"))
		_ = rPeer.Close()
	}()

	// Drain whatever arrives on the client peer; once the EOF propagates
	// via half-close, the read returns and we let the other direction wind down.
	buf := make([]byte, 16)
	_ = aPeer.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _ = aPeer.Read(buf)
	_ = aPeer.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("proxyConnections did not return after both directions ended")
	}

	if !clientHC.closeWriteCalled.Load() && !remoteHC.closeWriteCalled.Load() {
		t.Error("expected at least one CloseWrite invocation on graceful EOF; none recorded")
	}
}

