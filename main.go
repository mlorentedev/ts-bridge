package main

import (
	"context"
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

	"ts-bridge/cmd"
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
	// Backward compat: handle deprecated -version and -v flags.
	// These are the old flag-based entry points.
	// Cobra's version subcommand and --verbose flag are the new canonical ways.
	for i, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" {
			fmt.Printf("ts-bridge %s (commit %s)\n", version, commit)
			os.Exit(0)
		}
		if arg == "-v" {
			// Replace -v with --verbose in-place.
			newArgs := make([]string, 0, len(os.Args))
			newArgs = append(newArgs, os.Args[:i+1]...)
			newArgs[i] = "--verbose"
			newArgs = append(newArgs, os.Args[i+2:]...)
			os.Args = newArgs
			break
		}
	}

	// Wire build-time variables into the cmd package.
	cmd.BuildVersion = version
	cmd.BuildCommit = commit

	// Wire the bridge runner and logger into cmd package.
	cmd.Runner = run
	cmd.LoggerInit = initLogger

	// Let Cobra handle all flag parsing and command dispatch.
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
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

	// Also set the package-level logger in config so that YAML
	// warnings use structured logging (BUG-016).
	config.SetLogger(logger)
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

//nolint:unused // wired into CLI subcommands in CLI-002..CLI-006
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

//nolint:unused // helper for cleanupEphemeralStateDir
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

//nolint:unused // wired into CLI subcommands in CLI-002..CLI-006
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

	// Print banner BEFORE starting the health server to avoid concurrent
	// log output corrupting the banner (BUG-010). The health server's
	// goroutine logs "health server starting" which races with
	// printBanner()'s fmt.Println() calls.
	printBanner(cfg)

	var ready atomic.Bool
	var healthServer *http.Server
	if cfg.HealthAddr != "" {
		healthServer = health.StartServer(cfg.HealthAddr, &ready, logger)
	}

	sigCtx, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	go handleShutdown(sigCtx, &ready, listener, healthServer)

	ready.Store(true)
	var activeConns sync.WaitGroup
	dialer := &proxy.ReconnectDialer{
		Inner:       server,
		MaxRetries:  cfg.DialRetries,
		BaseBackoff: cfg.DialBackoffBase,
		MaxBackoff:  cfg.DialBackoffMax,
		Logger:      logger,
	}
	errAccept := proxy.AcceptLoop(listener, dialer, cfg, &activeConns, logger)

	drainActiveConnections(cfg, &activeConns)

	if err := server.Close(); err != nil {
		logger.Error("error closing tsnet server", "error", err)
	}

	return errAccept
}

//nolint:unused // wired into CLI subcommands in CLI-002..CLI-006
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
		if hint, remediation := diagnoseTailscaleInitError(err); hint != "" && logger != nil {
			logger.Warn(hint, "remediation", remediation)
		}
		// Release background goroutines and open file handles (tailscaled.log*)
		// before ephemeral cleanup runs — Windows cannot unlink files held by
		// the still-running tsnet workers.
		_ = server.Close()
		return nil, fmt.Errorf("tailscale init failed (control=%s): %w", controlURLForError(cfg.ControlURL), err)
	}
	logger.Info("tailscale ready", "ip", status.Self.TailscaleIPs[0])
	return server, nil
}

func controlURLForError(controlURL string) string {
	if controlURL == "" {
		return "https://controlplane.tailscale.com (default)"
	}
	return controlURL
}

// diagnoseTailscaleInitError inspects a tsnet.Up failure and returns an
// actionable hint when the error matches a known pattern. Returns empty
// strings for unrecognized errors so callers stay silent on noise.
func diagnoseTailscaleInitError(err error) (hint, remediation string) {
	if err == nil {
		return "", ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "api key does not exist"),
		strings.Contains(msg, "invalid key"),
		strings.Contains(msg, "key expired"):
		return "auth key rejected by control plane (likely expired, revoked, or single-use already consumed)",
			"regenerate a reusable+ephemeral auth key in your control plane admin and update TS_AUTHKEY on every client"
	case strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "i/o timeout"):
		return "control plane unreachable within TS_TIMEOUT",
			"check network access to the control URL; raise TS_TIMEOUT if the link is slow"
	}
	return "", ""
}

//nolint:unused // wired into CLI subcommands in CLI-002..CLI-006
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

//nolint:unused // wired into CLI subcommands in CLI-002..CLI-006
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


// bannerWidth is the number of characters available for dynamic content
// in the version line of the ASCII art banner. The inner border is 39
// characters wide ("|  ...  |"); "TAILSCALE BRIDGE " is 17 characters,
// leaving 22 for the version string. Versions longer than bannerWidth
// are truncated with "..." so the border never breaks.
const bannerWidth = 22

//nolint:unused // wired into CLI subcommands in CLI-002..CLI-006
func printBanner(cfg config.Config) {
	fmt.Println()
	fmt.Println("  +---------------------------------------+")
	v := version
	if len(v) > bannerWidth {
		v = v[:bannerWidth-3] + "..."
	}
	fmt.Printf("  |      TAILSCALE BRIDGE %-*s  |\n", bannerWidth, v)
	fmt.Println("  +---------------------------------------+")
	fmt.Printf("  |  Host:   %-26s  |\n", cfg.Hostname)
	fmt.Printf("  |  Local:  %-26s  |\n", cfg.LocalAddr)
	fmt.Printf("  |  Target: %-26s  |\n", cfg.Target)
	fmt.Println("  +---------------------------------------+")
	fmt.Println("  Waiting for connections...")
	fmt.Println()
}
