package host

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
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
	kinds        []string
	power        bool
	firewallRule bool // a named firewall rule; only the Windows step uses one
	ticket       string
}

var osCapabilities = map[string]osCapability{
	goosWindows: {kinds: []string{ServiceRDP}, power: true, firewallRule: true, ticket: "#455"},
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
// writes one warning to warn (nil discards them) for each unknown key, which
// it ignores.
func LoadFile(path, goos string, warn io.Writer) (File, error) {
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
		err = f.validate(goos)
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
	warnUnknownKeys(&doc, warn)
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
	if err := validateFirewall(f.Host.Firewall, goos, capab); err != nil {
		return err
	}
	if f.Host.Power != nil && !capab.power {
		return fmt.Errorf("host.power: %s has no power step (ADR-017 §5)", goos)
	}
	return validateControlURL(f.Host.ControlURL)
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
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("host.control_url %q: must be an http or https URL with a host", *raw)
	}
	if u.User != nil {
		// The URL is not echoed: it carries a secret.
		return errors.New("host.control_url: must not carry credentials; the host needs none")
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

// warnUnknownKeys walks the document as the decoder resolves it, so keys
// reached through an alias or a merge key are checked and a merge key itself
// is not reported as ignored.
func warnUnknownKeys(doc *yaml.Node, warn io.Writer) {
	var raw map[string]any
	if doc.Decode(&raw) != nil {
		return // the struct decode reports what is wrong
	}
	warnKeys(raw, "", topKeys, warn)
	hostMap := asMap(raw["host"])
	warnKeys(hostMap, "host", hostKeys, warn)
	warnKeys(asMap(hostMap["firewall"]), "host.firewall", firewallKeys, warn)
	warnKeys(asMap(hostMap["power"]), "host.power", powerKeys, warn)
	services, _ := hostMap["services"].([]any)
	for i, s := range services {
		warnKeys(asMap(s), fmt.Sprintf("host.services[%d]", i), serviceKeys, warn)
	}
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// warnKeys warns, in key order, about each key of m outside known.
func warnKeys(m map[string]any, prefix string, known []string, warn io.Writer) {
	for _, key := range slices.Sorted(maps.Keys(m)) {
		if slices.Contains(known, key) {
			continue
		}
		if prefix != "" {
			key = prefix + "." + key
		}
		fmt.Fprintf(warn, "WARNING: unknown host config key %q ignored\n", key)
	}
}
