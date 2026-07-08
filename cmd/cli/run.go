package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"tailscale.com/tsnet"

	"ts-bridge/internal/config"
	"ts-bridge/internal/health"
	"ts-bridge/internal/logging"
	"ts-bridge/internal/proxy"
)

const (
	cleanupMaxAttempts   = 5
	cleanupRetryDelay    = 150 * time.Millisecond
	stateDirPerms        = 0700
	defaultControlURL    = "https://control.tailscale.com"
)

// Logger is the global structured logger (console).
var logger *slog.Logger

// logDir is the current log file path, used by the banner.
var logDir string

// logging is the dual-output logger instance.
var loggingInstance *logging.Logger

// InitLogger initializes the dual structured logger (console + file).
func InitLogger(cfg config.Config) {
	logDir = logging.LogDirForPlatform()

	loggingInstance = logging.New(logging.Config{
		Verbose:   cfg.Verbose,
		LogFormat: cfg.LogFormat,
		LogDir:    logDir,
	})

	// Use the combined logger (writes to both console and file).
	logger = loggingInstance.Combined()

	// Also set the package-level logger in config so that YAML
	// warnings use structured logging (BUG-016).
	config.SetLogger(logger)
}

// LogFile returns the current log file path for banner display.
func LogFile() string {
	return logDir
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
	if runtime.GOOS != windowsOS && info.Mode().Perm()&0077 != 0 {
		logger.Warn("state directory has loose permissions", "path", dir, "perms", fmt.Sprintf("%o", info.Mode().Perm()))
	}
	return nil
}

//nolint:unused // wired into Runner = run
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

//nolint:unused // helper for cleanupEphemeralStateDir (called via Runner)
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

//nolint:unused // wired into Runner = Run
func Run(cfg config.Config) error {
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
	// goroutine logs "health server starting" which races with the
	// banner's stdout writes. The READY signal (#203) is emitted here for
	// the same reason — grouped with the banner, before any logger stdout —
	// and uses the actual bound address so auto-port mode reports the real
	// port. The listener already accepts (the OS queues connections until
	// AcceptLoop runs), so "READY" is accurate at this point.
	writeStartupBanner(os.Stdout, cfg)
	emitReady(os.Stdout, listener.Addr().String(), cfg.Target)

	var tunnelStatus health.TunnelStatus
	var healthServer *http.Server
	if cfg.HealthAddr != "" {
		healthServer = health.StartServer(cfg.HealthAddr, &tunnelStatus, logger)
	}

	sigCtx, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	ctx, cancelWithCause := context.WithCancelCause(sigCtx)
	defer func() {
		cancelWithCause(nil)
		sigCancel()
		if loggingInstance != nil {
			// #nosec G104 // cleanup: ignore close errors.
			loggingInstance.Close()
		}
	}()

	go handleShutdown(ctx, &tunnelStatus, listener, healthServer)

	tunnelStatus.MarkReady()
	var activeConns sync.WaitGroup
	dialer := &proxy.ReconnectDialer{
		Inner:       server,
		MaxRetries:  cfg.DialRetries,
		BaseBackoff: cfg.DialBackoffBase,
		MaxBackoff:  cfg.DialBackoffMax,
		Logger:      logger,
	}
	errAccept := proxy.AcceptLoop(listener, dialer, cfg, &activeConns, cancelWithCause, logger)

	drainActiveConnections(cfg, &activeConns)

	if err := server.Close(); err != nil {
		logger.Error("error closing tsnet server", "error", err)
	}

	return errAccept
}

//nolint:unused // wired into Runner = Run
func initTailscale(cfg config.Config) (*tsnet.Server, error) {
	var tsnetLogf func(string, ...any)
	if loggingInstance != nil {
		tsnetLogf = func(format string, args ...any) {
			loggingInstance.File().Debug(fmt.Sprintf(format, args...), "component", "tsnet")
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
		reason, hint, remediation := diagnoseTailscaleInitError(err)
		// Machine-readable startup-failure signal for programmatic callers
		// (#204) — always emitted (reason falls back to "unknown"), before
		// the human-oriented log line.
		emitError(os.Stderr, reason, err.Error())
		if hint != "" && logger != nil {
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

// diagnoseTailscaleInitError inspects a tsnet.Up failure and returns a
// stable machine-readable reason token (UX-004 / #204) plus an actionable
// human hint when the error matches a known pattern. The hint/remediation
// stay empty for unrecognized errors so the human log stays silent on
// noise, but reason is never empty for a non-nil error — it falls back to
// reasonUnknown so the ERROR signal is always emitted.
func diagnoseTailscaleInitError(err error) (reason, hint, remediation string) {
	if err == nil {
		return "", "", ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "api key does not exist"),
		strings.Contains(msg, "invalid key"),
		strings.Contains(msg, "key expired"):
		return reasonBadAuthKey,
			"auth key rejected by control plane (likely expired, revoked, or single-use already consumed)",
			"regenerate a reusable+ephemeral auth key in your control plane admin and update TS_AUTHKEY on every client"
	case strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "i/o timeout"):
		return reasonControlPlaneUnreachable,
			"control plane unreachable within TS_TIMEOUT",
			"check network access to the control URL; raise TS_TIMEOUT if the link is slow"
	}
	return reasonUnknown, "", ""
}

//nolint:unused // wired into Runner = Run
func handleShutdown(ctx context.Context, status *health.TunnelStatus, listener net.Listener, healthServer *http.Server) {
	<-ctx.Done()
	logger.Info("shutting down")
	cause := context.Cause(ctx)
	if cause != nil && !errors.Is(cause, context.Canceled) {
		logger.Error("tsnet session terminal", "reason", cause.Error())
		status.Store(false, cause.Error())
	} else {
		status.Store(false, "")
	}
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

//nolint:unused // wired into Runner = Run
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

// writeStartupBanner renders the decorative human banner to w. It is a
// no-op when cfg.Quiet is set (UX-004 / #203) — the machine-readable READY
// line is emitted separately and always. Takes an io.Writer so it is unit
// testable without capturing os.Stdout.
//
//nolint:unused // wired into Runner = Run
func writeStartupBanner(w io.Writer, cfg config.Config) {
	if cfg.Quiet {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  +---------------------------------------+")
	v := Version()
	if len(v) > bannerWidth {
		v = v[:bannerWidth-3] + "..."
	}
	fmt.Fprintf(w, "  |      TAILSCALE BRIDGE %-*s  |\n", bannerWidth, v)
	fmt.Fprintln(w, "  +---------------------------------------+")
	fmt.Fprintf(w, "  |  Host:   %-26s  |\n", cfg.Hostname)
	fmt.Fprintf(w, "  |  Local:  %-26s  |\n", cfg.LocalAddr)
	fmt.Fprintf(w, "  |  Target: %-26s  |\n", cfg.Target)
	if cfg.ControlURL != "" && cfg.ControlURL != defaultControlURL {
		fmt.Fprintf(w, "  |  Control: %-25s  |\n", cfg.ControlURL)
	}
	if cfg.HealthAddr != "" {
		fmt.Fprintf(w, "  |  Health:  %-26s  |\n", cfg.HealthAddr)
	}
	if cfg.EphemeralState {
		fmt.Fprintln(w, "  |  Node:    ephemeral (not persisted in admin console)  |")
	}
	if logDir != "" {
		fmt.Fprintf(w, "  |  Log:      %-26s  |\n", logDir)
	}
	fmt.Fprintln(w, "  +---------------------------------------+")
	fmt.Fprintln(w, "  Waiting for connections...")
	fmt.Fprintln(w)
}
