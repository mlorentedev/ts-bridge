package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedCredentialDocumentation(t *testing.T) {
	root := filepath.Join("..", "..")
	files := map[string][]string{
		".env.example": {
			"ts-bridge auth set --profile",
			"TS_AUTHKEY",
		},
		"README.md": {
			"ts-bridge auth set --profile",
			"connect --profile",
		},
		"site/src/content/docs/configuration.md": {
			"credential:",
			"auth set --profile",
			"Browser exception",
			"`TS_AUTHKEY` is ignored",
		},
		"site/src/content/docs/cli-reference.md": {
			"auth set",
			"auth status",
			"auth remove",
		},
		"docs/adr/adr-012-config-profiles-model.md": {
			"credential:",
			"ADR-015",
		},
		"docs/adr/adr-014-socks5-dynamic-mesh-proxy.md": {
			"managed profile credential",
		},
		"docs/troubleshooting/security-audit.md": {
			"managed credential store",
			"owner-only",
			"Structural validation",
			"tskey-auth-<id>-<secret>",
		},
	}

	for name, required := range files {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			content := string(data)
			for _, text := range required {
				if !strings.Contains(content, text) {
					t.Errorf("%s missing %q", name, text)
				}
			}
		})
	}
}
