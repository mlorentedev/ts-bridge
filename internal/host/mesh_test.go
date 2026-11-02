package host

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestLoadFile_Mesh(t *testing.T) {
	full := `host:
  control_url: https://hs.example
  mesh:
    bootstrap_ssh: tsb@jump.example:2222
    hostname: lab-pc-01
    state_dir: 'C:\ProgramData\ts-bridge\serve'
    auth_key_file: 'C:\ProgramData\ts-bridge\authkey'
    health_addr: 127.0.0.1:9099
    extra: 1
`
	var warn bytes.Buffer
	got, err := LoadFile(writeHostFile(t, full), goosWindows, &warn)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	want := &FileMesh{
		BootstrapSSH: strPtr("tsb@jump.example:2222"),
		Hostname:     strPtr("lab-pc-01"),
		StateDir:     strPtr(`C:\ProgramData\ts-bridge\serve`),
		AuthKeyFile:  strPtr(`C:\ProgramData\ts-bridge\authkey`),
		HealthAddr:   strPtr("127.0.0.1:9099"),
	}
	if !reflect.DeepEqual(got.Host.Mesh, want) {
		t.Errorf("Mesh = %+v, want %+v", got.Host.Mesh, want)
	}
	if w := warn.String(); strings.Count(w, "WARNING:") != 1 || !strings.Contains(w, `"host.mesh.extra"`) {
		t.Errorf("warnings = %q, want one naming host.mesh.extra", w)
	}

	_, err = LoadFile(writeHostFile(t, "host:\n  mesh:\n    hostname:\n"), goosWindows, nil)
	if err == nil || !strings.Contains(err.Error(), "host.mesh.hostname: no value") {
		t.Errorf("a key with no value: error = %v, want it named", err)
	}
}

func TestLoadFile_MeshInvalid(t *testing.T) {
	const cp = "  control_url: https://hs.example\n"
	mesh := func(kv string) string { return "host:\n" + cp + "  mesh:\n    " + kv + "\n" }
	tests := []struct {
		name, goos, content, wantErr string
	}{
		{"empty bootstrap_ssh", goosLinux, mesh(`bootstrap_ssh: ""`), "host.mesh.bootstrap_ssh: empty"},
		{"empty hostname", goosLinux, mesh(`hostname: ""`), "host.mesh.hostname: empty"},
		{"empty state_dir", goosLinux, mesh(`state_dir: ""`), "host.mesh.state_dir: empty"},
		{"empty auth_key_file", goosLinux, mesh(`auth_key_file: ""`), "host.mesh.auth_key_file: empty"},
		{"empty health_addr", goosLinux, mesh(`health_addr: ""`), "host.mesh.health_addr: empty"},
		{"ssh option as endpoint", goosLinux, mesh(`bootstrap_ssh: -oProxyCommand=x`), "host.mesh.bootstrap_ssh"},
		{"ssh user without host", goosLinux, mesh(`bootstrap_ssh: "user@"`), "host.mesh.bootstrap_ssh"},
		{"ssh port 0", goosLinux, mesh(`bootstrap_ssh: jump:0`), "host.mesh.bootstrap_ssh"},
		{"hostname with a space", goosLinux, mesh(`hostname: lab pc`), "host.mesh.hostname"},
		{"hostname too long", goosLinux, mesh("hostname: " + strings.Repeat("a", 64)), "host.mesh.hostname"},
		{"hostname ends in a hyphen", goosLinux, mesh("hostname: lab-"), "host.mesh.hostname"},
		{"relative state_dir", goosLinux, mesh("state_dir: serve"), "host.mesh.state_dir: must be an absolute path"},
		{"unix path on windows", goosWindows, mesh("state_dir: /var/lib/ts-bridge"), "host.mesh.state_dir: must be an absolute path"},
		{"relative auth_key_file", goosWindows, mesh("auth_key_file: authkey"), "host.mesh.auth_key_file: must be an absolute path"},
		{"health on every interface", goosLinux, mesh("health_addr: 0.0.0.0:9099"), "host.mesh.health_addr"},
		{"health on a LAN address", goosLinux, mesh("health_addr: 10.0.0.1:9099"), "host.mesh.health_addr"},
		{"health port 0", goosLinux, mesh("health_addr: 127.0.0.1:0"), "host.mesh.health_addr"},
		{"health without a port", goosLinux, mesh("health_addr: localhost"), "host.mesh.health_addr"},
		{"bootstrap without a control plane", goosLinux, "host:\n  mesh:\n    bootstrap_ssh: jump\n", "host.mesh.bootstrap_ssh needs host.control_url"},
		{"no serve on darwin", goosDarwin, mesh("hostname: mac"), "#459"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadFile(writeHostFile(t, tc.content), tc.goos, nil)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("LoadFile(%s) error = %v, want it to contain %q", tc.goos, err, tc.wantErr)
			}
		})
	}
}

func TestLoadFile_MeshValidPerOS(t *testing.T) {
	for goos, dir := range map[string]string{goosLinux: "/var/lib/ts-bridge", goosWindows: `C:\ProgramData\ts-bridge`} {
		content := "host:\n  mesh:\n    hostname: lab\n    state_dir: '" + dir + "'\n    health_addr: '[::1]:9099'\n"
		if _, err := LoadFile(writeHostFile(t, content), goos, nil); err != nil {
			t.Errorf("%s: LoadFile() error = %v", goos, err)
		}
	}
}
