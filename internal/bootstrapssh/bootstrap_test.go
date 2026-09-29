package bootstrapssh

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildSSHArgs(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     []string
		wantErr  string
	}{
		{
			name:     "user and host",
			endpoint: "deployer@bastion.example.com",
			want: []string{
				"-N", "-D", "127.0.0.1:1055",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=15",
				"-o", "ServerAliveCountMax=3",
				"-l", "deployer", "bastion.example.com",
			},
		},
		{
			name:     "custom SSH port",
			endpoint: "deployer@bastion.example.com:2222",
			want: []string{
				"-N", "-D", "127.0.0.1:1055",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=15",
				"-o", "ServerAliveCountMax=3",
				"-p", "2222", "-l", "deployer", "bastion.example.com",
			},
		},
		{
			name:     "SSH config alias",
			endpoint: "corp-bastion",
			want: []string{
				"-N", "-D", "127.0.0.1:1055",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=15",
				"-o", "ServerAliveCountMax=3",
				"corp-bastion",
			},
		},
		{name: "empty endpoint", endpoint: "", wantErr: "must not be empty"},
		{name: "option injection", endpoint: "-oProxyCommand=bad", wantErr: "must not start with"},
		{name: "invalid port", endpoint: "host:notaport", wantErr: "invalid SSH port"},
		{name: "empty explicit port", endpoint: "host:", wantErr: "invalid SSH port"},
		{name: "unbracketed IPv6", endpoint: "user@2001:db8::1", wantErr: "IPv6 addresses must be bracketed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildSSHArgs(tt.endpoint, "127.0.0.1:1055")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("buildSSHArgs() error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildSSHArgs() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildSSHArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestStartWaitsForSOCKSAndClosesProcess(t *testing.T) {
	started := make(chan struct{})
	deps := dependencies{
		lookPath: func(string) (string, error) { return "ssh", nil },
		commandContext: func(ctx context.Context, _ string, args ...string) *exec.Cmd {
			//nolint:gosec // os.Args[0] and the test selector are fixed test-process inputs.
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestSSHHelperProcess")
			cmd.Env = append(os.Environ(), "TS_BRIDGE_SSH_HELPER=wait")
			close(started)
			return cmd
		},
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			left, right := net.Pipe()
			_ = right.Close()
			return left, nil
		},
		pollInterval: time.Millisecond,
	}

	tunnel, err := start(context.Background(), Config{
		Endpoint:     "deployer@bastion.example.com",
		SOCKSAddr:    "127.0.0.1:1055",
		ReadyTimeout: time.Second,
	}, deps)
	if err != nil {
		t.Fatalf("start() error = %v", err)
	}
	<-started

	if err := tunnel.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case <-tunnel.Done():
	case <-time.After(time.Second):
		t.Fatal("SSH process did not stop after Close")
	}
}

func TestStartReportsEarlyExit(t *testing.T) {
	deps := dependencies{
		lookPath: func(string) (string, error) { return "ssh", nil },
		commandContext: func(ctx context.Context, _ string, args ...string) *exec.Cmd {
			//nolint:gosec // os.Args[0] and the test selector are fixed test-process inputs.
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestSSHHelperProcess")
			cmd.Env = append(os.Environ(), "TS_BRIDGE_SSH_HELPER=exit")
			return cmd
		},
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("not ready")
		},
		pollInterval: time.Millisecond,
	}

	_, err := start(context.Background(), Config{
		Endpoint:     "bastion.example.com",
		SOCKSAddr:    "127.0.0.1:1055",
		ReadyTimeout: time.Second,
	}, deps)
	if err == nil || !strings.Contains(err.Error(), "exited before SOCKS listener became ready") {
		t.Fatalf("start() error = %v", err)
	}
}

func TestStartReportsCleanEarlyExit(t *testing.T) {
	deps := dependencies{
		lookPath: func(string) (string, error) { return "ssh", nil },
		commandContext: func(ctx context.Context, _ string, args ...string) *exec.Cmd {
			//nolint:gosec // os.Args[0] and the test selector are fixed test-process inputs.
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestSSHHelperProcess")
			cmd.Env = append(os.Environ(), "TS_BRIDGE_SSH_HELPER=exit-zero")
			return cmd
		},
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("not ready")
		},
		pollInterval: time.Millisecond,
	}

	_, err := start(context.Background(), Config{
		Endpoint:     "bastion.example.com",
		SOCKSAddr:    "127.0.0.1:1055",
		ReadyTimeout: time.Second,
	}, deps)
	if err == nil || !strings.Contains(err.Error(), "exited successfully before SOCKS listener became ready") {
		t.Fatalf("start() error = %v", err)
	}
}

func TestStartReportsMissingOpenSSH(t *testing.T) {
	deps := dependencies{
		lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
	}
	_, err := start(context.Background(), Config{
		Endpoint:     "bastion.example.com",
		SOCKSAddr:    "127.0.0.1:1055",
		ReadyTimeout: time.Second,
	}, deps)
	if err == nil || !strings.Contains(err.Error(), "OpenSSH executable") {
		t.Fatalf("start() error = %v", err)
	}
}

func TestControlProxySelectsOnlyControlHostname(t *testing.T) {
	fallbackURL, _ := url.Parse("http://existing-proxy.example:8080")
	proxyFor, err := controlProxyFunc(
		"https://vpn.example.com:8443",
		"127.0.0.1:1055",
		func(*url.URL) (*url.URL, error) { return fallbackURL, nil },
	)
	if err != nil {
		t.Fatalf("controlProxyFunc() error = %v", err)
	}

	controlURL, _ := url.Parse("https://vpn.example.com:8443/key")
	got, err := proxyFor(controlURL)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "socks5h://127.0.0.1:1055" {
		t.Fatalf("control proxy = %q", got)
	}

	unrelatedURL, _ := url.Parse("https://login.tailscale.com/")
	got, err = proxyFor(unrelatedURL)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != fallbackURL.String() {
		t.Fatalf("unrelated proxy = %q, want %q", got, fallbackURL)
	}
}

func TestControlProxyPreservesHostnameThroughSOCKS(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	hostSeen := make(chan string, 1)
	go serveOneSOCKSHandshake(t, listener, hostSeen)

	proxyFor, err := controlProxyFunc("https://vpn.example.com", listener.Addr().String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return proxyFor(req.URL)
		},
	}
	client := &http.Client{Transport: transport, Timeout: time.Second}
	_, _ = client.Get("https://vpn.example.com/key")

	select {
	case got := <-hostSeen:
		if got != "vpn.example.com" {
			t.Fatalf("SOCKS destination host = %q, want vpn.example.com", got)
		}
	case <-time.After(time.Second):
		t.Fatal("SOCKS proxy did not receive a connection")
	}
}

func serveOneSOCKSHandshake(t *testing.T, listener net.Listener, hostSeen chan<- string) {
	t.Helper()
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	greeting := make([]byte, 3)
	if _, err := io.ReadFull(conn, greeting); err != nil {
		t.Errorf("read greeting: %v", err)
		return
	}
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		t.Errorf("write greeting: %v", err)
		return
	}

	header := make([]byte, 5)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Errorf("read request: %v", err)
		return
	}
	if header[3] != 3 {
		t.Errorf("SOCKS address type = %d, want domain name", header[3])
		return
	}
	host := make([]byte, int(header[4]))
	if _, err := io.ReadFull(conn, host); err != nil {
		t.Errorf("read host: %v", err)
		return
	}
	port := make([]byte, 2)
	if _, err := io.ReadFull(conn, port); err != nil {
		t.Errorf("read port: %v", err)
		return
	}
	hostSeen <- string(host)
	_, _ = conn.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
}

func TestSSHHelperProcess(t *testing.T) {
	switch os.Getenv("TS_BRIDGE_SSH_HELPER") {
	case "":
		return
	case "exit":
		os.Exit(7)
	case "exit-zero":
		os.Exit(0)
	case "wait":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	default:
		os.Exit(8)
	}
}
