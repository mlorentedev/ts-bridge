package profile

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ts-bridge/internal/oslock"
)

var (
	errProfileInUse = errors.New("already connected; close this profile's existing ts-bridge terminal with Ctrl+C before retrying")
	errLockHeld     = oslock.ErrHeld
	errStoreBusy    = errors.New("another ts-bridge command is updating the profile store; retry in a moment")
)

// storeLockWait bounds how long a mutation waits for the store lock. It is a
// variable so tests can shorten it.
var storeLockWait = 10 * time.Second

const storeLockPoll = 25 * time.Millisecond

// Lock acquires exclusive ownership until the returned file or process closes.
// The lock file must not be removed: other processes may already have it open.
func (s *Store) Lock(name string) (*os.File, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("profile name must not be empty")
	}
	file, err := s.tryLock(fmt.Sprintf("%x.lock", sha256.Sum256([]byte(name))))
	if errors.Is(err, errLockHeld) {
		err = errProfileInUse
	}
	if err != nil {
		return nil, fmt.Errorf("lock profile %q: %w", name, err)
	}
	return file, nil
}

// lockStore takes the per-user lock that serializes every mutation of the
// profile store and the credentials it references, waiting up to storeLockWait
// for another command to finish. Reads never take it. The kernel drops the lock
// when its holder exits, so a crashed command cannot leave it stuck.
func (s *Store) lockStore() (*os.File, error) {
	deadline := time.Now().Add(storeLockWait)
	for {
		file, err := s.tryLock("store.lock")
		if !errors.Is(err, errLockHeld) {
			if err != nil {
				return nil, fmt.Errorf("lock profile store: %w", err)
			}
			return file, nil
		}
		if !time.Now().Before(deadline) {
			return nil, errStoreBusy
		}
		time.Sleep(storeLockPoll)
	}
}

// tryLock opens the named lock file beside the store and locks it without
// waiting, returning errLockHeld when another holder has it.
func (s *Store) tryLock(fileName string) (*os.File, error) {
	directory := filepath.Join(filepath.Dir(s.path), "locks")
	// #nosec G703 -- root is the configured profile store; the directory suffix is fixed.
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	// #nosec G304 G703 -- configured store root and a fixed or SHA-256 filename, never a path-interpreted profile name.
	file, err := os.OpenFile(filepath.Join(directory, fileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := oslock.Try(file); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}
