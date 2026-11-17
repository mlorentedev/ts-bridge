package health

import (
	"io"
	"log/slog"
	"net"
	"testing"
	"time"
)

// With --health-allow-remote anyone on the network can open a request that never
// finishes. The server must drop it after its read timeout instead of holding it.
func TestHealthServerDropsARequestWhoseBodyNeverArrives(t *testing.T) {
	restore := serverReadTimeout
	serverReadTimeout = 200 * time.Millisecond
	t.Cleanup(func() { serverReadTimeout = restore })

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	var status TunnelStatus
	server := StartServer(addr, &status, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = server.Close() })

	var conn net.Conn
	for i := 0; i < 50; i++ {
		if conn, err = net.Dial("tcp", addr); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("POST /health/live HTTP/1.1\r\nHost: x\r\nContent-Length: 100\r\n\r\n"))
	// Whatever the server answers, it must close the connection within the bound.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, err = io.Copy(io.Discard, conn)
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("connection still open after 3s; the server has no read timeout")
	}
}
