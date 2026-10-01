package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"ts-bridge/internal/config"
	"ts-bridge/internal/config/envfile"
)

type BrowserOptions struct {
	StartURL    string
	UserDataDir string
	EdgePath    string
	PACAddr     string
}

var BrowserRunner func(config.Config, BrowserOptions) error

func newBrowserCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "browser",
		Short: "Open an isolated browser through an allow-listed mesh proxy",
		RunE:  runBrowser,
	}
	command.Flags().String("url", "", "Private HTTPS URL to open")
	command.Flags().String("user-data-dir", defaultBrowserUserDataDir(), "Isolated Edge user-data directory")
	command.Flags().String("edge-path", "", "Path to msedge.exe (auto-detected when omitted)")
	command.Flags().String("pac-addr", "127.0.0.1:0", "Loopback address for the local PAC server")
	command.Flags().String("socks5", "127.0.0.1:1080", "Loopback SOCKS5 listener address")
	command.Flags().StringArray("route", nil, "Allow-listed route SOURCE_HOST:PORT=MESH_HOST:PORT (repeatable)")
	command.Flags().String("auth-key-file", "", "Read auth key from a local file")
	command.Flags().String("profile", "", "Named profile supplying control plane and managed credential")
	command.Flags().String("control-url", "", "Custom Headscale control plane URL")
	command.Flags().String("hostname", "", "Tailscale hostname")
	command.Flags().String("state-dir", "", "State directory")
	command.Flags().Duration("timeout", 0, "Connect timeout for tsnet init")
	command.Flags().Duration("dial-timeout", 0, "Per-dial timeout")
	command.Flags().String("config", "", "Path to YAML config file")
	return command
}

func runBrowser(command *cobra.Command, _ []string) error {
	cfg, err := resolveBrowserConfig(command)
	if err != nil {
		return err
	}
	options := BrowserOptions{}
	options.StartURL, _ = command.Flags().GetString("url")
	options.UserDataDir, _ = command.Flags().GetString("user-data-dir")
	options.EdgePath, _ = command.Flags().GetString("edge-path")
	options.PACAddr, _ = command.Flags().GetString("pac-addr")
	if options.StartURL == "" {
		return fmt.Errorf("browser URL is required (provide --url)")
	}
	if LoggerInit != nil {
		LoggerInit(cfg)
	}
	if BrowserRunner == nil {
		return fmt.Errorf("browser runner not initialized")
	}
	return BrowserRunner(cfg, options)
}

func resolveBrowserConfig(command *cobra.Command) (config.Config, error) {
	authKeyFile, _ := command.Flags().GetString("auth-key-file")
	profileName, _ := command.Flags().GetString("profile")
	if authKeyFile == "" && profileName == "" {
		return config.Config{}, fmt.Errorf("browser requires --auth-key-file or --profile")
	}
	flags, err := collectBrowserFlags(command)
	if err != nil {
		return config.Config{}, err
	}
	if err := envfile.Load(".env"); err != nil {
		return config.Config{}, fmt.Errorf("load .env: %w", err)
	}
	yamlConfig, err := config.LoadYAMLConfig(flags.Config)
	if err != nil {
		return config.Config{}, fmt.Errorf("load YAML config: %w", err)
	}
	if err := applyBrowserProfile(&yamlConfig, &flags, profileName); err != nil {
		return config.Config{}, err
	}
	warnIgnoredBrowserEnvCredential(command, authKeyFile, profileName)
	if flags.SOCKS5Addr == "" && yamlConfig.SOCKS5Addr == "" && os.Getenv("TS_SOCKS5_ADDR") == "" {
		flags.SOCKS5Addr = "127.0.0.1:1080"
	}
	cfg, err := config.Merge(yamlConfig, flags)
	if err != nil {
		return config.Config{}, err
	}
	if cfg.SOCKS5Addr == "" || len(cfg.SOCKS5Routes) == 0 {
		return config.Config{}, fmt.Errorf("browser requires --socks5 and at least one --route")
	}
	return cfg, nil
}

func warnIgnoredBrowserEnvCredential(command *cobra.Command, authKeyFile, profileName string) {
	if profileName == "" || authKeyFile != "" || os.Getenv("TS_AUTHKEY") == "" {
		return
	}
	fmt.Fprintln(command.ErrOrStderr(), "WARNING: browser ignores TS_AUTHKEY when --profile supplies a managed credential; use --auth-key-file for an explicit override")
}

func applyBrowserProfile(yamlConfig *config.PartialConfig, flags *config.FlagSet, profileName string) error {
	if profileName == "" {
		return nil
	}
	selectedProfile, err := loadCommandProfile(profileName)
	if err != nil {
		return err
	}
	if yamlConfig.ControlURL == "" {
		yamlConfig.ControlURL = selectedProfile.ControlURL
	}
	if flags.AuthKey == "" {
		flags.AuthKey, err = loadManagedProfileCredential(selectedProfile)
		if err != nil {
			return err
		}
		if flags.AuthKey == "" {
			return fmt.Errorf("profile %q has no managed credential", profileName)
		}
	}
	return nil
}

func collectBrowserFlags(command *cobra.Command) (config.FlagSet, error) {
	var flags config.FlagSet
	if command.Flags().Changed("socks5") {
		flags.SOCKS5Addr, _ = command.Flags().GetString("socks5")
	}
	flags.SOCKS5Routes, _ = command.Flags().GetStringArray("route")
	flags.AuthKeyFile, _ = command.Flags().GetString("auth-key-file")
	flags.ControlURL, _ = command.Flags().GetString("control-url")
	flags.Hostname, _ = command.Flags().GetString("hostname")
	flags.StateDir, _ = command.Flags().GetString("state-dir")
	flags.Timeout, _ = command.Flags().GetDuration("timeout")
	flags.DialTimeout, _ = command.Flags().GetDuration("dial-timeout")
	flags.Config, _ = command.Flags().GetString("config")

	if flags.AuthKeyFile != "" {
		key, err := readAuthKeyFile(flags.AuthKeyFile)
		if err != nil {
			return config.FlagSet{}, fmt.Errorf("read auth key file: %w", err)
		}
		flags.AuthKey = key
	}
	return flags, nil
}

func defaultBrowserUserDataDir() string {
	if root := os.Getenv("LOCALAPPDATA"); root != "" {
		return filepath.Join(root, "ts-bridge", "browser")
	}
	return filepath.Join(os.TempDir(), "ts-bridge-browser")
}
