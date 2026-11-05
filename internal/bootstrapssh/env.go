package bootstrapssh

import "strings"

// secretPrefixes and secretSuffixes name the variables that carry a credential:
// TS_AUTHKEY, TS_HEADSCALE_API_KEY and any API key, OAuth client secret or
// token under the same namespaces.
var (
	secretPrefixes = []string{"TS_", "TAILSCALE_", "HEADSCALE_"}
	secretSuffixes = []string{"KEY", "SECRET", "TOKEN", "PASSWORD"}
)

// withoutSecrets drops the credential-bearing variables from environ. ssh runs
// whatever ssh_config names (ProxyCommand, LocalCommand, Match exec) and
// SendEnv can forward variables to the bastion, so the child must not inherit
// the mesh credentials. Everything else ssh needs (HOME, PATH, SSH_AUTH_SOCK)
// passes through.
func withoutSecrets(environ []string) []string {
	kept := make([]string, 0, len(environ))
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if !isSecretName(name) {
			kept = append(kept, kv)
		}
	}
	return kept
}

func isSecretName(name string) bool {
	name = strings.ToUpper(name) // Windows environment names are case-insensitive
	return hasAnyPrefix(name, secretPrefixes) && hasAnySuffix(name, secretSuffixes)
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func hasAnySuffix(s string, suffixes []string) bool {
	for _, p := range suffixes {
		if strings.HasSuffix(s, p) {
			return true
		}
	}
	return false
}
