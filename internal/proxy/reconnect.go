package proxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"strings"
	"time"
)

// maxBackoffAttempt caps the shift exponent before 2^attempt would overflow
// a time.Duration computation. We never need values beyond this since the
// MaxBackoff ceiling kicks in much earlier in any realistic configuration.
const maxBackoffAttempt = 30

// ReconnectDialer wraps a Dialer and retries Dial() on transient errors
// with exponential backoff + jitter. Permanent errors short-circuit the
// retry loop; context cancellation aborts pending backoff sleeps.
type ReconnectDialer struct {
	Inner       Dialer
	MaxRetries  int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Logger      *slog.Logger
}

var _ Dialer = (*ReconnectDialer)(nil)

func (r *ReconnectDialer) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	var lastErr error
	totalAttempts := r.MaxRetries + 1

	for attempt := 0; attempt < totalAttempts; attempt++ {
		conn, err := r.Inner.Dial(ctx, network, address)
		if err == nil {
			return conn, nil
		}
		lastErr = err

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if isPermanentDialError(err) {
			return nil, err
		}
		if attempt == totalAttempts-1 {
			break
		}

		backoff := computeBackoff(attempt, r.BaseBackoff, r.MaxBackoff)

		level := slog.LevelDebug
		if attempt > 0 {
			level = slog.LevelWarn
		}
		if r.Logger != nil {
			r.Logger.Log(ctx, level, "dial retry",
				"attempt", attempt+1,
				"of", totalAttempts,
				"backoff", backoff,
				"error", err)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("dial failed after %d attempts: %w", totalAttempts, lastErr)
}

// computeBackoff returns the wait duration for the given retry attempt with
// jitter in [0, base/2 after capping). attempt is the 0-indexed retry count
// (0 = first retry, 1 = second retry, ...). The exponent is capped to avoid
// overflow at very high attempt values; in practice the MaxBackoff ceiling
// is hit far earlier.
func computeBackoff(attempt int, base, maxBackoff time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	exp := attempt
	if exp < 0 {
		exp = 0
	}
	if exp > maxBackoffAttempt {
		exp = maxBackoffAttempt
	}
	d := base << exp
	if d <= 0 || d > maxBackoff {
		d = maxBackoff
	}
	jitterMax := int64(d / 2)
	if jitterMax <= 0 {
		return d
	}
	// #nosec G404 -- jitter is not security-sensitive; math/rand/v2 is fine here
	jitter := time.Duration(rand.Int64N(jitterMax))
	return d + jitter
}

// isPermanentDialError reports whether err indicates a failure that won't
// resolve on retry (DNS, address parse, terminal tsnet backend state).
// The default is "transient" — any unknown error is retried until MaxRetries.
func isPermanentDialError(err error) bool {
	if err == nil {
		return false
	}
	var addrErr *net.AddrError
	if errors.As(err, &addrErr) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		// DNS errors are mostly permanent for the lifetime of a retry window.
		// If the target was misconfigured, we want to fail fast.
		return true
	}

	// tsnet emits this string when the backend is in a terminal state
	// (Stopped, NeedsMachineAuth). Source: tsnet.Server.awaitRunning at
	// tailscale.com/tsnet@v1.80.0/tsnet.go:203.
	return strings.HasPrefix(err.Error(), "tsnet: backend in state ")
}
