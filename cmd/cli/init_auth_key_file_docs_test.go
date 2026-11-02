package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitAuthKeyFileDocumentation pins the supported non-interactive workflow:
// every documented `init --auth-key-file` example also passes --target, because
// without it init enters the wizard (#401). A shell continuation (`\`) carries the
// example onto the next line.
func TestInitAuthKeyFileDocumentation(t *testing.T) {
	root := filepath.Join("..", "..")
	files := []string{"README.md"}
	for _, pattern := range []string{"docs/runbooks/*.md", "site/src/content/docs/*.md"} {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			t.Fatal(err)
		}
		// A source that moves or changes extension must fail here, not drop out silently.
		if len(matches) == 0 {
			t.Fatalf("%s matches no file; the guard would stop reading that source", pattern)
		}
		for _, m := range matches {
			rel, _ := filepath.Rel(root, m)
			files = append(files, filepath.ToSlash(rel))
		}
	}

	examples := 0
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, example := range initAuthKeyFileExamples(string(data)) {
			examples++
			if !strings.Contains(example, "--target") {
				t.Errorf("%s: example %d passes --auth-key-file without --target: %q", name, i+1, example)
			}
		}
	}
	if examples == 0 {
		t.Fatal("no `init --auth-key-file` example found; the guard reads nothing")
	}
}

// initAuthKeyFileExamples returns each command line that runs
// `init --auth-key-file`, joined with its `\` continuation lines.
func initAuthKeyFileExamples(doc string) []string {
	lines := strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n")
	var examples []string
	for i := 0; i < len(lines); i++ {
		if !strings.Contains(lines[i], "init --auth-key-file") || strings.Contains(lines[i], "`init --auth-key-file`") {
			continue
		}
		example := lines[i]
		for strings.HasSuffix(strings.TrimSpace(lines[i]), `\`) && i+1 < len(lines) {
			i++
			example += " " + strings.TrimSpace(lines[i])
		}
		examples = append(examples, example)
	}
	return examples
}

func TestInitAuthKeyFileExamples(t *testing.T) {
	doc := "intro\n```sh\nts-bridge init --auth-key-file k \\\n  --target h:1\n" +
		"ts-bridge init --auth-key-file k\n```\nprose about `init --auth-key-file` here\n"
	got := initAuthKeyFileExamples(doc)
	want := []string{"ts-bridge init --auth-key-file k \\ --target h:1", "ts-bridge init --auth-key-file k"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("examples = %q, want %q", got, want)
	}
}
