package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// decodeYAML decodes YAML data into cfg, checking for unknown fields.
func decodeYAML(data []byte, cfg *PartialConfig) error {
	// First decode into a map to check for unknown fields.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse YAML: %w", err)
	}

	// Check for unknown fields.
	knownFields := map[string]bool{
		"version":          true,
		"target":           true,
		"hostname":         true,
		"control_url":      true,
		"state_dir":        true,
		"health_addr":      true,
		"log_format":       true,
		"auth_key":         true,
		"timeout":          true,
		"dial_timeout":     true,
		"drain_timeout":    true,
		"idle_timeout":     true,
		"dial_retries":     true,
		"dial_backoff_base": true,
		"dial_backoff_max": true,
		"max_connections":  true,
		"auto_instance":    true,
	}

	for key := range raw {
		lowerKey := strings.ToLower(key)
		if !knownFields[lowerKey] {
			fmt.Fprintf(os.Stderr, "WARNING: unknown YAML field %q — ignoring\n", key)
		}
	}

	// Reject auth_key in YAML.
	if v, ok := raw["auth_key"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return fmt.Errorf("auth key in YAML config is not allowed; use TS_AUTHKEY env var or --auth-key-file flag")
		}
	}

	// Decode into struct (no strict field checking — unknown fields are
	// already warned about above).
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(cfg); err != nil {
		return fmt.Errorf("decode YAML config: %w", err)
	}

	return nil
}
