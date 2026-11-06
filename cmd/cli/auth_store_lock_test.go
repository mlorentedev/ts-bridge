package cmd

import (
	"testing"
	"time"

	"ts-bridge/internal/credential"
	"ts-bridge/internal/profile"
)

// A control URL rewritten while auth set waits for the key must not take the
// new key, and must not cost the stored one (#391; the #664 review): the
// command fails with the original credential still in place.
func TestAuthSetKeepsTheStoredKeyWhenTheControlURLMovesMidCommand(t *testing.T) {
	profilePath, credentialDir := setupAuthCommandTest(t)
	profiles := profile.NewStore(profilePath)
	if err := profiles.Set("a", "host-a:22", "https://p.example.com"); err != nil {
		t.Fatal(err)
	}
	const original = "tskey-auth-original-1"    // #nosec G101 -- synthetic test credential
	const replacement = "tskey-auth-replaced-1" // #nosec G101 -- synthetic test credential
	authSecretReader = func(string) (string, error) { return original, nil }
	if _, _, err := executeAuthCommand(t, "set", "k", "--profile", "a"); err != nil {
		t.Fatal(err)
	}

	authSecretReader = func(string) (string, error) {
		selected, err := profiles.Get("a")
		if err != nil {
			t.Fatal(err)
		}
		selected.ControlURL = "https://q.example.com"
		if _, err := profiles.Configure("a", selected, true); err != nil {
			t.Fatal(err)
		}
		return replacement, nil
	}
	if _, _, err := executeAuthCommand(t, "set", "k", "--profile", "a", "--force"); err == nil {
		t.Fatal("auth set succeeded after the profile moved control planes mid-command")
	}
	if got, _ := credential.NewStore(credentialDir).Get("k"); got != original {
		t.Fatalf("credential k = %q after the refusal, want the original key kept", got)
	}
	if a, _ := profiles.Get("a"); a.Credential != "" {
		t.Fatalf("profile a references %q after the refusal, want no reference", a.Credential)
	}
}

// holdStoreLock runs a transition that keeps the store locked until the
// returned release is called, and waits until it holds the lock.
func holdStoreLock(t *testing.T, profilePath string) (release func()) {
	t.Helper()
	entered, done, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		finished <- profile.NewStore(profilePath).Update(func(*profile.Tx) error {
			close(entered)
			<-done
			return nil
		})
	}()
	<-entered
	var once bool
	release = func() {
		if once {
			return
		}
		once = true
		close(done)
		if err := <-finished; err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(release)
	return release
}

// Mutating commands queue behind a held store lock, so they run after the
// holder's transition and not inside it; read-only commands do not wait.
func TestAuthCommandsHonourTheStoreLock(t *testing.T) {
	profilePath, credentialDir := setupAuthCommandTest(t)
	profiles := profile.NewStore(profilePath)
	for _, name := range []string{"office", "backup"} {
		if err := profiles.Set(name, name+":22", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := profiles.SetCredential("backup", "old"); err != nil {
		t.Fatal(err)
	}
	if err := credential.NewStore(credentialDir).Set("old", "tskey-auth-old-1", false); err != nil {
		t.Fatal(err)
	}
	authSecretReader = func(string) (string, error) { return "tskey-auth-new-1", nil } // #nosec G101 -- synthetic test credential
	release := holdStoreLock(t, profilePath)

	for _, args := range [][]string{{"list"}, {"status", "--profile", "backup"}} {
		if _, _, err := executeAuthCommand(t, args...); err != nil {
			t.Fatalf("auth %v while the store is locked: %v", args, err)
		}
	}

	finished := make(chan error, 2)
	go func() { _, _, err := executeAuthCommand(t, "set", "new", "--profile", "office"); finished <- err }()
	go func() { _, _, err := executeAuthCommand(t, "remove", "--profile", "backup"); finished <- err }()
	select {
	case err := <-finished:
		t.Fatalf("a mutating command finished while the store was locked: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	release()
	for range 2 {
		if err := <-finished; err != nil {
			t.Fatalf("queued command failed after the lock was released: %v", err)
		}
	}
	if got, _ := profiles.Get("office"); got.Credential != "new" {
		t.Fatalf("office references %q, want new", got.Credential)
	}
	if got, _ := profiles.Get("backup"); got.Credential != "" {
		t.Fatalf("backup still references %q", got.Credential)
	}
}
