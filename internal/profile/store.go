package profile

import (
	"fmt"
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
}

// storeFile is the on-disk YAML shape.
type storeFile struct {
	DescriptorVersion int                     `yaml:"descriptor_version,omitempty"`
	Profiles          map[string]profileEntry `yaml:"profiles"`
}

// Profile is the resolved, caller-facing view of a named profile.
type Profile struct {
	Target     string
	ControlURL string
	Credential string
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

// Import writes (or updates) a named profile derived from d.
// Re-importing the same descriptor is idempotent — the file ends up with
// exactly one entry per name regardless of how many times it is called.
func (s *Store) Import(name string, d Descriptor) error {
	data, err := s.load()
	if err != nil {
		return err
	}

	target := fmt.Sprintf("%s:%d", d.Host, d.Port)
	existing := data.Profiles[name]
	data.Profiles[name] = profileEntry{
		Target:     target,
		ControlURL: d.ControlURL,
		Credential: existing.Credential,
	}
	data.DescriptorVersion = 1

	return s.save(data)
}

// secretPrefixes holds auth-key prefixes that must never appear in a profile target.
var secretPrefixes = []string{"tskey-", "hskey-"}

// Set writes (or updates) a named profile from raw parameters.
// Use this when no tsb:// descriptor is available (e.g. ts-bridge init --profile).
// Returns an error if name is empty or if target contains a known secret prefix.
func (s *Store) Set(name, target, controlURL string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("profile name must not be empty")
	}
	for _, pfx := range secretPrefixes {
		if strings.Contains(target, pfx) {
			return fmt.Errorf("profile target must not contain auth key material (found %q prefix)", pfx)
		}
	}

	data, err := s.load()
	if err != nil {
		return fmt.Errorf("load profile store: %w", err)
	}
	existing := data.Profiles[name]
	data.Profiles[name] = profileEntry{
		Target:     target,
		ControlURL: controlURL,
		Credential: existing.Credential,
	}
	data.DescriptorVersion = 1
	if err := s.save(data); err != nil {
		return fmt.Errorf("save profile store: %w", err)
	}
	return nil
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
