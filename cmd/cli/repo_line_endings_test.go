package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoModuleFilesCheckOutLF pins go.mod and go.sum to LF on every OS: the
// attribute that makes Git check them out LF, and the working copies themselves. Without
// it a Windows checkout with core.autocrlf=true gets CRLF copies, and
// `go mod tidy -diff` then reports a whole-file diff that Git does not (#498).
func TestGoModuleFilesCheckOutLF(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	root := filepath.Join("..", "..")
	//nolint:gosec // fixed arguments; git is resolved from PATH like any developer tool.
	out, err := exec.Command(git, "-C", root, "check-attr", "eol", "--", "go.mod", "go.sum").Output()
	if err != nil {
		t.Skipf("not a git checkout: %v", err)
	}
	for _, file := range []string{"go.mod", "go.sum"} {
		if want := file + ": eol: lf"; !strings.Contains(string(out), want) {
			t.Errorf("git check-attr eol = %q, want a line %q", out, want)
		}
		// The attribute only applies on checkout: an older CRLF working copy keeps
		// its bytes until it is checked out again, so read the file itself too.
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte("\r")) {
			t.Errorf("%s contains CR bytes; delete it and check it out again (git checkout -- %s)", file, file)
		}
	}
}
