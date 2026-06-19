package discover

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UpdateEnv adds or updates TS_TARGET in the .env file at the given path.
// If the file doesn't exist, it creates it with just TS_TARGET.
// The operation is non-destructive: existing variables are preserved.
func UpdateEnv(path, target string, port int) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0750); err != nil {
				return fmt.Errorf("create directory %s: %w", dir, err)
			}
		}
	}

	existingVars, otherLines := readEnvFile(path)

	// Update TS_TARGET.
	existingVars["TS_TARGET"] = fmt.Sprintf("%s:%d", target, port)

	if err := writeEnvFile(path, existingVars, otherLines); err != nil {
		return err
	}

	return nil
}

// readEnvFile parses a .env file into variables and comment/blank lines.
func readEnvFile(path string) (map[string]string, []string) {
	existingVars := make(map[string]string)
	var otherLines []string

	// #nosec G304 -- path is the .env file in the user's working directory,
	// not an external path. The user controls their own working directory.
	data, err := os.ReadFile(path)
	if err != nil {
		return existingVars, otherLines
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			otherLines = append(otherLines, line)
			continue
		}
		if key, val := parseEnvLine(line); key != "" {
			existingVars[key] = val
		}
	}

	return existingVars, otherLines
}

// parseEnvLine parses a KEY=VALUE line, returning (key, value) or ("","") for
// malformed lines.
func parseEnvLine(line string) (string, string) {
	idx := strings.Index(line, "=")
	if idx <= 0 {
		return "", ""
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:])
}

// knownTSKeys is the ordered list of TS_ variables written to .env.
var knownTSKeys = []string{
	"TS_AUTHKEY",
	"TS_TARGET",
	"TS_INSTANCE_NAME",
	"TS_PORT_RANGE",
	"TS_LOCAL_ADDR",
	"TS_HOSTNAME",
	"TS_STATE_DIR",
	"TS_CONTROL_URL",
	"TS_HEALTH_ADDR",
	"TS_LOG_FORMAT",
	"TS_IDLE_TIMEOUT",
	"TS_DIAL_TIMEOUT",
	"TS_DIAL_RETRIES",
	"TS_DIAL_BACKOFF_BASE",
	"TS_DIAL_BACKOFF_MAX",
	"TS_MAX_CONNECTIONS",
	"TS_TIMEOUT",
	"TS_DRAIN_TIMEOUT",
	"TS_VERBOSE",
}

// writeEnvFile writes the .env file from parsed variables and comment lines.
func writeEnvFile(path string, vars map[string]string, comments []string) error {
	var sb strings.Builder

	// Write comments/blanks first.
	for _, line := range comments {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Write known TS_ vars in stable order.
	for _, key := range knownTSKeys {
		if val, ok := vars[key]; ok {
			sb.WriteString(fmt.Sprintf("%s=%s\n", key, val))
			delete(vars, key)
		}
	}

	// Write any remaining TS_ vars not in the known list.
	for key, val := range vars {
		if strings.HasPrefix(key, "TS_") {
			sb.WriteString(fmt.Sprintf("%s=%s\n", key, val))
		}
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}

	return nil
}
