package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"ts-bridge/internal/config"
	"ts-bridge/internal/config/envfile"
	"ts-bridge/internal/profile"
)

// BUG-001/002: init should not overwrite existing config without confirmation.

func TestWriteEnvConfig_DetectsExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	// Pre-create .env file.
	if err := os.WriteFile(envPath, []byte("TS_TARGET=100.64.0.1:3389\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a mock command with non-terminal stdin (non-interactive).
	cmd := &cobra.Command{}
	f := initFlags{
		AuthKey: "tskey-auth-test-secret",
		Target:  "100.64.0.2:443",
		Format:  "env",
		Config:  envPath,
		Force:   false,
	}

	err := writeEnvConfig(cmd, f)
	if err == nil {
		t.Fatal("expected error for existing .env without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}

	// File should be unchanged.
	data, _ := os.ReadFile(envPath)
	if strings.Contains(string(data), "100.64.0.2") {
		t.Error("file should not have been overwritten")
	}
}

func TestWriteEnvConfig_OverwriteWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	if err := os.WriteFile(envPath, []byte("TS_TARGET=100.64.0.1:3389\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	f := initFlags{
		AuthKey: "tskey-auth-test-secret",
		Target:  "100.64.0.2:443",
		Format:  "env",
		Config:  envPath,
		Force:   true,
	}

	err := writeEnvConfig(cmd, f)
	if err != nil {
		t.Fatalf("unexpected error with --force: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	if !strings.Contains(string(data), "100.64.0.2") {
		t.Error("file should have been overwritten with --force")
	}
}

func TestWriteYAMLConfig_DetectsExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "ts-bridge.yaml")

	if err := os.WriteFile(yamlPath, []byte("version: 1\ntarget: 100.64.0.1:3389\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	f := initFlags{
		AuthKey: "tskey-auth-test-secret",
		Target:  "100.64.0.2:443",
		Format:  "yaml",
		Config:  yamlPath,
		Force:   false,
	}

	err := writeYAMLConfig(cmd, f)
	if err == nil {
		t.Fatal("expected error for existing YAML without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestWriteYAMLConfig_CreatesYamlAndEnv(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "ts-bridge.yaml")
	envPath := filepath.Join(tmpDir, ".env")

	cmd := &cobra.Command{}
	f := initFlags{
		AuthKey:   "tskey-auth-test-secret",
		Target:    "100.64.0.1:3389",
		Format:    "yaml",
		Config:    yamlPath,
		Instance:  "my-server",
		PortRange: "33389-34388",
		Force:     true,
	}

	err := writeYAMLConfig(cmd, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// YAML file should exist with target and hostname.
	yamlData, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("YAML file not created: %v", err)
	}
	if !strings.Contains(string(yamlData), "target:") {
		t.Error("YAML should contain target")
	}
	if !strings.Contains(string(yamlData), "my-server") {
		t.Error("YAML should contain hostname derived from instance")
	}

	// .env file should exist with TS_AUTHKEY.
	envData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf(".env file not created: %v", err)
	}
	if !strings.Contains(string(envData), "TS_AUTHKEY=tskey-auth-test-secret") {
		t.Error(".env should contain TS_AUTHKEY")
	}
	if !strings.Contains(string(envData), "--auth-key-file") {
		t.Error("YAML-mode .env sidecar should point to --auth-key-file as the secure alternative")
	}
}

func TestRunInitProfile(t *testing.T) {
	cases := []struct {
		name           string
		flags          initFlags
		preExisting    string // if non-empty, pre-write this target under f.Profile name
		wantErr        string // substring expected in error; empty = no error
		wantTarget     string
		wantControlURL string
	}{
		{
			name:       "writes profile without control URL",
			flags:      initFlags{Target: "host:3389", Profile: "work"},
			wantTarget: "host:3389",
		},
		{
			name:           "writes Headscale profile with control URL",
			flags:          initFlags{Target: "host:3389", Profile: "kubelab", ControlURL: "https://vpn.kubelab.live"},
			wantTarget:     "host:3389",
			wantControlURL: "https://vpn.kubelab.live",
		},
		{
			name:        "existing profile without --force returns error",
			flags:       initFlags{Target: "new-host:45000", Profile: "work", Force: false},
			preExisting: "old-host:3389",
			wantErr:     "already exists",
			wantTarget:  "old-host:3389", // unchanged
		},
		{
			name:        "existing profile with --force overwrites",
			flags:       initFlags{Target: "new-host:45000", Profile: "work", Force: true},
			preExisting: "old-host:3389",
			wantTarget:  "new-host:45000",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storePath := filepath.Join(t.TempDir(), "profiles.yaml")
			if tc.preExisting != "" {
				if err := profile.NewStore(storePath).Set(tc.flags.Profile, tc.preExisting, ""); err != nil {
					t.Fatalf("pre-write error: %v", err)
				}
			}

			err := runInitProfile(storePath, tc.flags)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantTarget != "" {
				p, getErr := profile.NewStore(storePath).Get(tc.flags.Profile)
				if getErr != nil {
					t.Fatalf("Get error: %v", getErr)
				}
				if p.Target != tc.wantTarget {
					t.Errorf("Target = %q, want %q", p.Target, tc.wantTarget)
				}
				if tc.wantControlURL != "" && p.ControlURL != tc.wantControlURL {
					t.Errorf("ControlURL = %q, want %q", p.ControlURL, tc.wantControlURL)
				}
			}
		})
	}
}

func TestWriteEnvConfig_CreatesFullConfig(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	cmd := &cobra.Command{}
	f := initFlags{
		AuthKey:   "tskey-auth-test-secret",
		Target:    "100.64.0.1:3389",
		Format:    "env",
		Config:    envPath,
		Instance:  "my-server",
		PortRange: "33389-34388",
	}

	err := writeEnvConfig(cmd, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf(".env file not created: %v", err)
	}

	// Should contain both required fields.
	content := string(data)
	if !strings.Contains(content, "TS_AUTHKEY=tskey-auth-test-secret") {
		t.Error("should contain TS_AUTHKEY")
	}
	if !strings.Contains(content, "TS_TARGET=100.64.0.1:3389") {
		t.Error("should contain TS_TARGET")
	}
	if !strings.Contains(content, "TS_INSTANCE_NAME=my-server") {
		t.Error("should contain TS_INSTANCE_NAME")
	}
	if !strings.Contains(content, "TS_PORT_RANGE=33389-34388") {
		t.Error("should contain TS_PORT_RANGE")
	}
	if !strings.Contains(content, "--auth-key-file") {
		t.Error("should contain security note promoting --auth-key-file")
	}
}

func TestInit_SecurityGuidance(t *testing.T) {
	cmd := newInitCmd()
	if !strings.Contains(cmd.Long, "ts-bridge init --auth-key-file") {
		t.Error("init command help should show the secure non-interactive key-file workflow")
	}
	if strings.Contains(cmd.Long, "init itself has no key-file flag") {
		t.Error("init command help should not claim the key-file flag is unavailable")
	}
	if !strings.Contains(cmd.Long, "child processes") {
		t.Error("init command help should explain child processes visibility")
	}
	authKeyFlag := cmd.Flags().Lookup("auth-key")
	if authKeyFlag == nil {
		t.Fatal("init should register --auth-key")
	}
	if !strings.Contains(authKeyFlag.Usage, "visible in process list") {
		t.Errorf("--auth-key usage should warn that the key is visible in the process list, got %q", authKeyFlag.Usage)
	}
	if cmd.Flags().Lookup("auth-key-file") == nil {
		t.Fatal("init should register --auth-key-file")
	}
}

func TestInitAuthKeyFile(t *testing.T) {
	tests := []struct {
		name           string
		keyFileContent *string
		emptyKeyPath   bool
		includeInline  bool
		profile        string
		wantErr        string
		wantKey        string
		wantWarning    bool
	}{
		{
			name:           "reads key from file",
			keyFileContent: stringPtr("tskey-auth-from-file\r\n"),
			wantKey:        "tskey-auth-from-file",
		},
		{
			name:           "file takes precedence over inline key",
			keyFileContent: stringPtr("tskey-auth-from-file"),
			includeInline:  true,
			wantKey:        "tskey-auth-from-file",
			wantWarning:    true,
		},
		{
			name:    "missing file returns clear error",
			wantErr: "read auth key file: stat auth key file",
		},
		{
			name:         "explicit empty file path returns clear error",
			emptyKeyPath: true,
			wantErr:      "--auth-key-file cannot be empty",
		},
		{
			name:           "empty file returns clear error",
			keyFileContent: stringPtr(""),
			wantErr:        "auth key file is empty",
		},
		{
			name:           "embedded newline returns clear error",
			keyFileContent: stringPtr("tskey-auth-from-file\nTS_LOCAL_ADDR=0.0.0.0:33389"),
			wantErr:        "auth key file contains embedded line break",
		},
		{
			name:           "malformed key returns validation error",
			keyFileContent: stringPtr("not-a-key"),
			wantErr:        "auth key invalid format",
		},
		{
			name:           "prefixed malformed key returns validation error",
			keyFileContent: stringPtr("tskey-auth-abc"),
			wantErr:        "auth key invalid format",
		},
		{
			name:           "profile mode rejects auth key file",
			keyFileContent: stringPtr("tskey-auth-from-file"),
			profile:        "work",
			wantErr:        "--auth-key-file is not compatible with --profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			keyPath := filepath.Join(tmpDir, "authkey")
			if tt.keyFileContent != nil {
				if err := os.WriteFile(keyPath, []byte(*tt.keyFileContent), 0600); err != nil {
					t.Fatal(err)
				}
			}

			args := []string{
				"--auth-key-file", keyPath,
				"--target", "100.64.0.1:3389",
				"--config", filepath.Join(tmpDir, ".env"),
			}
			if tt.emptyKeyPath {
				args[1] = ""
			}
			if tt.includeInline {
				args = append(args, "--auth-key", "tskey-auth-from-inline")
			}
			if tt.profile != "" {
				args = append(args, "--profile", tt.profile)
			}

			cmd := newInitCmd()
			cmd.SetArgs(args)
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			var runErr error
			_, stderr := captureInitOutput(t, func() {
				runErr = cmd.Execute()
			})

			if tt.wantErr != "" {
				if runErr == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(runErr.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", runErr, tt.wantErr)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("unexpected error: %v", runErr)
			}

			data, err := os.ReadFile(filepath.Join(tmpDir, ".env"))
			if err != nil {
				t.Fatalf("read generated config: %v", err)
			}
			if !strings.Contains(string(data), "TS_AUTHKEY="+tt.wantKey) {
				t.Errorf("generated config does not contain file key %q:\n%s", tt.wantKey, data)
			}
			if tt.includeInline && strings.Contains(string(data), "tskey-auth-from-inline") {
				t.Errorf("generated config contains lower-precedence inline key:\n%s", data)
			}
			if tt.wantWarning && !strings.Contains(stderr, "--auth-key is visible in the process list") {
				t.Errorf("stderr does not contain process-table warning:\n%s", stderr)
			}
		})
	}
}

func TestInitAuthKeyValidationMatchesConfigMerge(t *testing.T) {
	t.Setenv("TS_AUTHKEY", "")
	t.Setenv("TS_TARGET", "")
	t.Setenv("TS_CONTROL_URL", "")

	tests := []struct {
		name string
		key  string
	}{
		{name: "valid machine key", key: "tskey-auth-init-secret"},
		{name: "malformed prefixed key", key: "tskey-auth-abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initErr := validateAuthKey(tt.key)
			_, mergeErr := config.Merge(config.PartialConfig{}, config.FlagSet{
				Target:  "100.64.0.1:3389",
				AuthKey: tt.key,
			})
			if (initErr == nil) != (mergeErr == nil) {
				t.Fatalf("validator divergence for %q: init error=%v, merge error=%v", tt.key, initErr, mergeErr)
			}
		})
	}
}

func TestValidateAuthKeyPreservesLoginURLHint(t *testing.T) {
	err := validateAuthKey("https://login.tailscale.com/admin/auth/singleusekey/example")
	if err == nil || !strings.Contains(err.Error(), "did you paste a Tailscale login URL") {
		t.Fatalf("validateAuthKey() error = %v, want login URL remediation hint", err)
	}
}

// initTestEnv clears every environment variable that config loading reads, so a
// generated file is the only input to the round-trip assertion. t.Setenv
// restores the original values on cleanup, including any later overwrite.
func initTestEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"TS_AUTHKEY", "TS_TARGET", "TS_CONTROL_URL", "TS_INSTANCE_NAME",
		"TS_PORT_RANGE", "TS_MANUAL_MODE", "TS_LOCAL_ADDR", "TS_STATE_DIR",
		"TS_HOSTNAME", "TS_AUTO_INSTANCE",
	} {
		t.Setenv(key, "")
	}
}

// loadInitOutput loads exactly what init wrote, the way connect does: the .env
// beside the config, plus the YAML file when one was produced.
func loadInitOutput(t *testing.T, tmpDir, format string) (config.PartialConfig, error) {
	t.Helper()
	envPath := filepath.Join(tmpDir, ".env")
	if err := envfile.Load(envPath); err != nil {
		t.Fatalf("load generated .env: %v", err)
	}
	if format != formatYAML {
		return config.PartialConfig{}, nil
	}
	yamlPath := filepath.Join(tmpDir, "ts-bridge.yaml")
	partial, err := config.LoadYAMLConfig(yamlPath)
	if err != nil {
		t.Fatalf("load generated YAML: %v", err)
	}
	return partial, nil
}

// UX-355 round-7 finding: init in env/yaml mode accepted an hskey-* key with no
// control URL, wrote a config, and reported success while connect rejected that
// same config. init must either write a config connect accepts, or refuse to
// write one at all.
func TestInitHeadscaleControlURL(t *testing.T) {
	const controlURL = "https://headscale.example.com"

	tests := []struct {
		name       string
		format     string
		controlURL string
		wantErr    string
		wantInEnv  string
		wantInYAML string
	}{
		{
			name:       "env mode writes TS_CONTROL_URL for a Headscale key",
			format:     formatENV,
			controlURL: controlURL,
			wantInEnv:  "TS_CONTROL_URL=" + controlURL,
		},
		{
			name:       "yaml mode writes control_url for a Headscale key",
			format:     formatYAML,
			controlURL: controlURL,
			wantInYAML: "control_url: " + controlURL,
		},
		{
			name:    "env mode refuses a Headscale key without a control URL",
			format:  formatENV,
			wantErr: "--control-url",
		},
		{
			name:    "yaml mode refuses a Headscale key without a control URL",
			format:  formatYAML,
			wantErr: "--control-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initTestEnv(t)
			tmpDir := t.TempDir()

			keyPath := filepath.Join(tmpDir, "authkey")
			if err := os.WriteFile(keyPath, []byte("hskey-abcdef"), 0600); err != nil {
				t.Fatal(err)
			}
			yamlPath := filepath.Join(tmpDir, "ts-bridge.yaml")
			configPath := filepath.Join(tmpDir, ".env")
			if tt.format == formatYAML {
				configPath = yamlPath
			}

			args := []string{
				"--auth-key-file", keyPath,
				"--target", "100.64.0.1:3389",
				"--format", tt.format,
				"--config", configPath,
			}
			if tt.controlURL != "" {
				args = append(args, "--control-url", tt.controlURL)
			}

			cmd := newInitCmd()
			cmd.SetArgs(args)
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			var runErr error
			_, _ = captureInitOutput(t, func() { runErr = cmd.Execute() })

			if tt.wantErr != "" {
				if runErr == nil {
					t.Fatal("expected init to refuse an unusable configuration, got nil error")
				}
				if !strings.Contains(runErr.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", runErr, tt.wantErr)
				}
				if _, statErr := os.Stat(configPath); statErr == nil {
					t.Errorf("%s was written even though the configuration is unusable", configPath)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("unexpected error: %v", runErr)
			}

			if tt.wantInEnv != "" {
				data, err := os.ReadFile(filepath.Join(tmpDir, ".env"))
				if err != nil {
					t.Fatalf("read generated .env: %v", err)
				}
				if !strings.Contains(string(data), tt.wantInEnv) {
					t.Errorf("generated .env lacks %q:\n%s", tt.wantInEnv, data)
				}
			}
			if tt.wantInYAML != "" {
				data, err := os.ReadFile(yamlPath)
				if err != nil {
					t.Fatalf("read generated YAML: %v", err)
				}
				if !strings.Contains(string(data), tt.wantInYAML) {
					t.Errorf("generated YAML lacks %q:\n%s", tt.wantInYAML, data)
				}
			}

			// Round trip: feed what init wrote through the same merge connect uses.
			partial, err := loadInitOutput(t, tmpDir, tt.format)
			if err != nil {
				t.Fatalf("load generated config: %v", err)
			}
			if _, err := config.Merge(partial, config.FlagSet{}); err != nil {
				t.Fatalf("connect would reject the config init just wrote: %v", err)
			}
		})
	}
}

// The invariant rounds 4-7 kept finding broken: every configuration init
// accepts, connect accepts too, and every configuration connect would refuse,
// init refuses instead of writing it.
func TestInitAcceptedConfigIsAcceptedByConnect(t *testing.T) {
	keys := []string{"hskey-abcdef", "tskey-auth-init-secret"}
	controlURLs := []string{"", "https://headscale.example.com", "headscale.example.com", "http://127.0.0.1:8080"}
	formats := []string{formatENV, formatYAML}

	for _, key := range keys {
		for _, controlURL := range controlURLs {
			for _, format := range formats {
				name := fmt.Sprintf("key=%s url=%q format=%s", key, controlURL, format)
				t.Run(name, func(t *testing.T) {
					initTestEnv(t)
					tmpDir := t.TempDir()

					keyPath := filepath.Join(tmpDir, "authkey")
					if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
						t.Fatal(err)
					}
					configPath := filepath.Join(tmpDir, ".env")
					if format == formatYAML {
						configPath = filepath.Join(tmpDir, "ts-bridge.yaml")
					}

					args := []string{
						"--auth-key-file", keyPath,
						"--target", "100.64.0.1:3389",
						"--format", format,
						"--config", configPath,
					}
					if controlURL != "" {
						args = append(args, "--control-url", controlURL)
					}

					cmd := newInitCmd()
					cmd.SetArgs(args)
					cmd.SilenceUsage = true
					cmd.SilenceErrors = true

					var initErr error
					_, _ = captureInitOutput(t, func() { initErr = cmd.Execute() })

					mergeErr := error(nil)
					if initErr == nil {
						partial, err := loadInitOutput(t, tmpDir, format)
						if err != nil {
							t.Fatalf("load generated config: %v", err)
						}
						_, mergeErr = config.Merge(partial, config.FlagSet{})
					} else {
						// init refused: connect must have refused the same pairing too.
						_, mergeErr = config.Merge(config.PartialConfig{}, config.FlagSet{
							Target:     "100.64.0.1:3389",
							AuthKey:    key,
							ControlURL: controlURL,
						})
					}

					if (initErr == nil) != (mergeErr == nil) {
						t.Fatalf("init and connect disagree: init error=%v, connect error=%v", initErr, mergeErr)
					}
				})
			}
		}
	}
}

func stringPtr(value string) *string {
	return &value
}

func captureInitOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stdout, os.Stderr = wOut, wErr

	defer func() {
		os.Stdout, os.Stderr = oldStdout, oldStderr
	}()

	fn()

	wOut.Close()
	wErr.Close()
	out, err := io.ReadAll(rOut)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	errOut, err := io.ReadAll(rErr)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(out), string(errOut)
}

func TestPrintNextSteps(t *testing.T) {
	tests := []struct {
		name      string
		flags     initFlags
		required  []string
		forbidden []string
	}{
		{
			name:      "config mode shows explicit file guidance only",
			flags:     initFlags{Config: "ts-bridge.env", Format: "env"},
			required:  []string{"ts-bridge connect --auth-key-file"},
			forbidden: []string{"ts-bridge auth set --profile"},
		},
		{
			name:      "profile mode shows managed credential onboarding",
			flags:     initFlags{Profile: "office"},
			required:  []string{"ts-bridge auth set --profile office", "ts-bridge connect --profile office"},
			forbidden: []string{"ts-bridge connect --auth-key-file"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _ := captureInitOutput(t, func() { printNextSteps(tt.flags) })
			for _, text := range tt.required {
				if !strings.Contains(stdout, text) {
					t.Errorf("next steps missing %q:\n%s", text, stdout)
				}
			}
			for _, text := range tt.forbidden {
				if strings.Contains(stdout, text) {
					t.Errorf("next steps unexpectedly contain %q:\n%s", text, stdout)
				}
			}
		})
	}
}

// Every command that accepts an inline --auth-key must warn that the value is
// visible in the process list. Walking the whole tree instead of naming commands
// is the point: discover took the flag without the warning because the check
// named only init.
func TestAuthKeyFlags_WarnProcessList(t *testing.T) {
	var walk func(c *cobra.Command) int
	walk = func(c *cobra.Command) int {
		n := 0
		if f := c.Flags().Lookup("auth-key"); f != nil {
			n++
			if !strings.Contains(f.Usage, "visible in process list") {
				t.Errorf("%q: --auth-key usage lacks the process-list warning: %q", c.CommandPath(), f.Usage)
			}
		}
		for _, sub := range c.Commands() {
			n += walk(sub)
		}
		return n
	}
	if n := walk(NewRootCmd()); n < 3 {
		t.Fatalf("found --auth-key on %d commands, want at least connect, init and discover", n)
	}
}
