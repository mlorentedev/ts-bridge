//go:build windows

package credential

import (
	"reflect"
	"testing"
)

func TestHardenCredentialPathUsesOwnerOnlyWindowsACLs(t *testing.T) {
	t.Setenv("USERNAME", "test-user")
	original := runICACLS
	t.Cleanup(func() { runICACLS = original })

	var calls [][]string
	runICACLS = func(args ...string) error {
		calls = append(calls, append([]string(nil), args...))
		return nil
	}

	if err := hardenCredentialPath(`C:\store`, true); err != nil {
		t.Fatal(err)
	}
	if err := hardenCredentialPath(`C:\store\office.key`, false); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{`C:\store`, "/inheritance:r", "/grant:r", "test-user:(OI)(CI)F"},
		{`C:\store\office.key`, "/inheritance:r", "/grant:r", "test-user:F"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("icacls calls = %#v, want %#v", calls, want)
	}
}
