package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"tailscale.com/client/local"
	"tailscale.com/tsnet"

	"ts-bridge/internal/bootstrapssh"
	"ts-bridge/internal/config"
	"ts-bridge/internal/health"
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
	serveCheckKeyFile   = host.CheckSecretFile
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
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ready := &serve.Health{Status: new(health.TunnelStatus)}
	stopBootstrap, err := startServeBootstrap(ctx, plan, cmd.ErrOrStderr(), ready)
	if err != nil {
		emitError(cmd.ErrOrStderr(), reasonSSHBootstrapFailed, err.Error())
		return err
	}
	defer stopBootstrap()
	return runServeNode(ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(), plan, ready)
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
	node, err := serve.NewNodeConfig(mesh, *file.Host.ControlURL, serveCheckKeyFile)
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
// one, then supervises it: an exit after start is reported once on stderr and
// in readiness, and the tunnel is restarted with connect's backoff (AC4). The
// returned func stops the supervisor and closes the tunnel.
func startServeBootstrap(ctx context.Context, plan servePlan, stderr io.Writer, ready serve.Readiness) (func(), error) {
	if plan.mesh.BootstrapSSH == nil {
		return func() {}, nil
	}
	socks := defaultServeSOCKSAddr
	if plan.mesh.BootstrapSOCKSAddr != nil {
		socks = *plan.mesh.BootstrapSOCKSAddr
	}
	defaults := config.Defaults()
	tunnelCfg := bootstrapssh.Config{Endpoint: *plan.mesh.BootstrapSSH, SOCKSAddr: socks, ReadyTimeout: defaults.ConnectTimeout}
	start := func(ctx context.Context) (serve.Tunnel, error) {
		t, err := sshTunnelStarter(ctx, tunnelCfg)
		if err != nil {
			return nil, err
		}
		return t, nil
	}
	tunnel, err := start(ctx)
	if err != nil {
		return nil, fmt.Errorf("start SSH control-plane bootstrap: %w", err)
	}
	if err := controlProxyConfigurer(ctx, plan.node.ControlURL, socks); err != nil {
		_ = tunnel.Close()
		return nil, fmt.Errorf("configure SSH control-plane bootstrap: %w", err)
	}
	plan.logger.Info("SSH control-plane bootstrap ready", "endpoint", *plan.mesh.BootstrapSSH, "socks_addr", socks)
	sup := &serve.Supervisor{
		Start:     start,
		Readiness: ready,
		OnExit:    func(err error) { emitError(stderr, reasonSSHBootstrapFailed, err.Error()) },
		Base:      defaults.DialBackoffBase,
		Max:       defaults.DialBackoffMax,
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); sup.Run(runCtx, tunnel) }()
	return func() { cancel(); <-done }, nil
}

// runServeNode brings the node up, refuses it without tags, publishes the
// services, serves health, prints READY and serves until ctx ends.
func runServeNode(ctx context.Context, stdout, stderr io.Writer, plan servePlan, ready *serve.Health) error {
	node, tags, err := serveNodeStarter(ctx, plan.node)
	if err != nil {
		if ctx.Err() != nil {
			return nil
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
	watchServeControl(ctx, node, ready, plan.logger)
	ready.Up("serve")
	if plan.mesh.HealthAddr != nil {
		srv := health.StartServerWithReport(*plan.mesh.HealthAddr, ready.Status, serveReport(plan), plan.logger)
		defer shutdownHealth(srv)
	}
	fmt.Fprintln(stdout, formatServeReady(plan.node.Hostname, plan.services, plan.node.Enrolling))
	<-ctx.Done()
	return nil
}

// serveReport names the node and reports every declared service published:
// Publish opened them all, or serve did not get this far.
func serveReport(plan servePlan) health.Report {
	services := make([]health.ServiceState, 0, len(plan.services))
	for _, s := range plan.services {
		services = append(services, health.ServiceState{Kind: s.Kind, Port: s.Port, Published: true})
	}
	return func() (string, []health.ServiceState) { return plan.node.Hostname, services }
}

// localAPINode is a node that exposes its local API; the production node,
// *tsnet.Server, must stay one or the control session goes unwatched.
type localAPINode interface {
	LocalClient() (*local.Client, error)
}

var _ localAPINode = (*tsnet.Server)(nil)

// watchServeControl reports the node's control session on readiness when the
// node exposes its local API (a tsnet.Server does).
func watchServeControl(ctx context.Context, node serveNode, ready serve.Readiness, logger *slog.Logger) {
	n, ok := node.(localAPINode)
	if !ok {
		return
	}
	lc, err := n.LocalClient()
	if err == nil {
		var states <-chan string
		if states, err = serve.NodeStates(ctx, lc); err == nil {
			go func() {
				serve.WatchControlSession(ctx, states, ready)
				if ctx.Err() == nil {
					logger.Warn("control session watch ended; /health/ready reads control_plane_unreachable")
				}
			}()
			return
		}
	}
	logger.Warn("control session not watched; /health/ready will not report its loss", "error", err)
}

func shutdownHealth(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
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
