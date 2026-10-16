package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"

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
		return profile.Profile{}, fmt.Errorf("load profile %q: %w", name, err)
	}
	return selected, nil
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
