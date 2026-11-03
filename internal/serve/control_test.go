package serve

import (
	"context"
	"slices"
	"testing"

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
