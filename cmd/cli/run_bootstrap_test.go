package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"ts-bridge/internal/config"
)

type fakeControlBootstrap struct {
	done   chan struct{}
	err    error
	closed bool
}

func (f *fakeControlBootstrap) Done() <-chan struct{} { return f.done }
func (f *fakeControlBootstrap) Err() error            { return f.err }
func (f *fakeControlBootstrap) Close() error {
	f.closed = true
	return nil
}

func TestRunBootstrapFailureStopsBeforeTailscale(t *testing.T) {
	originalStarter := bootstrapStarter
	originalLogger := logger
	t.Cleanup(func() {
		bootstrapStarter = originalStarter
		logger = originalLogger
	})

	want := errors.New("ssh executable unavailable")
	bootstrapStarter = func(context.Context, config.Config) (controlBootstrap, error) {
		return nil, want
	}
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	var runErr error
	stdout, stderr := captureInitOutput(t, func() {
		runErr = Run(config.Config{
			StateDir:           t.TempDir(),
			BootstrapSSH:       "deployer@bastion.example.com",
			BootstrapSOCKSAddr: "127.0.0.1:1055",
			ConnectTimeout:     time.Second,
		})
	})
	if !errors.Is(runErr, want) {
		t.Fatalf("Run() error = %v, want %v", runErr, want)
	}
	if strings.Contains(stdout, "READY ") {
		t.Fatalf("stdout contains a false READY signal: %q", stdout)
	}
	if !strings.Contains(stderr, "ERROR reason=ssh_bootstrap_failed") {
		t.Fatalf("stderr = %q, want ssh_bootstrap_failed signal", stderr)
	}
}

func TestMonitorBootstrapExitCancelsRun(t *testing.T) {
	tunnel := &fakeControlBootstrap{
		done: closedChannel(),
		err:  errors.New("ssh connection lost"),
	}
	ctx, cancel := context.WithCancelCause(context.Background())

	monitorBootstrap(ctx, tunnel, cancel)

	cause := context.Cause(ctx)
	if cause == nil || !strings.Contains(cause.Error(), "SSH bootstrap exited") {
		t.Fatalf("context cause = %v", cause)
	}
	if !errors.Is(cause, errSSHBootstrapExited) {
		t.Fatalf("context cause = %v, want errSSHBootstrapExited", cause)
	}
}

func TestMonitorBootstrapIgnoresExitDuringShutdown(t *testing.T) {
	tunnel := &fakeControlBootstrap{
		done: closedChannel(),
		err:  errors.New("signal: killed"),
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(nil)

	monitorBootstrap(ctx, tunnel, cancel)

	if cause := context.Cause(ctx); !errors.Is(cause, context.Canceled) {
		t.Fatalf("context cause = %v, want context canceled", cause)
	}
}

func closedChannel() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func TestEmitRunCauseClassifiesBootstrapExit(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(fmt.Errorf("%w: connection lost", errSSHBootstrapExited))

	var stderr bytes.Buffer
	err := emitRunCause(ctx, &stderr)
	if !errors.Is(err, errSSHBootstrapExited) {
		t.Fatalf("emitRunCause() error = %v", err)
	}
	if !strings.Contains(stderr.String(), "ERROR reason=ssh_bootstrap_failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
