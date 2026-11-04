package bootstrapssh

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

// closerFunc is an io.Closer from a function.
type closerFunc func() error

func (f closerFunc) Close() error { return f() }

// waitHelperDeps runs the "wait" helper as OpenSSH, ready at once, with the
// given containment.
func waitHelperDeps(contain func(*exec.Cmd) (io.Closer, error)) dependencies {
	return dependencies{
		lookPath: func(string) (string, error) { return "ssh", nil },
		commandContext: func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			//nolint:gosec // os.Args[0] and the test selector are fixed test-process inputs.
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestSSHHelperProcess")
			cmd.Env = append(os.Environ(), "TS_BRIDGE_SSH_HELPER=wait")
			return cmd
		},
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			left, right := net.Pipe()
			_ = right.Close()
			return left, nil
		},
		contain:      contain,
		pollInterval: time.Millisecond,
	}
}

var containCfg = Config{Endpoint: "deployer@bastion.example.com", SOCKSAddr: "127.0.0.1:1056", ReadyTimeout: time.Second}

// Containment is best effort (#376): when the OS refuses it, OpenSSH still
// starts and closes exactly as without it.
func TestStartSurvivesContainmentFailure(t *testing.T) {
	tunnel, err := start(context.Background(), containCfg, waitHelperDeps(func(*exec.Cmd) (io.Closer, error) {
		return nil, errors.New("access denied")
	}))
	if err != nil {
		t.Fatalf("start() with failing containment = %v, want a tunnel", err)
	}
	if err := tunnel.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
}

// The containment is applied to the started process and released only after
// the process has been waited for, so the graceful path keeps its order:
// cancel, kill, wait, then release.
func TestStartContainsTheStartedProcessAndReleasesAfterWait(t *testing.T) {
	var tunnel *Tunnel
	contained, releasedAfterWait := false, false
	tunnel, err := start(context.Background(), containCfg, waitHelperDeps(func(cmd *exec.Cmd) (io.Closer, error) {
		contained = cmd.Process != nil
		return closerFunc(func() error {
			releasedAfterWait = cmd.ProcessState != nil
			return nil
		}), nil
	}))
	if err != nil {
		t.Fatalf("start() = %v", err)
	}
	if !contained {
		t.Error("containment ran before the process started")
	}
	if err := tunnel.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if !releasedAfterWait {
		t.Error("containment was not released after the process was waited for")
	}
}
