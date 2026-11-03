package host

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestHostPackage_DoesNotImportUserStores pins ADR-017 C9 at the package
// boundary: the host package, which runs elevated, never imports the per-user
// profile or config stores.
func TestHostPackage_DoesNotImportUserStores(t *testing.T) {
	forbidden := []string{"ts-bridge/internal/profile", "ts-bridge/internal/config", "ts-bridge/internal/credential"}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			for _, bad := range forbidden {
				if path == bad || strings.HasPrefix(path, bad+"/") {
					t.Errorf("%s imports %s (ADR-017 C9)", name, path)
				}
			}
		}
	}
}
