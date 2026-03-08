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
	"hash/fnv"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
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

	// Default runtime values.
	defaultLocalAddr     = "127.0.0.1:33389"
	defaultHostname      = "ts-bridge"
	defaultStateDir      = "./ts-state"
	defaultAutoPortRange = "33389-34388"
	cleanupMaxAttempts   = 5
	cleanupRetryDelay    = 150 * time.Millisecond

	// backoffMin/Max for accept loop error recovery.
	backoffMin = 100 * time.Millisecond
	backoffMax = 10 * time.Second

	// stateDirPerms ensures state directory is only readable by owner.
	stateDirPerms = 0700
)

// Dialer abstracts the remote connection mechanism.
// tsnet.Server satisfies this interface without an adapter.
type Dialer interface {
	Dial(ctx context.Context, network, addr string) (net.Conn, error)
}

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
	AutoInstance   bool
	EphemeralState bool
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

	cfg := Config{
		LocalAddr:      os.Getenv("TS_LOCAL_ADDR"),
		Target:         target,
		AuthKey:        authKey,
		Hostname:       os.Getenv("TS_HOSTNAME"),
		StateDir:       os.Getenv("TS_STATE_DIR"),
		ControlURL:     os.Getenv("TS_CONTROL_URL"),
		ConnectTimeout: timeout,
		MaxConnections: maxConns,
		HealthAddr:     os.Getenv("TS_HEALTH_ADDR"),
		Verbose:        verbose,
		LogFormat:      envOr("TS_LOG_FORMAT", "text"),
	}

	if err := applyAutoInstanceConfig(&cfg); err != nil {
		return Config{}, err
	}

	if cfg.LocalAddr == "" {
		cfg.LocalAddr = defaultLocalAddr
	}
	if cfg.Hostname == "" {
		cfg.Hostname = defaultHostname
	}
	if cfg.StateDir == "" {
		cfg.StateDir = defaultStateDir
	}

	return cfg, nil
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
	if !strings.HasPrefix(authKey, "tskey-") && !strings.HasPrefix(authKey, "hskey-") {
		return "", errors.New("TS_AUTHKEY: invalid format (must start with tskey- or hskey-)")
	}
	return authKey, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func applyAutoInstanceConfig(cfg *Config) error {
	cfg.AutoInstance = shouldEnableAutoInstance()
	if !cfg.AutoInstance {
		return nil
	}

	instanceName := os.Getenv("TS_INSTANCE_NAME")
	portRange := envOr("TS_PORT_RANGE", defaultAutoPortRange)

	if cfg.LocalAddr == "" {
		localAddr, err := deriveAutoLocalAddr(cfg.Target, instanceName, portRange)
		if err != nil {
			return err
		}
		cfg.LocalAddr = localAddr
	}

	if cfg.Hostname == "" {
		cfg.Hostname = deriveAutoHostname(cfg.Target, instanceName)
	}

	if cfg.StateDir == "" {
		cfg.StateDir = filepath.Join(os.TempDir(), "ts-bridge", cfg.Hostname)
		cfg.EphemeralState = true
	}

	return nil
}

func shouldEnableAutoInstance() bool {
	if parseBoolEnv(os.Getenv("TS_MANUAL_MODE")) {
		return false
	}

	rawAutoMode := strings.TrimSpace(os.Getenv("TS_AUTO_INSTANCE"))
	if rawAutoMode == "" {
		return true
	}

	return parseBoolEnv(rawAutoMode)
}

func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func deriveAutoLocalAddr(target, instanceName, portRange string) (string, error) {
	start, end, err := parsePortRange(portRange)
	if err != nil {
		return "", err
	}

	hostName, err := os.Hostname()
	if err != nil || hostName == "" {
		hostName = "unknown-host"
	}

	seed := fmt.Sprintf("%s|%s|%s", hostName, target, instanceName)
	port, err := selectAvailablePort(seed, start, end)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("127.0.0.1:%d", port), nil
}

func deriveAutoHostname(target, instanceName string) string {
	hostName, err := os.Hostname()
	if err != nil || hostName == "" {
		hostName = "unknown-host"
	}

	machine := sanitizeHostnameLabel(hostName)
	instance := sanitizeHostnameLabel(instanceName)
	if instance == "" {
		instance = machine
	}
	if instance == "" {
		instance = "bridge"
	}

	base := "tsb-" + instance
	if len(base) > 30 {
		base = strings.Trim(base[:30], "-")
	}
	if base == "" {
		base = "tsb-bridge"
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(machine + "|" + target + "|" + instanceName))
	hash := fmt.Sprintf("%06x", hasher.Sum32()&0xffffff)

	hostname := fmt.Sprintf("%s-%s-%d", base, hash, os.Getpid())
	if len(hostname) > 63 {
		hostname = strings.Trim(hostname[:63], "-")
	}
	if hostname == "" {
		return defaultHostname
	}
	return hostname
}

func sanitizeHostnameLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	previousDash := false

	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			previousDash = false
			continue
		}
		if !previousDash {
			b.WriteByte('-')
			previousDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}

func parsePortRange(value string) (int, int, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("TS_PORT_RANGE invalid format %q (expected START-END)", value)
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("TS_PORT_RANGE invalid start port: %w", err)
	}

	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("TS_PORT_RANGE invalid end port: %w", err)
	}

	if start < 1 || end > 65535 || start > end {
		return 0, 0, fmt.Errorf("TS_PORT_RANGE out of bounds: %d-%d", start, end)
	}

	return start, end, nil
}

func selectAvailablePort(seed string, start, end int) (int, error) {
	span := end - start + 1
	if span <= 0 {
		return 0, fmt.Errorf("TS_PORT_RANGE has invalid span: %d", span)
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(seed))
	offset := int(int64(hasher.Sum32()) % int64(span))

	for i := 0; i < span; i++ {
		port := start + ((offset + i) % span)
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		if err := listener.Close(); err != nil {
			continue
		}
		return port, nil
	}

	return 0, fmt.Errorf("TS_PORT_RANGE has no free ports in %d-%d", start, end)
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
	// Warn if permissions are too open (Unix-style perms are not reliable on Windows).
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		logger.Warn("state directory has loose permissions", "path", dir, "perms", fmt.Sprintf("%o", info.Mode().Perm()))
	}
	return nil
}

func cleanupEphemeralStateDir(dir string) {
	for attempt := 1; attempt <= cleanupMaxAttempts; attempt++ {
		err := os.RemoveAll(dir)
		if err == nil || os.IsNotExist(err) {
			if logger != nil {
				logger.Debug("cleaned up ephemeral state directory", "path", dir, "attempt", attempt)
			}
			return
		}

		if !isRetryableCleanupError(err) || attempt == cleanupMaxAttempts {
			if logger != nil {
				logger.Warn("failed to cleanup ephemeral state directory",
					"path", dir,
					"error", err,
					"attempts", attempt)
			}
			return
		}

		time.Sleep(time.Duration(attempt) * cleanupRetryDelay)
	}
}

func isRetryableCleanupError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "directory is not empty") ||
		strings.Contains(errStr, "access is denied") ||
		strings.Contains(errStr, "being used by another process") ||
		strings.Contains(errStr, "resource busy") ||
		strings.Contains(errStr, "device or resource busy")
}

func run(cfg Config) error {
	if err := ensureStateDir(cfg.StateDir); err != nil {
		return err
	}

	if cfg.EphemeralState {
		defer cleanupEphemeralStateDir(cfg.StateDir)
	}

	if cfg.AutoInstance {
		logger.Info("auto-instance mode enabled",
			"local_addr", cfg.LocalAddr,
			"hostname", cfg.Hostname,
			"state_dir", cfg.StateDir,
			"ephemeral_state", cfg.EphemeralState)
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
	fmt.Printf("  |  Host:   %-26s  |\n", cfg.Hostname)
	fmt.Printf("  |  Local:  %-26s  |\n", cfg.LocalAddr)
	fmt.Printf("  |  Target: %-26s  |\n", cfg.Target)
	fmt.Println("  +---------------------------------------+")
	fmt.Println("  Waiting for connections...")
	fmt.Println()
}

func acceptLoop(listener net.Listener, dialer Dialer, cfg Config) error {
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

		go handleConn(conn, dialer, cfg)
	}
}

// Buffer pool to reduce GC pressure.
var bufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, bufferSize)
		return &b
	},
}

func handleConn(client net.Conn, dialer Dialer, cfg Config) {
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

	remote, err := dialer.Dial(ctx, "tcp", cfg.Target)
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
