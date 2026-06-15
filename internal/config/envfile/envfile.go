// Package envfile provides a minimal .env loader that reads KEY=VALUE pairs
// from a file and sets them into the process environment.
//
// Design rationale:
//
//	ts-bridge init writes a .env file with TS_AUTHKEY, TS_TARGET, etc.
//	ts-bridge connect must auto-load it from CWD so the user doesn't have
//	to manually source it (matching docker-compose, webpack, rails conventions).
//
// The loader is intentionally minimal: no interpolation, no export prefix,
// no shell parsing. Just strip comments, skip blanks, set os.Setenv.
//
// Load() is idempotent — calling it multiple times is safe.
// If the file does not exist, Load() returns nil (not an error).
package envfile

import (
	"bufio"
	"os"
	"strings"
)

// Load reads the .env file at path and sets each KEY=VALUE into the process
// environment via os.Setenv. If the file does not exist, Load returns nil.
//
// Precedence: values in .env are set before the merge chain runs, so
// CLI flags and explicit TS_* env vars still override them (the merge
// chain applies flags > env > yaml > defaults).
func Load(path string) error {
	// #nosec G304 -- path is ".env" from CWD, never user-controlled.
	// The caller passes a hardcoded string literal; there is no path
	// injection vector. os.Open is the correct choice here.
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blanks and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first '=' only (values may contain '=').
		idx := strings.Index(line, "=")
		if idx < 1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Strip surrounding quotes from value.
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// #nosec G104 -- os.Setenv always returns nil in the standard library.
		os.Setenv(key, value)
	}

	return scanner.Err()
}
