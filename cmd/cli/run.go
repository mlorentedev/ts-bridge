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

	"ts-bridge/internal/bootstrapssh"
	"ts-bridge/internal/config"
	"ts-bridge/internal/health"
	"ts-bridge/internal/logging"
	"ts-bridge/internal/proxy"
)

const (
	cleanupMaxAttempts = 5
	cleanupRetryDelay  = 150 * time.Millisecond
	stateDirPerms      = 0700
	defaultControlURL  = "https://control.tailscale.com"
)

// Logger is the global structured logger (console).
var logger *slog.Logger

// logDir is the current log file path, used by the banner.
var logDir string

// logging is the dual-output logger instance.
var loggingInstance *logging.Logger

var errSSHBootstrapExited = errors.New("SSH bootstrap exited")

type controlBootstrap interface {
	Done() <-chan struct{}
	Err() error
	Close() error
}

var bootstrapStarter = startConfiguredBootstrap

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
	return run(cfg, nil)
}

func run(cfg config.Config, onReady func() error) error {
	if err := ensureStateDir(cfg.StateDir); err != nil {
		return err
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

	if cfg.EphemeralState {
		defer cleanupEphemeralStateDir(cfg.StateDir)
	}

	closeBootstrap, err := startBootstrapLifecycle(ctx, cancelWithCause, cfg)
	if err != nil {
		emitError(os.Stderr, reasonSSHBootstrapFailed, err.Error())
		return err
	}
	defer closeBootstrap()

	server, err := initTailscale(ctx, cfg)
	if err != nil {
		if runErr := emitRunCause(ctx, os.Stderr); runErr != nil {
			return runErr
		}
		return err
	}

	listenAddr := proxyListenerAddr(cfg)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		_ = server.Close()
		return fmt.Errorf("bind %s: %w", listenAddr, err)
	}
	if err := announceReady(ctx, listener, cfg, onReady); err != nil {
		_ = listener.Close()
		_ = server.Close()
		return err
	}

	var tunnelStatus health.TunnelStatus
	var healthServer *http.Server
	if cfg.HealthAddr != "" {
		healthServer = health.StartServer(cfg.HealthAddr, &tunnelStatus, logger)
	}

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
	var errAccept error
	if cfg.SOCKS5Addr != "" {
		resolver, resolveErr := socksTargetResolver(cfg)
		if resolveErr != nil {
			_ = listener.Close()
			_ = server.Close()
			return fmt.Errorf("configure SOCKS5 routes: %w", resolveErr)
		}
		errAccept = proxy.AcceptSOCKS5Loop(listener, dialer, cfg, resolver, &activeConns, cancelWithCause, logger)
	} else {
		errAccept = proxy.AcceptLoop(listener, dialer, cfg, &activeConns, cancelWithCause, logger)
	}

	drainActiveConnections(cfg, &activeConns)

	if err := server.Close(); err != nil {
		logger.Error("error closing tsnet server", "error", err)
	}

	if cause := emitRunCause(ctx, os.Stderr); cause != nil && !errors.Is(cause, context.Canceled) {
		return cause
	}
	return errAccept
}

func announceReady(ctx context.Context, listener net.Listener, cfg config.Config, onReady func() error) error {
	if cause := emitRunCause(ctx, os.Stderr); cause != nil {
		return cause
	}

	// Keep human and machine startup output together before concurrent health logs.
	writeStartupBanner(os.Stdout, cfg)
	if err := emitReadyIfActive(ctx, os.Stdout, listener.Addr().String(), readyTarget(cfg)); err != nil {
		_ = emitRunCause(ctx, os.Stderr)
		return err
	}
	if onReady != nil {
		if err := onReady(); err != nil {
			return fmt.Errorf("ready callback: %w", err)
		}
	}
	return nil
}

func startBootstrapLifecycle(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	cfg config.Config,
) (func(), error) {
	bootstrap, err := bootstrapStarter(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if bootstrap == nil {
		return func() {}, nil
	}
	go monitorBootstrap(ctx, bootstrap, cancel)
	return func() {
		_ = bootstrap.Close()
	}, nil
}

func startConfiguredBootstrap(ctx context.Context, cfg config.Config) (controlBootstrap, error) {
	if cfg.BootstrapSSH == "" {
		return nil, nil
	}

	tunnel, err := bootstrapssh.Start(ctx, bootstrapssh.Config{
		Endpoint:     cfg.BootstrapSSH,
		SOCKSAddr:    cfg.BootstrapSOCKSAddr,
		ReadyTimeout: cfg.ConnectTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("start SSH control-plane bootstrap: %w", err)
	}
	if err := bootstrapssh.ConfigureControlProxy(cfg.ControlURL, cfg.BootstrapSOCKSAddr); err != nil {
		_ = tunnel.Close()
		return nil, fmt.Errorf("configure SSH control-plane bootstrap: %w", err)
	}
	logger.Info("SSH control-plane bootstrap ready",
		"endpoint", cfg.BootstrapSSH,
		"socks_addr", cfg.BootstrapSOCKSAddr,
		"control_url", cfg.ControlURL)
	return tunnel, nil
}

func monitorBootstrap(
	ctx context.Context,
	tunnel controlBootstrap,
	cancel context.CancelCauseFunc,
) {
	select {
	case <-ctx.Done():
		return
	case <-tunnel.Done():
		if ctx.Err() != nil {
			return
		}
		err := tunnel.Err()
		if err == nil {
			err = errors.New("process exited without an error")
		}
		cancel(fmt.Errorf("%w: %v", errSSHBootstrapExited, err))
	}
}

func emitRunCause(ctx context.Context, stderr io.Writer) error {
	cause := context.Cause(ctx)
	if cause == nil {
		return nil
	}
	if errors.Is(cause, errSSHBootstrapExited) {
		emitError(stderr, reasonSSHBootstrapFailed, cause.Error())
	}
	return cause
}

func socksTargetResolver(cfg config.Config) (proxy.SOCKSTargetResolver, error) {
	if len(cfg.SOCKS5Routes) == 0 {
		return proxy.DirectSOCKSTarget(), nil
	}
	return proxy.NewAllowlistSOCKSTarget(cfg.SOCKS5Routes)
}

func proxyListenerAddr(cfg config.Config) string {
	if cfg.SOCKS5Addr != "" {
		return cfg.SOCKS5Addr
	}
	return cfg.LocalAddr
}

func readyTarget(cfg config.Config) string {
	if cfg.SOCKS5Addr != "" {
		return "dynamic-socks5"
	}
	return cfg.Target
}

//nolint:unused // wired into Runner = Run
func initTailscale(parentCtx context.Context, cfg config.Config) (*tsnet.Server, error) {
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

	ctx, cancel := context.WithTimeout(parentCtx, cfg.ConnectTimeout)
	defer cancel()

	status, err := server.Up(ctx)
	if err != nil {
		if cause := context.Cause(parentCtx); cause != nil {
			_ = server.Close()
			return nil, cause
		}
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
	fmt.Fprintf(w, "  |  Local:  %-26s  |\n", proxyListenerAddr(cfg))
	fmt.Fprintf(w, "  |  Target: %-26s  |\n", readyTarget(cfg))
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
