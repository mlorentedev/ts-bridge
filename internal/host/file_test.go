package host

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeHostFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "host.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }

func TestLoadFile_Schema(t *testing.T) {
	full := `version: 1
host:
  services:
    - kind: rdp
      port: 3390
  firewall:
    rule: FileRule
  power:
    prevent_sleep: false
  control_url: https://headscale.example
`
	tests := []struct {
		name     string
		content  string
		want     File
		wantErr  string
		wantWarn []string
	}{
		{
			name:    "full file",
			content: full,
			want: File{Version: 1, Host: FileHost{
				Services:   []FileService{{Kind: ServiceRDP, Port: intPtr(3390)}},
				Firewall:   &FileFirewall{Rule: strPtr("FileRule")},
				Power:      &FilePower{PreventSleep: boolPtr(false)},
				ControlURL: strPtr("https://headscale.example"),
			}},
		},
		{
			name:    "absent version is 1",
			content: "host:\n  services:\n    - kind: rdp\n",
			want:    File{Version: 1, Host: FileHost{Services: []FileService{{Kind: ServiceRDP}}}},
		},
		{
			name:    "empty file declares nothing",
			content: "",
			want:    File{Version: 1},
		},
		{
			name:    "unsupported version",
			content: "version: 2\n",
			wantErr: "reads version 1",
		},
		{
			name: "unknown keys warn and are ignored",
			content: `version: 1
theme: dark
host:
  extra: 1
  services:
    - kind: rdp
      label: office
  firewall:
    rule: R
    profile: any
  power:
    prevent_sleep: true
    hibernate: false
`,
			want: File{Version: 1, Host: FileHost{
				Services: []FileService{{Kind: ServiceRDP}},
				Firewall: &FileFirewall{Rule: strPtr("R")},
				Power:    &FilePower{PreventSleep: boolPtr(true)},
			}},
			wantWarn: []string{`"theme"`, `"host.extra"`, `"host.services[0].label"`, `"host.firewall.profile"`, `"host.power.hibernate"`},
		},
		{
			name:     "unknown keys behind an alias are warned",
			content:  "base: &h\n  services:\n    - kind: rdp\n      label: office\nhost: *h\n",
			want:     File{Version: 1, Host: FileHost{Services: []FileService{{Kind: ServiceRDP}}}},
			wantWarn: []string{`"base"`, `"host.services[0].label"`},
		},
		{
			name:     "a merge key is applied, not reported as ignored",
			content:  "base: &b\n  services:\n    - kind: rdp\n      label: x\nhost:\n  <<: *b\n",
			want:     File{Version: 1, Host: FileHost{Services: []FileService{{Kind: ServiceRDP}}}},
			wantWarn: []string{`"base"`, `"host.services[0].label"`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var warn bytes.Buffer
			got, err := LoadFile(writeHostFile(t, tc.content), "windows", &warn)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("LoadFile() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadFile() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("LoadFile() = %+v, want %+v", got, tc.want)
			}
			for _, w := range tc.wantWarn {
				if !strings.Contains(warn.String(), w) {
					t.Errorf("warnings %q do not name %s", warn.String(), w)
				}
			}
			if got := strings.Count(warn.String(), "WARNING:"); got != len(tc.wantWarn) {
				t.Errorf("got %d warnings, want one per unknown key (%d): %q", got, len(tc.wantWarn), warn.String())
			}
		})
	}

	t.Run("nil warning writer", func(t *testing.T) {
		if _, err := LoadFile(writeHostFile(t, "theme: dark\n"), "windows", nil); err != nil {
			t.Fatalf("LoadFile() error = %v", err)
		}
	})
}

func TestLoadFile_InvalidEntries(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr []string
	}{
		{"unknown kind", "host:\n  services:\n    - kind: ftp\n", []string{"host.services[0].kind", `"ftp"`, "rdp, ssh, vnc"}},
		{"missing kind", "host:\n  services:\n    - port: 3389\n", []string{"host.services[0].kind"}},
		{"port zero", "host:\n  services:\n    - kind: rdp\n      port: 0\n", []string{"host.services[0].port", "1 and 65535"}},
		{"port too high", "host:\n  services:\n    - kind: rdp\n      port: 70000\n", []string{"host.services[0].port", "70000"}},
		{"duplicate kind", "host:\n  services:\n    - kind: rdp\n    - kind: rdp\n      port: 3390\n", []string{"host.services[1].kind", "twice"}},
		{"empty services", "host:\n  services: []\n", []string{"host.services", "omit it"}},
		{"empty firewall rule", "host:\n  firewall:\n    rule: \"\"\n", []string{"host.firewall.rule", "empty"}},
		{"unsafe firewall rule", "host:\n  firewall:\n    rule: \"a;b\"\n", []string{"host.firewall.rule", "invalid characters"}},
		{"empty control_url", "host:\n  control_url: \"\"\n", []string{"host.control_url", "omit it"}},
		{"non-http control_url", "host:\n  control_url: ftp://x\n", []string{"host.control_url", "http or https"}},
		{"control_url without host", "host:\n  control_url: https://\n", []string{"host.control_url"}},
		{"control_url with only a port", "host:\n  control_url: http://:8080\n", []string{"host.control_url", "with a host"}},
		{"control_url with credentials", "host:\n  control_url: https://user:secret@hs.example\n", []string{"host.control_url", "credentials"}},
		{"two YAML documents", "host:\n  services:\n    - kind: rdp\n---\nhost:\n  control_url: https://hs.example\n", []string{"more than one YAML document"}},
		{"not YAML", "host: [\n", []string{"parse"}},
		{"services not a list", "host:\n  services: rdp\n", []string{"line 2"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadFile(writeHostFile(t, tc.content), "windows", &bytes.Buffer{})
			if err == nil {
				t.Fatal("LoadFile() error = nil, want an error")
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}

	t.Run("missing path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "absent.yaml")
		_, err := LoadFile(path, "linux", &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("LoadFile() error = %v, want it to name %s", err, path)
		}
	})

	t.Run("directory path", func(t *testing.T) {
		dir := t.TempDir()
		_, err := LoadFile(dir, "linux", &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), dir) {
			t.Fatalf("LoadFile() error = %v, want it to name %s", err, dir)
		}
	})
}

func TestLoadFile_OSCapability(t *testing.T) {
	svc := func(kind string) string { return "host:\n  services:\n    - kind: " + kind + "\n" }
	power := "host:\n  power:\n    prevent_sleep: true\n"
	firewall := "host:\n  firewall:\n    rule: R\n"
	tests := []struct {
		goos    string
		content string
		wantErr []string // empty: accepted
	}{
		{"windows", svc("rdp"), nil},
		{"linux", svc("rdp"), nil},
		{"windows", power, nil},
		{"windows", firewall, nil},
		{"linux", svc("ssh"), []string{"host.services[0].kind", `"ssh"`, "linux", "#454"}},
		{"linux", svc("vnc"), []string{"host.services[0].kind", `"vnc"`, "linux", "#454"}},
		{"windows", svc("ssh"), []string{"host.services[0].kind", `"ssh"`, "windows", "#455"}},
		{"windows", svc("vnc"), []string{"host.services[0].kind", `"vnc"`, "windows", "#455"}},
		{"darwin", svc("ssh"), []string{"host.services[0].kind", "darwin", "#459"}},
		{"darwin", svc("rdp"), []string{"darwin", "#459"}},
		{"linux", power, []string{"host.power", "linux"}},
		{"darwin", power, []string{"host.power", "darwin"}},
		{"linux", firewall, []string{"host.firewall", "linux"}},
		{"darwin", firewall, []string{"host.firewall", "darwin"}},
	}
	for _, tc := range tests {
		t.Run(tc.goos+"/"+strings.ReplaceAll(strings.TrimSpace(tc.content), "\n", " "), func(t *testing.T) {
			_, err := LoadFile(writeHostFile(t, tc.content), tc.goos, &bytes.Buffer{})
			if len(tc.wantErr) == 0 {
				if err != nil {
					t.Fatalf("LoadFile() error = %v, want accepted", err)
				}
				return
			}
			if err == nil {
				t.Fatal("LoadFile() error = nil, want a configuration error")
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}
