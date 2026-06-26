package profile

import (
	"errors"
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
