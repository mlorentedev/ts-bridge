package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"regexp"
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

// logFieldHasValue reports whether a text-handler record carries key=value as a
// whole field. A substring match would accept bytes_tx=160 for a want of 16.
func logFieldHasValue(logs, key string, value int64) bool {
	field := regexp.MustCompile(`(?m)(^|\s)` + regexp.QuoteMeta(fmt.Sprintf("%s=%d", key, value)) + `(\s|$)`)
	return field.MatchString(logs)
}

func assertLogField(t *testing.T, logs, key string, value int64) {
	t.Helper()
	if !logFieldHasValue(logs, key, value) {
		t.Errorf("close log lacks %s=%d: %s", key, value, logs)
	}
}

func TestLogFieldHasValue_RejectsPrefixMatches(t *testing.T) {
	const logs = `level=INFO msg="connection closed" bytes_tx=160 bytes_rx=420`
	tests := []struct {
		name  string
		key   string
		value int64
		want  bool
	}{
		{"exact tx", "bytes_tx", 160, true},
		{"exact rx at end of record", "bytes_rx", 420, true},
		{"tx prefix of a longer count", "bytes_tx", 16, false},
		{"rx prefix of a longer count", "bytes_rx", 42, false},
		{"key suffix of another key", "tx", 160, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := logFieldHasValue(logs, tt.key, tt.value); got != tt.want {
				t.Errorf("logFieldHasValue(%s=%d) = %v, want %v", tt.key, tt.value, got, tt.want)
			}
		})
	}
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
	assertLogField(t, logs.String(), "bytes_tx", tx)
	assertLogField(t, logs.String(), "bytes_rx", rx)
}

func TestHandleSOCKS5Conn_PublishesBytesWhileOpen(t *testing.T) {
	telemetry.ResetMetrics()
	client, server := net.Pipe()
	remote, remotePeer := net.Pipe()
	dialer := &mockDialer{dialFunc: func(context.Context, string, string) (net.Conn, error) {
		return remote, nil
	}}

	var logs bytes.Buffer
	done := make(chan struct{})
	go func() {
		_ = handleSOCKS5Conn(server, dialer, directSOCKSTarget,
			config.Config{DialTimeout: time.Second, IdleTimeout: 10 * time.Second},
			nil, slog.New(slog.NewTextHandler(&logs, nil)))
		close(done)
	}()

	writeAll(t, client, []byte{0x05, 0x01, 0x00})
	assertBytes(t, client, []byte{0x05, 0x00})
	writeAll(t, client, domainSOCKSRequest(0x01, "forge.example.internal", 443))
	assertBytes(t, client, []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	// The handshake is not tunnel traffic: the counters start at the CONNECT reply.
	tx, rx := exchangeLive(t, client, remotePeer, done)
	// Same close record as the TCP tunnel, with the SOCKS5 target on it (#630).
	for _, want := range []string{"connection closed", "target=forge.example.internal:443", "duration="} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("close log lacks %s: %s", want, logs.String())
		}
	}
	assertLogField(t, logs.String(), "bytes_tx", tx)
	assertLogField(t, logs.String(), "bytes_rx", rx)
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

// A tunnel's traffic spans many buffers: every write is published and the
// counts add up, the way copyDirection drives meteredWriter.
func TestMeteredWriter_AccumulatesAcrossBuffers(t *testing.T) {
	var published int64
	var calls int
	w := meteredWriter{w: io.Discard, add: func(n int64) { published += n; calls++ }}
	payload := bytes.Repeat([]byte("x"), 2*bufferSize+1)
	// Hide bytes.Reader's WriteTo so CopyBuffer goes through the buffer.
	src := struct{ io.Reader }{bytes.NewReader(payload)}
	n, err := io.CopyBuffer(w, src, make([]byte, bufferSize))
	if err != nil || n != int64(len(payload)) {
		t.Fatalf("CopyBuffer() = %d, %v, want %d, nil", n, err, len(payload))
	}
	if published != n || calls != 3 {
		t.Fatalf("published %d bytes in %d writes, want %d in 3", published, calls, n)
	}
}
