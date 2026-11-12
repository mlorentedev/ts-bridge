package cmd

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
)

// offlineClient is a local API whose node is offline; its bus can be made to fail.
type offlineClient struct{ busErr error }

func (c offlineClient) Status(context.Context) (*ipnstate.Status, error) {
	return &ipnstate.Status{Self: &ipnstate.PeerStatus{Online: false}}, nil
}

func (c offlineClient) WatchIPNBus(context.Context, ipn.NotifyWatchOpt) (*local.IPNBusWatcher, error) {
	return nil, c.busErr
}

type recordedReadiness struct {
	mu    sync.Mutex
	downs []string
}

func (r *recordedReadiness) Down(source, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.downs = append(r.downs, source+" "+reason)
}

func (r *recordedReadiness) Up(string) {}

func (r *recordedReadiness) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.downs)
}

// TestWatchControl_PollsSelfOnline pins the wiring the mesh e2e covers: serve
// starts the Self.Online poller, and starts it even when the IPN bus cannot be
// watched, since the poller only needs Status and the bus failing is no reason
// to stop seeing an outage (HOST-011).
func TestWatchControl_PollsSelfOnline(t *testing.T) {
	defer func(d time.Duration) { controlPollInterval = d }(controlPollInterval)
	controlPollInterval = 5 * time.Millisecond
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := &recordedReadiness{}
	// A client whose bus succeeds cannot be faked (IPNBusWatcher is concrete),
	// so the case that would have been missed is the one pinned.
	watchControl(ctx, offlineClient{busErr: errors.New("bus unavailable")}, ready, logger)
	deadline := time.Now().Add(2 * time.Second)
	for ready.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if ready.count() == 0 {
		t.Fatal("an offline node never reached readiness: the Self.Online poller did not start")
	}
}

// apiNode is a serve node whose local API is a real *local.Client on a socket
// nothing listens on, so watchServeControl runs as in production and every
// local API call fails.
type apiNode struct {
	serveNode
	socket string
}

func (n apiNode) LocalClient() (*local.Client, error) {
	return &local.Client{Socket: n.socket, UseSocketOnly: true}, nil
}

// TestWatchServeControl_PollsThroughTheEntryPoint drives the production entry
// point: a node whose local API does not answer must reach readiness as down,
// which only the Self.Online poller reports (the bus watch fails at once).
func TestWatchServeControl_PollsThroughTheEntryPoint(t *testing.T) {
	defer func(d time.Duration) { controlPollInterval = d }(controlPollInterval)
	controlPollInterval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := &recordedReadiness{}
	node := apiNode{socket: filepath.Join(t.TempDir(), "absent.sock")}
	watchServeControl(ctx, node, ready, slog.New(slog.NewTextHandler(io.Discard, nil)))
	deadline := time.Now().Add(5 * time.Second)
	for ready.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if ready.count() == 0 {
		t.Fatal("watchServeControl never reported an unreachable local API: the Self.Online poller is not wired")
	}
}
