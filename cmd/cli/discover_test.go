package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ts-bridge/internal/discover"
)

func TestFilterDevices(t *testing.T) {
	devices := []discover.Device{
		{Hostname: "acemagic-lab-1", Addresses: []string{"100.100.99.81/32"}, Authorized: true},
		{Hostname: "windows-desktop", Addresses: []string{"100.200.50.12/32"}, Authorized: true},
		{Hostname: "macbook-pro", Addresses: []string{"100.100.45.200/32"}, Authorized: false},
		{Hostname: "headscale-server", Addresses: []string{"100.64.0.1/32"}, Authorized: true},
	}

	tests := []struct {
		filter string
		want   []string // expected hostnames
	}{
		{"desktop", []string{"windows-desktop"}},
		{"100.64", []string{"headscale-server"}},
		{"lab", []string{"acemagic-lab-1"}},
		{"mac", []string{"macbook-pro"}},
		{"nonexistent", nil},
		{"", []string{"acemagic-lab-1", "windows-desktop", "macbook-pro", "headscale-server"}},
	}

	for _, tt := range tests {
		t.Run(tt.filter, func(t *testing.T) {
			result := filterDevices(devices, tt.filter)
			if tt.want == nil && len(result) != 0 {
				t.Errorf("filter(%q) = %d devices, want 0", tt.filter, len(result))
				return
			}
			if len(result) != len(tt.want) {
				t.Errorf("filter(%q) = %d devices, want %d", tt.filter, len(result), len(tt.want))
				for _, d := range result {
					t.Logf("  got: %s", d.Hostname)
				}
				return
			}
			for i, d := range result {
				if d.Hostname != tt.want[i] {
					t.Errorf("result[%d].Hostname = %q, want %q", i, d.Hostname, tt.want[i])
				}
			}
		})
	}
}

func TestConvertHeadscaleDevices(t *testing.T) {
	hsDevices := []discover.HeadscaleDevice{
		{Name: "host1", IPs: []string{"100.64.0.1"}, User: "alice", Authorized: true},
		{Name: "host2", IPs: []string{"100.64.0.2", "100.64.0.3"}, User: "bob", Authorized: false},
	}

	result := convertHeadscaleDevices(hsDevices)
	if len(result) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(result))
	}

	if result[0].Hostname != "host1" {
		t.Errorf("device[0].Hostname = %q, want %q", result[0].Hostname, "host1")
	}
	if result[0].User != "alice" {
		t.Errorf("device[0].User = %q, want %q", result[0].User, "alice")
	}
	if len(result[1].Addresses) != 2 {
		t.Errorf("device[1] has %d addresses, want 2", len(result[1].Addresses))
	}
}

func TestDiscoverCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range NewRootCmd().Commands() {
		if cmd.Name() == "discover" {
			found = true
			break
		}
	}
	if !found {
		t.Error("discover subcommand not registered with root command")
	}
}

// TestUpdateEnvFile_UsesNonDefaultPort verifies that a non-3389 port is written
// to .env without substituting the default — this is the "port capture" guarantee.
func TestUpdateEnvFile_UsesNonDefaultPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if err := updateEnvFile(path, "acemagic-office", 45000, ""); err != nil {
		t.Fatalf("updateEnvFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "TS_TARGET=acemagic-office:45000") {
		t.Errorf("expected TS_TARGET=acemagic-office:45000, got:\n%s", content)
	}
}

// TestUpdateEnvFile_DefaultPortNotSubstituted verifies port 3389 is also
// captured correctly (no accidental override to another value).
func TestUpdateEnvFile_DefaultPortNotSubstituted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if err := updateEnvFile(path, "host", 3389, ""); err != nil {
		t.Fatalf("updateEnvFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if !strings.Contains(string(data), "TS_TARGET=host:3389") {
		t.Errorf("expected TS_TARGET=host:3389, got:\n%s", string(data))
	}
}

// discover calls the control plane's *API*, which needs an API key, not the
// tsnet node key connect uses. The errors must name the key actually needed and
// a Headscale control URL must never fall through to the Tailscale SaaS API.
func TestDiscoverErrorsNameTheAPIKeyActuallyNeeded(t *testing.T) {
	tests := []struct {
		name       string
		controlURL string
		authKey    string
		want       []string
		notWant    string
	}{
		{"headscale without API key", "https://headscale.example.com", "", []string{"TS_HEADSCALE_API_KEY", "--api-key-file"}, "TS_TAILNET"},
		{"headscale does not take the node key", "https://headscale.example.com", "hskey-auth-node", []string{"TS_HEADSCALE_API_KEY"}, "TS_AUTHKEY or"},
		{"saas without key", "", "", []string{"API key", "TS_API_KEY", "--api-key-file"}, "TS_AUTHKEY or"},
		{"saas does not take the node key", "", "tskey-auth-node", []string{"TS_API_KEY", "TS_AUTHKEY"}, "set TS_AUTHKEY or"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TS_CONTROL_URL", tt.controlURL)
			t.Setenv("TS_AUTHKEY", tt.authKey)
			t.Setenv("TS_API_KEY", "")
			t.Setenv("TS_HEADSCALE_API_KEY", "")
			t.Setenv("TS_TAILNET", "")
			_, err := discoverList(t.Context(), discoverFlags{Port: 3389})
			if err == nil {
				t.Fatal("discoverList with no API key succeeded")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q; want it to mention %q", err, want)
				}
			}
			if strings.Contains(err.Error(), tt.notWant) {
				t.Errorf("error = %q; must not mention %q", err, tt.notWant)
			}
		})
	}
}

// The API key can come from a file (kept out of the process list), and the
// file wins over the inline flag and the environment.
func TestDiscoverList_ReadsAPIKeyFromFile(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode([]discover.HeadscaleDevice{{Name: "host-a", IPs: []string{"100.64.0.1"}, Authorized: true}})
	}))
	defer server.Close()

	keyFile := filepath.Join(t.TempDir(), "apikey")
	if err := os.WriteFile(keyFile, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TS_CONTROL_URL", server.URL)
	t.Setenv("TS_HEADSCALE_API_KEY", "from-env")

	devices, err := discoverList(t.Context(), discoverFlags{Port: 3389, APIKey: "from-flag", APIKeyFile: keyFile})
	if err != nil {
		t.Fatalf("discoverList: %v", err)
	}
	if len(devices) != 1 || devices[0].Hostname != "host-a" {
		t.Fatalf("devices = %+v; want host-a", devices)
	}
	if gotAuth != "Bearer from-file" {
		t.Errorf("Authorization = %q; want the file's key", gotAuth)
	}
}

func TestDiscoverList_UnreadableAPIKeyFileNamesTheFlag(t *testing.T) {
	t.Setenv("TS_CONTROL_URL", "https://headscale.example.com")
	_, err := discoverList(t.Context(), discoverFlags{Port: 3389, APIKeyFile: filepath.Join(t.TempDir(), "missing")})
	if err == nil || !strings.Contains(err.Error(), "--api-key-file") {
		t.Fatalf("error = %v; want it to name --api-key-file", err)
	}
}

// The "update .env?" prompt defaults to yes on a bare Enter, but EOF (no TTY,
// closed stdin) is not an answer and must not write the file.
func TestConfirmEnvUpdate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"EOF with no input", "", false},
		{"bare Enter takes the default", "\n", true},
		{"y", "y\n", true},
		{"yes without newline", "yes", true},
		{"n", "n\n", false},
		{"No", "No\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			if got := confirmEnvUpdate(strings.NewReader(tt.input), &out, "host", 3389); got != tt.want {
				t.Errorf("confirmEnvUpdate(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if !strings.Contains(out.String(), "TS_TARGET=host:3389") {
				t.Errorf("prompt = %q; want it to show the line it would add", out.String())
			}
		})
	}
}

func TestSortDevices(t *testing.T) {
	tests := []struct {
		name string
		in   []discover.Device
		want []string
	}{
		{"authorized first, then by hostname", []discover.Device{
			{Hostname: "b", Authorized: false}, {Hostname: "c", Authorized: true},
			{Hostname: "a", Authorized: false}, {Hostname: "d", Authorized: true},
		}, []string{"c", "d", "a", "b"}},
		{"all authorized sorts by hostname", []discover.Device{
			{Hostname: "z", Authorized: true}, {Hostname: "m", Authorized: true},
		}, []string{"m", "z"}},
		{"empty", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortDevices(tt.in)
			for i, want := range tt.want {
				if tt.in[i].Hostname != want {
					t.Fatalf("order = %v, want %v", hostnames(tt.in), tt.want)
				}
			}
		})
	}
}

func TestFindDevice(t *testing.T) {
	devices := []discover.Device{
		{Hostname: "alpha-1"}, {Hostname: "alpha-2"}, {Hostname: "Beta"}, {Hostname: "gamma"},
	}
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{"by number", "3", "Beta", ""},
		{"number out of range falls through to no match", "9", "", "no host matches"},
		{"exact hostname, case-insensitive", "BETA", "Beta", ""},
		{"exact beats partial", "alpha-1", "alpha-1", ""},
		{"unique partial", "gam", "gamma", ""},
		{"ambiguous partial", "alpha", "", "multiple hosts match"},
		{"no match", "zeta", "", "no host matches"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findDevice(devices, tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v; want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got.Hostname != tt.want {
				t.Fatalf("findDevice(%q) = %q, %v; want %q", tt.input, got.Hostname, err, tt.want)
			}
		})
	}
}

func hostnames(devices []discover.Device) []string {
	out := make([]string, len(devices))
	for i, d := range devices {
		out[i] = d.Hostname
	}
	return out
}

// A piped stdin that carries a "y" is still not a terminal: discover must not
// write .env on its say-so, and must say how to do it on purpose.
func TestPromptEnvUpdate_NonTerminalStdinIsNo(t *testing.T) {
	cmd := newDiscoverCmd()
	var out strings.Builder
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetOut(&out)

	if promptEnvUpdate(cmd, "host", 3389) {
		t.Fatal("promptEnvUpdate answered yes on a non-terminal stdin")
	}
	if !strings.Contains(out.String(), "--auto") {
		t.Errorf("output = %q; want it to point at --auto", out.String())
	}
}
