package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// recordingDialer is a Dialer test double that fails the first failuresBefore
// calls with errFail, then returns a usable net.Conn (the client end of a
// throwaway net.Pipe). Tracks attempt count atomically for assertions.
type recordingDialer struct {
	failuresBefore int32
	attempts       atomic.Int32
	errFail        error
}

func (r *recordingDialer) Dial(_ context.Context, _, _ string) (net.Conn, error) {
	n := r.attempts.Add(1)
	if n <= r.failuresBefore {
		return nil, r.errFail
	}
	a, b := net.Pipe()
	go func() { _, _ = io.Copy(io.Discard, b); _ = b.Close() }()
	return a, nil
}

func newReconnectDialer(inner Dialer, retries int, base, max time.Duration) *ReconnectDialer {
	return &ReconnectDialer{
		Inner:       inner,
		MaxRetries:  retries,
		BaseBackoff: base,
		MaxBackoff:  max,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestReconnectDialer_SucceedsFirstAttempt(t *testing.T) {
	inner := &recordingDialer{failuresBefore: 0, errFail: io.EOF}
	d := newReconnectDialer(inner, 3, 1*time.Millisecond, 10*time.Millisecond)

	start := time.Now()
	conn, err := d.Dial(context.Background(), "tcp", "x:1")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = conn.Close()
	if got := inner.attempts.Load(); got != 1 {
		t.Errorf("expected 1 attempt, got %d", got)
	}
	if elapsed > 5*time.Millisecond {
		t.Errorf("expected near-instant return, took %v", elapsed)
	}
}

func TestReconnectDialer_SucceedsAfterTransientFailures(t *testing.T) {
	inner := &recordingDialer{failuresBefore: 2, errFail: io.EOF}
	base := 5 * time.Millisecond
	d := newReconnectDialer(inner, 3, base, 50*time.Millisecond)

	start := time.Now()
	conn, err := d.Dial(context.Background(), "tcp", "x:1")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = conn.Close()
	if got := inner.attempts.Load(); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
	// Minimum elapsed = backoff(0) + backoff(1) = base + 2*base = 3*base.
	// Jitter adds up to 50% per step, so allow a generous upper bound.
	minExpected := 3 * base
	if elapsed < minExpected {
		t.Errorf("elapsed %v < min expected %v", elapsed, minExpected)
	}
}

func TestReconnectDialer_GivesUpAfterMaxRetries(t *testing.T) {
	inner := &recordingDialer{failuresBefore: 100, errFail: io.EOF}
	d := newReconnectDialer(inner, 2, 1*time.Millisecond, 10*time.Millisecond)

	_, err := d.Dial(context.Background(), "tcp", "x:1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected wrapped io.EOF, got %v", err)
	}
	// MaxRetries=2 means: 1 initial attempt + 2 retries = 3 total attempts.
	if got := inner.attempts.Load(); got != 3 {
		t.Errorf("expected 3 attempts (1 + 2 retries), got %d", got)
	}
	if !strings.Contains(err.Error(), "after") {
		t.Errorf("expected error message to mention attempt count, got %q", err)
	}
}

func TestReconnectDialer_PermanentErrorShortCircuits(t *testing.T) {
	permErr := errors.New("tsnet: backend in state Stopped")
	inner := &recordingDialer{failuresBefore: 100, errFail: permErr}
	d := newReconnectDialer(inner, 5, 100*time.Millisecond, 1*time.Second)

	start := time.Now()
	_, err := d.Dial(context.Background(), "tcp", "x:1")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if got := inner.attempts.Load(); got != 1 {
		t.Errorf("expected 1 attempt (no retry on permanent), got %d", got)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("permanent error should short-circuit, but took %v", elapsed)
	}
}

func TestReconnectDialer_ContextCancellationAbortsLoop(t *testing.T) {
	inner := &recordingDialer{failuresBefore: 100, errFail: io.EOF}
	d := newReconnectDialer(inner, 100, 500*time.Millisecond, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := d.Dial(ctx, "tcp", "x:1")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from ctx cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected ctx.Canceled error, got %v", err)
	}
	if elapsed > 600*time.Millisecond {
		t.Errorf("ctx cancellation should abort backoff sleep promptly, took %v", elapsed)
	}
}

func TestReconnectDialer_MaxRetriesZeroDisables(t *testing.T) {
	inner := &recordingDialer{failuresBefore: 5, errFail: io.EOF}
	d := newReconnectDialer(inner, 0, 100*time.Millisecond, 1*time.Second)

	start := time.Now()
	_, err := d.Dial(context.Background(), "tcp", "x:1")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	if got := inner.attempts.Load(); got != 1 {
		t.Errorf("expected exactly 1 attempt with MaxRetries=0, got %d", got)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("no retry should mean no backoff sleep, took %v", elapsed)
	}
}

func TestComputeBackoff_JitterBounded(t *testing.T) {
	base := 100 * time.Millisecond
	maxCap := 1 * time.Second

	// For attempt 0, expected backoff = base. With jitter in [0, base/2],
	// the result is in [base, base + base/2] = [100ms, 150ms].
	for i := 0; i < 200; i++ {
		got := computeBackoff(0, base, maxCap)
		if got < base || got > base+base/2 {
			t.Errorf("attempt 0: backoff %v outside [%v, %v]", got, base, base+base/2)
		}
	}

	// For attempt 4 with base=100ms: 2^4 * 100ms = 1.6s, capped at maxCap=1s.
	// Jitter is base/2 of the *capped* value = 500ms. So result in [1s, 1.5s].
	for i := 0; i < 200; i++ {
		got := computeBackoff(4, base, maxCap)
		if got < maxCap || got > maxCap+maxCap/2 {
			t.Errorf("attempt 4: backoff %v outside [%v, %v]", got, maxCap, maxCap+maxCap/2)
		}
	}
}

func TestComputeBackoff_AttemptOverflowProtection(t *testing.T) {
	// attempt=100 must not panic or overflow; should cap at maxCap.
	got := computeBackoff(100, 1*time.Second, 30*time.Second)
	if got < 30*time.Second || got > 45*time.Second {
		t.Errorf("attempt=100: expected ~30s..45s (cap+jitter), got %v", got)
	}
}

func TestIsPermanentDialError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "transient EOF", err: io.EOF, want: false},
		{name: "transient connection refused", err: errors.New("dial tcp: connect: connection refused"), want: false},
		{name: "transient i/o timeout", err: errors.New("read tcp: i/o timeout"), want: false},
		{name: "tsnet terminal stopped", err: errors.New("tsnet: backend in state Stopped"), want: true},
		{name: "tsnet terminal NeedsMachineAuth", err: errors.New("tsnet: backend in state NeedsMachineAuth"), want: true},
		{name: "address parse error", err: &net.AddrError{Err: "invalid address", Addr: "x:y"}, want: true},
		{name: "DNS NXDOMAIN", err: &net.DNSError{Err: "no such host", Name: "missing.example.com"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPermanentDialError(tt.err); got != tt.want {
				t.Errorf("isPermanentDialError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestReconnectDialer_TerminalErrorWrapped(t *testing.T) {
	tsnetErr := errors.New("tsnet: backend in state Stopped")
	inner := &recordingDialer{failuresBefore: 100, errFail: tsnetErr}
	d := newReconnectDialer(inner, 3, 1*time.Millisecond, 10*time.Millisecond)

	_, err := d.Dial(context.Background(), "tcp", "x:1")
	if err == nil {
		t.Fatal("expected error")
	}

	// Must short-circuit after one attempt (permanent).
	if got := inner.attempts.Load(); got != 1 {
		t.Errorf("expected 1 attempt for terminal error, got %d", got)
	}

	// Must be unwrappable as *TerminalDialError.
	var termErr *TerminalDialError
	if !errors.As(err, &termErr) {
		t.Errorf("expected *TerminalDialError via errors.As, got %T: %v", err, err)
	}

	// Inner cause must be preserved.
	if !errors.Is(err, tsnetErr) {
		t.Errorf("expected original tsnet error via errors.Is, got %v", err)
	}
}

func TestReconnectDialer_TransientErrorNotWrappedAsTerminal(t *testing.T) {
	inner := &recordingDialer{failuresBefore: 100, errFail: io.EOF}
	d := newReconnectDialer(inner, 1, 1*time.Millisecond, 10*time.Millisecond)

	_, err := d.Dial(context.Background(), "tcp", "x:1")
	if err == nil {
		t.Fatal("expected error")
	}

	var termErr *TerminalDialError
	if errors.As(err, &termErr) {
		t.Errorf("transient error should NOT be wrapped as *TerminalDialError, but got one")
	}
}

// Sanity: ReconnectDialer must satisfy the Dialer interface so it can be
// passed in place of *tsnet.Server.
var _ Dialer = (*ReconnectDialer)(nil)

// Compile-time check that we never accidentally drop the inner error.
func TestReconnectDialer_WrappedErrorPreservesInner(t *testing.T) {
	sentinel := fmt.Errorf("inner sentinel: %w", io.ErrUnexpectedEOF)
	inner := &recordingDialer{failuresBefore: 100, errFail: sentinel}
	d := newReconnectDialer(inner, 1, 1*time.Millisecond, 5*time.Millisecond)

	_, err := d.Dial(context.Background(), "tcp", "x:1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("inner error not preserved via errors.Is, got %v", err)
	}
}
