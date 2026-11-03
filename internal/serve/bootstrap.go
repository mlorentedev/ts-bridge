package serve

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"ts-bridge/internal/health"
)

// ReasonSSHBootstrapFailed is the readiness reason while the bootstrap is down;
// the same token as connect's ERROR reason.
const ReasonSSHBootstrapFailed = "ssh_bootstrap_failed"

// bootstrapSource names the bootstrap among the readiness sources.
const bootstrapSource = "bootstrap"

// Tunnel is a running SSH bootstrap (bootstrapssh.Tunnel).
type Tunnel interface {
	Done() <-chan struct{}
	Err() error
	Close() error
}

// Readiness takes per-source up/down reports (Health implements it).
type Readiness interface {
	Down(source, reason string)
	Up(source string)
}

// Supervisor keeps the SSH bootstrap up after READY: when the tunnel exits it
// reports the outage once and restarts it with capped exponential backoff,
// instead of stopping serve as connect does (HOST-008 AC4).
type Supervisor struct {
	Start     func(context.Context) (Tunnel, error)
	Readiness Readiness
	OnExit    func(error) // once per outage, e.g. to print the ERROR line
	Base, Max time.Duration
	Sleep     func(context.Context, time.Duration) error // nil sleeps on a timer
	Jitter    func(time.Duration) time.Duration          // nil adds up to 20%
}

// Run watches running until ctx ends, restarting it whenever it exits, and
// closes the tunnel that is running when it returns.
func (s *Supervisor) Run(ctx context.Context, running Tunnel) {
	for {
		select {
		case <-ctx.Done():
			_ = running.Close()
			return
		case <-running.Done():
		}
		if ctx.Err() != nil {
			return
		}
		s.OnExit(fmt.Errorf("SSH bootstrap exited: %v", running.Err()))
		s.Readiness.Down(bootstrapSource, ReasonSSHBootstrapFailed)
		// Release the dead tunnel (its SOCKS listener, its child) before the next.
		_ = running.Close()
		if running = s.restart(ctx); running == nil {
			return
		}
		s.Readiness.Up(bootstrapSource)
	}
}

// restart retries Start with backoff until it succeeds or ctx ends (nil).
func (s *Supervisor) restart(ctx context.Context) Tunnel {
	delay := s.Base
	for {
		if err := s.sleep(ctx, s.jitter(delay)); err != nil {
			return nil
		}
		if t, err := s.Start(ctx); err == nil {
			return t
		}
		delay = min(delay*2, s.Max)
	}
}

func (s *Supervisor) sleep(ctx context.Context, d time.Duration) error {
	if s.Sleep != nil {
		return s.Sleep(ctx, d)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *Supervisor) jitter(d time.Duration) time.Duration {
	if s.Jitter != nil {
		return s.Jitter(d)
	}
	// #nosec G404 -- backoff jitter, not a secret.
	return d + time.Duration(rand.Int64N(int64(d)/5+1))
}

// Health folds per-source readiness into /health/ready: ready when no source
// is down, otherwise not ready with the first down source's reason.
type Health struct {
	Status *health.TunnelStatus
	mu     sync.Mutex
	down   map[string]string
}

// Down marks source down with reason.
func (h *Health) Down(source, reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.down == nil {
		h.down = map[string]string{}
	}
	h.down[source] = reason
	h.store()
}

// Up marks source up again.
func (h *Health) Up(source string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.down, source)
	h.store()
}

func (h *Health) store() {
	if len(h.down) == 0 {
		h.Status.MarkReady()
		return
	}
	sources := make([]string, 0, len(h.down))
	for s := range h.down {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	h.Status.Store(false, h.down[sources[0]])
}
