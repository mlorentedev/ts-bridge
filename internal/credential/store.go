// Package credential manages local, profile-scoped auth keys.
package credential

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrCredentialExists = errors.New("credential already exists")

type Entry struct {
	Name    string
	Updated time.Time
}

type Store struct {
	dir    string
	rename func(string, string) error
}

func NewStore(dir string) *Store {
	return &Store{dir: dir, rename: os.Rename}
}

func (s *Store) path(name string) string {
	return filepath.Join(s.dir, name+".key")
}

func ValidateName(name string) error {
	if name == "" || name[0] == '.' || len(name) > 64 {
		return fmt.Errorf("credential name must be 1-64 letters, digits, dots, dashes, or underscores")
	}
	for _, r := range name {
		if !isNameRune(r) {
			return fmt.Errorf("credential name must be 1-64 letters, digits, dots, dashes, or underscores")
		}
	}
	return nil
}

func isNameRune(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' ||
		r == '.' || r == '-' || r == '_'
}

func (s *Store) Set(name, rawKey string, replace bool) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	key, err := normalizeKey(rawKey)
	if err != nil {
		return err
	}
	if err := s.ensureDir(); err != nil {
		return err
	}
	path := s.path(name)
	if !replace {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%w: %q", ErrCredentialExists, name)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat credential %q: %w", name, err)
		}
	}
	return s.writeAtomic(path, name, key)
}

func (s *Store) Get(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	// #nosec G304 -- validated name is joined beneath the configured credential directory.
	raw, err := os.ReadFile(s.path(name))
	if err != nil {
		return "", fmt.Errorf("read credential %q: %w", name, err)
	}
	key, err := normalizeKey(string(raw))
	if err != nil {
		return "", fmt.Errorf("stored credential %q is invalid: %w", name, err)
	}
	return key, nil
}

func (s *Store) List() ([]Entry, error) {
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list credential store: %w", err)
	}
	result := make([]Entry, 0, len(entries))
	for _, item := range entries {
		if item.IsDir() || filepath.Ext(item.Name()) != ".key" {
			continue
		}
		info, infoErr := item.Info()
		if infoErr != nil {
			return nil, fmt.Errorf("stat credential entry: %w", infoErr)
		}
		result = append(result, Entry{Name: strings.TrimSuffix(item.Name(), ".key"), Updated: info.ModTime()})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (s *Store) Remove(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if err := os.Remove(s.path(name)); err != nil {
		return fmt.Errorf("remove credential %q: %w", name, err)
	}
	return nil
}

func (s *Store) ensureDir() error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create credential directory: %w", err)
	}
	if err := hardenCredentialPath(s.dir, true); err != nil {
		return fmt.Errorf("secure credential directory: %w", err)
	}
	return nil
}

func (s *Store) writeAtomic(path, name, key string) error {
	temp, err := os.CreateTemp(s.dir, "."+name+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary credential: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()

	if err := writeCredentialTemp(temp, key); err != nil {
		return err
	}
	if err := hardenCredentialPath(tempPath, false); err != nil {
		return fmt.Errorf("secure temporary credential: %w", err)
	}
	if err := s.rename(tempPath, path); err != nil {
		return fmt.Errorf("replace credential %q: %w", name, err)
	}
	if err := hardenCredentialPath(path, false); err != nil {
		return fmt.Errorf("secure credential %q: %w", name, err)
	}
	return nil
}

func writeCredentialTemp(file *os.File, key string) error {
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("set credential permissions: %w", err)
	}
	if _, err := file.WriteString(key); err != nil {
		_ = file.Close()
		return fmt.Errorf("write credential: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync credential: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close credential: %w", err)
	}
	return nil
}

func normalizeKey(key string) (string, error) {
	key = strings.TrimRight(key, "\r\n")
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	return key, nil
}

// ValidateKey accepts node-registration credentials, never API/OAuth secrets.
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("credential is empty")
	}
	if containsCredentialWhitespace(key) {
		return fmt.Errorf("credential contains whitespace")
	}
	if strings.HasPrefix(key, "hskey-") {
		if len(key) == len("hskey-") {
			return fmt.Errorf("Headscale credential is incomplete")
		}
		return nil
	}
	return validateTailscaleKey(key)
}

func containsCredentialWhitespace(key string) bool {
	return strings.IndexFunc(key, func(r rune) bool {
		return r == '\r' || r == '\n' || r == ' ' || r == '\t'
	}) >= 0
}

func validateTailscaleKey(key string) error {
	if strings.HasPrefix(key, "tskey-api-") || strings.HasPrefix(key, "tskey-client-") {
		return fmt.Errorf("credential must not be a Tailscale API token or OAuth client secret")
	}
	if !strings.HasPrefix(key, "tskey-auth-") {
		return fmt.Errorf("credential must be a Tailscale machine auth key or Headscale pre-auth key")
	}
	parts := strings.Split(key, "-")
	if len(parts) != 4 || parts[0] != "tskey" || parts[1] != "auth" {
		return fmt.Errorf("credential must be a Tailscale machine auth key or Headscale pre-auth key")
	}
	for _, part := range parts[2:] {
		if part == "" {
			return fmt.Errorf("credential must be a Tailscale machine auth key or Headscale pre-auth key")
		}
	}
	return nil
}
