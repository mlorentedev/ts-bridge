package serve

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

// fakeTunnel is a bootstrap that runs until exit is called.
type fakeTunnel struct {
	done   chan struct{}
	once   sync.Once
	closed bool
}

func newFakeTunnel() *fakeTunnel { return &fakeTunnel{done: make(chan struct{})} }

func (f *fakeTunnel) exit()                 { f.once.Do(func() { close(f.done) }) }
func (f *fakeTunnel) Done() <-chan struct{} { return f.done }
func (f *fakeTunnel) Err() error            { return errors.New("connection reset") }
func (f *fakeTunnel) Close() error          { f.closed = true; f.exit(); return nil }

// fakeReadiness records each Down/Up in order.
type fakeReadiness struct {
	mu     sync.Mutex
	events []string
}

func (r *fakeReadiness) Down(source, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, "down "+source+" "+reason)
}

func (r *fakeReadiness) Up(source string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, "up "+source)
}

func TestSupervisedBootstrap_Retries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start fails 6 times, then returns a tunnel that the test ends later.
	var mu sync.Mutex
	failures := 6
	second := newFakeTunnel()
	started := make(chan *fakeTunnel, 2)
	var delays []time.Duration
	var errs []error
	ready := &fakeReadiness{}
	s := &Supervisor{
		Start: func(context.Context) (Tunnel, error) {
			mu.Lock()
			defer mu.Unlock()
			if failures > 0 {
				failures--
				return nil, errors.New("ssh: connect refused")
			}
			if len(started) == 0 && second != nil {
				started <- second
				t := second
				second = nil
				return t, nil
			}
			last := newFakeTunnel()
			started <- last
			return last, nil
		},
		Readiness: ready,
		OnExit:    func(err error) { errs = append(errs, err) },
		Base:      time.Second,
		Max:       30 * time.Second,
		Sleep: func(ctx context.Context, d time.Duration) error {
			delays = append(delays, d)
			return ctx.Err()
		},
		Jitter: func(d time.Duration) time.Duration { return d },
	}
	first := newFakeTunnel()
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx, first) }()

	first.exit()
	recovered := <-started
	if want := []time.Duration{1, 2, 4, 8, 16, 30, 30}; !slices.Equal(delays, scale(want)) {
		t.Errorf("backoff = %v, want %v (1 s doubling, capped at 30 s)", delays, scale(want))
	}
	delays = nil
	recovered.exit()
	last := <-started
	if want := scale([]time.Duration{1}); !slices.Equal(delays, want) {
		t.Errorf("backoff after recovery = %v, want %v (reset)", delays, want)
	}
	cancel()
	<-done
	if !first.closed || !recovered.closed {
		t.Errorf("dead tunnels closed: first %v, recovered %v; want both closed before a restart", first.closed, recovered.closed)
	}
	if !last.closed {
		t.Error("Run returned without closing the running tunnel")
	}
	if len(errs) != 2 {
		t.Errorf("OnExit called %d times, want once per outage (2): %v", len(errs), errs)
	}
	want := []string{"down bootstrap ssh_bootstrap_failed", "up bootstrap", "down bootstrap ssh_bootstrap_failed", "up bootstrap"}
	ready.mu.Lock()
	defer ready.mu.Unlock()
	if !slices.Equal(ready.events, want) {
		t.Errorf("readiness = %v, want %v", ready.events, want)
	}
}

func scale(ds []time.Duration) []time.Duration {
	out := make([]time.Duration, len(ds))
	for i, d := range ds {
		out[i] = d * time.Second
	}
	return out
}
