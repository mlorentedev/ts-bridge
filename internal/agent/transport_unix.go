//go:build !windows

package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// endpoint refuses a socket path that does not fit sun_path with its NUL, so a
// deep directory fails with the path named instead of bind's "invalid argument".
func endpoint(p Paths) (string, error) {
	path := filepath.Join(p.Run, socketName)
	if len(path) >= sunPathMax {
		return "", fmt.Errorf("agent socket path is too long (%d bytes, the limit is %d): %s", len(path), sunPathMax-1, path)
	}
	return path, nil
}

func currentIdentity() (Identity, error) {
	return Identity(strconv.Itoa(os.Geteuid())), nil
}

// listen binds the socket. The caller holds the agent lock, which is the only
// reason a leftover socket may be removed: with the lock held, no agent of this
// user is serving it.
func listen(p Paths) (net.Listener, string, error) {
	path, err := endpoint(p)
	if err != nil {
		return nil, "", err
	}
	if err := prepareDir(p.Run); err != nil {
		return nil, "", err
	}
	if err := removeStaleSocket(path); err != nil {
		return nil, "", err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, "", fmt.Errorf("listen on %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, "", errors.Join(fmt.Errorf("restrict socket %s: %w", path, err), ln.Close())
	}
	return ln, path, nil
}

// removeStaleSocket removes a socket left by a crashed agent. Anything else at
// the path is not ours to delete.
func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to replace %s: it is not a socket", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale socket %s: %w", path, err)
	}
	return nil
}

func dial(ctx context.Context, p Paths) (net.Conn, error) {
	self, err := currentIdentity()
	if err != nil {
		return nil, err
	}
	return dialAs(ctx, p, self)
}

// dialAs connects only to an agent run by self: the socket directory must be
// private to this user, and the server's peer credentials must name self. Both
// checks run before the caller can write a byte.
func dialAs(ctx context.Context, p Paths, self Identity) (net.Conn, error) {
	if err := checkPrivateDir(p.Run, os.Geteuid()); err != nil {
		return nil, err
	}
	path, err := endpoint(p)
	if err != nil {
		return nil, err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("dial agent at %s: %w", path, err)
	}
	server, err := peerIdentity(conn)
	if err == nil {
		err = Authorize(server, self)
	}
	if err != nil {
		return nil, errors.Join(fmt.Errorf("agent at %s: %w", path, err), conn.Close())
	}
	return conn, nil
}

// peerIdentity names the user on the other end of a connected socket. It works
// from either end: the server learns its client, the client its server.
func peerIdentity(conn net.Conn) (Identity, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return "", fmt.Errorf("peer credentials: %T is not a unix connection", conn)
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return "", fmt.Errorf("peer credentials: %w", err)
	}
	var uid uint32
	var credErr error
	if err := raw.Control(func(fd uintptr) { uid, credErr = peerUID(int(fd)) }); err != nil {
		return "", fmt.Errorf("peer credentials: %w", err)
	}
	if credErr != nil {
		return "", fmt.Errorf("peer credentials: %w", credErr)
	}
	return Identity(strconv.FormatUint(uint64(uid), 10)), nil
}
