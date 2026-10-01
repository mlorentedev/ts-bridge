package credential

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStoreSetGetAndList(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "credentials")
	store := NewStore(dir)

	if err := store.Set("office", "tskey-auth-office-secret", false); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := store.Set("kubelab", "hskey-auth-kubelab", false); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := store.Get("office")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != "tskey-auth-office-secret" {
		t.Fatal("Get() returned the wrong credential")
	}

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 2 || entries[0].Name != "kubelab" || entries[1].Name != "office" {
		t.Fatalf("List() = %#v, want sorted credential names", entries)
	}
}

func TestStoreRefusesOverwriteUnlessExplicit(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "credentials"))
	if err := store.Set("office", "tskey-auth-old-secret", false); err != nil {
		t.Fatal(err)
	}

	err := store.Set("office", "tskey-auth-new-secret", false)
	if !errors.Is(err, ErrCredentialExists) {
		t.Fatalf("Set() error = %v, want ErrCredentialExists", err)
	}
	got, getErr := store.Get("office")
	if getErr != nil || got != "tskey-auth-old-secret" {
		t.Fatalf("refused overwrite changed the stored key: key=%q err=%v", got, getErr)
	}

	if err := store.Set("office", "tskey-auth-new-secret", true); err != nil {
		t.Fatalf("forced Set() error = %v", err)
	}
	got, getErr = store.Get("office")
	if getErr != nil || got != "tskey-auth-new-secret" {
		t.Fatalf("forced overwrite did not replace key: key=%q err=%v", got, getErr)
	}
}

func TestStoreValidatesNamesAndKeysWithoutLeakingValues(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "credentials"))

	for _, name := range []string{"", "../office", `..\office`, "two words", ".hidden", "a/b"} {
		t.Run("name="+name, func(t *testing.T) {
			if err := store.Set(name, "tskey-auth-test-secret", false); err == nil {
				t.Fatalf("Set(%q) expected name validation error", name)
			}
		})
	}

	for _, key := range []string{
		"",
		"not-a-key",
		"tskey-api-id-secret",
		"tskey-client-id-secret",
		"tskey-auth",
		"tskey-foo",
		"tskey-auth-id-secret-auth-id-secret",
		"tskey-auth-one-secret\nsecond",
		"hskey-auth-one\rsecond",
	} {
		t.Run("invalid-key", func(t *testing.T) {
			err := store.Set("office", key, false)
			if err == nil {
				t.Fatal("Set() expected key validation error")
			}
			if key != "" && strings.Contains(err.Error(), key) {
				t.Fatal("validation error leaked the credential value")
			}
		})
	}
}

func TestStoreRemove(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "credentials"))
	if err := store.Set("office", "tskey-auth-test-secret", false); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove("office"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := store.Get("office"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Get() after remove error = %v, want os.ErrNotExist", err)
	}
}

func TestStoreUsesOwnerOnlyPermissionsOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows ACL behavior is covered by the Windows-specific test")
	}
	dir := filepath.Join(t.TempDir(), "credentials")
	store := NewStore(dir)
	if err := store.Set("office", "tskey-auth-test-secret", false); err != nil {
		t.Fatal(err)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("credential directory mode = %04o, want 0700", got)
	}
	fileInfo, err := os.Stat(filepath.Join(dir, "office.key"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("credential file mode = %04o, want 0600", got)
	}
}
