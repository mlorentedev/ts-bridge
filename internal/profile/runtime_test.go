package profile

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProfileRuntimePreserved(t *testing.T) {
	tests := []struct {
		name   string
		update func(*Store) error
	}{
		{"read", func(*Store) error { return nil }},
		{"set target", func(s *Store) error { return s.Set("work", "updated:443", "https://vpn.example.com") }},
		{"import descriptor", func(s *Store) error { return s.Import("work", Descriptor{Host: "imported", Port: 443}) }},
		{"set credential", func(s *Store) error { return s.SetCredential("work", "replacement") }},
		{"clear credential", func(s *Store) error { return s.ClearCredential("work") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profiles.yaml")
			source := `descriptor_version: 1
profiles:
  work:
    target: mesh:443
    credential: work-key
    bootstrap_ssh: bastion
    bootstrap_socks_addr: 127.0.0.1:1056
    hostname: work-bridge
    state_dir: /user/state/work
    local_addr: 127.0.0.1:34443
    health_addr: 127.0.0.1:9081
`
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			store := NewStore(path)
			if err := tt.update(store); err != nil {
				t.Fatal(err)
			}
			got, err := store.Get("work")
			if err != nil {
				t.Fatal(err)
			}
			raw, err := yaml.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]string
			if err := yaml.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			want := map[string]string{
				"bootstrap_ssh":        "bastion",
				"bootstrap_socks_addr": "127.0.0.1:1056",
				"hostname":             "work-bridge",
				"state_dir":            "/user/state/work",
				"local_addr":           "127.0.0.1:34443",
				"health_addr":          "127.0.0.1:9081",
			}
			for key, value := range want {
				if fields[key] != value {
					t.Errorf("%s = %q, want %q", key, fields[key], value)
				}
			}
		})
	}
}
