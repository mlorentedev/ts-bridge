// Package agent is the per-user process that owns ts-bridge sessions and
// serves the local API every other surface uses (ADR-018). This file holds its
// lifecycle: one agent per user under an OS-backed lock, an identity-checked
// listener, and a graceful stop.
package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ts-bridge/internal/oslock"
)

// ErrAlreadyRunning reports that another agent of this user holds the lock.
var ErrAlreadyRunning = errors.New("another ts-bridge agent is already running for this user")

const shutdownTimeout = 5 * time.Second

// Options configures Start. Paths is required; Version is reported by
// /v1/meta; a nil Log discards.
type Options struct {
	Paths   Paths
	Version string
	Log     *slog.Logger
}

// Agent is a started agent: it holds the lock and a listening socket.
type Agent struct {
	lock     *os.File
	ln       net.Listener
	endpoint string
	srv      *http.Server
	version  string
	started  time.Time

	closeOnce sync.Once
	closeErr  error
}

// Start takes the agent lock and listens. It fails with ErrAlreadyRunning while
// another agent of this user runs, and touches nothing of that agent's.
func Start(opts Options) (*Agent, error) {
	log := opts.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	lock, err := acquire(opts.Paths.Base)
	if err != nil {
		return nil, err
	}
	owner, err := currentIdentity()
	if err != nil {
		return nil, errors.Join(err, lock.Close())
	}
	ln, endpoint, err := listen(opts.Paths)
	if err != nil {
		return nil, errors.Join(err, lock.Close())
	}
	a := &Agent{
		lock:     lock,
		ln:       newAuthListener(ln, owner, peerIdentity, log),
		endpoint: endpoint,
		version:  opts.Version,
		started:  time.Now().UTC(),
	}
	a.srv = &http.Server{Handler: a.handler(), ReadHeaderTimeout: 5 * time.Second}
	return a, nil
}

// acquire locks base/agent.lock without waiting. The file is never removed:
// the kernel drops the lock with its holder, so a crash leaves nothing to clear.
func acquire(base string) (*os.File, error) {
	if err := prepareDir(base); err != nil {
		return nil, err
	}
	path := filepath.Join(base, lockName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- the path is the agent's own base
	if err != nil {
		return nil, fmt.Errorf("open agent lock: %w", err)
	}
	if err := oslock.Try(file); err != nil {
		_ = file.Close()
		if errors.Is(err, oslock.ErrHeld) {
			return nil, fmt.Errorf("%w (lock %s)", ErrAlreadyRunning, path)
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return file, nil
}

// Endpoint is where clients reach the agent.
func (a *Agent) Endpoint() string { return a.endpoint }

// Serve answers requests until ctx ends, then lets in-flight requests finish.
func (a *Agent) Serve(ctx context.Context) error {
	served := make(chan error, 1)
	go func() { served <- a.srv.Serve(a.ln) }()
	select {
	case err := <-served:
		return fmt.Errorf("agent stopped serving: %w", err)
	case <-ctx.Done():
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	err := a.srv.Shutdown(stopCtx)
	if serveErr := <-served; !errors.Is(serveErr, http.ErrServerClosed) {
		err = errors.Join(err, serveErr)
	}
	return err
}

// Close stops listening, which removes the socket, then releases the lock.
func (a *Agent) Close() error {
	a.closeOnce.Do(func() {
		err := a.ln.Close()
		if errors.Is(err, net.ErrClosed) {
			err = nil
		}
		a.closeErr = errors.Join(err, a.lock.Close())
	})
	return a.closeErr
}
