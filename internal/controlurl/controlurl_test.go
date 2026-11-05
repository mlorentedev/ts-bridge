package controlurl

import (
	"errors"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want error
	}{
		{"https", "https://hs.example", nil},
		{"plain http", "http://hs.lan", nil},
		{"port", "https://hs.example:8443", nil},
		{"trailing slash", "https://hs.example/", nil},
		{"path", "https://hs.example/headscale", nil},
		{"upper-case scheme", "HTTPS://hs.example", nil},
		{"ipv6 host", "https://[fd7a::1]:8443", nil},
		{"empty", "", ErrScheme},
		{"user and password", "https://user:secret@hs.example", ErrCredentials},
		{"user only", "https://token@hs.example", ErrCredentials},
		{"credentials on a bad scheme", "ftp://user:secret@hs.example", ErrCredentials},
		{"ftp", "ftp://hs.example", ErrScheme},
		{"no scheme", "hs.example", ErrScheme},
		{"no scheme with a port", "hs.example:8443", ErrScheme},
		{"no host", "https://", ErrScheme},
		{"only a port", "http://:8080", ErrScheme},
		{"opaque with credentials", "https:user:secret@hs.example", ErrScheme},
		{"no-scheme credentials", "user:secret@hs.example", ErrScheme},
		{"malformed", "https://user:secret@bad%zz", ErrNotURL},
		{"port zero", "https://hs.example:0", ErrPort},
		{"port out of range", "https://hs.example:99999", ErrPort},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.raw)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Validate(%q) = %v, want %v", tt.raw, err, tt.want)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Errorf("error echoes the credential: %v", err)
			}
		})
	}
}
