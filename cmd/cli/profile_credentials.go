package cmd

import (
	"fmt"

	"ts-bridge/internal/credential"
	"ts-bridge/internal/profile"
)

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
