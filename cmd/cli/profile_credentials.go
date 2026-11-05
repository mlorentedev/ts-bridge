package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"ts-bridge/internal/config"
	"ts-bridge/internal/credential"
	"ts-bridge/internal/profile"
)

func lockCommandProfile(command *cobra.Command) (func(), error) {
	name, _ := command.Flags().GetString("profile")
	if name == "" {
		return func() {}, nil
	}
	lock, err := profile.NewStore(defaultProfileStorePath).Lock(name)
	if err != nil {
		return nil, err
	}
	return func() {
		if err := lock.Close(); err != nil {
			fmt.Fprintf(command.ErrOrStderr(), "release profile %q ownership: %v\n", name, err)
		}
	}, nil
}

func applyProfileRuntime(dst *config.PartialConfig, selected profile.Profile) {
	for _, field := range []struct {
		dst   *string
		value string
	}{
		{&dst.BootstrapSSH, selected.BootstrapSSH},
		{&dst.BootstrapSOCKSAddr, selected.BootstrapSOCKSAddr},
		{&dst.Hostname, selected.Hostname},
		{&dst.StateDir, selected.StateDir},
		{&dst.LocalAddr, selected.LocalAddr},
		{&dst.HealthAddr, selected.HealthAddr},
	} {
		if *field.dst == "" {
			*field.dst = field.value
		}
	}
}

func importProfileConfig(path string, selected profile.Profile) (profile.Profile, error) {
	imported, err := readProfileConfig(path)
	if err != nil {
		return profile.Profile{}, err
	}
	for _, field := range []struct {
		dst   *string
		value string
	}{
		{&selected.Target, imported.Target}, {&selected.ControlURL, imported.ControlURL},
		{&selected.BootstrapSSH, imported.BootstrapSSH}, {&selected.BootstrapSOCKSAddr, imported.BootstrapSOCKSAddr},
		{&selected.Hostname, imported.Hostname}, {&selected.StateDir, imported.StateDir},
		{&selected.LocalAddr, imported.LocalAddr}, {&selected.HealthAddr, imported.HealthAddr},
	} {
		if field.value != "" {
			*field.dst = field.value
		}
	}
	return selected, nil
}

func readProfileConfig(path string) (profile.Profile, error) {
	// #nosec G304 -- path is the explicitly selected, local --config input.
	raw, err := os.ReadFile(path)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("read profile configuration: %w", err)
	}
	var imported struct {
		Version         int    `yaml:"version"`
		Target          string `yaml:"target"`
		ControlURL      string `yaml:"control_url"`
		profile.Runtime `yaml:",inline"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&imported); err != nil {
		return profile.Profile{}, fmt.Errorf("decode profile configuration: %w", err)
	}
	if imported.Version != 0 && imported.Version != 1 {
		return profile.Profile{}, fmt.Errorf("unsupported profile configuration version %d", imported.Version)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return profile.Profile{}, fmt.Errorf("profile configuration must contain exactly one YAML document")
	}
	return profile.Profile{
		Target: imported.Target, ControlURL: imported.ControlURL, Runtime: imported.Runtime,
	}, nil
}

func loadCommandProfile(name string) (profile.Profile, error) {
	if name == "" {
		return profile.Profile{}, nil
	}
	selected, err := profile.NewStore(defaultProfileStorePath).Get(name)
	if err != nil {
		return profile.Profile{}, profileLoadError(name, err)
	}
	return selected, nil
}

// loadBoundProfileCredential loads selected's managed credential only when the
// control URL the merge will resolve is the one the profile declares (#617).
// A layer above the profile (flag, environment, ./.env, YAML) that names another
// control plane would otherwise receive the key.
func loadBoundProfileCredential(name string, selected profile.Profile, flags config.FlagSet, yamlCfg config.PartialConfig) (string, error) {
	if selected.Credential == "" {
		return "", nil
	}
	if resolved, layer := overridingControlURL(flags, yamlCfg); layer != "" && !profile.SameControlURL(resolved, selected.ControlURL) {
		return "", fmt.Errorf("profile %q has managed credential %q bound to control URL %q, but %s sets control URL %q; refusing to send the credential there (pass --auth-key-file to use another key, or remove that override)",
			name, selected.Credential, displayControlURL(selected.ControlURL), layer, resolved)
	}
	key, err := loadManagedProfileCredential(selected)
	if err != nil {
		// A deleted or corrupted managed key is fixed by storing it again.
		return "", fmt.Errorf("%w\nstore it again: ts-bridge auth set --profile %s", err, name)
	}
	return key, nil
}

// overridingControlURL returns the control URL a layer above the profile will
// impose and that layer's name, in Merge's precedence order; layer is empty
// when none sets one.
func overridingControlURL(flags config.FlagSet, yamlCfg config.PartialConfig) (resolved, layer string) {
	candidates := []struct{ value, layer string }{
		{flags.ControlURL, "--control-url"},
		{os.Getenv("TS_CONTROL_URL"), "TS_CONTROL_URL (environment or ./.env)"},
		{yamlCfg.ControlURL, "the YAML config"},
	}
	for _, c := range candidates {
		if strings.TrimSpace(c.value) != "" {
			return strings.TrimSpace(c.value), c.layer
		}
	}
	return "", ""
}

func displayControlURL(raw string) string {
	if raw == "" {
		return "(Tailscale SaaS)"
	}
	return raw
}

// profileLoadError wraps a profile lookup failure; for a profile that does not
// exist it also names the paste-runnable command that creates one.
func profileLoadError(name string, err error) error {
	if errors.Is(err, profile.ErrNotFound) {
		return fmt.Errorf("load profile %q: %w\ncreate it first: ts-bridge import %s \"tsb://HOST:3389?cp=saas\"", name, err, name)
	}
	return fmt.Errorf("load profile %q: %w", name, err)
}

func loadManagedProfileCredential(selected profile.Profile) (string, error) {
	if selected.Credential == "" {
		return "", nil
	}
	key, err := credential.NewStore(defaultCredentialStoreDir).Get(selected.Credential)
	if err != nil {
		return "", fmt.Errorf("load managed credential %q: %w", selected.Credential, err)
	}
	return key, nil
}
