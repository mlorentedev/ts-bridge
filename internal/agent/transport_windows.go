package agent

import (
	"context"
	"errors"
	"net"
)

// errNoWindowsTransport holds the agent closed on Windows until its named pipe,
// with an SDDL for the user's SID and a client that verifies the server's SID,
// lands (ADR-018 §2, #782). Serving without those checks would be bare access.
var errNoWindowsTransport = errors.New("the ts-bridge agent does not run on Windows yet: its named-pipe transport is still being built (#782)")

func currentIdentity() (Identity, error) { return "", errNoWindowsTransport }

func listen(Paths) (net.Listener, string, error) { return nil, "", errNoWindowsTransport }

func dial(context.Context, Paths) (net.Conn, error) { return nil, errNoWindowsTransport }

func peerIdentity(net.Conn) (Identity, error) { return "", errNoWindowsTransport }
