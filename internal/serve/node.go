package serve

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"tailscale.com/tsnet"

	"ts-bridge/internal/host"
)

// stateFile is where tsnet keeps the node's identity inside its Dir. Its
// presence means the node has enrolled before.
const stateFile = "tailscaled.state"

// NodeConfig is how the serve node joins the tailnet.
type NodeConfig struct {
	Hostname   string
	StateDir   string
	ControlURL string
	AuthKey    string // set only when Enrolling
	Enrolling  bool   // no node state yet: this start enrols with AuthKey
}

// NewNodeConfig builds the node's settings from host.mesh. The auth key file
// is read only when state_dir holds no node state, so after the first
// enrolment it may be deleted (AC2).
func NewNodeConfig(mesh host.FileMesh, controlURL string) (NodeConfig, error) {
	if mesh.StateDir == nil {
		return NodeConfig{}, errors.New("host.mesh.state_dir: required; serve keeps the node identity there")
	}
	nc := NodeConfig{StateDir: *mesh.StateDir, ControlURL: controlURL}
	if mesh.Hostname != nil {
		nc.Hostname = *mesh.Hostname
	} else if name, err := os.Hostname(); err == nil {
		nc.Hostname = strings.ToLower(name)
	}
	_, err := os.Stat(filepath.Join(nc.StateDir, stateFile))
	switch {
	case err == nil:
		return nc, nil
	case !errors.Is(err, fs.ErrNotExist):
		return NodeConfig{}, fmt.Errorf("host.mesh.state_dir: read node state: %w", err)
	}
	nc.Enrolling = true
	if mesh.AuthKeyFile == nil {
		return NodeConfig{}, fmt.Errorf("host.mesh.auth_key_file: required for the first enrolment (no node state in %s)", nc.StateDir)
	}
	key, err := readAuthKey(*mesh.AuthKeyFile)
	if err != nil {
		return NodeConfig{}, err
	}
	nc.AuthKey = key
	return nc, nil
}

func readAuthKey(path string) (string, error) {
	// #nosec G304 -- path is the operator's declared host.mesh.auth_key_file.
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("host.mesh.auth_key_file: %w", err)
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("host.mesh.auth_key_file %s: empty", path)
	}
	return key, nil
}

// Server returns the persistent tsnet node for this config. logf nil discards
// tsnet's logs.
func (nc NodeConfig) Server(logf func(string, ...any)) *tsnet.Server {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &tsnet.Server{
		Hostname:   nc.Hostname,
		Dir:        nc.StateDir,
		ControlURL: nc.ControlURL,
		AuthKey:    nc.AuthKey,
		Ephemeral:  false,
		Logf:       logf,
	}
}
