package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"ts-bridge/internal/config"
	"ts-bridge/internal/credential"
	"ts-bridge/internal/profile"
)

var (
	defaultCredentialStoreDir = config.CredentialStoreDir()
	authSecretReader          = readMaskedInput
)

func newAuthCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "auth",
		Short: "Manage profile-scoped credentials",
	}
	command.AddCommand(
		newAuthSetCmd(),
		newAuthStatusCmd(),
		newAuthListCmd(),
		newAuthRemoveCmd(),
	)
	return command
}

func newAuthSetCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "set [credential]",
		Short: "Store a credential and associate it with a profile",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runAuthSet,
	}
	command.Flags().String("profile", "", "Profile to associate with the credential")
	command.Flags().Bool("stdin", false, "Read the credential from stdin instead of a masked prompt")
	command.Flags().Bool("force", false, "Replace an existing credential")
	return command
}

func newAuthStatusCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "status [credential]",
		Short: "Show whether a managed credential is stored",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runAuthStatus,
	}
	command.Flags().String("profile", "", "Resolve the credential associated with this profile")
	return command
}

func newAuthListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List managed credential names and profile references",
		Args:  cobra.NoArgs,
		RunE:  runAuthList,
	}
}

func newAuthRemoveCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "remove [credential]",
		Short: "Remove or unlink a managed credential",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runAuthRemove,
	}
	command.Flags().String("profile", "", "Unlink the credential associated with this profile")
	command.Flags().Bool("force", false, "Remove a credential referenced by profiles and clear every reference")
	return command
}

func runAuthSet(command *cobra.Command, args []string) error {
	profileName, _ := command.Flags().GetString("profile")
	if profileName == "" {
		return fmt.Errorf("--profile is required")
	}
	profileStore := profile.NewStore(defaultProfileStorePath)
	selectedProfile, err := profileStore.Get(profileName)
	if err != nil {
		return fmt.Errorf("load profile %q: %w", profileName, err)
	}
	credentialName := profileName
	if len(args) == 1 {
		credentialName = args[0]
	}
	key, err := readAuthInput(command)
	if err != nil {
		return err
	}
	if strings.HasPrefix(key, "hskey-") && selectedProfile.ControlURL == "" {
		return fmt.Errorf("Headscale credential requires a profile with a custom control URL")
	}
	force, _ := command.Flags().GetBool("force")
	if err := credential.NewStore(defaultCredentialStoreDir).Set(credentialName, key, force); err != nil {
		return fmt.Errorf("store credential %q: %w", credentialName, err)
	}
	if err := profileStore.SetCredential(profileName, credentialName); err != nil {
		return fmt.Errorf("credential stored but profile association failed: %w", err)
	}
	_, _ = fmt.Fprintf(command.OutOrStdout(), "Credential %q stored for profile %q\n", credentialName, profileName)
	return nil
}

func readAuthInput(command *cobra.Command) (string, error) {
	fromStdin, _ := command.Flags().GetBool("stdin")
	if !fromStdin {
		key, err := authSecretReader("Auth key (masked): ")
		if err != nil {
			return "", fmt.Errorf("read credential: %w", err)
		}
		return key, nil
	}
	key, err := bufio.NewReader(command.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read credential from stdin: %w", err)
	}
	return key, nil
}

func runAuthStatus(command *cobra.Command, args []string) error {
	name, profileName, err := resolveAuthSelection(command, args)
	if err != nil {
		return err
	}
	if _, err := credential.NewStore(defaultCredentialStoreDir).Get(name); err != nil {
		return fmt.Errorf("credential %q is not available: %w", name, err)
	}
	if profileName != "" {
		_, _ = fmt.Fprintf(command.OutOrStdout(), "Credential %q is stored for profile %q\n", name, profileName)
	} else {
		_, _ = fmt.Fprintf(command.OutOrStdout(), "Credential %q is stored\n", name)
	}
	return nil
}

func resolveAuthSelection(command *cobra.Command, args []string) (string, string, error) {
	profileName, _ := command.Flags().GetString("profile")
	if len(args) == 1 && profileName != "" {
		return "", "", fmt.Errorf("provide a credential name or --profile, not both")
	}
	if len(args) == 1 {
		return args[0], "", nil
	}
	if profileName == "" {
		return "", "", fmt.Errorf("credential name or --profile is required")
	}
	selected, err := profile.NewStore(defaultProfileStorePath).Get(profileName)
	if err != nil {
		return "", "", fmt.Errorf("load profile %q: %w", profileName, err)
	}
	if selected.Credential == "" {
		return "", "", fmt.Errorf("profile %q has no managed credential", profileName)
	}
	return selected.Credential, profileName, nil
}

func runAuthList(command *cobra.Command, _ []string) error {
	entries, err := credential.NewStore(defaultCredentialStoreDir).List()
	if err != nil {
		return err
	}
	references, err := credentialReferences()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		_, _ = fmt.Fprintf(command.OutOrStdout(), "%s\t%s\n", entry.Name, strings.Join(references[entry.Name], ","))
	}
	return nil
}

func credentialReferences() (map[string][]string, error) {
	profiles, err := profile.NewStore(defaultProfileStorePath).ListProfiles()
	if err != nil {
		return nil, err
	}
	result := make(map[string][]string)
	for name, selected := range profiles {
		if selected.Credential != "" {
			result[selected.Credential] = append(result[selected.Credential], name)
		}
	}
	for name := range result {
		sort.Strings(result[name])
	}
	return result, nil
}

func runAuthRemove(command *cobra.Command, args []string) error {
	name, profileName, err := resolveAuthSelection(command, args)
	if err != nil {
		return err
	}
	force, _ := command.Flags().GetBool("force")
	references, err := credentialReferences()
	if err != nil {
		return err
	}
	if profileName != "" {
		return removeProfileCredential(command, name, profileName, references[name])
	}
	if len(references[name]) > 0 && !force {
		return fmt.Errorf("credential %q is referenced by profiles %s; use --force to remove it",
			name, strings.Join(references[name], ", "))
	}
	if force {
		if err := clearCredentialReferences(name, references[name]); err != nil {
			return err
		}
	}
	if err := credential.NewStore(defaultCredentialStoreDir).Remove(name); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(command.OutOrStdout(), "Credential %q removed\n", name)
	return nil
}

func removeProfileCredential(command *cobra.Command, name, profileName string, references []string) error {
	profileStore := profile.NewStore(defaultProfileStorePath)
	if err := profileStore.ClearCredential(profileName); err != nil {
		return err
	}
	if len(references) > 1 {
		_, _ = fmt.Fprintf(command.OutOrStdout(), "Credential %q unlinked from profile %q\n", name, profileName)
		return nil
	}
	if err := credential.NewStore(defaultCredentialStoreDir).Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, _ = fmt.Fprintf(command.OutOrStdout(), "Credential %q removed with profile %q association\n", name, profileName)
	return nil
}

func clearCredentialReferences(credentialName string, profileNames []string) error {
	store := profile.NewStore(defaultProfileStorePath)
	for _, profileName := range profileNames {
		selected, err := store.Get(profileName)
		if err != nil {
			return err
		}
		if selected.Credential == credentialName {
			if err := store.ClearCredential(profileName); err != nil {
				return err
			}
		}
	}
	return nil
}
