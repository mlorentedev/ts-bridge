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

// maxSocketPath keeps a socket path under sun_path (104 bytes on macOS, 108 on
// Linux), so a deep state directory fails with a clear error instead of bind's
// "invalid argument".
const maxSocketPath = 100

func endpoint(dir string) (string, error) {
	path := filepath.Join(dir, socketName)
	if len(path) > maxSocketPath {
		return "", fmt.Errorf("agent socket path is too long (%d bytes, at most %d): %s", len(path), maxSocketPath, path)
	}
	return path, nil
}

func currentIdentity() (Identity, error) {
	return Identity(strconv.Itoa(os.Geteuid())), nil
}

// listen binds the socket. The caller holds the agent lock, which is the only
// reason a leftover socket may be removed: with the lock held, no agent of this
// user is serving it.
func listen(dir string) (net.Listener, error) {
	path, err := endpoint(dir)
	if err != nil {
		return nil, err
	}
	if err := removeStaleSocket(path); err != nil {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, errors.Join(fmt.Errorf("restrict socket %s: %w", path, err), ln.Close())
	}
	return ln, nil
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

func dial(ctx context.Context, dir string) (net.Conn, error) {
	path, err := endpoint(dir)
	if err != nil {
		return nil, err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("dial agent at %s: %w", path, err)
	}
	return conn, nil
}

// peerIdentity names the user on the other end of an accepted socket.
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
