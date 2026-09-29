package bootstrapssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"tailscale.com/net/tshttpproxy"
)

const defaultReadyTimeout = 30 * time.Second
const defaultPollInterval = 50 * time.Millisecond

type Config struct {
	Endpoint     string
	SOCKSAddr    string
	ReadyTimeout time.Duration
}

type dependencies struct {
	lookPath       func(string) (string, error)
	commandContext func(context.Context, string, ...string) *exec.Cmd
	dialContext    func(context.Context, string, string) (net.Conn, error)
	pollInterval   time.Duration
}

type Tunnel struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func Start(ctx context.Context, cfg Config) (*Tunnel, error) {
	return start(ctx, cfg, dependencies{
		lookPath:       exec.LookPath,
		commandContext: exec.CommandContext,
		dialContext:    new(net.Dialer).DialContext,
		pollInterval:   defaultPollInterval,
	})
}

func start(ctx context.Context, cfg Config, deps dependencies) (*Tunnel, error) {
	sshPath, err := deps.lookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("find OpenSSH executable %q in PATH: %w", "ssh", err)
	}

	args, err := buildSSHArgs(cfg.Endpoint, cfg.SOCKSAddr)
	if err != nil {
		return nil, err
	}

	childCtx, cancel := context.WithCancel(ctx)
	probe, err := net.Listen("tcp", cfg.SOCKSAddr)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot bind bootstrap SOCKS address %q: %w", cfg.SOCKSAddr, err)
	}
	_ = probe.Close()

	cmd := deps.commandContext(childCtx, sshPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start OpenSSH bootstrap: %w", err)
	}

	tunnel := &Tunnel{cancel: cancel, done: make(chan struct{})}
	go func() {
		tunnel.err = cmd.Wait()
		close(tunnel.done)
	}()

	if err := waitUntilReady(ctx, tunnel, cfg, deps); err != nil {
		_ = tunnel.Close()
		return nil, err
	}
	return tunnel, nil
}

func waitUntilReady(ctx context.Context, tunnel *Tunnel, cfg Config, deps dependencies) error {
	timeout := cfg.ReadyTimeout
	if timeout <= 0 {
		timeout = defaultReadyTimeout
	}
	pollInterval := deps.pollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		conn, err := deps.dialContext(readyCtx, "tcp", cfg.SOCKSAddr)
		if err == nil {
			_ = conn.Close()
			select {
			case <-tunnel.Done():
				return exitBeforeReadyError(tunnel.Err())
			default:
				return nil
			}
		}

		select {
		case <-readyCtx.Done():
			if ctx.Err() != nil {
				return fmt.Errorf("wait for OpenSSH bootstrap: %w", ctx.Err())
			}
			return fmt.Errorf("OpenSSH bootstrap SOCKS listener %s was not ready within %s", cfg.SOCKSAddr, timeout)
		case <-tunnel.Done():
			return exitBeforeReadyError(tunnel.Err())
		case <-ticker.C:
		}
	}
}

func (t *Tunnel) Done() <-chan struct{} {
	return t.done
}

func (t *Tunnel) Err() error {
	return t.err
}

func (t *Tunnel) Close() error {
	t.cancel()
	<-t.done
	return nil
}

func buildSSHArgs(endpoint, socksAddr string) ([]string, error) {
	user, host, port, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	socksHost, _, err := net.SplitHostPort(socksAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid bootstrap SOCKS address %q: %w", socksAddr, err)
	}
	ip := net.ParseIP(socksHost)
	if socksHost != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("bootstrap SOCKS listener must bind to loopback, got %q", socksHost)
	}

	args := []string{
		"-N", "-D", socksAddr,
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
	}
	if port != "" {
		args = append(args, "-p", port)
	}
	if user != "" {
		args = append(args, "-l", user)
	}
	return append(args, host), nil
}

func parseEndpoint(endpoint string) (user, host, port string, err error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", "", "", fmt.Errorf("SSH bootstrap endpoint must not be empty")
	}
	if strings.HasPrefix(endpoint, "-") {
		return "", "", "", fmt.Errorf("SSH bootstrap endpoint must not start with %q", "-")
	}
	if strings.ContainsAny(endpoint, " \t\r\n") {
		return "", "", "", fmt.Errorf("SSH bootstrap endpoint must not contain whitespace")
	}

	user, hostPort, err := splitSSHUser(endpoint)
	if err != nil {
		return "", "", "", err
	}
	host, port, err = splitSSHHostPort(endpoint, hostPort)
	if err != nil {
		return "", "", "", err
	}
	if host == "" || strings.HasPrefix(host, "-") {
		return "", "", "", fmt.Errorf("invalid SSH host in endpoint %q", endpoint)
	}
	if err := validateSSHPort(port); err != nil {
		return "", "", "", err
	}
	return user, host, port, nil
}

func splitSSHUser(endpoint string) (user, hostPort string, err error) {
	at := strings.LastIndex(endpoint, "@")
	if at < 0 {
		return "", endpoint, nil
	}
	user, hostPort = endpoint[:at], endpoint[at+1:]
	if user == "" || strings.HasPrefix(user, "-") {
		return "", "", fmt.Errorf("invalid SSH user in endpoint %q", endpoint)
	}
	return user, hostPort, nil
}

func splitSSHHostPort(endpoint, hostPort string) (host, port string, err error) {
	switch {
	case strings.HasPrefix(hostPort, "["):
		if strings.Contains(hostPort, "]:") {
			host, port, err = net.SplitHostPort(hostPort)
			if err != nil {
				return "", "", fmt.Errorf("invalid SSH endpoint %q: %w", endpoint, err)
			}
		} else if strings.HasSuffix(hostPort, "]") {
			host = strings.TrimSuffix(strings.TrimPrefix(hostPort, "["), "]")
		} else {
			return "", "", fmt.Errorf("invalid bracketed SSH host %q", hostPort)
		}
	case strings.Count(hostPort, ":") == 0:
		host = hostPort
	case strings.Count(hostPort, ":") == 1:
		host, port, _ = strings.Cut(hostPort, ":")
		if port == "" {
			return "", "", fmt.Errorf("invalid SSH port %q", port)
		}
	default:
		return "", "", fmt.Errorf("IPv6 addresses must be bracketed in SSH endpoint %q", endpoint)
	}
	return host, port, nil
}

func validateSSHPort(port string) error {
	if port == "" {
		return nil
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("invalid SSH port %q", port)
	}
	return nil
}

func exitBeforeReadyError(err error) error {
	if err == nil {
		return errors.New("OpenSSH bootstrap exited successfully before SOCKS listener became ready")
	}
	return fmt.Errorf("OpenSSH bootstrap exited before SOCKS listener became ready: %w", err)
}

func controlProxyFunc(
	controlURL string,
	socksAddr string,
	fallback func(*url.URL) (*url.URL, error),
) (func(*url.URL) (*url.URL, error), error) {
	control, err := url.Parse(controlURL)
	if err != nil {
		return nil, fmt.Errorf("parse control URL %q: %w", controlURL, err)
	}
	if control.Hostname() == "" {
		return nil, fmt.Errorf("control URL %q has no hostname", controlURL)
	}
	proxyURL, err := url.Parse("socks5h://" + socksAddr)
	if err != nil {
		return nil, fmt.Errorf("parse bootstrap SOCKS proxy: %w", err)
	}

	return func(requestURL *url.URL) (*url.URL, error) {
		if strings.EqualFold(requestURL.Hostname(), control.Hostname()) {
			return proxyURL, nil
		}
		if fallback == nil {
			return nil, nil
		}
		return fallback(requestURL)
	}, nil
}

func ConfigureControlProxy(controlURL, socksAddr string) error {
	fallback := func(requestURL *url.URL) (*url.URL, error) {
		return http.ProxyFromEnvironment(&http.Request{URL: requestURL})
	}
	proxyFor, err := controlProxyFunc(controlURL, socksAddr, fallback)
	if err != nil {
		return err
	}
	if err := tshttpproxy.SetProxyFunc(proxyFor); err != nil {
		return fmt.Errorf("configure Tailscale control proxy before network startup: %w", err)
	}
	return nil
}
