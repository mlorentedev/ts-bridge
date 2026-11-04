package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
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
// totals stay exact once the tunnel is torn down (#415). It returns the
// expected totals, tx (target to client) and rx (client to target).
func exchangeLive(t *testing.T, client, remotePeer net.Conn, done <-chan struct{}) (tx, rx int64) {
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
	return int64(len(reply)), int64(len(request))
}

func TestHandleConn_PublishesBytesWhileOpen(t *testing.T) {
	telemetry.ResetMetrics()
	client, server := net.Pipe()
	remote, remotePeer := net.Pipe()
	dialer := &mockDialer{dialFunc: func(context.Context, string, string) (net.Conn, error) {
		return remote, nil
	}}
	cfg := config.Config{Target: "100.64.0.1:3389", ConnectTimeout: time.Second, DialTimeout: time.Second}

	var logs bytes.Buffer
	done := make(chan struct{})
	go func() {
		handleConn(server, dialer, cfg, nil, slog.New(slog.NewTextHandler(&logs, nil)))
		close(done)
	}()

	tx, rx := exchangeLive(t, client, remotePeer, done)
	// The per-connection totals still reach the close log, read after the
	// handler returned.
	for _, want := range []string{fmt.Sprintf("bytes_tx=%d", tx), fmt.Sprintf("bytes_rx=%d", rx)} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("close log lacks %s: %s", want, logs.String())
		}
	}
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
	_, _ = exchangeLive(t, client, remotePeer, done)
}

// shortWriter accepts only the first n bytes of each write and reports err.
type shortWriter struct {
	n   int
	err error
}

func (w shortWriter) Write([]byte) (int, error) { return w.n, w.err }

// A write that moves some bytes and then fails still moved them: the counter
// must reflect the bytes the peer received, not only fully successful writes.
func TestMeteredWriter_PublishesPartialWrite(t *testing.T) {
	var published int64
	w := meteredWriter{w: shortWriter{n: 3, err: io.ErrShortWrite}, add: func(n int64) { published += n }}
	n, err := w.Write([]byte("abcdef"))
	if n != 3 || !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("Write() = %d, %v, want 3, io.ErrShortWrite", n, err)
	}
	if published != 3 {
		t.Fatalf("published = %d, want 3", published)
	}
}
