package profile

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// Tx is the store while its mutation lock is held. A function given to Update
// changes the store only through it: the public mutators lock for themselves,
// and the lock does not nest.
type Tx struct {
	store *Store
}

// Update runs fn with the store locked against every other mutating ts-bridge
// process, so the reads and writes in fn form one transition. Reads outside
// Update stay available meanwhile and see the store before or after fn, as each
// write replaces the file whole.
func (s *Store) Update(fn func(*Tx) error) (err error) {
	lock, err := s.lockStore()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := lock.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("release profile store lock: %w", closeErr))
		}
	}()
	return fn(&Tx{store: s})
}

// Get returns the named profile as it is now, inside the lock.
func (t *Tx) Get(name string) (Profile, error) {
	return t.store.Get(name)
}

// ListProfiles returns every profile as it is now, inside the lock.
func (t *Tx) ListProfiles() (map[string]Profile, error) {
	return t.store.ListProfiles()
}

// SetCredential associates a managed credential name with an existing profile.
// It refuses a name another profile references on a different control plane.
func (t *Tx) SetCredential(profileName, credentialName string) error {
	return t.store.associate(profileName, credentialName)
}

// CheckCredentialShare is Store.CheckCredentialShare inside the lock.
func (t *Tx) CheckCredentialShare(profileName, credentialName string) error {
	return t.store.CheckCredentialShare(profileName, credentialName)
}

// ClearCredential removes the managed credential reference from a profile.
func (t *Tx) ClearCredential(profileName string) error {
	return t.store.updateCredential(profileName, "")
}

// renameWithRetry replaces the store file, retrying briefly: on Windows the
// rename fails while a reader has the file open, and every command reads it.
var renameWithRetry = retrying(os.Rename)

func retrying(rename func(string, string) error) func(string, string) error {
	return func(from, to string) error {
		var err error
		for attempt := range 5 {
			if err = rename(from, to); err == nil {
				return nil
			}
			time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
		}
		return err
	}
}
