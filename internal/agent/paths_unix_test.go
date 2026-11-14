//go:build !windows

package agent

import (
	"os"
	"os/user"
	"strconv"
	"strings"
	"testing"
)

// The base must not follow the environment: a login supervisor and a desktop
// session see different ones, and two bases would mean two agents.
func TestDefaultPathsIgnoreTheEnvironment(t *testing.T) {
	u, err := user.LookupId(strconv.Itoa(os.Geteuid()))
	if err != nil {
		t.Skipf("the user database has no entry for this uid: %v", err)
	}
	decoy := t.TempDir()
	for _, key := range []string{"HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "TMPDIR"} {
		t.Setenv(key, decoy)
	}
	got, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.Base, u.HomeDir) {
		t.Fatalf("base %q is not under the user database home %q", got.Base, u.HomeDir)
	}
	for _, p := range []string{got.Base, got.Run} {
		if strings.HasPrefix(p, decoy) {
			t.Fatalf("path %q follows the environment", p)
		}
	}
}

func TestRunUserUsable(t *testing.T) {
	uid := os.Geteuid()
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  bool
	}{
		{"private directory", func(t *testing.T) string { return privateDir(t, 0o700) }, true},
		{"group-readable", func(t *testing.T) string { return privateDir(t, 0o750) }, false},
		{"absent", func(t *testing.T) string { return shortDir(t) + "/absent" }, false},
		{"symlink to a private directory", func(t *testing.T) string {
			link := shortDir(t) + "/link"
			if err := os.Symlink(privateDir(t, 0o700), link); err != nil {
				t.Fatal(err)
			}
			return link
		}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := runUserUsable(tc.setup(t), uid); got != tc.want {
				t.Fatalf("runUserUsable = %v, want %v", got, tc.want)
			}
		})
	}
	if runUserUsable(privateDir(t, 0o700), uid+1) {
		t.Fatal("a directory owned by another uid was accepted")
	}
}

func privateDir(t *testing.T, perm os.FileMode) string {
	t.Helper()
	dir := shortDir(t) + "/d"
	if err := os.Mkdir(dir, perm); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, perm); err != nil {
		t.Fatal(err)
	}
	return dir
}
