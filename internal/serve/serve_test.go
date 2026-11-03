package serve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"ts-bridge/internal/config"
	"ts-bridge/internal/host"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeNode stands in for tsnet.Server: each Listen opens a loopback listener
// and records the address the node was asked for.
type fakeNode struct {
	mu    sync.Mutex
	addrs []string
	lns   map[string]net.Listener
	fail  string // an address whose Listen fails
}

func (n *fakeNode) Listen(network, addr string) (net.Listener, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.addrs = append(n.addrs, addr)
	if addr == n.fail {
		return nil, errors.New("port in use")
	}
	ln, err := net.Listen(network, "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	if n.lns == nil {
		n.lns = map[string]net.Listener{}
	}
	n.lns[addr] = ln
	return ln, nil
}

// mapDialer sends each requested loopback target to a test address.
type mapDialer struct {
	mu     sync.Mutex
	asked  []string
	target map[string]string
}

func (d *mapDialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	d.mu.Lock()
	d.asked = append(d.asked, addr)
	to, ok := d.target[addr]
	d.mu.Unlock()
	if !ok {
		return nil, errors.New("connection refused")
	}
	return new(net.Dialer).DialContext(ctx, network, to)
}

func echoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()
	return ln.Addr().String()
}

var rdpAndSSH = []Service{{Kind: host.ServiceRDP, Port: 3389}, {Kind: host.ServiceSSH, Port: 22}}

func TestPublish_OnlyDeclaredPorts(t *testing.T) {
	node := &fakeNode{}
	p, err := Publish(node, rdpAndSSH, &mapDialer{}, config.Defaults(), discard)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	defer p.Close()
	if want := []string{":3389", ":22"}; !slices.Equal(node.addrs, want) {
		t.Errorf("node listened on %v, want exactly %v", node.addrs, want)
	}
	if got := p.Services(); !slices.Equal(got, rdpAndSSH) {
		t.Errorf("Services() = %v, want %v", got, rdpAndSSH)
	}
}

func TestPublish_ListenFailureClosesOpened(t *testing.T) {
	node := &fakeNode{fail: ":22"}
	if _, err := Publish(node, rdpAndSSH, &mapDialer{}, config.Defaults(), discard); err == nil || !strings.Contains(err.Error(), "ssh") {
		t.Fatalf("Publish() error = %v, want it to name the ssh service", err)
	}
	if _, err := node.lns[":3389"].Accept(); !errors.Is(err, net.ErrClosed) {
		t.Errorf("the rdp listener stayed open after the ssh one failed: %v", err)
	}
}

func TestPublish_ForwardsToLoopback(t *testing.T) {
	node := &fakeNode{}
	dialer := &mapDialer{target: map[string]string{"127.0.0.1:3389": echoServer(t)}}
	p, err := Publish(node, rdpAndSSH[:1], dialer, config.Defaults(), discard)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	published := node.lns[":3389"].Addr().String()
	c, err := net.DialTimeout("tcp", published, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("round trip = %q, %v; want ping", buf, err)
	}
	_ = c.Close()
	dialer.mu.Lock()
	asked := slices.Clone(dialer.asked)
	dialer.mu.Unlock()
	if !slices.Equal(asked, []string{"127.0.0.1:3389"}) {
		t.Errorf("dialed %v, want only 127.0.0.1:3389", asked)
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
	if c, err := net.DialTimeout("tcp", published, time.Second); err == nil {
		_ = c.Close()
		t.Error("the published port still accepts after Close")
	}
}

func TestNodeConfig_Enrolment(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "authkey")
	if err := os.WriteFile(keyFile, []byte("tskey-auth-abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	str := func(s string) *string { return &s }
	enrolled := t.TempDir()
	if err := os.WriteFile(filepath.Join(enrolled, stateFile), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name          string
		mesh          host.FileMesh
		wantKey       string
		wantEnrolling bool
		wantErr       string
	}{
		{"first enrolment reads the key", host.FileMesh{Hostname: str("lab"), StateDir: str(t.TempDir()), AuthKeyFile: str(keyFile)}, "tskey-auth-abc", true, ""},
		{"state present never opens the key", host.FileMesh{Hostname: str("lab"), StateDir: str(enrolled), AuthKeyFile: str(filepath.Join(t.TempDir(), "gone"))}, "", false, ""},
		{"state present needs no key file", host.FileMesh{Hostname: str("lab"), StateDir: str(enrolled)}, "", false, ""},
		{"first enrolment without a key", host.FileMesh{Hostname: str("lab"), StateDir: str(t.TempDir())}, "", false, "host.mesh.auth_key_file"},
		{"first enrolment with a missing key", host.FileMesh{Hostname: str("lab"), StateDir: str(t.TempDir()), AuthKeyFile: str(filepath.Join(t.TempDir(), "gone"))}, "", false, "gone"},
		{"no state_dir", host.FileMesh{Hostname: str("lab")}, "", false, "host.mesh.state_dir"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nc, err := NewNodeConfig(tc.mesh, "https://hs.example")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("NewNodeConfig() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewNodeConfig() error = %v", err)
			}
			if nc.AuthKey != tc.wantKey || nc.Enrolling != tc.wantEnrolling {
				t.Errorf("AuthKey %q, Enrolling %v; want %q, %v", nc.AuthKey, nc.Enrolling, tc.wantKey, tc.wantEnrolling)
			}
			s := nc.Server()
			if s.Ephemeral || s.Dir != *tc.mesh.StateDir || s.Hostname != "lab" || s.ControlURL != "https://hs.example" || s.AuthKey != tc.wantKey {
				t.Errorf("tsnet.Server = {Ephemeral %v Dir %q Hostname %q ControlURL %q}, want a persistent node from the file",
					s.Ephemeral, s.Dir, s.Hostname, s.ControlURL)
			}
		})
	}
}

// The tsnet node logs through NodeConfig.Logf, the path host serve uses to
// show tsnet's lines under --verbose.
func TestNodeConfig_ServerLogs(t *testing.T) {
	var got []string
	nc := NodeConfig{Hostname: "lab", StateDir: t.TempDir(), Logf: func(format string, args ...any) {
		got = append(got, fmt.Sprintf(format, args...))
	}}
	nc.Server().Logf("control: %s", "refused")
	if len(got) != 1 || got[0] != "control: refused" {
		t.Errorf("tsnet log lines = %q, want them passed to NodeConfig.Logf", got)
	}
	NodeConfig{Hostname: "lab"}.Server().Logf("discarded") // nil Logf must not panic
}
