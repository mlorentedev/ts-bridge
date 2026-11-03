package cmd_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	cmdpkg "ts-bridge/cmd/cli"
	"ts-bridge/internal/bootstrapssh"
	"ts-bridge/internal/serve"
)

// fakeServeNode stands in for the tsnet node: Listen opens a loopback
// listener and records the address serve asked for.
type fakeServeNode struct {
	mu        sync.Mutex
	addrs     []string
	closed    bool
	listenErr error
}

func (n *fakeServeNode) Listen(network, addr string) (net.Listener, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.addrs = append(n.addrs, addr)
	if n.listenErr != nil {
		return nil, n.listenErr
	}
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
	return runHostServeOn(t, &fakeServeNode{}, goos, args, tags, ensure)
}

// runHostServeOn is runHostServe on a given fake node.
func runHostServeOn(t *testing.T, node *fakeServeNode, goos string, args []string, tags []string, ensure func(string) error) serveRun {
	t.Helper()
	run := serveRun{node: node}
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

// syncBuffer is a bytes.Buffer safe to read while serve writes to it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// liveServe is a host serve run that keeps running until stop.
type liveServe struct {
	stdout, stderr *syncBuffer
	done           chan error
	stop           context.CancelFunc
}

// startHostServe runs host serve in the background with a tagged fake node.
func startHostServe(t *testing.T, args []string) *liveServe {
	t.Helper()
	start := func(context.Context, serve.NodeConfig) (cmdpkg.ServeNodeForTest, []string, error) {
		return &fakeServeNode{}, []string{"tag:lab"}, nil
	}
	c := cmdpkg.HostServeCmdForTest(t, serveGOOS(), start, func(string) error { return nil })
	ls := &liveServe{stdout: &syncBuffer{}, stderr: &syncBuffer{}, done: make(chan error, 1)}
	c.SetOut(ls.stdout)
	c.SetErr(ls.stderr)
	c.SetArgs(args)
	ctx, cancel := context.WithCancel(context.Background())
	ls.stop = cancel
	go func() { ls.done <- c.ExecuteContext(ctx) }()
	t.Cleanup(func() { cancel(); <-ls.done })
	waitFor(t, "READY", func() bool { return strings.Contains(ls.stdout.String(), "READY ") })
	return ls
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

// After READY a dead bootstrap is reported once and restarted: serve keeps
// running and /health/ready reads not ready until the tunnel is back (AC4).
func TestHostServeCmd_BootstrapRestarts(t *testing.T) {
	addr := freeLoopbackAddr(t)
	path, _ := serveFile(t, "    bootstrap_ssh: tsb@jump.example\n    health_addr: "+addr+"\n")
	tunnels := make(chan *fakeBootstrap, 4)
	cmdpkg.SetServeBootstrapForTest(t,
		func(context.Context, bootstrapssh.Config) (cmdpkg.ControlBootstrapForTest, error) {
			b := &fakeBootstrap{done: make(chan struct{})}
			tunnels <- b
			return b, nil
		},
		func(context.Context, string, string) error { return nil })
	ls := startHostServe(t, []string{"--config", path})
	first := <-tunnels
	waitFor(t, "ready health", func() bool { code, _ := getReady(addr); return code == 200 })

	_ = first.Close() // the SSH child dies
	waitFor(t, "ERROR line", func() bool { return strings.Contains(ls.stderr.String(), "ERROR reason=ssh_bootstrap_failed") })
	waitFor(t, "not ready during the outage", func() bool {
		code, body := getReady(addr)
		return code == http.StatusServiceUnavailable && strings.Contains(body, `"reason":"ssh_bootstrap_failed"`)
	})
	select {
	case err := <-ls.done:
		t.Fatalf("serve exited on a bootstrap outage: %v", err)
	default:
	}
	select {
	case <-tunnels:
	case <-time.After(10 * time.Second):
		t.Fatal("the bootstrap was not restarted")
	}
	waitFor(t, "ready again", func() bool { code, _ := getReady(addr); return code == 200 })
	if n := strings.Count(ls.stderr.String(), "ERROR reason=ssh_bootstrap_failed"); n != 1 {
		t.Errorf("%d ERROR lines for one outage, want 1", n)
	}
}

// /health/ready on host.mesh.health_addr names the node and what serve
// publishes, so host check can trust it (AC6).
func TestHostServeCmd_Health(t *testing.T) {
	addr := freeLoopbackAddr(t)
	path, _ := serveFile(t, "    health_addr: "+addr+"\n")
	startHostServe(t, []string{"--config", path})
	var code int
	var body string
	waitFor(t, "the health server", func() bool { code, body = getReady(addr); return code != 0 })
	want := `"node":"lab-01","services":[{"kind":"rdp","port":3390,"published":true}]`
	if code != 200 || !strings.Contains(body, want) {
		t.Errorf("/health/ready = %d %s, want 200 with %s", code, body, want)
	}
}

func getReady(addr string) (int, string) {
	resp, err := (&http.Client{Timeout: time.Second}).Get("http://" + addr + "/health/ready")
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
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
	// The key is never read again once state_dir holds the node, so the advice
	// must name the steps that get a tagged key read (review of #558).
	for _, want := range []string{"remove the node", "empty state_dir", "tagged"} {
		if !strings.Contains(run.stderr, want) {
			t.Errorf("stderr %q does not name %q", run.stderr, want)
		}
	}
	if len(run.node.addrs) != 0 || !run.node.closed {
		t.Errorf("node listened on %v (closed %v), want nothing published and the node closed", run.node.addrs, run.node.closed)
	}
}

// fakeBootstrap is an SSH bootstrap that runs until closed.
type fakeBootstrap struct {
	done   chan struct{}
	closed bool
}

func (b *fakeBootstrap) Done() <-chan struct{} { return b.done }
func (b *fakeBootstrap) Err() error            { return nil }
func (b *fakeBootstrap) Close() error {
	if !b.closed {
		b.closed = true
		close(b.done)
	}
	return nil
}

// TestHostServeCmd_Bootstrap: host.mesh.bootstrap_ssh starts the CLI-363
// tunnel on the declared (or default) SOCKS address and routes the file's
// control plane through it before the node starts; a tunnel that cannot
// start emits ssh_bootstrap_failed and starts no node.
func TestHostServeCmd_Bootstrap(t *testing.T) {
	tests := []struct {
		name, extra, wantSOCKS string
		startErr               error
	}{
		{"default SOCKS address", "    bootstrap_ssh: tsb@jump.example\n", "127.0.0.1:1055", nil},
		{"declared SOCKS address", "    bootstrap_ssh: tsb@jump.example\n    bootstrap_socks_addr: 127.0.0.1:1056\n", "127.0.0.1:1056", nil},
		{"tunnel fails", "    bootstrap_ssh: tsb@jump.example\n", "127.0.0.1:1055", errors.New("ssh: connection refused")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, _ := serveFile(t, tc.extra)
			var got bootstrapssh.Config
			var proxied []string
			tunnel := &fakeBootstrap{done: make(chan struct{})}
			cmdpkg.SetServeBootstrapForTest(t,
				func(_ context.Context, cfg bootstrapssh.Config) (cmdpkg.ControlBootstrapForTest, error) {
					got = cfg
					if tc.startErr != nil {
						return nil, tc.startErr
					}
					return tunnel, nil
				},
				func(_ context.Context, controlURL, socks string) error {
					proxied = []string{controlURL, socks}
					return nil
				})
			run := runHostServe(t, serveGOOS(), []string{"--config", path}, []string{"tag:lab"}, nil)
			if got.Endpoint != "tsb@jump.example" || got.SOCKSAddr != tc.wantSOCKS {
				t.Errorf("tunnel config %+v, want endpoint tsb@jump.example on %s", got, tc.wantSOCKS)
			}
			if tc.startErr != nil {
				if run.starts != 0 || cmdpkg.ExitCode(run.err) != 1 || !strings.Contains(run.stderr, "ERROR reason=ssh_bootstrap_failed") {
					t.Errorf("starts %d, exit %d, stderr %q; want no node, exit 1 and ssh_bootstrap_failed", run.starts, cmdpkg.ExitCode(run.err), run.stderr)
				}
				return
			}
			if want := []string{"https://hs.example", tc.wantSOCKS}; !slices.Equal(proxied, want) {
				t.Errorf("control proxy %v, want %v", proxied, want)
			}
			if run.err != nil || !strings.Contains(run.stdout, "READY node=lab-01") || !tunnel.closed {
				t.Errorf("err %v, stdout %q, tunnel closed %v; want READY and the tunnel closed on exit", run.err, run.stdout, tunnel.closed)
			}
		})
	}
}

// A listener that cannot open on the node is a classified failure, not a bare
// error: serve prints ERROR reason=publish_failed and no READY.
func TestHostServeCmd_PublishFailure(t *testing.T) {
	path, _ := serveFile(t, "")
	node := &fakeServeNode{listenErr: errors.New("address already in use")}
	run := runHostServeOn(t, node, serveGOOS(), []string{"--config", path}, []string{"tag:lab"}, nil)
	if cmdpkg.ExitCode(run.err) != 1 || !strings.Contains(run.stderr, "ERROR reason=publish_failed") || strings.Contains(run.stdout, "READY") {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1, ERROR reason=publish_failed and no READY", cmdpkg.ExitCode(run.err), run.stdout, run.stderr)
	}
	if !node.closed {
		t.Error("the node stayed up after publishing failed")
	}
}

// The READY line's enrolled field is defined in the help, since it is a
// machine-readable contract: true means this start enrolled with the key.
func TestHostServeCmd_HelpDefinesEnrolled(t *testing.T) {
	c := cmdpkg.HostServeCmdForTest(t, serveGOOS(), nil, nil)
	for _, want := range []string{"enrolled=true", "enrolled=false", "reason=publish_failed"} {
		if !strings.Contains(c.Long, want) {
			t.Errorf("host serve help does not define %s", want)
		}
	}
}

// The production command tree reaches host serve (the #279 / ADR-013 wiring
// class): tests that build the command directly cannot catch a missing edge.
func TestRootCmd_ReachesHostServe(t *testing.T) {
	c, _, err := cmdpkg.NewRootCmd().Find([]string{"host", "serve"})
	if err != nil || c.Name() != "serve" || c.Parent() == nil || c.Parent().Name() != "host" {
		t.Fatalf("root.Find(host serve) = %v, %v; want the serve command under host", c, err)
	}
	if c.Flags().Lookup("config") == nil {
		t.Error("host serve has no local --config")
	}
}

// --verbose shows tsnet's own log lines, which explain a failed join; without
// it they stay out of the operator's output (review of #558).
func TestHostServeCmd_VerboseShowsNodeLogs(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		path, _ := serveFile(t, "")
		args := []string{"--config", path}
		if verbose {
			args = append(args, "--verbose")
		}
		node := &fakeServeNode{}
		var errOut bytes.Buffer
		start := func(_ context.Context, nc serve.NodeConfig) (cmdpkg.ServeNodeForTest, []string, error) {
			if nc.Logf == nil {
				t.Fatal("NodeConfig.Logf is nil: tsnet's logs are discarded")
			}
			nc.Logf("control: %s", "map response refused")
			return node, []string{"tag:lab"}, nil
		}
		c := cmdpkg.HostServeCmdForTest(t, serveGOOS(), start, func(string) error { return nil })
		c.SetOut(io.Discard)
		c.SetErr(&errOut)
		c.SetArgs(args)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := c.ExecuteContext(ctx); err != nil {
			t.Fatalf("verbose=%v: %v", verbose, err)
		}
		if got := strings.Contains(errOut.String(), "map response refused"); got != verbose {
			t.Errorf("verbose=%v: node log shown %v, stderr %q", verbose, got, errOut.String())
		}
	}
}
