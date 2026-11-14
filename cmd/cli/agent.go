package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"ts-bridge/internal/agent"
)

// newAgentCmd constructs "ts-bridge agent": the per-user process that will own
// sessions and serves the local API (ADR-018). It runs in the foreground until
// interrupted.
func newAgentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agent",
		Short: "Run the per-user agent that serves the local API (foreground)",
		Long: `Run the per-user ts-bridge agent in the foreground until interrupted.

The agent serves a local HTTP API under /v1/ on a socket only this user can
reach, and refuses to start while another agent of the same user runs. Session
management arrives in later releases; today it answers /v1/meta and /v1/status.`,
		Args: cobra.NoArgs,
		// Hidden until it manages sessions (#782): a release should not list a
		// command whose only use today is answering /v1/meta and /v1/status.
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := agent.DefaultPaths()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runAgent(ctx, paths, cmd.ErrOrStderr())
		},
	}
}

// runAgent starts the agent on paths and serves it until ctx ends.
func runAgent(ctx context.Context, paths agent.Paths, out io.Writer) error {
	a, err := agent.Start(agent.Options{Paths: paths, Version: Version()})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "ts-bridge agent listening on %s\n", a.Endpoint())
	return errors.Join(a.Serve(ctx), a.Close())
}
