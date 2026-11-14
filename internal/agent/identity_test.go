package agent

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"
)

func TestAuthorize(t *testing.T) {
	tests := []struct {
		name        string
		peer, owner Identity
		want        error
	}{
		{"unix owner", "1000", "1000", nil},
		{"unix other user", "1001", "1000", ErrForeignPeer},
		{"unix root is not the owner", "0", "1000", ErrForeignPeer},
		{"windows owner", "S-1-5-21-1-2-3-1001", "S-1-5-21-1-2-3-1001", nil},
		{"windows other user", "S-1-5-21-1-2-3-1002", "S-1-5-21-1-2-3-1001", ErrForeignPeer},
		{"windows SYSTEM is not the owner", "S-1-5-18", "S-1-5-21-1-2-3-1001", ErrForeignPeer},
		{"a prefix of the owner is not the owner", "100", "1000", ErrForeignPeer},
		{"empty peer", "", "1000", ErrForeignPeer},
		{"empty owner fails closed", "", "", ErrForeignPeer},
		{"empty owner, named peer", "1000", "", ErrForeignPeer},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := Authorize(tc.peer, tc.owner); !errors.Is(err, tc.want) {
				t.Fatalf("Authorize(%q, %q) = %v, want %v", tc.peer, tc.owner, err, tc.want)
			}
		})
	}
}

// pipeListener hands out in-memory connections, so the identity check can be
// driven with any fake peer on every OS.
type pipeListener struct {
	conns  chan net.Conn
	closed chan struct{}
}

func newPipeListener() *pipeListener {
	return &pipeListener{conns: make(chan net.Conn, 4), closed: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error   { close(l.closed); return nil }
func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "fake" }

// taggedConn is the server side of a fake connection that carries the identity
// the test wants identify to report.
type taggedConn struct {
	net.Conn
	id  Identity
	err error
}

func identifyTagged(c net.Conn) (Identity, error) {
	tc := c.(*taggedConn)
	return tc.id, tc.err
}

// dialPipe queues a server-side connection on l and returns the client side.
func dialPipe(l *pipeListener, id Identity, err error) net.Conn {
	server, client := net.Pipe()
	l.conns <- &taggedConn{Conn: server, id: id, err: err}
	return client
}

// assertClosedByPeer fails unless the far side closes the connection without
// the client having to send anything.
func assertClosedByPeer(t *testing.T, client net.Conn) {
	t.Helper()
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("read from a refused peer = %v, want EOF (closed by the listener)", err)
	}
}

func newTestListener(inner *pipeListener) net.Listener {
	return newAuthListener(inner, "1000", identifyTagged, slog.New(slog.DiscardHandler))
}

// acceptAsync runs Accept in the background so a refused connection, which
// Accept never returns, does not block the test.
func acceptAsync(ln net.Listener) <-chan net.Conn {
	got := make(chan net.Conn, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			got <- c
		}
	}()
	return got
}

func TestListenerRefusesAPeerThatIsNotTheOwner(t *testing.T) {
	tests := []struct {
		name string
		id   Identity
		err  error
	}{
		{"foreign uid", "1001", nil},
		{"foreign SID", "S-1-5-21-1-2-3-1002", nil},
		{"root", "0", nil},
		{"unidentifiable", "", errors.New("no credentials")},
		{"identified as nobody", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inner := newPipeListener()
			ln := newTestListener(inner)
			defer func() { _ = ln.Close() }()
			got := acceptAsync(ln)

			assertClosedByPeer(t, dialPipe(inner, tc.id, tc.err))
			select {
			case <-got:
				t.Fatal("a refused peer was handed to the server")
			default:
			}
		})
	}
}

func TestListenerKeepsServingAfterARefusal(t *testing.T) {
	inner := newPipeListener()
	ln := newTestListener(inner)
	defer func() { _ = ln.Close() }()
	got := acceptAsync(ln)

	foreign := dialPipe(inner, "1001", nil)
	owner := dialPipe(inner, "1000", nil)
	defer func() { _ = owner.Close() }()

	assertClosedByPeer(t, foreign)
	select {
	case c := <-got:
		if c.(*taggedConn).id != "1000" {
			t.Fatalf("Accept returned the connection of %q, want the owner", c.(*taggedConn).id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the listener stopped serving after refusing one peer")
	}
}
