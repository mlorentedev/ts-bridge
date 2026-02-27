// Package main implements a TCP bridge over Tailscale's mesh network.
// It creates an ephemeral tsnet node and forwards local connections
// to a remote target through Tailscale's encrypted tunnel.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"tailscale.com/tsnet"
)

// Build-time variables set via ldflags.
var (
	version = "dev"
	commit  = "unknown"
)

// Constants for tuning.
const (
	// bufferSize is the size of the copy buffer. 32KB chosen as sweet spot
	// for RDP traffic: large enough for efficiency, small enough to avoid
	// memory pressure with many concurrent connections.
	bufferSize = 32 * 1024

	// keepAliveInterval for TCP connections. 3 minutes is standard for
	// most NAT/firewall idle timeouts.
	keepAliveInterval = 3 * time.Minute

	// defaultTimeout for tsnet initialization and dial operations.
	defaultTimeout = 30 * time.Second

	// defaultMaxConnections prevents resource exhaustion.
	defaultMaxConnections = 1000

	// backoffMin/Max for accept loop error recovery.
	backoffMin = 100 * time.Millisecond
	backoffMax = 10 * time.Second

	// stateDirPerms ensures state directory is only readable by owner.
	stateDirPerms = 0700
)

// Config holds the bridge configuration.
type Config struct {
	LocalAddr      string
	Target         string
	AuthKey        string // #nosec G117 -- internal struct, never serialized
	Hostname       string
	StateDir       string
	ControlURL     string
	ConnectTimeout time.Duration
	MaxConnections int64
	HealthAddr     string
	Verbose        bool
	LogFormat      string
}

// Metrics tracks operational statistics.
type Metrics struct {
	ActiveConnections int64 `json:"active_connections"`
	TotalConnections  int64 `json:"total_connections"`
	TotalBytesTx      int64 `json:"total_bytes_tx"`
	TotalBytesRx      int64 `json:"total_bytes_rx"`
	TotalErrors       int64 `json:"total_errors"`
	RejectedConns     int64 `json:"rejected_connections"`
}

var metrics Metrics

// Logger is the global structured logger.
var logger *slog.Logger

func main() {
	showVersion := flag.Bool("version", false, "Show version and exit")
	verbose := flag.Bool("v", false, "Enable verbose logging")
	flag.BoolVar(verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ts-bridge %s (%s)\n", version, commit)
		os.Exit(0)
	}

	cfg, err := loadConfig(*verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	initLogger(cfg)
	logger.Info("ts-bridge starting", "version", version, "commit", commit)

	if err := run(cfg); err != nil {
		logger.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func initLogger(cfg Config) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if cfg.Verbose {
		opts.Level = slog.LevelDebug
	}

	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	logger = slog.New(handler)
}

func loadConfig(verboseFlag bool) (Config, error) {
	target, err := parseTarget()
	if err != nil {
		return Config{}, err
	}

	authKey, err := parseAuthKey()
	if err != nil {
		return Config{}, err
	}

	timeout := defaultTimeout
	if t := os.Getenv("TS_TIMEOUT"); t != "" {
		d, err := time.ParseDuration(t)
		if err != nil {
			return Config{}, fmt.Errorf("TS_TIMEOUT invalid: %w", err)
		}
		timeout = d
	}

	maxConns := int64(defaultMaxConnections)
	if m := os.Getenv("TS_MAX_CONNECTIONS"); m != "" {
		n, err := strconv.ParseInt(m, 10, 64)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("TS_MAX_CONNECTIONS invalid: %w", err)
		}
		maxConns = n
	}

	verbose := verboseFlag || os.Getenv("TS_VERBOSE") == "true" || os.Getenv("TS_VERBOSE") == "1"

	return Config{
		LocalAddr:      envOr("TS_LOCAL_ADDR", "127.0.0.1:33389"),
		Target:         target,
		AuthKey:        authKey,
		Hostname:       envOr("TS_HOSTNAME", "ts-bridge"),
		StateDir:       envOr("TS_STATE_DIR", "./ts-state"),
		ControlURL:     os.Getenv("TS_CONTROL_URL"),
		ConnectTimeout: timeout,
		MaxConnections: maxConns,
		HealthAddr:     os.Getenv("TS_HEALTH_ADDR"),
		Verbose:        verbose,
		LogFormat:      envOr("TS_LOG_FORMAT", "text"),
	}, nil
}

func parseTarget() (string, error) {
	target := os.Getenv("TS_TARGET")
	if target == "" {
		return "", errors.New("TS_TARGET is required (format: HOST:PORT)")
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return "", fmt.Errorf("TS_TARGET invalid format: %w", err)
	}
	if host == "" {
		return "", errors.New("TS_TARGET: host cannot be empty")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("TS_TARGET: invalid port %q", portStr)
	}
	return target, nil
}

func parseAuthKey() (string, error) {
	authKey := os.Getenv("TS_AUTHKEY")
	if authKey == "" {
		return "", errors.New("TS_AUTHKEY is required")
	}
	if !strings.HasPrefix(authKey, "tskey-") {
		return "", errors.New("TS_AUTHKEY: invalid format (must start with tskey-)")
	}
	return authKey, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ensureStateDir(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, stateDirPerms); err != nil {
			return fmt.Errorf("create state directory: %w", err)
		}
		logger.Debug("created state directory", "path", dir, "perms", fmt.Sprintf("%o", stateDirPerms))
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat state directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("state path exists but is not a directory: %s", dir)
	}
	// Warn if permissions are too open (only on Unix)
	if info.Mode().Perm()&0077 != 0 {
		logger.Warn("state directory has loose permissions", "path", dir, "perms", fmt.Sprintf("%o", info.Mode().Perm()))
	}
	return nil
}

func run(cfg Config) error {
	if err := ensureStateDir(cfg.StateDir); err != nil {
		return err
	}

	var tsnetLogf func(string, ...any)
	if cfg.Verbose {
		tsnetLogf = func(format string, args ...any) {
			logger.Debug(fmt.Sprintf(format, args...), "component", "tsnet")
		}
	} else {
		tsnetLogf = func(string, ...any) {}
	}

	server := &tsnet.Server{
		Hostname:   cfg.Hostname,
		AuthKey:    cfg.AuthKey,
		Dir:        cfg.StateDir,
		ControlURL: cfg.ControlURL,
		Ephemeral:  true,
		Logf:       tsnetLogf,
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	status, err := server.Up(ctx)
	if err != nil {
		return fmt.Errorf("tailscale init failed: %w", err)
	}
	logger.Info("tailscale ready", "ip", status.Self.TailscaleIPs[0])

	listener, err := net.Listen("tcp", cfg.LocalAddr)
	if err != nil {
		return fmt.Errorf("bind %s: %w", cfg.LocalAddr, err)
	}

	// Start health server if configured
	var ready atomic.Bool
	var healthServer *http.Server
	if cfg.HealthAddr != "" {
		healthServer = startHealthServer(cfg.HealthAddr, &ready)
	}

	printBanner(cfg)

	sigCtx, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	go func() {
		<-sigCtx.Done()
		logger.Info("shutting down")
		ready.Store(false)
		if err := listener.Close(); err != nil {
			logger.Error("error closing listener", "error", err)
		}
		if healthServer != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := healthServer.Shutdown(shutdownCtx); err != nil {
				logger.Error("error closing health server", "error", err)
			}
		}
		if err := server.Close(); err != nil {
			logger.Error("error closing tsnet server", "error", err)
		}
	}()

	ready.Store(true)
	return acceptLoop(listener, server, cfg)
}

func startHealthServer(addr string, ready *atomic.Bool) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_ready"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		snapshot := Metrics{
			ActiveConnections: atomic.LoadInt64(&metrics.ActiveConnections),
			TotalConnections:  atomic.LoadInt64(&metrics.TotalConnections),
			TotalBytesTx:      atomic.LoadInt64(&metrics.TotalBytesTx),
			TotalBytesRx:      atomic.LoadInt64(&metrics.TotalBytesRx),
			TotalErrors:       atomic.LoadInt64(&metrics.TotalErrors),
			RejectedConns:     atomic.LoadInt64(&metrics.RejectedConns),
		}
		_ = json.NewEncoder(w).Encode(snapshot)
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("health server starting", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("health server error", "error", err)
		}
	}()

	return server
}

func printBanner(cfg Config) {
	fmt.Println()
	fmt.Println("  +---------------------------------------+")
	fmt.Printf("  |      TAILSCALE BRIDGE %-14s  |\n", version)
	fmt.Println("  +---------------------------------------+")
	fmt.Printf("  |  Local:  %-26s  |\n", cfg.LocalAddr)
	fmt.Printf("  |  Target: %-26s  |\n", cfg.Target)
	fmt.Println("  +---------------------------------------+")
	fmt.Println("  Waiting for connections...")
	fmt.Println()
}

func acceptLoop(listener net.Listener, server *tsnet.Server, cfg Config) error {
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
		current := atomic.LoadInt64(&metrics.ActiveConnections)
		if current >= cfg.MaxConnections {
			atomic.AddInt64(&metrics.RejectedConns, 1)
			logger.Warn("connection rejected: limit reached",
				"current", current,
				"max", cfg.MaxConnections,
				"client", conn.RemoteAddr())
			_ = conn.Close()
			continue
		}

		go handleConn(conn, server, cfg)
	}
}

// Buffer pool to reduce GC pressure.
var bufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, bufferSize)
		return &b
	},
}

func handleConn(client net.Conn, server *tsnet.Server, cfg Config) {
	// Track metrics
	atomic.AddInt64(&metrics.ActiveConnections, 1)
	atomic.AddInt64(&metrics.TotalConnections, 1)
	defer atomic.AddInt64(&metrics.ActiveConnections, -1)

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

	remote, err := server.Dial(ctx, "tcp", cfg.Target)
	if err != nil {
		atomic.AddInt64(&metrics.TotalErrors, 1)
		logger.Error("dial failed", "client", addr, "target", cfg.Target, "error", err)
		_ = client.Close()
		return
	}

	logger.Debug("tunnel established", "client", addr, "target", cfg.Target)

	bytesTx, bytesRx := proxyConnections(client, remote, addr)

	atomic.AddInt64(&metrics.TotalBytesTx, bytesTx)
	atomic.AddInt64(&metrics.TotalBytesRx, bytesRx)

	duration := time.Since(connStart)
	logger.Info("connection closed",
		"client", addr,
		"duration", duration,
		"bytes_tx", bytesTx,
		"bytes_rx", bytesRx)
}

// proxyConnections performs bidirectional copy between client and remote,
// returning the bytes transferred in each direction.
func proxyConnections(client, remote net.Conn, addr string) (tx, rx int64) {
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
		atomic.AddInt64(counter, n)

		if err != nil && !isExpectedCloseError(err) {
			atomic.AddInt64(&metrics.TotalErrors, 1)
			logger.Warn("copy error",
				"client", addr,
				"direction", direction,
				"bytes", n,
				"error", err)
		}
	}

	go copyConn(client, remote, "rx", &rx)
	copyConn(remote, client, "tx", &tx)

	return tx, rx
}

// isExpectedCloseError returns true for errors that occur during normal connection close.
func isExpectedCloseError(err error) bool {
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
	errStr := err.Error()
	if strings.Contains(errStr, "use of closed network connection") {
		return true
	}
	if strings.Contains(errStr, "connection reset by peer") {
		return true
	}
	return false
}
