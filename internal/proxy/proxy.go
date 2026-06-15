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

// idleConn wraps a net.Conn and enforces a per-Read deadline. Each call to
// Read resets the read deadline to now + idle. If no bytes arrive within
// idle, the underlying Read returns a timeout error which propagates out of
// io.CopyBuffer, letting the proxy classify and close the bridge cleanly.
//
// Per-direction semantics: an idle conn on the rx side does not signal idle
// on the tx side. Either direction reaching its deadline closes the pair.
type idleConn struct {
	net.Conn
	idle time.Duration
}

func (c *idleConn) Read(b []byte) (int, error) {
	if c.idle > 0 {
		_ = c.Conn.SetReadDeadline(time.Now().Add(c.idle))
	}
	return c.Conn.Read(b)
}

// withIdleTimeout returns c wrapped with a Read deadline of idle on every
// read. When idle is 0 or negative, returns c unchanged so callers in the
// disabled path pay no overhead.
func withIdleTimeout(c net.Conn, idle time.Duration) net.Conn {
	if idle <= 0 {
		return c
	}
	return &idleConn{Conn: c, idle: idle}
}

// isIdleTimeoutErr reports whether err is a deadline-exceeded error from a
// net.Conn Read. Used to classify idle closures distinctly from transport
// errors so they are logged at info level and not counted in error metrics.
func isIdleTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
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

		// Atomically claim a connection slot. The CAS loop in TryClaimConnection
		// closes the check-then-act race where a burst of accepts could each
		// observe (cur < cap) and all proceed past the limit.
		if !telemetry.TryClaimConnection(cfg.MaxConnections) {
			telemetry.AddRejectedConn()
			logger.Warn("connection rejected: limit reached",
				"max", cfg.MaxConnections,
				"client", conn.RemoteAddr())
			_ = conn.Close()
			continue
		}

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			// Release the claimed slot once the per-conn work returns,
			// regardless of how it terminates (success, dial fail, panic-free abort).
			defer telemetry.AddActiveConnection(-1)
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
	// Active-connection slot management lives in AcceptLoop (atomic claim +
	// release via defer in the spawned goroutine). We only count the total
	// here so direct callers (unit tests that bypass AcceptLoop) still see
	// the per-call increment.
	telemetry.AddTotalConnection()

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

	// DialTimeout is intentionally smaller than ConnectTimeout: ConnectTimeout
	// covers the one-time tsnet init (control plane handshake, can be slow),
	// DialTimeout covers each per-connection target dial. With ReconnectDialer
	// retrying, a large ConnectTimeout would let one stuck client hog a slot
	// for many minutes; DialTimeout=5s keeps the worst case bounded.
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	remote, err := dialer.Dial(ctx, "tcp", cfg.Target)
	if err != nil {
		telemetry.AddError()
		logger.Error("dial failed", "client", addr, "target", cfg.Target, "error", err)
		_ = client.Close()
		return
	}

	logger.Debug("tunnel established", "client", addr, "target", cfg.Target)

	client = withIdleTimeout(client, cfg.IdleTimeout)
	remote = withIdleTimeout(remote, cfg.IdleTimeout)

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

// halfCloser is satisfied by net.Conn implementations that support a
// uni-directional write shutdown. *net.TCPConn and tsnet/gonet conns
// both implement this. net.Pipe does not.
type halfCloser interface {
	CloseWrite() error
}

// halfCloseWrite tries to half-close the write side of dst so the peer
// sees EOF on its read while the opposite-direction copy keeps draining.
// Unwraps idleConn to reach the underlying conn before the type assert,
// since idleConn embeds net.Conn and does not promote CloseWrite.
// Returns true if a half-close was actually performed.
func halfCloseWrite(c net.Conn) bool {
	if ic, ok := c.(*idleConn); ok {
		c = ic.Conn
	}
	if cw, ok := c.(halfCloser); ok {
		_ = cw.CloseWrite()
		return true
	}
	return false
}

// proxyConnections performs bidirectional copy between client and remote,
// returning the bytes transferred in each direction. Each direction runs
// in its own goroutine; when one direction's source EOFs, the destination's
// write half is closed so the peer sees end-of-stream without tearing down
// the opposite (still-active) direction. Both ends are fully closed only
// after both directions complete.
func proxyConnections(client, remote net.Conn, addr string, logger *slog.Logger) (tx, rx int64) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		copyDirection(remote, client, "rx", &rx, addr, logger)
	}()
	go func() {
		defer wg.Done()
		copyDirection(client, remote, "tx", &tx, addr, logger)
	}()
	wg.Wait()

	// Idempotent full close after both directions completed — guarantees
	// the conns are released even if a half-close path ran.
	_ = client.Close()
	_ = remote.Close()

	return tx, rx
}

// copyDirection copies data from src to dst in one direction, managing the
// buffer pool, error classification, and connection cleanup.
func copyDirection(dst, src net.Conn, direction string, counter *int64, addr string, logger *slog.Logger) {
	bufPtrRaw := bufferPool.Get()
	bufPtr, ok := bufPtrRaw.(*[]byte)
	if !ok {
		// Should never happen — bufferPool.New always returns *[]byte.
		// Fallback: allocate a fresh buffer to avoid panicking.
		b := make([]byte, bufferSize)
		bufPtr = &b
	}
	defer bufferPool.Put(bufPtr)

	n, err := io.CopyBuffer(dst, src, *bufPtr)
	*counter = n

	if err == nil || IsExpectedCloseError(err) {
		finishGraceful(dst)
	} else if isIdleTimeoutErr(err) {
		closeWithIdleLog(dst, src, addr, direction, n, logger)
	} else {
		closeWithError(dst, src, addr, direction, n, err, logger)
	}
}

// finishGraceful half-closes dst (or full-closes if half-close unsupported).
func finishGraceful(dst net.Conn) {
	if !halfCloseWrite(dst) {
		_ = dst.Close()
	}
}

// closeWithIdleLog closes both ends after an idle timeout, logging at info level.
func closeWithIdleLog(dst, src net.Conn, addr, direction string, n int64, logger *slog.Logger) {
	logger.Info("connection closed (idle timeout)",
		"client", addr,
		"direction", direction,
		"bytes", n)
	_ = dst.Close()
	_ = src.Close()
}

// closeWithError closes both ends after a copy error, logging at warn level
// and incrementing the error counter.
func closeWithError(dst, src net.Conn, addr, direction string, n int64, err error, logger *slog.Logger) {
	telemetry.AddError()
	logger.Warn("copy error",
		"client", addr,
		"direction", direction,
		"bytes", n,
		"error", err)
	_ = dst.Close()
	_ = src.Close()
}

// IsExpectedCloseError returns true for errors that occur during normal connection close.
func IsExpectedCloseError(err error) bool {
	if err == nil {
		return true
	}

	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return true
	}

	// Check for common syscall errors during close
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ENOTCONN) {
		return true
	}

	// Fallback for error messages (cross-platform compatibility)
	errStr := strings.ToLower(err.Error())
	expectedMsgs := []string{
		"use of closed network connection",
		"connection reset by peer",
		"forcibly closed by the remote host",
		"closed pipe",
	}

	for _, msg := range expectedMsgs {
		if strings.Contains(errStr, msg) {
			return true
		}
	}

	return false
}
