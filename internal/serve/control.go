package serve

import (
	"context"
	"time"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
)

// ReasonControlPlaneUnreachable is the readiness reason while the node's
// control session is down; the same token as connect's ERROR reason.
const ReasonControlPlaneUnreachable = "control_plane_unreachable"

// controlSource names the control session among the readiness sources.
const controlSource = "control"

// controlOnlineSource names the node's own view of its control connection.
const controlOnlineSource = "control-online"

// ControlPollInterval is how often serve reads the node's Self.Online, and
// offlineSamplesDown how many offline reads in a row mark it down: about 30 s,
// so one late map response does not flap readiness (HOST-011).
const (
	ControlPollInterval = 10 * time.Second
	offlineSamplesDown  = 3
)

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
				// The bus ended while serve runs: the session can no longer be
				// seen, so health must not keep reading ready.
				if ctx.Err() == nil {
					r.Down(controlSource, ReasonControlPlaneUnreachable)
				}
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

// WatchControlOnline reports the node's Self.Online to r on every tick until
// ticks closes or ctx ends. The IPN bus stays Running through a control-plane
// outage, while Self.Online drops within seconds (measured for HOST-011), so
// this is the watcher that sees one. offlineSamplesDown offline reads in a row
// mark the source down; the first online read marks it up. A read that fails
// counts as offline, so a broken local API does not read as healthy.
func WatchControlOnline(ctx context.Context, status func(context.Context) (*ipnstate.Status, error), ticks <-chan time.Time, r Readiness) {
	var s onlineDebounce
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ticks:
			if !ok {
				return
			}
		}
		online := nodeOnline(ctx, status)
		if ctx.Err() != nil {
			return
		}
		s.observe(online, r)
	}
}

// onlineDebounce turns Self.Online reads into readiness changes.
type onlineDebounce struct {
	offline int
	down    bool
}

func (s *onlineDebounce) observe(online bool, r Readiness) {
	if online {
		s.offline = 0
		if s.down {
			s.down = false
			r.Up(controlOnlineSource)
		}
		return
	}
	if s.down {
		return
	}
	if s.offline++; s.offline >= offlineSamplesDown {
		s.down = true
		r.Down(controlOnlineSource, ReasonControlPlaneUnreachable)
	}
}

func nodeOnline(ctx context.Context, status func(context.Context) (*ipnstate.Status, error)) bool {
	st, err := status(ctx)
	return err == nil && st != nil && st.Self != nil && st.Self.Online
}
