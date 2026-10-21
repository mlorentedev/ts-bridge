package browser

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestEdgeSearchFor covers every platform on any runner: the OS is an input
// (lesson 040), so the darwin bundle paths are tested without a Mac.
func TestEdgeSearchFor(t *testing.T) {
	const home = "/home/test"
	env := map[string]string{
		"PROGRAMFILES(X86)": `C:\PF86`,
		"PROGRAMFILES":      `C:\PF`,
	}
	getenv := func(key string) string { return env[key] }
	macBundle := filepath.Join("Microsoft Edge.app", "Contents", "MacOS", "Microsoft Edge")
	tests := []struct {
		goos  string
		names []string
		paths []string
	}{
		{"windows", []string{"msedge"}, []string{
			filepath.Join(`C:\PF86`, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(`C:\PF`, "Microsoft", "Edge", "Application", "msedge.exe"),
		}},
		{"darwin", nil, []string{
			filepath.Join("/Applications", macBundle),
			filepath.Join(home, "Applications", macBundle),
		}},
		{"linux", []string{"microsoft-edge", "microsoft-edge-stable"}, nil},
		{"freebsd", []string{"microsoft-edge", "microsoft-edge-stable"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			got := edgeSearchFor(tt.goos, home, getenv)
			if !reflect.DeepEqual(got.names, tt.names) || !reflect.DeepEqual(got.paths, tt.paths) {
				t.Errorf("edgeSearchFor(%q) = %+v, want names=%v paths=%v", tt.goos, got, tt.names, tt.paths)
			}
		})
	}
}

func TestEdgeSearchForSkipsUnsetProgramFiles(t *testing.T) {
	got := edgeSearchFor("windows", "", func(string) string { return "" })
	if len(got.paths) != 0 {
		t.Errorf("paths = %v, want none when Program Files variables are unset", got.paths)
	}
}

func TestFindEdgeIn(t *testing.T) {
	search := edgeSearch{names: []string{"edge-on-path"}, paths: []string{"/first", "/second"}}
	tests := []struct {
		name     string
		explicit string
		onPath   map[string]string
		files    map[string]bool
		want     string
		wantErr  string
	}{
		{name: "explicit wins over discovery", explicit: "/custom", onPath: map[string]string{"edge-on-path": "/usr/bin/edge"},
			files: map[string]bool{"/custom": true, "/first": true}, want: "/custom"},
		{name: "missing explicit path is an error, never a fallback", explicit: "/custom",
			files: map[string]bool{"/first": true}, wantErr: "Edge executable not found: /custom"},
		{name: "PATH name before install paths", onPath: map[string]string{"edge-on-path": "/usr/bin/edge"},
			files: map[string]bool{"/first": true}, want: "/usr/bin/edge"},
		{name: "first existing install path", files: map[string]bool{"/second": true}, want: "/second"},
		{name: "nothing found asks for --edge-path", wantErr: "provide --edge-path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookPath := func(name string) (string, error) {
				if p, ok := tt.onPath[name]; ok {
					return p, nil
				}
				return "", errors.New("not found")
			}
			isFile := func(path string) bool { return tt.files[path] }
			got, err := findEdgeIn(tt.explicit, search, lookPath, isFile)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("findEdgeIn = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
