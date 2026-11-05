package profile

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestDescriptor_String_SaaS(t *testing.T) {
	d := Descriptor{Host: "acemagic-office", Port: 3389, ControlURL: ""}
	got := d.String()
	want := "tsb://acemagic-office:3389?cp=saas"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestDescriptor_String_Headscale(t *testing.T) {
	d := Descriptor{Host: "acemagic-office", Port: 45000, ControlURL: "https://vpn.example.com"}
	got := d.String()
	want := "tsb://acemagic-office:45000?cp=https%3A%2F%2Fvpn.example.com"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestDescriptor_String_NonDefaultPort(t *testing.T) {
	d := Descriptor{Host: "host", Port: 45000, ControlURL: ""}
	got := d.String()
	if got == "" {
		t.Fatal("String() returned empty")
	}
	if got == "tsb://host:3389?cp=saas" {
		t.Errorf("String() reported default port 3389 instead of 45000: %q", got)
	}
}

func TestParse_RoundTrip_SaaS(t *testing.T) {
	cases := []Descriptor{
		{Host: "acemagic-office", Port: 3389, ControlURL: ""},
		{Host: "acemagic-lab-1", Port: 45000, ControlURL: ""},
	}
	for _, want := range cases {
		raw := want.String()
		got, err := Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", raw, err)
		}
		if got.Host != want.Host || got.Port != want.Port || got.ControlURL != want.ControlURL {
			t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
		}
	}
}

func TestParse_RoundTrip_Headscale(t *testing.T) {
	want := Descriptor{Host: "host", Port: 45000, ControlURL: "https://vpn.example.com"}
	raw := want.String()
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", raw, err)
	}
	if got.Host != want.Host || got.Port != want.Port || got.ControlURL != want.ControlURL {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestParse_ToleratesUnknownQueryParams(t *testing.T) {
	raw := "tsb://host:3389?cp=saas&future=xyz&another=123"
	_, err := Parse(raw)
	if err != nil {
		t.Errorf("Parse with unknown params should succeed, got: %v", err)
	}
}

func TestParse_MalformedInputNeverPanics(t *testing.T) {
	cases := []string{
		"",
		"not-a-url",
		"http://host:3389",
		"tsb://",
		"tsb://host",
		"tsb://host:notaport",
		"tsb://:3389",
		"tsb://host:0",
		"tsb://host:-1",
		"tsb://host:99999",
	}
	for _, raw := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Parse(%q) panicked: %v", raw, r)
				}
			}()
			_, _ = Parse(raw)
		}()
	}
}

func TestParse_RejectsWrongScheme(t *testing.T) {
	_, err := Parse("http://host:3389?cp=saas")
	if err == nil {
		t.Error("expected error for wrong scheme, got nil")
	}
}

func TestParse_RejectsMissingCP(t *testing.T) {
	_, err := Parse("tsb://host:3389")
	if err == nil {
		t.Error("expected error when cp param is absent, got nil")
	}
}

func TestParse_RejectsPortZero(t *testing.T) {
	_, err := Parse("tsb://host:0?cp=saas")
	if err == nil {
		t.Error("expected error for port 0, got nil")
	}
}

func TestParse_RejectsPortOutOfRange(t *testing.T) {
	_, err := Parse("tsb://host:99999?cp=saas")
	if err == nil {
		t.Error("expected error for port > 65535, got nil")
	}
}

func TestParse_ErrorType(t *testing.T) {
	_, err := Parse("not-a-url")
	if err == nil {
		t.Fatal("expected error")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Errorf("expected *ParseError, got %T: %v", err, err)
	}
}

// Parse is the producer of the control URL that connect later consumes, so it
// refuses what connect and the host file refuse (#618).
func TestParseRejectsControlURLsConnectWouldRefuse(t *testing.T) {
	for _, raw := range []string{
		"tsb://office:3389?cp=ftp://hs.example",
		"tsb://office:3389?cp=hs.example",
		"tsb://office:3389?cp=https://user:secret@hs.example",
	} {
		t.Run(raw, func(t *testing.T) {
			if d, err := Parse(raw); err == nil {
				t.Fatalf("Parse(%q) accepted control URL %q", raw, d.ControlURL)
			}
		})
	}
}

// The error is printed by import and may be logged, so it must not carry the
// credentials a shared descriptor can hold (#618).
func TestParseErrorsDoNotEchoCredentials(t *testing.T) {
	for _, raw := range []string{
		"tsb://office:3389?cp=https://user:secret@hs.example",
		"tsb://office:3389?cp=ftp://user:secret@hs.example",
		"tsb://office:3389?cp=https:user:secret@hs.example",
		"tsb://office:3389?cp=user:secret@hs.example",
		"tsb://user:secret@office:3389?cp=saas&v=0",
		"tsb://user:secret@office:notaport?cp=saas",
		"http://user:secret@office:3389?cp=saas",
		"secret:x@office:3389?cp=saas",
		"tsb://office:3389?cp=https://user:secret%zz@hs.example",
		"tsb://office:3389?v=1&cp=https://user:secret%zz@hs.example",
		"tsb://user:secret@office%zz:3389?cp=saas",
		"tsb://office:3389?cp=saas&cp=https://user:secret@hs.example:99999",
		"tsb://office:3389?cp=https://user:secret@hs.example&cp=saas&v=0",
		"tsb://office:3389?cp=saas&v=https://user:secret@hs.example",
		"tsb://office:3389?cp=saas&v=1&v=https://user:secret@hs.example",
		"tsb://office:3389?cp=saas&future=user:secret@hs.example&v=0",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := Parse(raw)
			if err == nil {
				t.Fatal("Parse accepted the descriptor")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("error echoes the credential: %v", err)
			}
		})
	}
}

func TestParseAcceptsHeadscaleControlURLForms(t *testing.T) {
	for _, cp := range []string{"http://hs.lan:8080", "https://hs.example/", "saas"} {
		if _, err := Parse("tsb://office:3389?cp=" + url.QueryEscape(cp)); err != nil {
			t.Errorf("Parse rejected cp=%q: %v", cp, err)
		}
	}
}

// ParseError.Input keeps what locates the descriptor (scheme, host, port) and
// drops the rest: the query and the user info are where credentials live, and
// a per-parameter scrub is the patch the next parameter escapes (#618).
func TestParseErrorInputOmitsQueryAndUserInfo(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"tsb://user:secret@office:3389?cp=saas&v=0", "tsb://office:3389?\u2026"},
		{"tsb://office:3389?cp=https://user:secret@hs.example&cp=saas", "tsb://office:3389?\u2026"},
		{"http://office:3389", "http://office:3389"},
		{"tsb://office:3389/x?cp=ftp://hs.example", "tsb://office:3389/x?\u2026"},
		{"tsb://office:3389?cp=https://user:secret%zz@hs.example", "tsb://office:3389?\u2026"},
		{"secret:x@office:3389", "(unparseable)"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			var pe *ParseError
			_, err := Parse(tt.raw)
			if !errors.As(err, &pe) {
				t.Fatalf("Parse(%q) error = %v, want *ParseError", tt.raw, err)
			}
			if pe.Input != tt.want {
				t.Errorf("Input = %q, want %q", pe.Input, tt.want)
			}
		})
	}
}
