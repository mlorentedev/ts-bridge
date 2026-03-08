package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"ts-bridge/internal/config"
	"ts-bridge/internal/telemetry"
)

const (
	// bufferSize is the size of the copy buffer. 32KB chosen as sweet spot
	// for RDP traffic: large enough for efficiency, small enough to avoid
	// memory pressure with many concurrent connections.
	bufferSize = 32 * 1024

	// keepAliveInterval for TCP connections. 3 minutes is standard for
	// most NAT/firewall idle timeouts.
	keepAliveInterval = 3 * time.Minute

	// backoffMin/Max for accept loop error recovery.
	backoffMin = 100 * time.Millisecond
	backoffMax = 10 * time.Second
)

// Dialer abstracts the remote connection mechanism.
// tsnet.Server satisfies this interface without an adapter.
type Dialer interface {
	Dial(ctx context.Context, network, addr string) (net.Conn, error)
}

// AcceptLoop accepts incoming connections and routes them to the dialer.
func AcceptLoop(listener net.Listener, dialer Dialer, cfg config.Config, wg *sync.WaitGroup, logger *slog.Logger) error {
	backoff := backoffMin

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}

			logger.Warn("accept error", "error", err, "backoff", backoff)
			time.Sleep(backoff)
			backoff = min(backoff*2, backoffMax)
			continue
		}

		// Reset backoff on successful accept
		backoff = backoffMin

		// Check connection limit
		current := telemetry.GetActiveConnections()
		if current >= cfg.MaxConnections {
			telemetry.AddRejectedConn()
			logger.Warn("connection rejected: limit reached",
				"current", current,
				"max", cfg.MaxConnections,
				"client", conn.RemoteAddr())
			_ = conn.Close()
			continue
		}

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			handleConn(c, dialer, cfg, logger)
		}(conn)
	}
}

// Buffer pool to reduce GC pressure.
var bufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, bufferSize)
		return &b
	},
}

func handleConn(client net.Conn, dialer Dialer, cfg config.Config, logger *slog.Logger) {
	// Track metrics
	telemetry.AddActiveConnection(1)
	telemetry.AddTotalConnection()
	defer telemetry.AddActiveConnection(-1)

	addr := client.RemoteAddr().String()
	connStart := time.Now()

	if tcpConn, ok := client.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(true); err != nil {
			logger.Debug("failed to set keepalive", "error", err)
		}
		if err := tcpConn.SetKeepAlivePeriod(keepAliveInterval); err != nil {
			logger.Debug("failed to set keepalive period", "error", err)
		}
	}

	logger.Info("connection opened", "client", addr)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	remote, err := dialer.Dial(ctx, "tcp", cfg.Target)
	if err != nil {
		telemetry.AddError()
		logger.Error("dial failed", "client", addr, "target", cfg.Target, "error", err)
		_ = client.Close()
		return
	}

	logger.Debug("tunnel established", "client", addr, "target", cfg.Target)

	bytesTx, bytesRx := proxyConnections(client, remote, addr, logger)

	telemetry.AddBytesTx(bytesTx)
	telemetry.AddBytesRx(bytesRx)

	duration := time.Since(connStart)
	logger.Info("connection closed",
		"client", addr,
		"duration", duration,
		"bytes_tx", bytesTx,
		"bytes_rx", bytesRx)
}

// proxyConnections performs bidirectional copy between client and remote,
// returning the bytes transferred in each direction.
func proxyConnections(client, remote net.Conn, addr string, logger *slog.Logger) (tx, rx int64) {
	var once sync.Once
	closeAll := func() {
		once.Do(func() {
			_ = client.Close()
			_ = remote.Close()
		})
	}

	copyConn := func(dst, src net.Conn, direction string, counter *int64) {
		defer closeAll()

		bufPtr := bufferPool.Get().(*[]byte)
		defer bufferPool.Put(bufPtr)

		n, err := io.CopyBuffer(dst, src, *bufPtr)
		*counter = n

		if err != nil && !IsExpectedCloseError(err) {
			telemetry.AddError()
			logger.Warn("copy error",
				"client", addr,
				"direction", direction,
				"bytes", n,
				"error", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		copyConn(client, remote, "rx", &rx)
	}()
	copyConn(remote, client, "tx", &tx)
	wg.Wait()

	return tx, rx
}

// IsExpectedCloseError returns true for errors that occur during normal connection close.
func IsExpectedCloseError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	// Check for common syscall errors during close
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	if errors.Is(err, syscall.ENOTCONN) {
		return true
	}
	// Fallback for error messages (cross-platform compatibility)
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "use of closed network connection") {
		return true
	}
	if strings.Contains(errStr, "connection reset by peer") {
		return true
	}
	if strings.Contains(errStr, "forcibly closed by the remote host") {
		return true
	}
	if strings.Contains(errStr, "closed pipe") {
		return true
	}
	return false
}
