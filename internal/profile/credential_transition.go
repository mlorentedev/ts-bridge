package profile

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"ts-bridge/internal/credential"
)

// Binding is one request to store a credential key and reference it from a profile.
type Binding struct {
	Profile, Credential, Key string
	// Replace allows overwriting a stored credential of the same name.
	Replace bool
	// ControlURL is the profile's control URL as the caller read it before it
	// prompted for the key. The transition refuses if the profile moved since.
	ControlURL string
}

// BindCredential stores b.Key and references it from b.Profile as one
// transition under the store lock. Everything that can refuse is checked, with
// the profile re-read, before the stored key is touched; if the reference
// cannot be written afterwards, the credential store is put back as it was.
func (s *Store) BindCredential(creds *credential.Store, b Binding) error {
	return s.Update(func(tx *Tx) error {
		selected, err := tx.Get(b.Profile)
		if err != nil {
			return err
		}
		if err := checkBinding(b, selected); err != nil {
			return err
		}
		previous, previousErr := creds.Get(b.Credential)
		if err := creds.Set(b.Credential, b.Key, b.Replace); err != nil {
			return fmt.Errorf("store credential %q: %w", b.Credential, err)
		}
		assocErr := tx.SetCredential(b.Profile, b.Credential)
		if assocErr == nil {
			return nil
		}
		restore := func() error { return creds.Remove(b.Credential) }
		if previousErr == nil {
			restore = func() error { return creds.Set(b.Credential, previous, true) }
		}
		if err := restore(); err != nil {
			return fmt.Errorf("credential stored but profile association failed, and restoring the previous credential failed: %w",
				errors.Join(assocErr, err))
		}
		return fmt.Errorf("profile association failed, credential store restored: %w", assocErr)
	})
}

func checkBinding(b Binding, selected Profile) error {
	if !SameControlURL(selected.ControlURL, b.ControlURL) {
		return fmt.Errorf("profile %q moved from control plane %s to %s while the credential was being read; run auth set again",
			b.Profile, displayControlURL(b.ControlURL), displayControlURL(selected.ControlURL))
	}
	if strings.HasPrefix(b.Key, "hskey-") && selected.ControlURL == "" {
		return fmt.Errorf("Headscale credential requires a profile with a custom control URL")
	}
	return nil
}

// Removal selects the credential to remove: by name, or through a profile.
type Removal struct {
	Credential, Profile string
	// Force removes a credential other profiles reference and clears those references.
	Force bool
}

// RemoveOutcome says what UnbindCredential did.
type RemoveOutcome int

const (
	// Unlinked cleared the profile's reference; other profiles still use the key.
	Unlinked RemoveOutcome = iota
	// RemovedWithProfile deleted the key and the profile's reference.
	RemovedWithProfile
	// Removed deleted the key and every reference to it.
	Removed
)

// RemoveResult reports which credential UnbindCredential acted on and how.
type RemoveResult struct {
	Credential string
	Outcome    RemoveOutcome
}

// UnbindCredential removes a credential or a profile's reference to it as one
// transition under the store lock, so the references it acts on cannot change
// mid-way. The key is deleted before any reference is cleared: a failed deletion
// then changes nothing, and an interruption leaves only a dangling reference,
// which running the removal again clears.
func (s *Store) UnbindCredential(creds *credential.Store, r Removal) (RemoveResult, error) {
	var result RemoveResult
	err := s.Update(func(tx *Tx) error {
		name, err := removalTarget(tx, r)
		if err != nil {
			return err
		}
		references, err := referencesTo(tx, name)
		if err != nil {
			return err
		}
		result.Credential = name
		result.Outcome, err = removeReferenced(tx, creds, r, name, references)
		return err
	})
	return result, err
}

// removalTarget names the credential r selects, resolving a profile's own reference.
func removalTarget(tx *Tx, r Removal) (string, error) {
	if r.Credential != "" || r.Profile == "" {
		return r.Credential, nil
	}
	selected, err := tx.Get(r.Profile)
	if err != nil {
		return "", fmt.Errorf("load profile %q: %w", r.Profile, err)
	}
	if selected.Credential == "" {
		return "", fmt.Errorf("profile %q has no managed credential", r.Profile)
	}
	return selected.Credential, nil
}

func removeReferenced(tx *Tx, creds *credential.Store, r Removal, name string, references []string) (RemoveOutcome, error) {
	if r.Profile != "" {
		if len(references) > 1 {
			return Unlinked, tx.ClearCredential(r.Profile)
		}
		if err := removeKey(creds, name, len(references) > 0); err != nil {
			return 0, err
		}
		return RemovedWithProfile, tx.ClearCredential(r.Profile)
	}
	if len(references) > 0 && !r.Force {
		return 0, fmt.Errorf("credential %q is referenced by profiles %s; use --force to remove it", name, strings.Join(references, ", "))
	}
	if err := removeKey(creds, name, len(references) > 0); err != nil {
		return 0, err
	}
	for _, profileName := range references {
		if err := tx.ClearCredential(profileName); err != nil {
			return 0, err
		}
	}
	return Removed, nil
}

// removeKey deletes the stored key. A key already gone is fine while a
// reference to it remains: that is an earlier removal interrupted.
func removeKey(creds *credential.Store, name string, dangling bool) error {
	err := creds.Remove(name)
	if dangling && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// referencesTo lists, sorted, the profiles that reference the credential.
func referencesTo(tx *Tx, name string) ([]string, error) {
	profiles, err := tx.ListProfiles()
	if err != nil {
		return nil, err
	}
	var names []string
	for profileName, selected := range profiles {
		if selected.Credential == name {
			names = append(names, profileName)
		}
	}
	sort.Strings(names)
	return names, nil
}
