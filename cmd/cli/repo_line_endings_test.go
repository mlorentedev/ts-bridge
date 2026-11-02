package cmd

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoModuleFilesCheckOutLF pins go.mod and go.sum to LF on every OS. Without
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
	}
}
