package agent

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
)

// ErrForeignPeer reports a connection whose caller is not the agent's user, or
// whose caller could not be named.
var ErrForeignPeer = errors.New("peer is not the user running the agent")

// Identity names a user in the form the OS reports it: the decimal uid on Unix,
// the SID string on Windows. The empty Identity means "unknown" and is never
// authorized, so a platform that cannot name a peer fails closed.
type Identity string

// Authorize accepts a peer only when it is exactly the owner. It is the one
// comparison behind every platform's check, so it is tested with fake
// identities: a cross-user connection cannot be made in CI.
func Authorize(peer, owner Identity) error {
	if peer == "" || owner == "" || peer != owner {
		return fmt.Errorf("%w (peer %q, owner %q)", ErrForeignPeer, peer, owner)
	}
	return nil
}

// authListener identifies the caller of every accepted connection and closes
// the ones that are not the owner. A refusal never stops the accept loop:
// returning an error from Accept would end http.Server.Serve for everyone.
type authListener struct {
	net.Listener
	owner    Identity
	identify func(net.Conn) (Identity, error)
	log      *slog.Logger
}

func newAuthListener(inner net.Listener, owner Identity, identify func(net.Conn) (Identity, error), log *slog.Logger) net.Listener {
	return &authListener{Listener: inner, owner: owner, identify: identify, log: log}
}

func (l *authListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if err := l.check(conn); err != nil {
			l.log.Warn("refused a local API connection", "reason", err)
			_ = conn.Close()
			continue
		}
		return conn, nil
	}
}

func (l *authListener) check(conn net.Conn) error {
	peer, err := l.identify(conn)
	if err != nil {
		return fmt.Errorf("identify peer: %w", err)
	}
	return Authorize(peer, l.owner)
}
