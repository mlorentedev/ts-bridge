package host

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

// fileVersion is the host configuration schema version this build reads.
const fileVersion = 1

// Service kinds a host may declare besides ServiceRDP (ADR-017 §1).
const (
	ServiceSSH = "ssh"
	ServiceVNC = "vnc"
)

// GOOS values the capability table and the step lists key on.
const (
	goosWindows = "windows"
	goosLinux   = "linux"
	goosDarwin  = "darwin"
)

// defaultPorts is the port each service kind listens on when the file omits it.
var defaultPorts = map[string]int{ServiceRDP: defaultRDPPort, ServiceSSH: 22, ServiceVNC: 5900}

// File is the host configuration file: the host's declared state (ADR-017 §1).
// Pointers and a nil Services slice tell an absent key from an empty one.
type File struct {
	Version int      `yaml:"version"`
	Host    FileHost `yaml:"host"`
}

// FileHost is the host section of the configuration file.
type FileHost struct {
	Services   []FileService `yaml:"services"`
	Firewall   *FileFirewall `yaml:"firewall"`
	Power      *FilePower    `yaml:"power"`
	ControlURL *string       `yaml:"control_url"`
}

// FileService is one declared remote-access service. A nil Port is the
// kind's default port.
type FileService struct {
	Kind string `yaml:"kind"`
	Port *int   `yaml:"port"`
}

// FileFirewall is the firewall section of the configuration file.
type FileFirewall struct {
	Rule *string `yaml:"rule"`
}

// FilePower is the power section of the configuration file.
type FilePower struct {
	PreventSleep *bool `yaml:"prevent_sleep"`
}

// osCapability is what one OS's steps can converge in this version. Each
// per-OS ticket widens its row (ADR-017 §5); ticket names that work in errors.
type osCapability struct {
	kinds  []string
	power  bool
	ticket string
}

var osCapabilities = map[string]osCapability{
	goosWindows: {kinds: []string{ServiceRDP}, power: true, ticket: "#455"},
	goosLinux:   {kinds: []string{ServiceRDP}, ticket: "#454"},
	goosDarwin:  {ticket: "#459"},
}

// capabilityFor returns goos's row. Any other OS runs the Linux steps (setupFor).
func capabilityFor(goos string) osCapability {
	if c, ok := osCapabilities[goos]; ok {
		return c
	}
	return osCapabilities[goosLinux]
}

// LoadFile reads the host configuration at path and validates it for goos. It
// writes one warning to warn for each unknown key, which it ignores.
func LoadFile(path, goos string, warn io.Writer) (File, error) {
	// #nosec G304 -- path is the operator's explicit --config.
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read host config: %w", err)
	}
	f, err := decodeFile(data, warn)
	if err == nil {
		err = f.validate(goos)
	}
	if err != nil {
		return File{}, fmt.Errorf("host config %s: %w", path, err)
	}
	return f, nil
}

func decodeFile(data []byte, warn io.Writer) (File, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return File{}, fmt.Errorf("parse YAML: %w", err)
	}
	f := File{Version: fileVersion}
	if len(root.Content) == 0 {
		return f, nil
	}
	doc := root.Content[0]
	warnUnknownKeys(doc, warn)
	if err := doc.Decode(&f); err != nil {
		return File{}, fmt.Errorf("decode: %w", err)
	}
	return f, nil
}

func (f File) validate(goos string) error {
	if f.Version != fileVersion {
		return fmt.Errorf("version %d: this ts-bridge reads version %d", f.Version, fileVersion)
	}
	capab := capabilityFor(goos)
	if err := validateServices(f.Host.Services, goos, capab); err != nil {
		return err
	}
	if fw := f.Host.Firewall; fw != nil && fw.Rule != nil {
		if _, err := sanitizeFirewallRule(*fw.Rule); err != nil {
			return fmt.Errorf("host.firewall.rule: %w", err)
		}
	}
	if f.Host.Power != nil && !capab.power {
		return fmt.Errorf("host.power: %s has no power step (ADR-017 §5)", goos)
	}
	return validateControlURL(f.Host.ControlURL)
}

func validateServices(services []FileService, goos string, capab osCapability) error {
	if services == nil {
		return nil
	}
	if len(services) == 0 {
		return errors.New("host.services: empty; omit it to declare the OS default")
	}
	seen := make(map[string]bool, len(services))
	for i, s := range services {
		field := fmt.Sprintf("host.services[%d]", i)
		if err := validateService(field, s, goos, capab); err != nil {
			return err
		}
		if seen[s.Kind] {
			return fmt.Errorf("%s.kind %q: declared twice", field, s.Kind)
		}
		seen[s.Kind] = true
	}
	return nil
}

func validateService(field string, s FileService, goos string, capab osCapability) error {
	if _, ok := defaultPorts[s.Kind]; !ok {
		return fmt.Errorf("%s.kind %q: must be one of rdp, ssh, vnc", field, s.Kind)
	}
	if !slices.Contains(capab.kinds, s.Kind) {
		return fmt.Errorf("%s.kind %q: %s has no %s step in this version (%s)", field, s.Kind, goos, s.Kind, capab.ticket)
	}
	if s.Port != nil && (*s.Port < 1 || *s.Port > 65535) {
		return fmt.Errorf("%s.port %d: must be between 1 and 65535", field, *s.Port)
	}
	return nil
}

// validateControlURL accepts an absent control_url (detect) and rejects an
// empty one, so unset and empty cannot be confused (ADR-017 §4, #282).
func validateControlURL(raw *string) error {
	if raw == nil {
		return nil
	}
	if *raw == "" {
		return errors.New("host.control_url: empty; omit it to use the detected control plane")
	}
	u, err := url.Parse(*raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("host.control_url %q: must be an http or https URL with a host", *raw)
	}
	return nil
}

// Known keys per level of the file. Anything else is warned about and ignored.
var (
	topKeys      = []string{"version", "host"}
	hostKeys     = []string{"services", "firewall", "power", "control_url"}
	serviceKeys  = []string{"kind", "port"}
	firewallKeys = []string{"rule"}
	powerKeys    = []string{"prevent_sleep"}
)

func warnUnknownKeys(doc *yaml.Node, warn io.Writer) {
	warnMapping(doc, "", topKeys, warn)
	hostNode := mappingValue(doc, "host")
	warnMapping(hostNode, "host", hostKeys, warn)
	warnMapping(mappingValue(hostNode, "firewall"), "host.firewall", firewallKeys, warn)
	warnMapping(mappingValue(hostNode, "power"), "host.power", powerKeys, warn)
	if svcs := mappingValue(hostNode, "services"); svcs != nil && svcs.Kind == yaml.SequenceNode {
		for i, s := range svcs.Content {
			warnMapping(s, fmt.Sprintf("host.services[%d]", i), serviceKeys, warn)
		}
	}
}

// warnMapping warns about each key of mapping n outside known. A nil or
// non-mapping n is left for the decoder to reject.
func warnMapping(n *yaml.Node, prefix string, known []string, warn io.Writer) {
	if n == nil || n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		if slices.Contains(known, key) {
			continue
		}
		if prefix != "" {
			key = prefix + "." + key
		}
		fmt.Fprintf(warn, "WARNING: unknown host config key %q ignored\n", key)
	}
}

func mappingValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
