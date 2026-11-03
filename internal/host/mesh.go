package host

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"ts-bridge/internal/bootstrapssh"
)

// host.mesh keys named in more than one place.
const (
	keyStateDir    = "state_dir"
	keyAuthKeyFile = "auth_key_file"
)

// FileMesh is the host.mesh section: what host serve alone needs to join the
// tailnet as a persistent node (HOST-008). Pointers tell absent from empty.
type FileMesh struct {
	BootstrapSSH       *string `yaml:"bootstrap_ssh"`
	BootstrapSOCKSAddr *string `yaml:"bootstrap_socks_addr"`
	Hostname           *string `yaml:"hostname"`
	StateDir           *string `yaml:"state_dir"`
	AuthKeyFile        *string `yaml:"auth_key_file"`
	HealthAddr         *string `yaml:"health_addr"`
}

// hostnameLabel is one DNS label: what the control plane accepts as a node name.
var hostnameLabel = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

// meshField is one string key of host.mesh.
type meshField struct {
	key   string
	value *string
}

func validateMesh(m *FileMesh, hasControlURL bool, goos string, capab osCapability) error {
	if m == nil {
		return nil
	}
	if !capab.serve {
		return fmt.Errorf("host.mesh: %s has no host serve in this version (#459)", goos)
	}
	if err := checkMeshEmpty(m); err != nil {
		return err
	}
	if err := validateBootstrapSSH(m.BootstrapSSH, hasControlURL); err != nil {
		return err
	}
	if m.Hostname != nil && !hostnameLabel.MatchString(*m.Hostname) {
		return errors.New("host.mesh.hostname: must be 1-63 letters, digits or inner hyphens")
	}
	if err := checkMeshPaths(m, goos); err != nil {
		return err
	}
	if err := validateLoopbackAddr("bootstrap_socks_addr", m.BootstrapSOCKSAddr); err != nil {
		return err
	}
	return validateLoopbackAddr("health_addr", m.HealthAddr)
}

// checkMeshEmpty rejects an empty key, so it cannot pass for an absent one.
func checkMeshEmpty(m *FileMesh) error {
	for _, f := range []meshField{
		{"bootstrap_ssh", m.BootstrapSSH}, {"bootstrap_socks_addr", m.BootstrapSOCKSAddr}, {"hostname", m.Hostname},
		{keyStateDir, m.StateDir}, {keyAuthKeyFile, m.AuthKeyFile}, {"health_addr", m.HealthAddr},
	} {
		if f.value != nil && *f.value == "" {
			return fmt.Errorf("host.mesh.%s: empty; omit it or give it a value", f.key)
		}
	}
	return nil
}

func checkMeshPaths(m *FileMesh, goos string) error {
	for _, p := range []meshField{{keyStateDir, m.StateDir}, {keyAuthKeyFile, m.AuthKeyFile}} {
		if p.value != nil && !isAbsFor(goos, *p.value) {
			return fmt.Errorf("host.mesh.%s: must be an absolute path on %s", p.key, goos)
		}
	}
	return nil
}

func validateBootstrapSSH(endpoint *string, hasControlURL bool) error {
	if endpoint == nil {
		return nil
	}
	if err := bootstrapssh.ValidateEndpoint(*endpoint); err != nil {
		return fmt.Errorf("host.mesh.bootstrap_ssh: %w", err)
	}
	if !hasControlURL {
		return errors.New("host.mesh.bootstrap_ssh needs host.control_url: the bootstrap carries that control plane")
	}
	return nil
}

// validateLoopbackAddr accepts a loopback host:port only: the bootstrap SOCKS
// listener and the health endpoint must not be reachable from the network.
func validateLoopbackAddr(key string, addr *string) error {
	if addr == nil {
		return nil
	}
	h, p, err := net.SplitHostPort(*addr)
	if err != nil {
		return fmt.Errorf("host.mesh.%s: %w", key, err)
	}
	if ip := net.ParseIP(h); h != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("host.mesh.%s: must be a loopback address (127.0.0.1, [::1] or localhost)", key)
	}
	if n, err := strconv.Atoi(p); err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("host.mesh.%s: port must be between 1 and 65535", key)
	}
	return nil
}

// isAbsFor reports whether p is absolute on goos, which need not be the OS
// running the check: a drive or UNC path on Windows, a rooted path elsewhere.
func isAbsFor(goos, p string) bool {
	if goos != goosWindows {
		return strings.HasPrefix(p, "/")
	}
	if strings.HasPrefix(p, `\\`) {
		return true
	}
	return len(p) >= 3 && isASCIILetter(p[0]) && p[1] == ':' && (p[2] == '\\' || p[2] == '/')
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
