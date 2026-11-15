package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
)

// pipePrefix is the local named-pipe namespace; Paths.Run holds only the name.
const pipePrefix = `\\.\pipe\`

// endpoint is the pipe path. A name with a separator would reach outside the
// pipe namespace, so it is refused rather than joined.
func endpoint(p Paths) (string, error) {
	if p.Run == "" || strings.ContainsAny(p.Run, `\/`) {
		return "", fmt.Errorf("agent pipe name %q is not a bare pipe name", p.Run)
	}
	return pipePrefix + p.Run, nil
}

func currentIdentity() (Identity, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("identify the current user: %w", err)
	}
	return Identity(user.User.Sid.String()), nil
}

// pipeSDDL grants the owner's SID full access and nobody else anything: the
// protected DACL (P) inherits no entry. It is built from a SID string the OS
// gave us, never from user input.
func pipeSDDL(owner Identity) string { return "D:P(A;;GA;;;" + string(owner) + ")" }

// listen creates the pipe. winio creates its first instance with FILE_CREATE
// (FILE_FLAG_FIRST_PIPE_INSTANCE) and rejects remote clients, so a name another
// process already owns fails here and is never retried under another name.
func listen(p Paths) (net.Listener, string, error) {
	path, err := endpoint(p)
	if err != nil {
		return nil, "", err
	}
	owner, err := currentIdentity()
	if err != nil {
		return nil, "", err
	}
	ln, err := winio.ListenPipe(path, &winio.PipeConfig{SecurityDescriptor: pipeSDDL(owner)})
	if err != nil {
		return nil, "", fmt.Errorf("listen on %s (another process may own this name): %w", path, err)
	}
	return ln, path, nil
}

func dial(ctx context.Context, p Paths) (net.Conn, error) {
	self, err := currentIdentity()
	if err != nil {
		return nil, err
	}
	return dialAs(ctx, p, self)
}

// dialAs connects only to an agent run by self. The client offers no more than
// identification (SECURITY_SQOS_PRESENT | SECURITY_IDENTIFICATION), so a
// squatting server cannot act as this user, and the server process's token
// must name self before the caller can write a byte.
func dialAs(ctx context.Context, p Paths, self Identity) (net.Conn, error) {
	path, err := endpoint(p)
	if err != nil {
		return nil, err
	}
	conn, err := winio.DialPipeAccessImpLevel(ctx, path, windows.GENERIC_READ|windows.GENERIC_WRITE, winio.PipeImpLevelIdentification)
	if err != nil {
		return nil, fmt.Errorf("dial agent at %s: %w", path, err)
	}
	server, err := pipeEndIdentity(conn, windows.GetNamedPipeServerProcessId)
	if err == nil {
		err = Authorize(server, self)
	}
	if err != nil {
		return nil, errors.Join(fmt.Errorf("agent at %s: %w", path, err), conn.Close())
	}
	return conn, nil
}

// peerIdentity names the client of a connection the agent accepted. The DACL
// already admits only the owner; this per-connection check is the second gate
// ADR-018 §2 requires.
func peerIdentity(conn net.Conn) (Identity, error) {
	return pipeEndIdentity(conn, windows.GetNamedPipeClientProcessId)
}

// pipeEndIdentity names the user of the process at the other end of the pipe;
// pid reports that process (the client's or the server's id for the handle).
func pipeEndIdentity(conn net.Conn, pid func(windows.Handle, *uint32) error) (Identity, error) {
	f, ok := conn.(interface{ Fd() uintptr })
	if !ok {
		return "", fmt.Errorf("pipe peer: %T is not a pipe connection", conn)
	}
	var id uint32
	if err := pid(windows.Handle(f.Fd()), &id); err != nil {
		return "", fmt.Errorf("pipe peer process: %w", err)
	}
	return processIdentity(id)
}

// processIdentity reads the user SID from a process's token. A process this
// user may not query (another user's, or one at a higher integrity level)
// fails, and the caller fails closed.
func processIdentity(pid uint32) (Identity, error) {
	proc, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", fmt.Errorf("open peer process %d: %w", pid, err)
	}
	defer func() { _ = windows.CloseHandle(proc) }()
	var token windows.Token
	if err := windows.OpenProcessToken(proc, windows.TOKEN_QUERY, &token); err != nil {
		return "", fmt.Errorf("open token of peer process %d: %w", pid, err)
	}
	defer func() { _ = token.Close() }()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("read user of peer process %d: %w", pid, err)
	}
	return Identity(user.User.Sid.String()), nil
}
