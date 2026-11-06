package internal

import (
	"go/build"
	"io/fs"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// projectImports returns the sorted internal packages dir imports, relative to
// ts-bridge/internal, ignoring test files.
func projectImports(t *testing.T, dir string) []string {
	t.Helper()
	pkg, err := build.ImportDir(dir, 0)
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

// TestPackageDependencies pins the import edges that ADR-007 records. A new
// edge fails here until the ADR's dependency table and this one change
// together; a stale table is how the ADR claimed "config depends on nothing
// project-local" long after config imported credential (#630).
func TestPackageDependencies(t *testing.T) {
	want := map[string][]string{
		"bootstrapssh":   nil,
		"browser":        nil,
		"config":         {"controlurl", "credential"},
		"config/envfile": nil,
		"controlurl":     nil,
		"credential":     {"secretfile"},
		"discover":       {"secretfile"},
		"health":         {"telemetry"},
		"host":           {"bootstrapssh", "controlurl"},
		"logging":        nil,
		"profile":        {"controlurl", "credential"},
		"proxy":          {"config", "telemetry"},
		"secretfile":     nil,
		"serve":          {"config", "health", "host", "proxy"},
		"telemetry":      nil,
	}
	for name, deps := range want {
		got := projectImports(t, filepath.FromSlash(name))
		if !reflect.DeepEqual(got, deps) && (len(got) != 0 || len(deps) != 0) {
			t.Errorf("internal/%s imports %v, want %v (update ADR-007 with this table)", name, got, deps)
		}
	}

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == "." {
			return err
		}
		name := filepath.ToSlash(path)
		if _, ok := want[name]; !ok && hasGo(path) {
			t.Errorf("internal/%s is not in the dependency table", name)
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
