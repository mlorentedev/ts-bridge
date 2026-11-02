package cmd_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	cmdpkg "ts-bridge/cmd/cli"
	"ts-bridge/internal/serve"
)

// fakeServeNode stands in for the tsnet node: Listen opens a loopback
// listener and records the address serve asked for.
type fakeServeNode struct {
	mu     sync.Mutex
	addrs  []string
	closed bool
}

func (n *fakeServeNode) Listen(network, addr string) (net.Listener, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.addrs = append(n.addrs, addr)
	return net.Listen(network, "127.0.0.1:0")
}

func (n *fakeServeNode) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.closed = true
	return nil
}

// serveRun is what one `host serve` run did.
type serveRun struct {
	starts         int
	nc             serve.NodeConfig
	node           *fakeServeNode
	err            error
	stdout, stderr string
}

// runHostServe runs `host serve` as goos with a node starter that reports
// tags. The context is already cancelled, so a run that gets to READY
// returns right after printing it.
func runHostServe(t *testing.T, goos string, args []string, tags []string, ensure func(string) error) serveRun {
	t.Helper()
	run := serveRun{node: &fakeServeNode{}}
	start := func(_ context.Context, nc serve.NodeConfig) (cmdpkg.ServeNodeForTest, []string, error) {
		run.starts++
		run.nc = nc
		return run.node, tags, nil
	}
	if ensure == nil {
		ensure = func(string) error { return nil }
	}
	c := cmdpkg.HostServeCmdForTest(t, goos, start, ensure)
	var out, errOut bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errOut)
	c.SetArgs(args)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run.err = c.ExecuteContext(ctx)
	run.stdout, run.stderr = out.String(), errOut.String()
	return run
}

// serveGOOS is the OS serve runs as here: this one, or linux on macOS, which
// has no serve until #459 but whose temp paths are absolute Unix paths.
func serveGOOS() string {
	if runtime.GOOS == "windows" {
		return "windows"
	}
	return "linux"
}

// serveFile writes a host file whose mesh keeps its state in a fresh
// directory, and returns the file and that directory.
func serveFile(t *testing.T, extra string) (path, stateDir string) {
	t.Helper()
	dir := isolateHostEnv(t)
	stateDir = filepath.Join(dir, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	key := writeTestFile(t, filepath.Join(dir, "authkey"), "tskey-auth-test\n")
	body := "host:\n  control_url: https://hs.example\n  services:\n    - kind: rdp\n      port: 3390\n" +
		"  mesh:\n    hostname: lab-01\n    state_dir: '" + stateDir + "'\n    auth_key_file: '" + key + "'\n" + extra
	return writeTestFile(t, filepath.Join(dir, "host.yaml"), body), stateDir
}

func TestHostServeCmd_Requires(t *testing.T) {
	tagged := []string{"tag:lab"}
	t.Run("no --config", func(t *testing.T) {
		isolateHostEnv(t)
		run := runHostServe(t, serveGOOS(), nil, tagged, nil)
		assertServeRefused(t, run, "--config")
	})
	files := []struct{ name, body, want string }{
		{"no host.mesh", "host:\n  control_url: https://hs.example\n", "host.mesh"},
		{"no control_url", "host:\n  mesh:\n    hostname: lab\n", "host.control_url"},
		{"no state_dir", "host:\n  control_url: https://hs.example\n  mesh:\n    hostname: lab\n", "host.mesh.state_dir"},
	}
	for _, tc := range files {
		t.Run(tc.name, func(t *testing.T) {
			dir := isolateHostEnv(t)
			path := writeTestFile(t, filepath.Join(dir, "host.yaml"), tc.body)
			assertServeRefused(t, runHostServe(t, serveGOOS(), []string{"--config", path}, tagged, nil), tc.want)
		})
	}
	t.Run("a loose state_dir", func(t *testing.T) {
		path, stateDir := serveFile(t, "")
		var checked string
		ensure := func(p string) error { checked = p; return errors.New("state_dir " + p + ": mode 0755, want 0700") }
		assertServeRefused(t, runHostServe(t, serveGOOS(), []string{"--config", path}, tagged, ensure), "mode 0755")
		if checked != stateDir {
			t.Errorf("EnsureStateDir(%q), want the declared state_dir %q", checked, stateDir)
		}
	})
}

func assertServeRefused(t *testing.T, run serveRun, want string) {
	t.Helper()
	if run.starts != 0 {
		t.Errorf("the node started %d time(s), want none", run.starts)
	}
	if cmdpkg.ExitCode(run.err) != 1 || run.err == nil || !strings.Contains(run.err.Error(), want) {
		t.Errorf("err = %v (exit %d), want exit 1 naming %q", run.err, cmdpkg.ExitCode(run.err), want)
	}
	if strings.Contains(run.stdout, "READY") {
		t.Errorf("stdout = %q, want no READY", run.stdout)
	}
}

func TestHostServeCmd_Ready(t *testing.T) {
	for _, enrolled := range []bool{false, true} {
		path, stateDir := serveFile(t, "")
		if enrolled {
			writeTestFile(t, filepath.Join(stateDir, "tailscaled.state"), "{}")
		}
		run := runHostServe(t, serveGOOS(), []string{"--config", path}, []string{"tag:lab"}, nil)
		if run.err != nil {
			t.Fatalf("enrolled=%v: err = %v, stderr %q", enrolled, run.err, run.stderr)
		}
		want := "READY node=lab-01 services=rdp:3390 enrolled=true\n"
		if enrolled {
			want = "READY node=lab-01 services=rdp:3390 enrolled=false\n"
		}
		if !strings.Contains(run.stdout, want) {
			t.Errorf("stdout = %q, want %q", run.stdout, want)
		}
		if run.nc.ControlURL != "https://hs.example" || run.nc.StateDir != stateDir {
			t.Errorf("node config %+v, want the file's control plane and state_dir", run.nc)
		}
		if len(run.node.addrs) != 1 || run.node.addrs[0] != ":3390" || !run.node.closed {
			t.Errorf("node listened on %v (closed %v), want only :3390 and closed on exit", run.node.addrs, run.node.closed)
		}
	}
}

func TestHostServeCmd_RefusesUntaggedNode(t *testing.T) {
	path, _ := serveFile(t, "")
	run := runHostServe(t, serveGOOS(), []string{"--config", path}, nil, nil)
	if cmdpkg.ExitCode(run.err) != 1 {
		t.Errorf("exit %d (err %v), want 1", cmdpkg.ExitCode(run.err), run.err)
	}
	if !strings.Contains(run.stderr, "ERROR reason=node_not_tagged") || strings.Contains(run.stdout, "READY") {
		t.Errorf("stdout %q, stderr %q; want ERROR reason=node_not_tagged and no READY", run.stdout, run.stderr)
	}
	if len(run.node.addrs) != 0 || !run.node.closed {
		t.Errorf("node listened on %v (closed %v), want nothing published and the node closed", run.node.addrs, run.node.closed)
	}
}
