package cmd_test

import (
	"reflect"
	"testing"

	cmdpkg "ts-bridge/cmd/cli"
)

func TestNormalizeArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "verbose shorthand after subcommand does not corrupt preceding flag value",
			in:   []string{"ts-bridge", "connect", "--target", "100.64.0.1:22", "-v"},
			want: []string{"ts-bridge", "connect", "--target", "100.64.0.1:22", "--verbose"},
		},
		{
			name: "verbose shorthand before subcommand",
			in:   []string{"ts-bridge", "-v", "connect"},
			want: []string{"ts-bridge", "--verbose", "connect"},
		},
		{
			name: "no shorthand left untouched",
			in:   []string{"ts-bridge", "connect", "--target", "100.64.0.1:22"},
			want: []string{"ts-bridge", "connect", "--target", "100.64.0.1:22"},
		},
		{
			name: "program name only",
			in:   []string{"ts-bridge"},
			want: []string{"ts-bridge"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cmdpkg.NormalizeArgs(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeArgs(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeArgsDoesNotMutateInput(t *testing.T) {
	in := []string{"ts-bridge", "connect", "--target", "100.64.0.1:22", "-v"}
	cp := make([]string, len(in))
	copy(cp, in)

	_ = cmdpkg.NormalizeArgs(in)

	if !reflect.DeepEqual(in, cp) {
		t.Errorf("NormalizeArgs mutated its input: got %q, want %q", in, cp)
	}
}

func TestLegacyVersionRequested(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want bool
	}{
		{name: "single dash version", argv: []string{"ts-bridge", "-version"}, want: true},
		{name: "double dash version", argv: []string{"ts-bridge", "--version"}, want: true},
		{name: "version subcommand is not the legacy flag", argv: []string{"ts-bridge", "version"}, want: false},
		{name: "unrelated command", argv: []string{"ts-bridge", "connect", "--target", "x:22"}, want: false},
		{name: "program name only", argv: []string{"ts-bridge"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cmdpkg.LegacyVersionRequested(tt.argv); got != tt.want {
				t.Errorf("LegacyVersionRequested(%q) = %v, want %v", tt.argv, got, tt.want)
			}
		})
	}
}
