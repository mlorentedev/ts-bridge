//go:build !windows

package bootstrapssh

import (
	"context"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The ssh bootstrap child inherits the whole process environment. connect
// puts TS_AUTHKEY there (explicit env, or .env loaded with os.Setenv), so the
// auth key reaches ssh and everything ssh_config makes it run (ProxyCommand,
// LocalCommand, Match exec), and a SendEnv TS_* or * forwards it to the
// bootstrap server.
func TestBootstrapSSHDoesNotInheritAuthKey(t *testing.T) {
	t.Setenv("TS_AUTHKEY", "tskey-auth-audit-secret")
	var captured *exec.Cmd
	deps := dependencies{
		lookPath: func(string) (string, error) { return "ssh", nil },
		commandContext: func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			captured = exec.CommandContext(ctx, "sleep", "5")
			return captured
		},
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			left, right := net.Pipe()
			_ = right.Close()
			return left, nil
		},
		prepare:      prepareContainment,
		contain:      containProcess,
		pollInterval: time.Millisecond,
	}
	tunnel, err := start(context.Background(), Config{
		Endpoint: "deployer@bastion.example.com", SOCKSAddr: "127.0.0.1:1056", ReadyTimeout: time.Second,
	}, deps)
	if err != nil {
		t.Fatalf("start() error = %v", err)
	}
	defer tunnel.Close()

	for _, kv := range captured.Environ() {
		if strings.HasPrefix(kv, "TS_AUTHKEY=") {
			t.Fatal("ssh bootstrap child environment carries TS_AUTHKEY")
		}
	}
}
