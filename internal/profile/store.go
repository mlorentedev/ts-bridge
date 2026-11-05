package profile

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"ts-bridge/internal/credential"
)

// profileEntry is one named profile in the YAML store.
// Follows the #185-shaped schema so CFG-002 and the future full profiles
// model can coexist without migration.
type profileEntry struct {
	Target     string `yaml:"target"`
	ControlURL string `yaml:"control_url,omitempty"`
	Credential string `yaml:"credential,omitempty"`
	Runtime    `yaml:",inline"`
}

// storeFile is the on-disk YAML shape.
type storeFile struct {
	DescriptorVersion int                     `yaml:"descriptor_version,omitempty"`
	Profiles          map[string]profileEntry `yaml:"profiles"`
}

// Profile is the resolved, caller-facing view of a named profile.
type Profile struct {
	Target     string `yaml:"target"`
	ControlURL string `yaml:"control_url,omitempty"`
	Credential string `yaml:"credential,omitempty"`
	Runtime    `yaml:",inline"`
}

// Runtime holds non-secret, machine-local connection settings.
type Runtime struct {
	BootstrapSSH       string `yaml:"bootstrap_ssh,omitempty"`
	BootstrapSOCKSAddr string `yaml:"bootstrap_socks_addr,omitempty"`
	Hostname           string `yaml:"hostname,omitempty"`
	StateDir           string `yaml:"state_dir,omitempty"`
	LocalAddr          string `yaml:"local_addr,omitempty"`
	HealthAddr         string `yaml:"health_addr,omitempty"`
}

// Store is a persistent, file-backed named-profile store.
type Store struct {
	path string
}

// NewStore returns a Store backed by the given YAML file path.
// The file need not exist yet; it is created on the first Import.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// ControlPlaneChangeError reports an update that would move a credentialed
// profile to another control plane: the managed credential was issued for the
// old one and must not follow the profile to the new one (#617).
type ControlPlaneChangeError struct {
	Name       string
	From, To   string
	Credential string
}

func (e *ControlPlaneChangeError) Error() string {
	return fmt.Sprintf("profile %q has managed credential %q bound to control URL %q; refusing to move it to %q (use --force to drop the credential reference)",
		e.Name, e.Credential, displayControlURL(e.From), displayControlURL(e.To))
}

// displayControlURL names the empty control URL for what it means.
func displayControlURL(raw string) string {
	if raw == "" {
		return "(Tailscale SaaS)"
	}
	return raw
}

// bindCredential returns existing's credential for a profile that will use
// controlURL. A credential survives only on the control plane it was issued
// for: a change refuses unless force is set, which clears the reference.
// cleared reports that the reference was dropped.
func bindCredential(name string, existing profileEntry, controlURL string, force bool) (credentialName string, cleared bool, err error) {
	if existing.Credential == "" || SameControlURL(existing.ControlURL, controlURL) {
		return existing.Credential, false, nil
	}
	if !force {
		return "", false, &ControlPlaneChangeError{Name: name, From: existing.ControlURL, To: controlURL, Credential: existing.Credential}
	}
	return "", true, nil
}

// Import writes (or updates) a named profile derived from d.
// Re-importing the same descriptor is idempotent — the file ends up with
// exactly one entry per name regardless of how many times it is called.
// Changing the control URL of a credentialed profile refuses unless force is
// set; force clears the credential reference and reports it through cleared.
func (s *Store) Import(name string, d Descriptor, force bool) (cleared bool, err error) {
	data, err := s.load()
	if err != nil {
		return false, err
	}

	existing := data.Profiles[name]
	credentialName, cleared, err := bindCredential(name, existing, d.ControlURL, force)
	if err != nil {
		return false, err
	}
	data.Profiles[name] = profileEntry{
		Target:     fmt.Sprintf("%s:%d", d.Host, d.Port),
		ControlURL: d.ControlURL,
		Credential: credentialName,
		Runtime:    existing.Runtime,
	}
	data.DescriptorVersion = 1

	if err := s.save(data); err != nil {
		return false, err
	}
	return cleared, nil
}

// secretPrefixes holds auth-key prefixes that must never appear in a profile target.
var secretPrefixes = []string{"tskey-", "hskey-"}

// Set writes (or updates) a named profile from raw parameters.
// Use this when no tsb:// descriptor is available (e.g. ts-bridge init --profile).
// Returns an error if name is empty or if target contains a known secret prefix.
func (s *Store) Set(name, target, controlURL string) error {
	_, err := s.set(name, target, controlURL, nil, false)
	return err
}

// Configure replaces connection settings while preserving the managed credential
// on the control plane it was issued for. Moving a credentialed profile to
// another control URL refuses unless force is set, which clears the reference.
func (s *Store) Configure(name string, selected Profile, force bool) (cleared bool, err error) {
	for _, value := range []string{
		selected.ControlURL,
		selected.BootstrapSSH, selected.BootstrapSOCKSAddr, selected.Hostname,
		selected.StateDir, selected.LocalAddr, selected.HealthAddr,
	} {
		for _, prefix := range secretPrefixes {
			if strings.Contains(value, prefix) {
				return false, fmt.Errorf("profile configuration must not contain auth key material")
			}
		}
	}
	return s.set(name, selected.Target, selected.ControlURL, &selected.Runtime, force)
}

func (s *Store) set(name, target, controlURL string, settings *Runtime, force bool) (bool, error) {
	if strings.TrimSpace(name) == "" {
		return false, fmt.Errorf("profile name must not be empty")
	}
	for _, pfx := range secretPrefixes {
		if strings.Contains(target, pfx) {
			return false, fmt.Errorf("profile target must not contain auth key material (found %q prefix)", pfx)
		}
	}

	data, err := s.load()
	if err != nil {
		return false, fmt.Errorf("load profile store: %w", err)
	}
	existing := data.Profiles[name]
	credentialName, cleared, err := bindCredential(name, existing, controlURL, force)
	if err != nil {
		return false, err
	}
	if settings != nil {
		existing.Runtime = *settings
	}
	data.Profiles[name] = profileEntry{
		Target:     target,
		ControlURL: controlURL,
		Credential: credentialName,
		Runtime:    existing.Runtime,
	}
	data.DescriptorVersion = 1
	if err := s.save(data); err != nil {
		return false, fmt.Errorf("save profile store: %w", err)
	}
	return cleared, nil
}

// Get returns the named profile or an error if it does not exist.
func (s *Store) Get(name string) (Profile, error) {
	data, err := s.load()
	if err != nil {
		return Profile{}, err
	}
	e, ok := data.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q not found in %s", name, s.path)
	}
	return profileFromEntry(e), nil
}

// List returns all profile names in the store.
func (s *Store) List() ([]string, error) {
	data, err := s.load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(data.Profiles))
	for n := range data.Profiles {
		names = append(names, n)
	}
	return names, nil
}

// ListProfiles returns every profile keyed by its local name.
func (s *Store) ListProfiles() (map[string]Profile, error) {
	data, err := s.load()
	if err != nil {
		return nil, err
	}
	result := make(map[string]Profile, len(data.Profiles))
	for name, entry := range data.Profiles {
		result[name] = profileFromEntry(entry)
	}
	return result, nil
}

// SetCredential associates a managed credential name with an existing profile.
func (s *Store) SetCredential(profileName, credentialName string) error {
	if err := credential.ValidateName(credentialName); err != nil {
		return fmt.Errorf("invalid credential reference: %w", err)
	}
	return s.updateCredential(profileName, credentialName)
}

// ClearCredential removes the managed credential reference from a profile.
func (s *Store) ClearCredential(profileName string) error {
	return s.updateCredential(profileName, "")
}

func (s *Store) updateCredential(profileName, credentialName string) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	entry, ok := data.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile %q not found in %s", profileName, s.path)
	}
	entry.Credential = credentialName
	data.Profiles[profileName] = entry
	return s.save(data)
}

func profileFromEntry(entry profileEntry) Profile {
	return Profile{
		Target:     entry.Target,
		ControlURL: entry.ControlURL,
		Credential: entry.Credential,
		Runtime:    entry.Runtime,
	}
}

func (s *Store) load() (storeFile, error) {
	f := storeFile{Profiles: make(map[string]profileEntry)}

	// #nosec G304 -- path is supplied by the caller (config or default); not user-controlled input.
	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return f, nil
	}
	if err != nil {
		return f, fmt.Errorf("read profile store %s: %w", s.path, err)
	}

	if err := yaml.Unmarshal(raw, &f); err != nil {
		return f, fmt.Errorf("parse profile store %s: %w", s.path, err)
	}
	if f.Profiles == nil {
		f.Profiles = make(map[string]profileEntry)
	}
	return f, nil
}

func (s *Store) save(data storeFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("create profile store directory: %w", err)
	}

	raw, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal profile store: %w", err)
	}

	// #nosec G306 -- profile store is user-readable config, not secrets.
	if err := os.WriteFile(s.path, raw, 0600); err != nil {
		return fmt.Errorf("write profile store %s: %w", s.path, err)
	}
	return nil
}

// SameControlURL reports whether two control URLs name one control plane.
// Scheme and host are case-insensitive and surrounding space and a trailing
// slash do not matter; the path is compared exactly. Both empty means the
// default Tailscale SaaS control plane.
func SameControlURL(a, b string) bool {
	return normalizeControlURL(a) == normalizeControlURL(b)
}

func normalizeControlURL(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	u.Scheme, u.Host = strings.ToLower(u.Scheme), strings.ToLower(u.Host)
	return u.String()
}
