package profile

import (
	"strings"
	"testing"
)

func TestParse_AbsentVersionDefaultsToOne(t *testing.T) {
	d, err := Parse("tsb://host:3389?cp=saas")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Version != 1 {
		t.Errorf("expected Version=1, got %d", d.Version)
	}
}

func TestParse_ExplicitVersionOne(t *testing.T) {
	d, err := Parse("tsb://host:3389?cp=saas&v=1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Version != 1 {
		t.Errorf("expected Version=1, got %d", d.Version)
	}
}

func TestParse_VersionPreservedInRoundTrip(t *testing.T) {
	want := Descriptor{Host: "h", Port: 3389, ControlURL: "", Version: 2}
	raw := want.String()
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", raw, err)
	}
	if got.Version != 2 {
		t.Errorf("Version not preserved in round-trip: got %d, want 2", got.Version)
	}
}

func TestString_OmitsVersionWhenOne(t *testing.T) {
	d := Descriptor{Host: "h", Port: 3389, Version: 1}
	raw := d.String()
	// v param must not appear for version 1 (keeps common case compact)
	if contains(raw, "v=1") {
		t.Errorf("String() should omit v=1, got %q", raw)
	}
}

func TestString_IncludesVersionWhenAboveOne(t *testing.T) {
	d := Descriptor{Host: "h", Port: 3389, Version: 2}
	raw := d.String()
	if !contains(raw, "v=2") {
		t.Errorf("String() should include v=2 for Version=2, got %q", raw)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
