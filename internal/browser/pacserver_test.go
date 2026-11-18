package browser

import (
	"io"
	"net"
	"testing"
	"time"
)

// A request that announces a body and never sends it holds an active connection
// open, which also blocks Shutdown. The server must drop it after its read timeout.
func TestPACServerDropsARequestWhoseBodyNeverArrives(t *testing.T) {
	restore := pacReadTimeout
	pacReadTimeout = 200 * time.Millisecond
	t.Cleanup(func() { pacReadTimeout = restore })

	server, url, err := StartPACServer("127.0.0.1:0", "function FindProxyForURL(){return \"DIRECT\"}")
	if err != nil {
		t.Fatalf("StartPACServer: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })

	conn, err := net.Dial("tcp", url[len("http://"):len(url)-len("/proxy.pac")])
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("POST /proxy.pac HTTP/1.1\r\nHost: x\r\nContent-Length: 100\r\n\r\n"))
	// Whatever the server answers, it must close the connection within the bound.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, err = io.Copy(io.Discard, conn)
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("connection still open after 3s; the server has no read timeout")
	}
}
