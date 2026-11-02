package serve

import (
	"context"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
)

// ReasonControlPlaneUnreachable is the readiness reason while the node's
// control session is down; the same token as connect's ERROR reason.
const ReasonControlPlaneUnreachable = "control_plane_unreachable"

// controlSource names the control session among the readiness sources.
const controlSource = "control"

// WatchControlSession reports the node's control session to r until states
// closes or ctx ends: Running is up, any other state is down. tsnet's control
// client does the reconnecting; this only makes /health/ready say so.
func WatchControlSession(ctx context.Context, states <-chan string, r Readiness) {
	reported, wasUp := false, false
	for {
		select {
		case <-ctx.Done():
			return
		case s, ok := <-states:
			if !ok {
				return
			}
			up := s == ipn.Running.String()
			if reported && up == wasUp {
				continue
			}
			reported, wasUp = true, up
			if up {
				r.Up(controlSource)
			} else {
				r.Down(controlSource, ReasonControlPlaneUnreachable)
			}
		}
	}
}

// NodeStates streams the node's backend states from its IPN bus, starting
// with the current one. The channel closes when ctx ends or the bus fails.
func NodeStates(ctx context.Context, lc *local.Client) (<-chan string, error) {
	w, err := lc.WatchIPNBus(ctx, ipn.NotifyInitialState)
	if err != nil {
		return nil, err
	}
	states := make(chan string)
	go func() {
		defer close(states)
		defer func() { _ = w.Close() }()
		for {
			n, err := w.Next()
			if err != nil {
				return
			}
			if n.State == nil {
				continue
			}
			select {
			case states <- n.State.String():
			case <-ctx.Done():
				return
			}
		}
	}()
	return states, nil
}
