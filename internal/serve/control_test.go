package serve

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"tailscale.com/ipn/ipnstate"

	"ts-bridge/internal/health"
)

func TestServeHealth_ControlSession(t *testing.T) {
	states := make(chan string)
	ready := &fakeReadiness{}
	done := make(chan struct{})
	go func() { defer close(done); WatchControlSession(context.Background(), states, ready) }()
	for _, s := range []string{"Running", "Starting", "NoState", "Running", "Running"} {
		states <- s
	}
	close(states)
	<-done
	// The bus ending while serve still runs reads as down, not a frozen "up".
	want := []string{"up control", "down control control_plane_unreachable", "up control", "down control control_plane_unreachable"}
	if !slices.Equal(ready.events, want) {
		t.Errorf("readiness = %v, want %v (one report per change)", ready.events, want)
	}

	// Ending because serve stops reports nothing more.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	quiet := &fakeReadiness{}
	WatchControlSession(ctx, make(chan string), quiet)
	if len(quiet.events) != 0 {
		t.Errorf("readiness after shutdown = %v, want none", quiet.events)
	}
}

func TestHealth_FoldsSources(t *testing.T) {
	var h Health
	h.Status = new(health.TunnelStatus)
	h.Down(bootstrapSource, ReasonSSHBootstrapFailed)
	h.Down(controlSource, ReasonControlPlaneUnreachable)
	h.Up(bootstrapSource)
	if ok, reason := h.Status.Load(); ok || reason != ReasonControlPlaneUnreachable {
		t.Errorf("one source down: ready %v reason %q, want not ready with its reason", ok, reason)
	}
	h.Up(controlSource)
	if ok, reason := h.Status.Load(); !ok || reason != "" {
		t.Errorf("every source up: ready %v reason %q, want ready", ok, reason)
	}
}

// TestWatchControlOnline pins HOST-011: the IPN bus stays Running through a
// control-plane outage (measured 2026-11-11), so readiness also follows the
// node's Self.Online, debounced over offlineSamplesDown polls.
func TestWatchControlOnline(t *testing.T) {
	on, off, fail := sample{online: true}, sample{}, sample{err: errors.New("local API down")}
	down := "down " + controlOnlineSource + " " + ReasonControlPlaneUnreachable
	up := "up " + controlOnlineSource
	tests := []struct {
		name    string
		samples []sample
		want    []string
	}{
		{"online stays quiet", []sample{on, on, on}, nil},
		{"two offline samples do not flap", []sample{on, off, off, on}, nil},
		{"the third offline sample is down", []sample{on, off, off, off}, []string{down}},
		{"down once, not per sample", []sample{off, off, off, off, off}, []string{down}},
		{"the first online sample after a down is up", []sample{off, off, off, on, on}, []string{down, up}},
		{"an online sample resets the count", []sample{off, off, on, off, off}, nil},
		{"a failed status read counts as offline", []sample{fail, fail, fail}, []string{down}},
		{"a node with no self entry counts as offline", []sample{{noSelf: true}, {noSelf: true}, {noSelf: true}}, []string{down}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready := &fakeReadiness{}
			ticks := make(chan time.Time)
			done := make(chan struct{})
			i := 0
			status := func(context.Context) (*ipnstate.Status, error) {
				s := tt.samples[i]
				i++
				if s.err != nil {
					return nil, s.err
				}
				if s.noSelf {
					return &ipnstate.Status{}, nil
				}
				return &ipnstate.Status{Self: &ipnstate.PeerStatus{Online: s.online}}, nil
			}
			go func() { defer close(done); WatchControlOnline(context.Background(), status, ticks, ready) }()
			for range tt.samples {
				ticks <- time.Time{}
			}
			close(ticks)
			<-done
			if !slices.Equal(ready.events, tt.want) {
				t.Errorf("readiness = %v, want %v", ready.events, tt.want)
			}
		})
	}

	// Ending because serve stops polls nothing and reports nothing.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	quiet := &fakeReadiness{}
	WatchControlOnline(ctx, func(context.Context) (*ipnstate.Status, error) {
		t.Error("polled after shutdown")
		return nil, errors.New("unreachable")
	}, make(chan time.Time), quiet)
	if len(quiet.events) != 0 {
		t.Errorf("readiness after shutdown = %v, want none", quiet.events)
	}
}

type sample struct {
	online bool
	noSelf bool
	err    error
}

// TestHealth_FoldsControlOnline: with the bootstrap and the bus both up, an
// offline node alone makes /health/ready say control_plane_unreachable, and
// with the bootstrap also down the answer is still not ready.
func TestHealth_FoldsControlOnline(t *testing.T) {
	var h Health
	h.Status = new(health.TunnelStatus)
	h.Down(controlOnlineSource, ReasonControlPlaneUnreachable)
	if ok, reason := h.Status.Load(); ok || reason != ReasonControlPlaneUnreachable {
		t.Errorf("node offline: ready %v reason %q, want not ready with %q", ok, reason, ReasonControlPlaneUnreachable)
	}
	h.Down(bootstrapSource, ReasonSSHBootstrapFailed)
	h.Up(controlOnlineSource)
	if ok, _ := h.Status.Load(); ok {
		t.Error("bootstrap still down: want not ready")
	}
	h.Up(bootstrapSource)
	if ok, _ := h.Status.Load(); !ok {
		t.Error("every source up: want ready")
	}
}
