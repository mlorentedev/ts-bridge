package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"ts-bridge/internal/config"
	"ts-bridge/internal/profile"
)

// defaultProfileStorePath is the on-disk path used when no --store flag is
// given. It is a package-level var (not a const) so tests can inject a
// temp-dir path without touching the real user store.
var defaultProfileStorePath = config.ProfileStorePath()

// importCmd is the "ts-bridge import" subcommand.
var importCmd = &cobra.Command{
	Use:   "import <name> <descriptor>",
	Short: "Import a named connection profile from a tsb:// descriptor",
	Long: `Import a named connection profile from a tsb:// descriptor string.

The descriptor encodes the target host, port, and optional control-plane URL
in a portable, shareable format. After importing, use the profile by name with
  ts-bridge connect --profile <name>

Examples:
  ts-bridge import home "tsb://acemagic-office:45000?cp=saas"
  ts-bridge import work "tsb://rdp.corp.example.com:3389?cp=https://vpn.corp.example.com"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runImportDirect(args[0], args[1], defaultProfileStorePath)
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
}

// runImportDirect is the pure, testable core of the import command.
// It parses the descriptor and writes it to the store at storePath.
func runImportDirect(name, rawDescriptor, storePath string) error {
	d, err := profile.Parse(rawDescriptor)
	if err != nil {
		return fmt.Errorf("invalid descriptor: %w", err)
	}

	s := profile.NewStore(storePath)
	if err := s.Import(name, d); err != nil {
		return fmt.Errorf("import profile %q: %w", name, err)
	}

	fmt.Printf("Profile %q imported (%s)\n", name, d.String())
	return nil
}

// applyProfile resolves a named profile from storePath and populates cfg with
// its target and control URL. If name is empty it is a no-op. If cfg.Target
// is already set (from flag/env/YAML) it is preserved unchanged so existing
// --target usage is fully backward compatible.
func applyProfile(cfg *config.Config, name, storePath string) error {
	if name == "" {
		return nil
	}

	s := profile.NewStore(storePath)
	p, err := s.Get(name)
	if err != nil {
		return fmt.Errorf("load profile %q: %w", name, err)
	}

	if cfg.Target == "" {
		cfg.Target = p.Target
	}
	if cfg.ControlURL == "" && p.ControlURL != "" {
		cfg.ControlURL = p.ControlURL
	}
	return nil
}
