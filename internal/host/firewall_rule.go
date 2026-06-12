package host

import (
	"fmt"
	"regexp"
)

const maxFirewallRuleName = 64

var validFirewallRuleRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// sanitizeFirewallRule validates and sanitizes a firewall rule name.
// Only alphanumeric, dash, and underscore are allowed. Max 64 characters.
// Returns the sanitized name or an error if invalid.
func sanitizeFirewallRule(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("firewall rule name must not be empty")
	}
	if len(name) > maxFirewallRuleName {
		return "", fmt.Errorf("firewall rule name must be at most %d characters (got %d)", maxFirewallRuleName, len(name))
	}
	if !validFirewallRuleRe.MatchString(name) {
		return "", fmt.Errorf("firewall rule name contains invalid characters (only A-Z, a-z, 0-9, -, _ allowed): %q", name)
	}
	return name, nil
}
