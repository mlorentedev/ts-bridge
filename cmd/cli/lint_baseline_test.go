package cmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// baselineMarker tags a production function that predates the gocyclo and funlen
// limits in .golangci.yml (#629). The tag is what lets the baseline be audited.
const baselineMarker = "// baseline #629"

// baselineCeiling is how many functions each linter may still exempt. It can only go
// down: fix a function, delete its directive (nolintlint fails on one that no
// longer suppresses anything), and lower the number here in the same change.
var baselineCeiling = map[string]int{"gocyclo": 32, "funlen": 21}

var nolintDirective = regexp.MustCompile(`//nolint:([a-z,]+)`)

// TestComplexityBaselineOnlyShrinks scans the production sources for gocyclo and
// funlen suppressions. A new one must carry the baseline marker and fit under the
// ceiling, so a function that crosses a limit cannot be waved through by a bare
// //nolint with a fresh excuse.
func TestComplexityBaselineOnlyShrinks(t *testing.T) {
	root := filepath.Join("..", "..")
	counts := map[string]int{}
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(data), "\n") {
				m := nolintDirective.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				for _, linter := range strings.Split(m[1], ",") {
					if _, limited := baselineCeiling[linter]; !limited {
						continue
					}
					counts[linter]++
					if !strings.Contains(line, baselineMarker) {
						t.Errorf("%s:%d suppresses %s without %q: split the function instead of exempting it", path, i+1, linter, baselineMarker)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for linter, ceiling := range baselineCeiling {
		if counts[linter] > ceiling {
			t.Errorf("%s exemptions = %d, ceiling %d: the baseline may not grow", linter, counts[linter], ceiling)
		}
		if counts[linter] < ceiling {
			t.Errorf("%s exemptions = %d, ceiling %d: lower baselineCeiling to match", linter, counts[linter], ceiling)
		}
	}
}
