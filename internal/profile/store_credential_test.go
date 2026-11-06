package profile

import (
	"errors"
	"path/filepath"
	"testing"
)

const officeControl = "https://hs.office.example"

// credentialedStore returns a store holding profile "office" bound to a
// credential on officeControl.
func credentialedStore(t *testing.T) *Store {
	t.Helper()
	store := NewStore(filepath.Join(t.TempDir(), "profiles.yaml"))
	if err := store.Set("office", "acemagic-office:45000", officeControl); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCredential("office", "office-key"); err != nil {
		t.Fatal(err)
	}
	return store
}

// Every path that can change a profile's control URL must refuse to carry the
// managed credential to a control plane it was not issued for, and --force
// must drop the reference instead.
func TestControlURLChangeOfCredentialedProfile(t *testing.T) {
	writers := map[string]func(s *Store, controlURL string, force bool) (bool, error){
		"import": func(s *Store, controlURL string, force bool) (bool, error) {
			return s.Import("office", Descriptor{Host: "acemagic-office", Port: 45000, ControlURL: controlURL, Version: 1}, force)
		},
		"configure": func(s *Store, controlURL string, force bool) (bool, error) {
			return s.Configure("office", Profile{Target: "acemagic-office:45000", ControlURL: controlURL}, force)
		},
		"set": func(s *Store, controlURL string, force bool) (bool, error) {
			return false, s.Set("office", "acemagic-office:45000", controlURL)
		},
	}
	tests := []struct {
		name        string
		controlURL  string
		force       bool
		wantRefused bool
		wantCleared bool
	}{
		{name: "same URL keeps credential", controlURL: officeControl},
		{name: "equivalent URL keeps credential", controlURL: "HTTPS://HS.Office.Example/"},
		{name: "other URL refuses", controlURL: "https://evil.example", wantRefused: true},
		{name: "SaaS refuses", controlURL: "", wantRefused: true},
		{name: "other URL with force clears credential", controlURL: "https://evil.example", force: true, wantCleared: true},
	}
	for writerName, write := range writers {
		for _, tt := range tests {
			t.Run(writerName+"/"+tt.name, func(t *testing.T) {
				if writerName == "set" && tt.force {
					t.Skip("Set has no force form")
				}
				store := credentialedStore(t)
				cleared, err := write(store, tt.controlURL, tt.force)
				var changeErr *ControlPlaneChangeError
				if refused := errors.As(err, &changeErr); refused != tt.wantRefused {
					t.Fatalf("refused = %v (err %v), want %v", refused, err, tt.wantRefused)
				}
				if !tt.wantRefused && err != nil {
					t.Fatal(err)
				}
				if writerName != "set" && cleared != tt.wantCleared {
					t.Fatalf("cleared = %v, want %v", cleared, tt.wantCleared)
				}
				assertCredentialBinding(t, store, tt)
			})
		}
	}
}

func assertCredentialBinding(t *testing.T, store *Store, tt struct {
	name        string
	controlURL  string
	force       bool
	wantRefused bool
	wantCleared bool
}) {
	t.Helper()
	got, err := store.Get("office")
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case tt.wantRefused:
		if got.ControlURL != officeControl || got.Credential != "office-key" {
			t.Fatalf("refused update changed the profile: %+v", got)
		}
	case tt.wantCleared:
		if got.Credential != "" || got.ControlURL != tt.controlURL {
			t.Fatalf("forced move kept credential %q on %q", got.Credential, got.ControlURL)
		}
	default:
		if got.Credential != "office-key" {
			t.Fatalf("credential lost on an unchanged control plane: %+v", got)
		}
	}
}

// A profile without a credential moves freely: there is nothing to protect.
func TestControlURLChangeOfUncredentialedProfile(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "profiles.yaml"))
	if err := store.Set("office", "acemagic-office:45000", officeControl); err != nil {
		t.Fatal(err)
	}
	cleared, err := store.Import("office", Descriptor{Host: "acemagic-office", Port: 45000, ControlURL: "https://other.example", Version: 1}, false)
	if err != nil || cleared {
		t.Fatalf("Import = (%v, %v), want (false, nil)", cleared, err)
	}
	got, err := store.Get("office")
	if err != nil {
		t.Fatal(err)
	}
	if got.ControlURL != "https://other.example" {
		t.Fatalf("ControlURL = %q, want %q", got.ControlURL, "https://other.example")
	}
}

func TestSameControlURL(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"", "", true},
		{"", "https://hs.example", false},
		{"https://hs.example", "https://hs.example/", true},
		{" https://HS.example ", "https://hs.example", true},
		{"https://hs.example", "http://hs.example", false},
		{"https://hs.example/a", "https://hs.example/A", false},
		{"https://hs.example", "https://hs.example:8443", false},
		// A default port names the same plane as no port.
		{"https://hs.example", "https://hs.example:443", true},
		{"http://hs.example", "http://hs.example:80", true},
		{"https://hs.example", "https://hs.example:80", false},
		{"https://[::1]:443", "https://[::1]", true},
		{"", "https://controlplane.tailscale.com:443", true},
		// Empty means Tailscale SaaS, so its own URLs name the same plane.
		{"", "https://controlplane.tailscale.com", true},
		{"", "https://Login.Tailscale.com/", true},
		{"", "http://controlplane.tailscale.com", false},
		{"", "https://controlplane.tailscale.com/x", false},
	}
	for _, tt := range tests {
		t.Run(tt.a+"|"+tt.b, func(t *testing.T) {
			if got := SameControlURL(tt.a, tt.b); got != tt.want {
				t.Fatalf("SameControlURL(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
