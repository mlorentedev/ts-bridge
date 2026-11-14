package agent

import (
	"path/filepath"
	"testing"
)

func TestPathsFor(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "ana")
	runUser := filepath.Join(string(filepath.Separator), "run", "user", "1000")
	usable := func(want string) func(string) bool {
		return func(dir string) bool { return dir == want }
	}
	never := func(string) bool { return false }

	tests := []struct {
		name    string
		goos    string
		runUser func(string) bool
		want    Paths
	}{
		{"linux with a private /run/user", "linux", usable(runUser), Paths{
			Base: filepath.Join(home, ".local", "state", "ts-bridge"),
			Run:  filepath.Join(runUser, "ts-bridge"),
		}},
		{"linux without /run/user falls back to /tmp by uid", "linux", never, Paths{
			Base: filepath.Join(home, ".local", "state", "ts-bridge"),
			Run:  filepath.Join(string(filepath.Separator), "tmp", "ts-bridge-1000"),
		}},
		{"darwin keeps the socket under the base", "darwin", usable(runUser), Paths{
			Base: filepath.Join(home, "Library", "Application Support", "ts-bridge"),
			Run:  filepath.Join(home, "Library", "Application Support", "ts-bridge", "run"),
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathsFor(tc.goos, 1000, home, tc.runUser); got != tc.want {
				t.Fatalf("pathsFor = %+v, want %+v", got, tc.want)
			}
		})
	}
}
