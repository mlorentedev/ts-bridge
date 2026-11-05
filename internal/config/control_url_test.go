package config

import (
	"strings"
	"testing"
)

// A control URL carrying user info is refused here so connect never prints it
// in the banner, the bootstrap log line or tsnet init errors, which land in the
// JSON log file (#618).
func TestValidateControlPlaneRejectsUserInfo(t *testing.T) {
	if err := ValidateControlPlane("tskey-auth-secret", "https://user:secret@hs.example"); err == nil {
		t.Fatal("ValidateControlPlane accepted a control URL carrying credentials")
	}
}

// The refusal names the problem, never the URL: it may carry credentials, and
// the error is printed and logged (#618).
func TestValidateControlPlaneErrorsDoNotEchoTheURL(t *testing.T) {
	for _, raw := range []string{
		"https://user:secret@hs.example",
		"ftp://user:secret@hs.example",
		"user:secret@hs.example",
		"https:user:secret@hs.example",
		"https://user:secret@bad%zz",
	} {
		t.Run(raw, func(t *testing.T) {
			err := ValidateControlPlane("tskey-auth-x", raw)
			if err == nil {
				t.Fatal("ValidateControlPlane accepted the URL")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("error echoes the credential: %v", err)
			}
		})
	}
}

// Forms operators already use keep working: plain-http Headscale, a trailing
// slash, a path and a port.
func TestValidateControlPlaneAcceptsExistingForms(t *testing.T) {
	for _, raw := range []string{
		"https://headscale.example.com",
		"http://headscale.lan:8080",
		"https://headscale.example.com/",
		"HTTPS://Headscale.Example.com/prefix",
		"  https://headscale.example.com  ",
	} {
		t.Run(raw, func(t *testing.T) {
			if err := ValidateControlPlane("hskey-x", raw); err != nil {
				t.Errorf("ValidateControlPlane rejected %q: %v", raw, err)
			}
		})
	}
}
