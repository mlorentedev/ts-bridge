package bootstrapssh

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tailscale.com/derp"
	"tailscale.com/derp/derphttp"
	"tailscale.com/derp/derpserver"
	_ "tailscale.com/feature/useproxy"
	"tailscale.com/net/netmon"
	"tailscale.com/net/socks5"
	"tailscale.com/net/tshttpproxy"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
)

func TestControlProxySDKConsumers(t *testing.T) {
	// The SDK proxy hook is process-global and cannot be replaced after first use.
	//nolint:gosec // The executable and test selector are fixed test-process inputs.
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestControlProxySDKConsumersHelper$", "-test.v")
	cmd.Env = append(os.Environ(), "TS_BRIDGE_PROXY_HELPER=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("SDK proxy consumers: %v\n%s", err, output)
	}
}

func TestControlProxySDKConsumersHelper(t *testing.T) {
	if os.Getenv("TS_BRIDGE_PROXY_HELPER") != "1" {
		return
	}
	upstream, hostname, socksAddr, destinations := proxySDKFixture(t)
	_, portText, err := net.SplitHostPort(upstream.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	target := net.JoinHostPort(hostname, portText)
	controlURL := "https://" + target
	if err := ConfigureControlProxy(t.Context(), controlURL, socksAddr); err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(upstream.Certificate())
	tlsConfig := &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	transport := &http.Transport{
		Proxy: tshttpproxy.ProxyFromEnvironment, TLSClientConfig: tlsConfig,
	}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	response, err := client.Get(controlURL + "/key")
	if err != nil {
		t.Fatalf("control HTTPS through SOCKS: %v", err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "control-ok" {
		t.Fatalf("control response: status=%d body=%q error=%v", response.StatusCode, body, readErr)
	}
	assertSOCKSDestination(t, destinations, target)

	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	relay := derphttp.NewRegionClient(key.NewNode(), t.Logf, netmon.NewStatic(), func() *tailcfg.DERPRegion {
		return &tailcfg.DERPRegion{RegionID: 999, Nodes: []*tailcfg.DERPNode{{
			Name: "test-relay", RegionID: 999, HostName: hostname, DERPPort: port,
			IPv4: "127.0.0.2", IPv6: "none",
		}}}
	})
	relay.TLSConfig = tlsConfig
	t.Cleanup(func() { _ = relay.Close() })
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = relay.Close() })
	defer stop()
	if err := relay.Connect(ctx); err != nil {
		t.Fatalf("actual DERP consumer through SOCKS: %v", err)
	}
	message, err := relay.Recv()
	if _, ok := message.(derp.ServerInfoMessage); err != nil || !ok {
		t.Fatalf("authenticated DERP server response: message=%T error=%v", message, err)
	}
	assertSOCKSDestination(t, destinations, target)
}

func proxySDKFixture(t *testing.T) (*httptest.Server, string, string, <-chan string) {
	t.Helper()
	relay := derpserver.New(key.NewNode(), t.Logf)
	t.Cleanup(func() { _ = relay.Close() })
	mux := http.NewServeMux()
	mux.Handle("/derp", derpserver.Handler(relay))
	mux.HandleFunc("/key", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "control-ok")
	})
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)
	if len(server.Certificate().DNSNames) == 0 {
		t.Fatal("TLS fixture needs a DNS SAN")
	}
	hostname := server.Certificate().DNSNames[0]
	destinations := make(chan string, 8)
	socksAddr := startTestSOCKS(t, func(ctx context.Context, network, address string) (net.Conn, error) {
		if host, _, err := net.SplitHostPort(address); err != nil || host != hostname {
			return nil, fmt.Errorf("unexpected SOCKS destination %q", address)
		}
		destinations <- address
		return new(net.Dialer).DialContext(ctx, network, server.Listener.Addr().String())
	})
	return server, hostname, socksAddr, destinations
}

func startTestSOCKS(t *testing.T, dial func(context.Context, string, string) (net.Conn, error)) string {
	t.Helper()
	socks := &socks5.Server{Logf: t.Logf, Dialer: dial}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = socks.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
	return listener.Addr().String()
}

func assertSOCKSDestination(t *testing.T, destinations <-chan string, want string) {
	t.Helper()
	select {
	case got := <-destinations:
		if got != want {
			t.Fatalf("SOCKS destination = %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SOCKS server did not receive the original destination hostname")
	}
}

func TestControlProxyHTTP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Host+" "+r.URL.Path)
	}))
	t.Cleanup(upstream.Close)
	socksAddr := startTestSOCKS(t, func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "control.test:8080" {
			return nil, fmt.Errorf("unexpected remote DNS destination %q", address)
		}
		return new(net.Dialer).DialContext(ctx, network, upstream.Listener.Addr().String())
	})
	proxyURL, closeProxy, err := startControlProxy(t.Context(), "http://control.test:8080", socksAddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeProxy)
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	response, err := client.Get("http://control.test:8080/key")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || string(body) != "control.test:8080 /key" {
		t.Fatalf("HTTP forwarding: status=%d body=%q error=%v", response.StatusCode, body, err)
	}
}

func TestControlProxyRejectsUnrelatedDestinations(t *testing.T) {
	var calls atomic.Int32
	socksAddr := startTestSOCKS(t, func(context.Context, string, string) (net.Conn, error) {
		calls.Add(1)
		return nil, fmt.Errorf("must not dial a denied destination")
	})
	proxyURL, closeProxy, err := startControlProxy(t.Context(), "https://control.test", socksAddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeProxy)
	for _, requestLine := range []string{
		"CONNECT unrelated.test:443 HTTP/1.1",
		"CONNECT control.test.evil:443 HTTP/1.1",
		"GET http://unrelated.test/key HTTP/1.1",
	} {
		t.Run(requestLine, func(t *testing.T) {
			conn, response := sendProxyRequest(t, proxyURL.Host, requestLine)
			defer conn.Close()
			defer response.Body.Close()
			if response.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", response.StatusCode)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("denied destination reached the SOCKS dialer")
	}
}

func TestControlProxyUpstreamFailure(t *testing.T) {
	socksAddr := startTestSOCKS(t, func(context.Context, string, string) (net.Conn, error) {
		return nil, fmt.Errorf("synthetic upstream failure")
	})
	proxyURL, closeProxy, err := startControlProxy(t.Context(), "https://control.test", socksAddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeProxy)
	conn, response := sendProxyRequest(t, proxyURL.Host, "CONNECT control.test:443 HTTP/1.1")
	defer conn.Close()
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want explicit upstream failure 502", response.StatusCode)
	}
}

func TestControlProxyCancel(t *testing.T) {
	testControlProxyShutdown(t, true)
}

func TestControlProxyCleanupActiveTunnel(t *testing.T) {
	testControlProxyShutdown(t, false)
}

func TestControlProxyCleanupCanceledStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	proxyURL, closeProxy, err := startControlProxy(ctx, "https://control.test", "127.0.0.1:1")
	if closeProxy != nil {
		t.Cleanup(closeProxy)
	}
	if err == nil || !errors.Is(err, context.Canceled) || proxyURL != nil {
		t.Fatalf("canceled startup: URL=%v error=%v", proxyURL, err)
	}
}

func testControlProxyShutdown(t *testing.T, cancelParent bool) {
	t.Helper()
	echoAddr := startTestEcho(t)
	socksAddr := startTestSOCKS(t, func(ctx context.Context, network, _ string) (net.Conn, error) {
		return new(net.Dialer).DialContext(ctx, network, echoAddr)
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	proxyURL, closeProxy, err := startControlProxy(ctx, "https://control.test", socksAddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeProxy)
	conn, response := sendProxyRequest(t, proxyURL.Host, "CONNECT control.test:443 HTTP/1.1")
	defer conn.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d", response.StatusCode)
	}
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 4)
	if _, err := io.ReadFull(conn, data); err != nil || string(data) != "ping" {
		t.Fatalf("relay echo = %q, error=%v", data, err)
	}
	if cancelParent {
		cancel()
	} else {
		closeProxy()
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Read(data); err == nil {
		t.Fatal("closed proxy still transferred bytes")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatalf("proxy cleanup left the active tunnel open: %v", err)
	}
}

func TestControlProxyBufferedCONNECT(t *testing.T) {
	echoAddr := startTestEcho(t)
	socksAddr := startTestSOCKS(t, func(ctx context.Context, network, _ string) (net.Conn, error) {
		return new(net.Dialer).DialContext(ctx, network, echoAddr)
	})
	proxyURL, closeProxy, err := startControlProxy(t.Context(), "https://control.test", socksAddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeProxy)
	conn, err := net.DialTimeout("tcp", proxyURL.Host, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	payload := strings.Repeat("prefetched-tunnel-bytes", 256)
	if _, err := fmt.Fprintf(conn, "CONNECT control.test:443 HTTP/1.1\r\nHost: control.test:443\r\n\r\n%s", payload); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT response = %v, error=%v", response, err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(reader, got); err != nil || string(got) != payload {
		t.Fatalf("pipelined CONNECT bytes were lost: error=%v", err)
	}
}

func startTestEcho(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		stop := context.AfterFunc(t.Context(), func() { _ = conn.Close() })
		defer stop()
		_, _ = io.Copy(conn, conn)
	}()
	t.Cleanup(func() { _ = listener.Close(); <-done })
	return listener.Addr().String()
}

func sendProxyRequest(t *testing.T, proxyAddr, requestLine string) (net.Conn, *http.Response) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", proxyAddr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(conn, "%s\r\nHost: control.test:443\r\n\r\n", requestLine); err != nil {
		t.Fatal(err)
	}
	method, _, _ := strings.Cut(requestLine, " ")
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: method})
	if err != nil {
		t.Fatal(err)
	}
	return conn, response
}
