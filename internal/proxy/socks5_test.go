package proxy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"ts-bridge/internal/config"
	"ts-bridge/internal/telemetry"
)

func TestHandleSOCKS5Conn_DialsRequestedDomainAndProxiesBytes(t *testing.T) {
	telemetry.ResetMetrics()
	client, server := net.Pipe()
	remote, remotePeer := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = remotePeer.Close()
	})

	dialed := make(chan string, 1)
	dialer := &mockDialer{
		dialFunc: func(_ context.Context, network, addr string) (net.Conn, error) {
			if network != "tcp" {
				t.Errorf("network = %q, want tcp", network)
			}
			dialed <- addr
			return remote, nil
		},
	}

	handlerDone := make(chan error, 1)
	go func() {
		handlerDone <- handleSOCKS5Conn(
			server,
			dialer,
			directSOCKSTarget,
			config.Config{DialTimeout: time.Second, IdleTimeout: time.Second},
			nil,
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
	}()

	writeAll(t, client, []byte{0x05, 0x01, 0x00})
	assertBytes(t, client, []byte{0x05, 0x00})

	host := "forge.example.internal"
	request := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	request = append(request, []byte(host)...)
	request = append(request, 0x01, 0xbb)
	writeAll(t, client, request)
	assertBytes(t, client, []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	select {
	case got := <-dialed:
		if got != host+":443" {
			t.Fatalf("dialed target = %q, want %q", got, host+":443")
		}
	case <-time.After(time.Second):
		t.Fatal("dialer was not called")
	}

	payload := []byte("GET / HTTP/1.1\r\nHost: forge.example.internal\r\n\r\n")
	writeAll(t, client, payload)
	assertBytes(t, remotePeer, payload)

	reply := []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")
	writeAll(t, remotePeer, reply)
	assertBytes(t, client, reply)

	_ = client.Close()
	_ = remotePeer.Close()
	select {
	case err := <-handlerDone:
		if err != nil {
			t.Fatalf("handleSOCKS5Conn returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handleSOCKS5Conn did not finish")
	}
	metrics := telemetry.GetMetrics()
	if metrics.TotalBytesTx == 0 || metrics.TotalBytesRx == 0 {
		t.Fatalf("SOCKS5 byte telemetry = tx %d rx %d, want both non-zero", metrics.TotalBytesTx, metrics.TotalBytesRx)
	}
}

func TestReadSOCKS5Connect_ParsesIPTargets(t *testing.T) {
	ipv6 := net.ParseIP("fd7a:115c:a1e0::25").To16()
	tests := []struct {
		name     string
		request  []byte
		wantHost string
		wantPort uint16
	}{
		{
			name:     "IPv4",
			request:  []byte{0x05, 0x01, 0x00, 0x01, 100, 64, 0, 25, 0x00, 0x16},
			wantHost: "100.64.0.25",
			wantPort: 22,
		},
		{
			name:     "IPv6",
			request:  append([]byte{0x05, 0x01, 0x00, 0x04}, append(ipv6, 0x01, 0xbb)...),
			wantHost: "fd7a:115c:a1e0::25",
			wantPort: 443,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, reader := net.Pipe()
			t.Cleanup(func() { _ = writer.Close() })
			go func() {
				_, _ = writer.Write(tt.request)
			}()

			host, port, err := readSOCKS5Connect(reader)
			_ = reader.Close()
			if err != nil {
				t.Fatalf("readSOCKS5Connect returned error: %v", err)
			}
			if host != tt.wantHost || port != tt.wantPort {
				t.Fatalf("target = %s:%d, want %s:%d", host, port, tt.wantHost, tt.wantPort)
			}
		})
	}
}

func TestAcceptSOCKS5Loop_AcceptsAndForwardsConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	remote, remotePeer := net.Pipe()
	t.Cleanup(func() { _ = remotePeer.Close() })

	dialer := &mockDialer{
		dialFunc: func(_ context.Context, _, _ string) (net.Conn, error) {
			return remote, nil
		},
	}
	cfg := config.Config{MaxConnections: 1}
	var wg sync.WaitGroup
	loopDone := make(chan error, 1)
	go func() {
		loopDone <- AcceptSOCKS5Loop(
			listener,
			dialer,
			cfg,
			directSOCKSTarget,
			&wg,
			nil,
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
	}()

	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	writeAll(t, client, []byte{0x05, 0x01, 0x00})
	assertBytes(t, client, []byte{0x05, 0x00})

	host := "mesh-host"
	request := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	request = append(request, []byte(host)...)
	request = append(request, 0x00, 0x16)
	writeAll(t, client, request)
	assertBytes(t, client, []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	payload := []byte("ssh")
	writeAll(t, client, payload)
	assertBytes(t, remotePeer, payload)

	_ = client.Close()
	_ = remotePeer.Close()
	_ = listener.Close()
	handlersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(handlersDone)
	}()
	select {
	case <-handlersDone:
	case <-time.After(time.Second):
		t.Fatal("SOCKS5 connection handler did not stop")
	}
	select {
	case err := <-loopDone:
		if err != nil {
			t.Fatalf("AcceptSOCKS5Loop returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("AcceptSOCKS5Loop did not stop")
	}
}

func TestNewAllowlistSOCKSTarget(t *testing.T) {
	resolve, err := NewAllowlistSOCKSTarget(map[string]string{
		"Forge.Example.Internal:443": "apps:443",
	})
	if err != nil {
		t.Fatalf("NewAllowlistSOCKSTarget returned error: %v", err)
	}

	got, err := resolve("forge.example.internal", 443)
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if got != "apps:443" {
		t.Fatalf("resolved target = %q, want apps:443", got)
	}

	_, err = resolve("unlisted.example.internal", 443)
	if err == nil || !errors.Is(err, ErrSOCKSTargetDenied) {
		t.Fatalf("unlisted resolve error = %v, want ErrSOCKSTargetDenied", err)
	}
}

func TestNewAllowlistSOCKSTargetRejectsInvalidRoutes(t *testing.T) {
	tests := []struct {
		name   string
		routes map[string]string
	}{
		{name: "source without port", routes: map[string]string{"forge.example.internal": "apps:443"}},
		{name: "destination without port", routes: map[string]string{"forge.example.internal:443": "apps"}},
		{name: "empty source", routes: map[string]string{"": "apps:443"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewAllowlistSOCKSTarget(tt.routes); err == nil {
				t.Fatal("expected invalid route error")
			}
		})
	}
}

func TestSOCKS5AllowlistPreservesTLSClientHelloSNI(t *testing.T) {
	certificate := testTLSCertificate(t)
	client, server := net.Pipe()
	remote, remotePeer := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = remotePeer.Close()
	})

	dialer := &mockDialer{
		dialFunc: func(_ context.Context, _, addr string) (net.Conn, error) {
			if addr != "apps:443" {
				t.Fatalf("dial target = %q, want apps:443", addr)
			}
			return remote, nil
		},
	}
	resolve, err := NewAllowlistSOCKSTarget(map[string]string{
		"forge.example.internal:443": "apps:443",
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = handleSOCKS5Conn(
			server,
			dialer,
			resolve,
			config.Config{DialTimeout: time.Second, IdleTimeout: time.Second},
			nil,
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
	}()

	writeAll(t, client, []byte{0x05, 0x01, 0x00})
	assertBytes(t, client, []byte{0x05, 0x00})
	host := "forge.example.internal"
	request := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	request = append(request, []byte(host)...)
	request = append(request, 0x01, 0xbb)
	writeAll(t, client, request)
	assertBytes(t, client, []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	observedSNI := make(chan string, 1)
	tlsServer := tls.Server(remotePeer, &tls.Config{
		Certificates: []tls.Certificate{certificate},
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			observedSNI <- hello.ServerName
			return nil, nil
		},
	})
	serverDone := make(chan error, 1)
	go func() { serverDone <- tlsServer.Handshake() }()

	tlsClient := tls.Client(client, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true, // #nosec G402 -- local test certificate.
	})
	if err := tlsClient.Handshake(); err != nil {
		t.Fatalf("TLS client handshake failed: %v", err)
	}
	if err := <-serverDone; err != nil {
		t.Fatalf("TLS server handshake failed: %v", err)
	}
	select {
	case got := <-observedSNI:
		if got != host {
			t.Fatalf("observed SNI = %q, want %q", got, host)
		}
	case <-time.After(time.Second):
		t.Fatal("TLS server did not observe ClientHello")
	}
}

func TestHandleSOCKS5Conn_ReturnsProtocolDenialReplies(t *testing.T) {
	tests := []struct {
		name    string
		request []byte
		resolve SOCKSTargetResolver
		wantREP byte
	}{
		{
			name:    "allow-list denial",
			request: domainSOCKSRequest(0x01, "denied.example", 443),
			resolve: func(string, uint16) (string, error) {
				return "", ErrSOCKSTargetDenied
			},
			wantREP: 0x02,
		},
		{
			name:    "unsupported command",
			request: []byte{0x05, 0x02, 0x00, 0x03},
			resolve: directSOCKSTarget,
			wantREP: 0x07,
		},
		{
			name:    "unsupported address type",
			request: []byte{0x05, 0x01, 0x00, 0x09},
			resolve: directSOCKSTarget,
			wantREP: 0x08,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			t.Cleanup(func() { _ = client.Close() })
			dialer := &mockDialer{dialFunc: func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("dialer must not run for a denied request")
				return nil, nil
			}}
			done := make(chan error, 1)
			go func() {
				done <- handleSOCKS5Conn(
					server,
					dialer,
					tt.resolve,
					config.Config{DialTimeout: time.Second},
					nil,
					slog.New(slog.NewTextHandler(io.Discard, nil)),
				)
			}()

			writeAll(t, client, []byte{0x05, 0x01, 0x00})
			assertBytes(t, client, []byte{0x05, 0x00})
			writeAll(t, client, tt.request)
			reply := make([]byte, 10)
			_ = client.SetReadDeadline(time.Now().Add(time.Second))
			if _, err := io.ReadFull(client, reply); err != nil {
				t.Fatalf("read denial reply: %v", err)
			}
			if reply[1] != tt.wantREP {
				t.Fatalf("SOCKS REP = %#x, want %#x", reply[1], tt.wantREP)
			}
			if err := <-done; err == nil {
				t.Fatal("handler should return the denial error")
			}
		})
	}
}

func TestHandleSOCKS5Conn_AppliesDialTimeoutAndTerminalCancel(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })

	cancelled := make(chan error, 1)
	cancel := func(err error) { cancelled <- err }
	dialer := &mockDialer{
		dialFunc: func(ctx context.Context, _, _ string) (net.Conn, error) {
			<-ctx.Done()
			return nil, &TerminalDialError{Cause: ctx.Err()}
		},
	}
	done := make(chan error, 1)
	go func() {
		done <- handleSOCKS5Conn(
			server,
			dialer,
			directSOCKSTarget,
			config.Config{DialTimeout: 20 * time.Millisecond},
			cancel,
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
	}()

	writeAll(t, client, []byte{0x05, 0x01, 0x00})
	assertBytes(t, client, []byte{0x05, 0x00})
	writeAll(t, client, domainSOCKSRequest(0x01, "mesh-host", 22))
	reply := make([]byte, 10)
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := io.ReadFull(client, reply); err != nil {
		t.Fatalf("read dial failure reply: %v", err)
	}
	if reply[1] != 0x05 {
		t.Fatalf("SOCKS REP = %#x, want connection-refused %#x", reply[1], byte(0x05))
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("terminal dial failure did not cancel the bridge")
	}
	if err := <-done; err == nil {
		t.Fatal("handler should return dial failure")
	}
}

func domainSOCKSRequest(command byte, host string, port uint16) []byte {
	request := []byte{0x05, command, 0x00, 0x03, byte(len(host))}
	request = append(request, []byte(host)...)
	return append(request, byte(port>>8), byte(port))
}

func testTLSCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "forge.example.internal"},
		DNSNames:     []string{"forge.example.internal"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  privateKey,
	}
}

func writeAll(t *testing.T, conn net.Conn, data []byte) {
	t.Helper()
	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}

func assertBytes(t *testing.T, conn net.Conn, want []byte) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	got := make([]byte, len(want))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("bytes = %v, want %v", got, want)
	}
}
