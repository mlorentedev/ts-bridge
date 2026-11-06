package cmd

import (
	"bytes"
	"context"
	"errors"
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

// fakeNode stands in for the tsnet server: every dial reaches an in-process
// echo peer, and Close is recorded so a test can assert when it happened.
type fakeNode struct {
	closed chan struct{}
	once   sync.Once
}

func newFakeNode() *fakeNode { return &fakeNode{closed: make(chan struct{})} }

func (f *fakeNode) Dial(context.Context, string, string) (net.Conn, error) {
	local, peer := net.Pipe()
	go func() {
		defer peer.Close()
		_, _ = io.Copy(peer, peer)
	}()
	return local, nil
}

func (f *fakeNode) Close() error {
	f.once.Do(func() { close(f.closed) })
	return nil
}

func (f *fakeNode) isClosed() bool {
	select {
	case <-f.closed:
		return true
	default:
		return false
	}
}

// useFakeNode swaps the node starter and a discarding logger for the test.
func useFakeNode(t *testing.T, node tailnetNode, startErr error) {
	t.Helper()
	oldStarter, oldLogger := tailnetNodeStarter, logger
	t.Cleanup(func() { tailnetNodeStarter, logger = oldStarter, oldLogger })
	tailnetNodeStarter = func(context.Context, config.Config) (tailnetNode, error) {
		if startErr != nil {
			return nil, startErr
		}
		return node, nil
	}
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

// startBridge runs runBridge against node and returns once the listener is
// accepting, along with the cancel function and the channel the result lands on.
func startBridge(t *testing.T, cfg config.Config) (net.Conn, context.CancelCauseFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancelCause(context.Background())
	t.Cleanup(func() { cancel(nil) })
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runBridge(ctx, cancel, cfg, func() error { close(ready); return nil })
	}()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("runBridge returned before it was ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("runBridge was not ready in time")
	}

	conn, err := net.Dial("tcp", cfg.LocalAddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, 5)
	if _, err := io.ReadFull(conn, echo); err != nil || string(echo) != "hello" {
		t.Fatalf("echo through the bridge = %q, %v", echo, err)
	}
	return conn, cancel, done
}

func bridgeConfig(t *testing.T, drain time.Duration) config.Config {
	t.Helper()
	return config.Config{
		LocalAddr:    freeLoopbackAddr(t),
		Target:       "mesh-host:22",
		StateDir:     t.TempDir(),
		Quiet:        true,
		DrainTimeout: drain,
		DialTimeout:  time.Second,
		// A zero limit rejects every connection.
		MaxConnections: 100,
	}
}

// Canceling the run must stop accepting, wait for the connections already
// open, and only then close the node: closing it first would cut a live session.
func TestRunBridge_DrainsOnCancel(t *testing.T) {
	node := newFakeNode()
	useFakeNode(t, node, nil)

	conn, cancel, done := startBridge(t, bridgeConfig(t, 10*time.Second))

	cancel(nil)

	select {
	case err := <-done:
		t.Fatalf("runBridge returned with a connection still open: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if node.isClosed() {
		t.Fatal("node was closed while a connection was still draining")
	}
	if c, err := net.DialTimeout("tcp", conn.RemoteAddr().String(), 200*time.Millisecond); err == nil {
		_ = c.Close()
		t.Fatal("listener still accepts after cancel")
	}

	_ = conn.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runBridge error after a clean drain = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runBridge did not return after the last connection closed")
	}
	if !node.isClosed() {
		t.Fatal("node was not closed after the drain")
	}
}

func TestRunBridge_DrainTimeoutForcesShutdown(t *testing.T) {
	node := newFakeNode()
	useFakeNode(t, node, nil)
	const drain = 200 * time.Millisecond

	_, cancel, done := startBridge(t, bridgeConfig(t, drain))

	start := time.Now()
	cancel(nil)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runBridge did not give up on the open connection")
	}
	if elapsed := time.Since(start); elapsed < drain {
		t.Errorf("runBridge returned after %v, before the %v drain timeout", elapsed, drain)
	}
	if !node.isClosed() {
		t.Error("node was not closed after the drain timeout")
	}
}

func TestRunBridge_NodeStartFailure(t *testing.T) {
	startErr := errors.New("control plane unreachable")
	terminal := errors.New("ssh bootstrap died")
	tests := []struct {
		name  string
		cause error
		want  error
	}{
		{"start error is returned", nil, startErr},
		{"run cause wins over the start error", terminal, terminal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useFakeNode(t, nil, startErr)
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			if tt.cause != nil {
				cancel(tt.cause)
			}

			err := runBridge(ctx, cancel, bridgeConfig(t, 0), nil)

			if !errors.Is(err, tt.want) {
				t.Fatalf("runBridge error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCleanupEphemeralStateDir_RetriesThenWarns(t *testing.T) {
	busy := errors.New("remove state: directory is not empty")
	tests := []struct {
		name         string
		failures     []error // returned by successive RemoveAll calls; then nil
		wantCalls    int
		wantSleeps   []time.Duration
		wantLogParts []string
	}{
		{
			name:         "retryable error recovers",
			failures:     []error{busy, busy},
			wantCalls:    3,
			wantSleeps:   []time.Duration{cleanupRetryDelay, 2 * cleanupRetryDelay},
			wantLogParts: []string{"cleaned up ephemeral state directory", "attempt=3"},
		},
		{
			name:         "retryable error exhausts the attempts and warns",
			failures:     []error{busy, busy, busy, busy, busy, busy},
			wantCalls:    cleanupMaxAttempts,
			wantSleeps:   []time.Duration{cleanupRetryDelay, 2 * cleanupRetryDelay, 3 * cleanupRetryDelay, 4 * cleanupRetryDelay},
			wantLogParts: []string{"level=WARN", "failed to cleanup ephemeral state directory", "attempts=5"},
		},
		{
			name:         "permanent error warns at once",
			failures:     []error{errors.New("permission denied")},
			wantCalls:    1,
			wantLogParts: []string{"level=WARN", "attempts=1", "permission denied"},
		},
		{
			name:         "missing directory is a success",
			failures:     []error{os.ErrNotExist},
			wantCalls:    1,
			wantLogParts: []string{"cleaned up ephemeral state directory"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldRemove, oldSleep, oldLogger := removeAllFn, cleanupSleep, logger
			t.Cleanup(func() { removeAllFn, cleanupSleep, logger = oldRemove, oldSleep, oldLogger })
			var buf bytes.Buffer
			logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			calls := 0
			removeAllFn = func(string) error {
				calls++
				if calls <= len(tt.failures) {
					return tt.failures[calls-1]
				}
				return nil
			}
			var sleeps []time.Duration
			cleanupSleep = func(d time.Duration) { sleeps = append(sleeps, d) }

			cleanupEphemeralStateDir("/state/dir")

			if calls != tt.wantCalls {
				t.Errorf("RemoveAll called %d times, want %d", calls, tt.wantCalls)
			}
			if len(sleeps) != len(tt.wantSleeps) {
				t.Fatalf("slept %v, want %v", sleeps, tt.wantSleeps)
			}
			for i := range sleeps {
				if sleeps[i] != tt.wantSleeps[i] {
					t.Errorf("sleep %d = %v, want %v", i, sleeps[i], tt.wantSleeps[i])
				}
			}
			for _, part := range tt.wantLogParts {
				if !strings.Contains(buf.String(), part) {
					t.Errorf("log %q should contain %q", buf.String(), part)
				}
			}
		})
	}
}

func TestHandleShutdown_RecordsTheCauseAndClosesTheListener(t *testing.T) {
	tests := []struct {
		name       string
		cause      error
		wantReason string
	}{
		{"clean cancel", nil, ""},
		{"terminal cause", errors.New("tsnet session lost"), "tsnet session lost"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldLogger := logger
			t.Cleanup(func() { logger = oldLogger })
			logger = slog.New(slog.NewTextHandler(io.Discard, nil))
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			var status health.TunnelStatus
			status.MarkReady()
			ctx, cancel := context.WithCancelCause(context.Background())

			done := make(chan struct{})
			go func() {
				handleShutdown(ctx, &status, listener, nil)
				close(done)
			}()
			cancel(tt.cause)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("handleShutdown did not return")
			}

			if ready, reason := status.Load(); ready || reason != tt.wantReason {
				t.Errorf("status = ready %v, reason %q; want not ready, reason %q", ready, reason, tt.wantReason)
			}
			// A deadline turns a listener left open into a failed assertion rather
			// than an Accept that blocks until the package timeout.
			_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
			if _, err := listener.Accept(); err == nil {
				t.Error("listener still accepts after shutdown")
			} else if errors.Is(err, os.ErrDeadlineExceeded) {
				t.Error("listener was not closed by shutdown")
			}
		})
	}
}

func TestAnnounceReady(t *testing.T) {
	oldLogger := logger
	t.Cleanup(func() { logger = oldLogger })
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cfg := config.Config{Quiet: true, Target: "mesh-host:22"}
	callbackErr := errors.New("callback refused")
	terminal := errors.New("run already failed")

	t.Run("prints READY then calls back", func(t *testing.T) {
		called := false
		var announceErr error
		stdout, _ := captureInitOutput(t, func() {
			announceErr = announceReady(context.Background(), listener, cfg, func() error { called = true; return nil })
		})
		if announceErr != nil || !called {
			t.Fatalf("announceReady = %v, callback called %v", announceErr, called)
		}
		if !strings.Contains(stdout, "READY ") || !strings.Contains(stdout, listener.Addr().String()) {
			t.Errorf("stdout = %q, want a READY line naming %s", stdout, listener.Addr())
		}
	})

	t.Run("callback failure is wrapped", func(t *testing.T) {
		var announceErr error
		captureInitOutput(t, func() {
			announceErr = announceReady(context.Background(), listener, cfg, func() error { return callbackErr })
		})
		if !errors.Is(announceErr, callbackErr) || !strings.Contains(announceErr.Error(), "ready callback") {
			t.Fatalf("announceReady = %v, want a wrapped %v", announceErr, callbackErr)
		}
	})

	t.Run("a canceled run announces nothing", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(terminal)
		called := false
		var announceErr error
		stdout, _ := captureInitOutput(t, func() {
			announceErr = announceReady(ctx, listener, cfg, func() error { called = true; return nil })
		})
		if !errors.Is(announceErr, terminal) || called || strings.Contains(stdout, "READY ") {
			t.Fatalf("announceReady = %v, callback %v, stdout %q", announceErr, called, stdout)
		}
	})
}

// A run canceled before the node is up must report its own cause, not a
// misleading tsnet timeout. The control URL is never contacted either, which
// the test observes by pointing it at a listener that counts connections: tsnet
// reaches the control plane from Up even when its context is already dead.
func TestInitTailscale_ReturnsTheRunCauseWhenCancelled(t *testing.T) {
	oldLogger := logger
	t.Cleanup(func() { logger = oldLogger })
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	control, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var contacts atomic.Int32
	go func() {
		for {
			conn, err := control.Accept()
			if err != nil {
				return
			}
			contacts.Add(1)
			_ = conn.Close()
		}
	}()
	cause := errors.New("ssh bootstrap died")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)

	server, err := initTailscale(ctx, config.Config{
		Hostname:       "lifecycle-test",
		AuthKey:        "tskey-auth-test123-secret",
		StateDir:       t.TempDir(),
		ControlURL:     "http://" + control.Addr().String(),
		ConnectTimeout: 5 * time.Second,
	})
	_ = control.Close()
	if n := contacts.Load(); n != 0 {
		t.Errorf("control URL contacted %d times, want 0", n)
	}

	if server != nil {
		t.Errorf("server = %v, want nil", server)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("initTailscale error = %v, want %v", err, cause)
	}
}
