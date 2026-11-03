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
	Enrolling  bool   // no node state yet: this start enrolls with AuthKey
	// Logf receives tsnet's logs; nil discards them.
	Logf func(format string, args ...any)
}

// NewNodeConfig builds the node's settings from host.mesh. The auth key file
// is read only when state_dir holds no node state, so after the first
// enrolment it may be deleted (AC2). checkKey refuses a key file others may
// read before it is opened (host.CheckSecretFile in production, AC5).
func NewNodeConfig(mesh host.FileMesh, controlURL string, checkKey func(string) error) (NodeConfig, error) {
	if mesh.StateDir == nil {
		return NodeConfig{}, errors.New("host.mesh.state_dir: required; serve keeps the node identity there")
	}
	nc := NodeConfig{StateDir: *mesh.StateDir, ControlURL: controlURL, Hostname: NodeHostname(mesh)}
	_, err := os.Stat(filepath.Join(nc.StateDir, stateFile))
	switch {
	case err == nil:
		// Enrolled: the key is not read, but a key file left behind is still a
		// live credential, so it must not be readable by others either.
		return nc, checkLeftoverKey(mesh.AuthKeyFile, checkKey)
	case !errors.Is(err, fs.ErrNotExist):
		return NodeConfig{}, fmt.Errorf("host.mesh.state_dir: read node state: %w", err)
	}
	nc.Enrolling = true
	if mesh.AuthKeyFile == nil {
		return NodeConfig{}, fmt.Errorf("host.mesh.auth_key_file: required for the first enrolment (no node state in %s)", nc.StateDir)
	}
	if err := checkKey(*mesh.AuthKeyFile); err != nil {
		return NodeConfig{}, err
	}
	key, err := readAuthKey(*mesh.AuthKeyFile)
	if err != nil {
		return NodeConfig{}, err
	}
	nc.AuthKey = key
	return nc, nil
}

// checkLeftoverKey runs checkKey on a declared key file that still exists after
// enrolment; a deleted one, as the runbook asks for, is fine.
func checkLeftoverKey(path *string, checkKey func(string) error) error {
	if path == nil {
		return nil
	}
	if _, err := os.Lstat(*path); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return checkKey(*path)
}

// NodeHostname is the node's name: host.mesh.hostname, else the OS hostname in
// lower case. host check compares serve's health answer against it.
func NodeHostname(mesh host.FileMesh) string {
	if mesh.Hostname != nil {
		return *mesh.Hostname
	}
	name, _ := os.Hostname()
	return strings.ToLower(name)
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

// Server returns the persistent tsnet node for this config, logging through
// nc.Logf (nil discards tsnet's logs).
func (nc NodeConfig) Server() *tsnet.Server {
	logf := nc.Logf
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
