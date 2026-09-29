package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapSSHDocumentation(t *testing.T) {
	root := filepath.Join("..", "..")
	files := map[string][]string{
		".env.example": {
			"TS_BOOTSTRAP_SSH",
			"TS_BOOTSTRAP_SOCKS_ADDR",
		},
		"README.md": {
			"--bootstrap-ssh",
			"without editing the hosts file",
		},
		"site/src/content/docs/configuration.md": {
			"TS_BOOTSTRAP_SSH",
			"bootstrap_socks_addr",
		},
		"site/src/content/docs/cli-reference.md": {
			"--bootstrap-ssh",
			"--bootstrap-socks-addr",
		},
		"docs/adr/adr-005-headscale-compat.md": {
			"SSH bootstrap",
			"dynamic SOCKS",
		},
		"docs/lessons/lesson-014-corporate-tls-inspection-breaks-headscale-tcp.md": {
			"--bootstrap-ssh",
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
