package proxy

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"ts-bridge/internal/config"
	"ts-bridge/internal/telemetry"
)

// waitForBytes polls the global byte counters until they reach the wanted
// totals. The copy loop publishes a write only after dst.Write returns, which
// can trail the peer's read by a scheduling slice, so an immediate read races.
func waitForBytes(t *testing.T, wantTx, wantRx int64, phase string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		m := telemetry.GetMetrics()
		if m.TotalBytesTx == wantTx && m.TotalBytesRx == wantRx {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: byte metrics = tx %d rx %d, want tx %d rx %d",
				phase, m.TotalBytesTx, m.TotalBytesRx, wantTx, wantRx)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// exchangeLive sends one payload each way through an open tunnel and checks
// that the global counters advance before either side closes, then that the
// totals stay exact once the tunnel is torn down (#415).
func exchangeLive(t *testing.T, client, remotePeer net.Conn, done <-chan struct{}) {
	t.Helper()
	request := []byte("client to target, while the tunnel is open")
	reply := []byte("target to client")

	writeAll(t, client, request)
	assertBytes(t, remotePeer, request)
	waitForBytes(t, 0, int64(len(request)), "after the request, before close")

	writeAll(t, remotePeer, reply)
	assertBytes(t, client, reply)
	waitForBytes(t, int64(len(reply)), int64(len(request)), "after the reply, before close")

	_ = client.Close()
	_ = remotePeer.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not finish after both sides closed")
	}
	waitForBytes(t, int64(len(reply)), int64(len(request)), "after close")
}

func TestHandleConn_PublishesBytesWhileOpen(t *testing.T) {
	telemetry.ResetMetrics()
	client, server := net.Pipe()
	remote, remotePeer := net.Pipe()
	dialer := &mockDialer{dialFunc: func(context.Context, string, string) (net.Conn, error) {
		return remote, nil
	}}
	cfg := config.Config{Target: "100.64.0.1:3389", ConnectTimeout: time.Second, DialTimeout: time.Second}

	done := make(chan struct{})
	go func() {
		handleConn(server, dialer, cfg, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()

	exchangeLive(t, client, remotePeer, done)
}

func TestHandleSOCKS5Conn_PublishesBytesWhileOpen(t *testing.T) {
	telemetry.ResetMetrics()
	client, server := net.Pipe()
	remote, remotePeer := net.Pipe()
	dialer := &mockDialer{dialFunc: func(context.Context, string, string) (net.Conn, error) {
		return remote, nil
	}}

	done := make(chan struct{})
	go func() {
		_ = handleSOCKS5Conn(server, dialer, directSOCKSTarget,
			config.Config{DialTimeout: time.Second, IdleTimeout: 10 * time.Second},
			nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()

	writeAll(t, client, []byte{0x05, 0x01, 0x00})
	assertBytes(t, client, []byte{0x05, 0x00})
	writeAll(t, client, domainSOCKSRequest(0x01, "forge.example.internal", 443))
	assertBytes(t, client, []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	// The handshake is not tunnel traffic: the counters start at the CONNECT reply.
	exchangeLive(t, client, remotePeer, done)
}
