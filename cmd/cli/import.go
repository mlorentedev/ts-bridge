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

// newImportCmd constructs the "ts-bridge import" subcommand.
func newImportCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "import <name> <descriptor>",
		Short: "Import a named connection profile from a tsb:// descriptor",
		Long: `Import a named connection profile from a tsb:// descriptor string.

The descriptor encodes the target host, port, and optional control-plane URL
in a portable, shareable format. After importing, use the profile by name with
  ts-bridge connect --profile <name>

Examples:
  ts-bridge import home "tsb://acemagic-office:45000?cp=saas"
  ts-bridge import work "tsb://rdp.corp.example.com:3389?cp=https://vpn.corp.example.com"

A managed credential stays bound to the control URL it was issued for. Importing
a descriptor that changes the control URL of a credentialed profile is refused
unless --force is given, which drops the profile's credential reference.`,
		Args: exactArgsWithUsage(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			return runImportDirect(args[0], args[1], defaultProfileStorePath, force)
		},
	}
	command.Flags().Bool("force", false, "Move a credentialed profile to another control URL, dropping its credential reference")
	return command
}

// exactArgsWithUsage is cobra.ExactArgs(n) whose error also carries the usage
// line: arity errors are not flag errors, so cobra prints no usage for them and
// "accepts 2 arg(s), received 0" does not say what the arguments are.
func exactArgsWithUsage(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(n)(cmd, args); err != nil {
			return fmt.Errorf("%w\nUsage: %s", err, cmd.UseLine())
		}
		return nil
	}
}

// runImportDirect is the pure, testable core of the import command.
// It parses the descriptor and writes it to the store at storePath.
func runImportDirect(name, rawDescriptor, storePath string, force bool) error {
	// Parse errors already read "invalid descriptor ...".
	d, err := profile.Parse(rawDescriptor)
	if err != nil {
		return err
	}

	s := profile.NewStore(storePath)
	cleared, err := s.Import(name, d, force)
	if err != nil {
		return fmt.Errorf("import profile %q: %w", name, err)
	}
	if cleared {
		fmt.Printf("Credential reference of profile %q cleared: its control URL changed. Run \"ts-bridge auth set\" to bind a credential for the new control plane.\n", name)
	}

	fmt.Printf("Profile %q imported (%s)\n", name, d.String())
	fmt.Printf("Next: ts-bridge auth set --profile %s\n      ts-bridge connect --profile %s\n", name, name)
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
