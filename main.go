package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"tailscale.com/tsnet"

	"ts-bridge/internal/config"
	"ts-bridge/internal/health"
	"ts-bridge/internal/proxy"
)

// Build-time variables set via ldflags.
var (
	version = "dev"
	commit  = "unknown"
)

const (
	cleanupMaxAttempts   = 5
	cleanupRetryDelay    = 150 * time.Millisecond
	stateDirPerms        = 0700
)

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

	cfg, err := config.LoadConfig(*verbose)
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

func initLogger(cfg config.Config) {
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

func run(cfg config.Config) error {
	if err := ensureStateDir(cfg.StateDir); err != nil {
		return err
	}

	if cfg.EphemeralState {
		defer cleanupEphemeralStateDir(cfg.StateDir)
	}

	server, err := initTailscale(cfg)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", cfg.LocalAddr)
	if err != nil {
		_ = server.Close()
		return fmt.Errorf("bind %s: %w", cfg.LocalAddr, err)
	}

	var ready atomic.Bool
	var healthServer *http.Server
	if cfg.HealthAddr != "" {
		healthServer = health.StartServer(cfg.HealthAddr, &ready, logger)
	}

	printBanner(cfg)

	sigCtx, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	go handleShutdown(sigCtx, &ready, listener, healthServer)

	ready.Store(true)
	var activeConns sync.WaitGroup
	errAccept := proxy.AcceptLoop(listener, server, cfg, &activeConns, logger)

	drainActiveConnections(cfg, &activeConns)

	if err := server.Close(); err != nil {
		logger.Error("error closing tsnet server", "error", err)
	}

	return errAccept
}

func initTailscale(cfg config.Config) (*tsnet.Server, error) {
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
		return nil, fmt.Errorf("tailscale init failed: %w", err)
	}
	logger.Info("tailscale ready", "ip", status.Self.TailscaleIPs[0])
	return server, nil
}

func handleShutdown(ctx context.Context, ready *atomic.Bool, listener net.Listener, healthServer *http.Server) {
	<-ctx.Done()
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
}

func drainActiveConnections(cfg config.Config, wg *sync.WaitGroup) {
	if cfg.DrainTimeout <= 0 {
		return
	}

	logger.Info("draining active connections", "timeout", cfg.DrainTimeout)
	drainCtx, drainCancel := context.WithTimeout(context.Background(), cfg.DrainTimeout)
	defer drainCancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("all active connections drained gracefully")
	case <-drainCtx.Done():
		logger.Warn("drain timeout exceeded, forcing shutdown")
	}
}


func printBanner(cfg config.Config) {
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
