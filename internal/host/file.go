package host

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"

	"gopkg.in/yaml.v3"

	"ts-bridge/internal/controlurl"
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
	Services   []FileService `yaml:"services,omitempty"`
	Firewall   *FileFirewall `yaml:"firewall,omitempty"`
	Power      *FilePower    `yaml:"power,omitempty"`
	ControlURL *string       `yaml:"control_url,omitempty"`
	Mesh       *FileMesh     `yaml:"mesh,omitempty"`
}

// FileService is one declared remote-access service. A nil Port is the
// kind's default port.
type FileService struct {
	Kind string `yaml:"kind"`
	Port *int   `yaml:"port,omitempty"`
}

// FileFirewall is the firewall section of the configuration file.
type FileFirewall struct {
	Rule *string `yaml:"rule,omitempty"`
}

// FilePower is the power section of the configuration file.
type FilePower struct {
	PreventSleep *bool `yaml:"prevent_sleep,omitempty"`
}

// osCapability is what one OS's steps can converge in this version. Each
// per-OS ticket widens its row (ADR-017 §5).
type osCapability struct {
	kinds        []string
	power        bool
	firewallRule bool              // a named firewall rule; only the Windows step uses one
	serve        bool              // host serve runs here (HOST-008)
	planned      map[string]string // kind -> the ticket adding its step, where one exists
}

var osCapabilities = map[string]osCapability{
	goosWindows: {kinds: []string{ServiceRDP}, power: true, firewallRule: true, serve: true, planned: map[string]string{ServiceSSH: "#521"}},
	goosLinux:   {kinds: []string{ServiceRDP}, serve: true, planned: map[string]string{ServiceSSH: "#521"}},
	goosDarwin:  {planned: map[string]string{ServiceSSH: "#459", ServiceVNC: "#459"}},
}

// capabilityFor returns goos's row. Any other OS runs the Linux steps (setupFor).
func capabilityFor(goos string) osCapability {
	if c, ok := osCapabilities[goos]; ok {
		return c
	}
	return osCapabilities[goosLinux]
}

// LoadFile reads the host configuration at path and validates it for goos. It
// writes one warning to warn (nil discards them) for each unknown key, which
// it ignores.
func LoadFile(path, goos string, warn io.Writer) (File, error) {
	return loadFile(path, goos, warn, capabilityFor(goos))
}

// LoadServeFile reads a host file for publication, allowing an existing SSH
// service on OSes that support serve without claiming an SSH setup/check step.
func LoadServeFile(path, goos string, warn io.Writer) (File, error) {
	capab := capabilityFor(goos)
	if capab.serve {
		capab.kinds = append(slices.Clone(capab.kinds), ServiceSSH)
	}
	return loadFile(path, goos, warn, capab)
}

func loadFile(path, goos string, warn io.Writer, capab osCapability) (File, error) {
	if warn == nil {
		warn = io.Discard
	}
	// #nosec G304 -- path is the operator's explicit --config.
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read host config: %w", err)
	}
	f, err := decodeFile(data, warn)
	if err == nil {
		err = f.validate(goos, capab)
	}
	if err != nil {
		return File{}, fmt.Errorf("host config %s: %w", path, err)
	}
	return f, nil
}

// decodeFile decodes the single YAML document in data. A second document is
// an error rather than silently dropped.
func decodeFile(data []byte, warn io.Writer) (File, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	f := File{Version: fileVersion}
	var doc yaml.Node
	switch err := dec.Decode(&doc); {
	case errors.Is(err, io.EOF):
		return f, nil
	case err != nil:
		return File{}, fmt.Errorf("parse YAML: %w", err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return File{}, errors.New("more than one YAML document; declare the host in a single document")
	}
	if err := checkKeys(&doc, warn); err != nil {
		return File{}, err
	}
	if err := doc.Decode(&f); err != nil {
		return File{}, fmt.Errorf("decode: %w", err)
	}
	return f, nil
}

func (f File) validate(goos string, capab osCapability) error {
	if f.Version != fileVersion {
		return fmt.Errorf("version %d: this ts-bridge reads version %d", f.Version, fileVersion)
	}
	if err := validateServices(f.Host.Services, goos, capab); err != nil {
		return err
	}
	if err := validateFirewall(f.Host.Firewall, goos, capab); err != nil {
		return err
	}
	if f.Host.Power != nil && !capab.power {
		return fmt.Errorf("host.power: %s has no power step (ADR-017 §5)", goos)
	}
	if err := validateControlURL(f.Host.ControlURL); err != nil {
		return err
	}
	return validateMesh(f.Host.Mesh, f.Host.ControlURL != nil, goos, capab)
}

func validateFirewall(fw *FileFirewall, goos string, capab osCapability) error {
	if fw == nil {
		return nil
	}
	if !capab.firewallRule {
		return fmt.Errorf("host.firewall: %s has no named firewall rule (ADR-017 §5)", goos)
	}
	if fw.Rule == nil {
		return nil
	}
	if _, err := sanitizeFirewallRule(*fw.Rule); err != nil {
		return fmt.Errorf("host.firewall.rule: %w", err)
	}
	return nil
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
		ref, ok := capab.planned[s.Kind]
		if !ok {
			ref = "ADR-017 §5"
		}
		return fmt.Errorf("%s.kind %q: %s has no %s step in this version (%s)", field, s.Kind, goos, s.Kind, ref)
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
	if err := controlurl.Validate(*raw); err != nil {
		return fmt.Errorf("host.control_url: %w", err)
	}
	return nil
}

// Known keys per level of the file. Anything else is warned about and ignored.
var (
	topKeys      = []string{"version", "host"}
	hostKeys     = []string{"services", "firewall", "power", "control_url", "mesh"}
	serviceKeys  = []string{"kind", keyPort}
	firewallKeys = []string{"rule"}
	powerKeys    = []string{"prevent_sleep"}
	meshKeys     = []string{"bootstrap_ssh", "bootstrap_socks_addr", "hostname", keyStateDir, keyAuthKeyFile, "health_addr"}
)

// checkKeys walks the document as the decoder resolves it, so keys reached
// through an alias or a merge key are checked and a merge key itself is not
// reported as ignored. Unknown keys are warned about; a known key with no
// value is an error, so it cannot pass for an absent one (ADR-017 §4, #282).
func checkKeys(doc *yaml.Node, warn io.Writer) error {
	var raw map[string]any
	if err := doc.Decode(&raw); err != nil {
		// Failing here must not skip the checks below while the struct decode,
		// which ignores unknown subtrees, still succeeds.
		return fmt.Errorf("decode: %w", err)
	}
	hostMap := asMap(raw["host"])
	levels := []keyLevel{
		{raw, "", topKeys},
		{hostMap, "host", hostKeys},
		{asMap(hostMap["firewall"]), "host.firewall", firewallKeys},
		{asMap(hostMap["power"]), "host.power", powerKeys},
		{asMap(hostMap["mesh"]), "host.mesh", meshKeys},
	}
	services, _ := hostMap["services"].([]any)
	for i, s := range services {
		levels = append(levels, keyLevel{asMap(s), fmt.Sprintf("host.services[%d]", i), serviceKeys})
	}
	for _, l := range levels {
		if err := l.check(warn); err != nil {
			return err
		}
	}
	return nil
}

// keyLevel is one mapping of the file and the keys it may hold.
type keyLevel struct {
	m      map[string]any
	prefix string
	known  []string
}

func (l keyLevel) check(warn io.Writer) error {
	for _, key := range slices.Sorted(maps.Keys(l.m)) {
		path := key
		if l.prefix != "" {
			path = l.prefix + "." + key
		}
		if !slices.Contains(l.known, key) {
			fmt.Fprintf(warn, "WARNING: unknown host config key %q ignored\n", path)
			continue
		}
		if l.m[key] == nil {
			return fmt.Errorf("%s: no value; omit the key or give it one", path)
		}
	}
	return nil
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
