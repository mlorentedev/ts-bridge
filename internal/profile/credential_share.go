package profile

import (
	"fmt"
	"sort"

	"ts-bridge/internal/credential"
)

// CredentialSharedError reports a credential name that another profile
// references on a different control plane. A key is issued by one control
// plane, so sharing its name across two would send it to the wrong one (#653).
type CredentialSharedError struct {
	Credential, Profile, ControlURL string
}

func (e *CredentialSharedError) Error() string {
	return fmt.Sprintf("credential %q is used by profile %q on control plane %s; pick another credential name",
		e.Credential, e.Profile, displayControlURL(e.ControlURL))
}

// CheckCredentialShare reports a CredentialSharedError when a profile other
// than profileName references credentialName on another control plane. Call it
// before replacing the credential, so a refused share leaves the key intact.
func (s *Store) CheckCredentialShare(profileName, credentialName string) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	entry, ok := data.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile %q %w in %s", profileName, ErrNotFound, s.path)
	}
	return sharedConflict(data, profileName, credentialName, entry.ControlURL)
}

// associate checks the reference and writes it. Callers hold the store lock
// (Tx.SetCredential), so the check and the write are one transition (#391).
func (s *Store) associate(profileName, credentialName string) error {
	if err := credential.ValidateName(credentialName); err != nil {
		return fmt.Errorf("invalid credential reference: %w", err)
	}
	data, err := s.load()
	if err != nil {
		return err
	}
	entry, ok := data.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile %q %w in %s", profileName, ErrNotFound, s.path)
	}
	if err := sharedConflict(data, profileName, credentialName, entry.ControlURL); err != nil {
		return err
	}
	entry.Credential = credentialName
	data.Profiles[profileName] = entry
	return s.save(data)
}

// sharedConflict finds another profile referencing name on a control plane
// other than controlURL, visiting profiles in name order so the error is stable.
func sharedConflict(data storeFile, profileName, name, controlURL string) error {
	names := make([]string, 0, len(data.Profiles))
	for other := range data.Profiles {
		names = append(names, other)
	}
	sort.Strings(names)
	for _, other := range names {
		entry := data.Profiles[other]
		if other != profileName && entry.Credential == name && !SameControlURL(entry.ControlURL, controlURL) {
			return &CredentialSharedError{Credential: name, Profile: other, ControlURL: entry.ControlURL}
		}
	}
	return nil
}
