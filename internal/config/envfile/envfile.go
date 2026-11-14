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

// Values parses the .env file at path into its KEY=VALUE pairs without touching
// the process environment, so a caller that must know what a .env *will yield*
// (a writer validating the layer its consumer will read) does not need a second
// copy of the parsing rules. A missing file yields an empty map and a nil error,
// matching Load.
func Values(path string) (map[string]string, error) { //nolint:funlen,gocyclo // baseline #629
	values := make(map[string]string)

	// #nosec G304 -- callers pass a path they already resolved (CWD .env, or the
	// directory of the config file being written).
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return values, nil
		}
		return nil, err
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

		values[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

// Load reads the .env file at path and sets each KEY=VALUE whose variable has
// no value in the process environment. If the file does not exist, Load
// returns nil.
//
// Precedence: a variable the operator set to a value is never replaced, so the
// chain stays flags > env > .env > YAML > defaults. A .env in a directory the
// operator merely cd'd into must not redirect a key they exported, for example
// by setting TS_CONTROL_URL (#801). An empty variable counts as unset, as it
// does in the merge chain, which reads every TS_* value with os.Getenv.
func Load(path string) error {
	values, err := Values(path)
	if err != nil {
		return err
	}
	for key, value := range values {
		if os.Getenv(key) != "" {
			continue
		}
		// #nosec G104 -- os.Setenv always returns nil in the standard library.
		os.Setenv(key, value)
	}
	return nil
}
