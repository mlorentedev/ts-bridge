package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"ts-bridge/internal/bootstrapssh"
	"ts-bridge/internal/config"
	"ts-bridge/internal/host"
	"ts-bridge/internal/serve"
)

// defaultServeSOCKSAddr is where serve's SSH bootstrap listens unless
// host.mesh.bootstrap_socks_addr says otherwise: connect's default.
const defaultServeSOCKSAddr = "127.0.0.1:1055"

// serveNode is the started tailnet node serve publishes on (a tsnet.Server).
type serveNode interface {
	serve.Node
	Close() error
}

var (
	serveNodeStarter    = startServeNode
	serveEnsureStateDir = host.EnsureStateDir
)

func newHostServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Join the tailnet as a persistent node and publish the declared services",
		Long: `Join the tailnet as a persistent node and publish exactly the services
the host file declares (host.services), each forwarded to 127.0.0.1:<port>
on this host. Nothing else answers on the node (HOST-008, ADR-017 Amendment A).

The host file must declare host.control_url and host.mesh: state_dir keeps the
node identity (0700 root, or SYSTEM and Administrators on Windows), and
auth_key_file is read only for the first enrolment. bootstrap_ssh reaches the
control plane through system OpenSSH, as connect --bootstrap-ssh does.

Serve runs in the foreground. The OS starts and restarts it (a Windows
Scheduled Task as SYSTEM, a systemd unit on Linux); ts-bridge installs nothing.

Signals:
  READY node=<hostname> services=<kind:port,...> enrolled=<true|false>  (stdout)
    enrolled=true   this start enrolled the node with auth_key_file (first start;
                    the provisioning may now delete the key file)
    enrolled=false  this start reused the node identity already in state_dir
  ERROR reason=<token> detail="..."                                      (stderr)
    reason=node_not_tagged  the node came up without tags; nothing is published
    reason=publish_failed   a declared service could not listen on the node
    reason=ssh_bootstrap_failed, control_plane_unreachable, bad_authkey as connect

Examples:
  ts-bridge host serve --config /etc/ts-bridge/host.yaml
`,
		RunE: runHostServe,
	}
	cmd.Flags().String("config", "", "Host configuration file (YAML) declaring host.mesh and the services (required)")
	cmd.Flags().Bool("verbose", false, "Enable verbose (debug) logging")
	return cmd
}

// servePlan is everything serve resolved from the host file before starting.
type servePlan struct {
	node     serve.NodeConfig
	mesh     host.FileMesh
	services []serve.Service
	logger   *slog.Logger
}

func runHostServe(cmd *cobra.Command, _ []string) error {
	plan, err := loadServePlan(cmd)
	if err != nil {
		return err
	}
	sigCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancelCause(sigCtx)
	defer cancel(nil)

	closeBootstrap, err := startServeBootstrap(ctx, cancel, plan)
	if err != nil {
		emitError(cmd.ErrOrStderr(), reasonSSHBootstrapFailed, err.Error())
		return err
	}
	defer closeBootstrap()
	return runServeNode(ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(), plan)
}

// loadServePlan reads the host file and refuses, before anything starts, one
// that serve cannot run from.
func loadServePlan(cmd *cobra.Command) (servePlan, error) {
	path := mustString(cmd, "config")
	if path == "" {
		return servePlan{}, errors.New("host serve: --config is required; serve publishes only what a host file declares")
	}
	file, err := host.LoadFile(path, hostGOOS, cmd.ErrOrStderr())
	if err != nil {
		return servePlan{}, err
	}
	if file.Host.Mesh == nil {
		return servePlan{}, fmt.Errorf("host serve: %s declares no host.mesh", path)
	}
	if file.Host.ControlURL == nil {
		return servePlan{}, fmt.Errorf("host serve: %s declares no host.control_url", path)
	}
	mesh := *file.Host.Mesh
	if mesh.StateDir == nil {
		return servePlan{}, fmt.Errorf("host serve: %s declares no host.mesh.state_dir", path)
	}
	if err := serveEnsureStateDir(*mesh.StateDir); err != nil {
		return servePlan{}, err
	}
	node, err := serve.NewNodeConfig(mesh, *file.Host.ControlURL)
	if err != nil {
		return servePlan{}, err
	}
	cfg := host.Resolve(hostGOOS, file, host.Flags{Verbose: mustBool(cmd, "verbose"), VerboseSet: cmd.Flags().Changed("verbose")})
	logger := newHostLogger(cfg, cmd.ErrOrStderr())
	// tsnet's own logs go to the serve logger at debug, so --verbose shows why a
	// node failed to join.
	node.Logf = func(format string, args ...any) {
		logger.Debug(fmt.Sprintf(format, args...), "component", "tsnet")
	}
	return servePlan{node: node, mesh: mesh, services: serveServices(cfg), logger: logger}, nil
}

func serveServices(cfg host.Config) []serve.Service {
	services := make([]serve.Service, 0, len(cfg.Services))
	for _, s := range cfg.Services {
		services = append(services, serve.Service{Kind: s.Kind, Port: s.Port})
	}
	return services
}

// startServeBootstrap starts the CLI-363 SSH bootstrap when host.mesh declares
// one. Until PR 3's supervisor, its exit stops serve.
func startServeBootstrap(ctx context.Context, cancel context.CancelCauseFunc, plan servePlan) (func(), error) {
	if plan.mesh.BootstrapSSH == nil {
		return func() {}, nil
	}
	socks := defaultServeSOCKSAddr
	if plan.mesh.BootstrapSOCKSAddr != nil {
		socks = *plan.mesh.BootstrapSOCKSAddr
	}
	tunnel, err := sshTunnelStarter(ctx, bootstrapssh.Config{
		Endpoint:     *plan.mesh.BootstrapSSH,
		SOCKSAddr:    socks,
		ReadyTimeout: config.Defaults().ConnectTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("start SSH control-plane bootstrap: %w", err)
	}
	if err := controlProxyConfigurer(ctx, plan.node.ControlURL, socks); err != nil {
		_ = tunnel.Close()
		return nil, fmt.Errorf("configure SSH control-plane bootstrap: %w", err)
	}
	plan.logger.Info("SSH control-plane bootstrap ready", "endpoint", *plan.mesh.BootstrapSSH, "socks_addr", socks)
	go monitorBootstrap(ctx, tunnel, cancel)
	return func() { _ = tunnel.Close() }, nil
}

// runServeNode brings the node up, refuses it without tags, publishes the
// services, prints READY and serves until ctx ends.
func runServeNode(ctx context.Context, stdout, stderr io.Writer, plan servePlan) error {
	node, tags, err := serveNodeStarter(ctx, plan.node)
	if err != nil {
		if cause := emitRunCause(ctx, stderr); cause != nil {
			return cause
		}
		reason, _, _ := diagnoseTailscaleInitError(err)
		emitError(stderr, reason, err.Error())
		return fmt.Errorf("start the serve node (control=%s): %w", plan.node.ControlURL, err)
	}
	defer func() { _ = node.Close() }()
	if len(tags) == 0 {
		err := fmt.Errorf("node %s has no tags, so the host would be a user's node: remove the node from the control plane, empty state_dir %s, put a tagged auth key in auth_key_file and start again (the key is read only while state_dir is empty)", plan.node.Hostname, plan.node.StateDir)
		emitError(stderr, reasonNodeNotTagged, err.Error())
		return err
	}
	published, err := serve.Publish(node, plan.services, serve.LoopbackDialer{}, config.Defaults(), plan.logger)
	if err != nil {
		emitError(stderr, reasonPublishFailed, err.Error())
		return err
	}
	defer func() { _ = published.Close() }()
	fmt.Fprintln(stdout, formatServeReady(plan.node.Hostname, plan.services, plan.node.Enrolling))
	<-ctx.Done()
	if cause := emitRunCause(ctx, stderr); cause != nil && !errors.Is(cause, context.Canceled) {
		return cause
	}
	return nil
}

func formatServeReady(node string, services []serve.Service, enrolled bool) string {
	parts := make([]string, 0, len(services))
	for _, s := range services {
		parts = append(parts, s.Kind+":"+strconv.Itoa(s.Port))
	}
	return fmt.Sprintf("READY node=%s services=%s enrolled=%t", node, strings.Join(parts, ","), enrolled)
}

// startServeNode brings the persistent tsnet node up and returns its tags.
func startServeNode(ctx context.Context, nc serve.NodeConfig) (serveNode, []string, error) {
	s := nc.Server()
	upCtx, cancel := context.WithTimeout(ctx, config.Defaults().ConnectTimeout)
	defer cancel()
	status, err := s.Up(upCtx)
	if err != nil {
		_ = s.Close()
		return nil, nil, err
	}
	var tags []string
	if status.Self != nil && status.Self.Tags != nil {
		tags = status.Self.Tags.AsSlice()
	}
	return s, tags, nil
}
