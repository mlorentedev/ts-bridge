package internal

import (
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// projectImports returns the sorted internal packages dir imports, relative to
// ts-bridge/internal, ignoring test files. Build constraints are ignored, so an
// edge that exists only behind a GOOS suffix or tag counts on every platform.
func projectImports(t *testing.T, dir string) []string {
	t.Helper()
	ctx := build.Default
	ctx.UseAllFiles = true
	pkg, err := ctx.ImportDir(dir, 0)
	if err != nil {
		t.Fatalf("import %s: %v", dir, err)
	}
	var got []string
	for _, imp := range pkg.Imports {
		if rel, ok := strings.CutPrefix(imp, "ts-bridge/internal/"); ok {
			got = append(got, rel)
		}
	}
	slices.Sort(got)
	return got
}

// codeSpans returns the `code spans` of a markdown table cell, in order.
func codeSpans(cell string) []string {
	var spans []string
	parts := strings.Split(cell, "`")
	for i := 1; i < len(parts); i += 2 {
		spans = append(spans, parts[i])
	}
	return spans
}

// adrTable reads the dependency table of the ADR-007 amendment: each row names
// a group of packages and the project-local packages they import.
func adrTable(t *testing.T) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash("../docs/adr/adr-007-multi-package-split.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, amendment, found := strings.Cut(string(data), "\n## Amendment")
	if !found {
		t.Fatal("ADR-007 has no Amendment section")
	}
	table := map[string][]string{}
	for _, line := range strings.Split(amendment, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) != 4 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "`") {
			continue
		}
		deps := codeSpans(cells[2])
		slices.Sort(deps)
		for _, name := range codeSpans(cells[1]) {
			table[name] = deps
		}
	}
	if len(table) == 0 {
		t.Fatal("ADR-007 amendment has no dependency table")
	}
	return table
}

// TestPackageDependencies checks the import edges against the table in ADR-007's
// amendment, which it reads. A new edge, a new package or a stale row fails here
// until the ADR and the code agree, which is how the ADR claimed "config depends
// on nothing project-local" long after config imported credential (#630).
func TestPackageDependencies(t *testing.T) {
	want := adrTable(t)
	for name, deps := range want {
		if !hasGo(filepath.FromSlash(name)) {
			t.Errorf("ADR-007 lists internal/%s, which has no Go files", name)
			continue
		}
		got := projectImports(t, filepath.FromSlash(name))
		if !reflect.DeepEqual(got, deps) && (len(got) != 0 || len(deps) != 0) {
			t.Errorf("internal/%s imports %v, ADR-007 says %v", name, got, deps)
		}
	}

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == "." {
			return err
		}
		name := filepath.ToSlash(path)
		if _, ok := want[name]; !ok && hasGo(path) {
			t.Errorf("internal/%s is not in the ADR-007 dependency table", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func hasGo(dir string) bool {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	return len(files) > 0
}
