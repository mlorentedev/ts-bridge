package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ts-bridge/internal/credential"
)

const (
	keyOriginal    = "tskey-auth-original-1" // #nosec G101 -- synthetic test credential
	keyReplacement = "tskey-auth-replaced-1" // #nosec G101 -- synthetic test credential
)

type transitionFixture struct {
	path     string
	store    *Store
	creds    *credential.Store
	credsDir string
}

func newTransitionFixture(t *testing.T, profiles map[string]string) transitionFixture {
	t.Helper()
	f := transitionFixture{path: filepath.Join(t.TempDir(), "profiles.yaml"), credsDir: filepath.Join(t.TempDir(), "credentials")}
	f.store, f.creds = NewStore(f.path), credential.NewStore(f.credsDir)
	for name, controlURL := range profiles {
		if err := f.store.Set(name, name+":22", controlURL); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f transitionFixture) key(t *testing.T, name string) string {
	t.Helper()
	key, err := f.creds.Get(name)
	if err != nil {
		return ""
	}
	return key
}

func (f transitionFixture) reference(t *testing.T, profile string) string {
	t.Helper()
	got, err := f.store.Get(profile)
	if err != nil {
		t.Fatal(err)
	}
	return got.Credential
}

func TestBindCredentialStoresTheKeyAndReferencesIt(t *testing.T) {
	f := newTransitionFixture(t, map[string]string{"office": "https://p.example.com"})
	err := f.store.BindCredential(f.creds, Binding{Profile: "office", Credential: "k", Key: keyOriginal, ControlURL: "https://p.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.key(t, "k"); got != keyOriginal {
		t.Fatalf("stored key = %q", got)
	}
	if got := f.reference(t, "office"); got != "k" {
		t.Fatalf("profile references %q, want k", got)
	}
}

// The control URL moving between the caller's read and the transition (#391,
// the #664 review) is refused before the stored key is replaced.
func TestBindCredentialRefusesAMovedControlPlaneBeforeReplacingTheKey(t *testing.T) {
	f := newTransitionFixture(t, map[string]string{"office": "https://p.example.com"})
	if err := f.store.BindCredential(f.creds, Binding{Profile: "office", Credential: "k", Key: keyOriginal, ControlURL: "https://p.example.com"}); err != nil {
		t.Fatal(err)
	}
	selected, err := f.store.Get("office")
	if err != nil {
		t.Fatal(err)
	}
	selected.ControlURL = "https://q.example.com"
	if _, err := f.store.Configure("office", selected, true); err != nil {
		t.Fatal(err)
	}

	err = f.store.BindCredential(f.creds, Binding{Profile: "office", Credential: "k", Key: keyReplacement, Replace: true, ControlURL: "https://p.example.com"})
	if err == nil || !strings.Contains(err.Error(), "q.example.com") {
		t.Fatalf("BindCredential() = %v, want a refusal naming the new control plane", err)
	}
	if got := f.key(t, "k"); got != keyOriginal {
		t.Fatalf("stored key = %q after the refusal, want the original kept", got)
	}
	if got := f.reference(t, "office"); got != "" {
		t.Fatalf("profile references %q after the refusal", got)
	}
}

// A name another profile references on a different control plane is refused
// inside the transition, before the stored key is replaced (#653).
func TestBindCredentialRefusesASharedNameBeforeReplacingTheKey(t *testing.T) {
	f := newTransitionFixture(t, map[string]string{"office": "https://p.example.com", "home": ""})
	if err := f.store.BindCredential(f.creds, Binding{Profile: "office", Credential: "k", Key: keyOriginal, ControlURL: "https://p.example.com"}); err != nil {
		t.Fatal(err)
	}

	err := f.store.BindCredential(f.creds, Binding{Profile: "home", Credential: "k", Key: keyReplacement, Replace: true})
	var shared *CredentialSharedError
	if !errors.As(err, &shared) || shared.Profile != "office" {
		t.Fatalf("BindCredential(home, k) = %v, want a CredentialSharedError naming office", err)
	}
	if got := f.key(t, "k"); got != keyOriginal {
		t.Fatalf("stored key = %q after the refusal, want the original kept", got)
	}
	if got := f.reference(t, "home"); got != "" {
		t.Fatalf("profile home references %q after the refusal", got)
	}
}

func TestBindCredentialRefusesAHeadscaleKeyForTheSaaSControlPlane(t *testing.T) {
	f := newTransitionFixture(t, map[string]string{"office": ""})
	err := f.store.BindCredential(f.creds, Binding{Profile: "office", Credential: "k", Key: "hskey-auth-wrong-plane"})
	if err == nil || !strings.Contains(err.Error(), "custom control URL") {
		t.Fatalf("BindCredential() = %v, want a control-plane mismatch", err)
	}
	if got := f.key(t, "k"); got != "" {
		t.Fatalf("a refused binding stored %q", got)
	}
}

func TestBindCredentialUnknownProfileIsNotFoundAndStoresNothing(t *testing.T) {
	f := newTransitionFixture(t, nil)
	err := f.store.BindCredential(f.creds, Binding{Profile: "ghost", Credential: "k", Key: keyOriginal})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("BindCredential() = %v, want ErrNotFound", err)
	}
	if got := f.key(t, "k"); got != "" {
		t.Fatalf("a refused binding stored %q", got)
	}
}

// The reference write failing after the key was stored undoes the key: a
// replaced key comes back, a new one is removed (#391, association failure).
func TestBindCredentialUndoesTheKeyWhenTheAssociationFails(t *testing.T) {
	tests := []struct {
		name     string
		existing bool
		want     string
	}{
		{"replacement restores the previous key", true, keyOriginal},
		{"new credential is removed", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTransitionFixture(t, map[string]string{"office": ""})
			if tt.existing {
				if err := f.creds.Set("k", keyOriginal, false); err != nil {
					t.Fatal(err)
				}
			}
			f.store.rename = func(string, string) error { return errors.New("disk gone") }

			err := f.store.BindCredential(f.creds, Binding{Profile: "office", Credential: "k", Key: keyReplacement, Replace: true})
			if err == nil || !strings.Contains(err.Error(), "association") {
				t.Fatalf("BindCredential() = %v, want an association failure", err)
			}
			if got := f.key(t, "k"); got != tt.want {
				t.Fatalf("credential k = %q after the failed association, want %q", got, tt.want)
			}
			f.store.rename = renameWithRetry
			if got := f.reference(t, "office"); got != "" {
				t.Fatalf("profile references %q after the failed association", got)
			}
		})
	}
}

// Without --force two creators of one name must not both pass the absence
// check: exactly one wins and the others keep out of the store.
func TestBindCredentialExclusiveCreationHasOneWinner(t *testing.T) {
	const writers = 8
	profiles := make(map[string]string, writers)
	for i := range writers {
		profiles[fmt.Sprintf("p%d", i)] = ""
	}
	f := newTransitionFixture(t, profiles)

	var wg sync.WaitGroup
	results := make(chan error, writers)
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			binding := Binding{Profile: fmt.Sprintf("p%d", i), Credential: "k", Key: fmt.Sprintf("tskey-auth-writer%d-1", i)}
			results <- NewStore(f.path).BindCredential(credential.NewStore(f.credsDir), binding)
		}()
	}
	wg.Wait()
	close(results)

	winners := 0
	for err := range results {
		switch {
		case err == nil:
			winners++
		case !errors.Is(err, credential.ErrCredentialExists):
			t.Fatalf("loser failed with %v, want ErrCredentialExists", err)
		}
	}
	if winners != 1 {
		t.Fatalf("%d writers created credential k, want exactly one", winners)
	}
	references := 0
	for i := range writers {
		if f.reference(t, fmt.Sprintf("p%d", i)) == "k" {
			references++
		}
	}
	if references != 1 {
		t.Fatalf("%d profiles reference k, want the winner only", references)
	}
}

// A reference added while a removal waits for the lock is seen by the removal:
// the shared credential is unlinked, not deleted (#391, reference change).
func TestUnbindCredentialSeesAReferenceAddedWhileItWaits(t *testing.T) {
	f := newTransitionFixture(t, map[string]string{"office": "", "backup": ""})
	if err := f.store.BindCredential(f.creds, Binding{Profile: "office", Credential: "k", Key: keyOriginal}); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	updated := make(chan error, 1)
	go func() {
		updated <- f.store.Update(func(tx *Tx) error {
			close(entered)
			<-release
			return tx.SetCredential("backup", "k")
		})
	}()
	<-entered

	type result struct {
		removal RemoveResult
		err     error
	}
	removed := make(chan result, 1)
	go func() {
		removal, err := NewStore(f.path).UnbindCredential(f.creds, Removal{Profile: "office"})
		removed <- result{removal, err}
	}()
	time.Sleep(100 * time.Millisecond)
	close(release)
	if err := <-updated; err != nil {
		t.Fatal(err)
	}
	got := <-removed
	if got.err != nil || got.removal != (RemoveResult{"k", Unlinked}) {
		t.Fatalf("UnbindCredential() = %v, %v; want k Unlinked", got.removal, got.err)
	}
	if key := f.key(t, "k"); key != keyOriginal {
		t.Fatalf("shared credential = %q after the unlink, want it kept", key)
	}
	if f.reference(t, "office") != "" || f.reference(t, "backup") != "k" {
		t.Fatal("references after the unlink: want office cleared, backup kept")
	}
}

func TestUnbindCredentialRemovesTheKeyOfTheLastReference(t *testing.T) {
	f := newTransitionFixture(t, map[string]string{"office": ""})
	if err := f.store.BindCredential(f.creds, Binding{Profile: "office", Credential: "k", Key: keyOriginal}); err != nil {
		t.Fatal(err)
	}
	result, err := f.store.UnbindCredential(f.creds, Removal{Profile: "office"})
	if err != nil || result != (RemoveResult{"k", RemovedWithProfile}) {
		t.Fatalf("UnbindCredential() = %v, %v; want k RemovedWithProfile", result, err)
	}
	if f.key(t, "k") != "" || f.reference(t, "office") != "" {
		t.Fatal("the key or its reference survived the removal")
	}
}

func TestUnbindCredentialForceClearsEveryReference(t *testing.T) {
	f := newTransitionFixture(t, map[string]string{"office": "", "backup": ""})
	for _, name := range []string{"office", "backup"} {
		if err := f.store.BindCredential(f.creds, Binding{Profile: name, Credential: "k", Key: keyOriginal, Replace: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.store.UnbindCredential(f.creds, Removal{Credential: "k"}); err == nil {
		t.Fatal("removing a referenced credential without force succeeded")
	}
	result, err := f.store.UnbindCredential(f.creds, Removal{Credential: "k", Force: true})
	if err != nil || result != (RemoveResult{"k", Removed}) {
		t.Fatalf("UnbindCredential(force) = %v, %v; want k Removed", result, err)
	}
	if f.key(t, "k") != "" || f.reference(t, "office") != "" || f.reference(t, "backup") != "" {
		t.Fatal("the key or a reference survived the forced removal")
	}
}

// Deleting the key fails: nothing else changes, so no secret the user believes
// deleted is left behind with its reference cleared (#391, deletion failure).
func TestUnbindCredentialKeepsReferencesWhenTheKeyCannotBeDeleted(t *testing.T) {
	f := newTransitionFixture(t, map[string]string{"office": ""})
	if err := f.store.BindCredential(f.creds, Binding{Profile: "office", Credential: "k", Key: keyOriginal}); err != nil {
		t.Fatal(err)
	}
	// A non-empty directory in the key's place cannot be removed on any OS.
	keyPath := filepath.Join(f.credsDir, "k.key")
	if err := os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(keyPath, "x"), 0o700); err != nil {
		t.Fatal(err)
	}

	if _, err := f.store.UnbindCredential(f.creds, Removal{Profile: "office"}); err == nil {
		t.Fatal("UnbindCredential() succeeded although the key could not be deleted")
	}
	if got := f.reference(t, "office"); got != "k" {
		t.Fatalf("reference = %q after the failed deletion, want k kept", got)
	}
	if _, err := f.store.UnbindCredential(f.creds, Removal{Credential: "k", Force: true}); err == nil {
		t.Fatal("forced UnbindCredential() succeeded although the key could not be deleted")
	}
	if got := f.reference(t, "office"); got != "k" {
		t.Fatalf("reference = %q after the failed forced deletion, want k kept", got)
	}
}

// A removal interrupted after the key was deleted leaves a dangling reference;
// running it again clears the reference instead of failing on the missing key.
func TestUnbindCredentialRerunClearsAReferenceToADeletedKey(t *testing.T) {
	f := newTransitionFixture(t, map[string]string{"office": ""})
	if err := f.store.SetCredential("office", "k"); err != nil {
		t.Fatal(err)
	}
	for _, removal := range []Removal{{Profile: "office"}, {Credential: "k", Force: true}} {
		if err := f.store.SetCredential("office", "k"); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.UnbindCredential(f.creds, removal); err != nil {
			t.Fatalf("UnbindCredential(%+v) with the key already gone: %v", removal, err)
		}
		if got := f.reference(t, "office"); got != "" {
			t.Fatalf("reference = %q after the rerun, want it cleared", got)
		}
	}
}
