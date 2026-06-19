package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateEnv_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	err := UpdateEnv(path, "desktop", 3389)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}

	if !strings.Contains(string(data), "TS_TARGET=desktop:3389") {
		t.Errorf("expected TS_TARGET=desktop:3389 in .env, got:\n%s", data)
	}
}

func TestUpdateEnv_UpdatesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	// Write existing .env with other vars.
	existing := `# ts-bridge config
TS_AUTHKEY=tskey-test-123
TS_TARGET=old-host:3389
TS_INSTANCE_NAME=mybridge
`
	if err := os.WriteFile(path, []byte(existing), 0600); err != nil {
		t.Fatalf("write initial .env: %v", err)
	}

	err := UpdateEnv(path, "new-host", 3390)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}

	content := string(data)

	// TS_TARGET updated.
	if !strings.Contains(content, "TS_TARGET=new-host:3390") {
		t.Errorf("expected TS_TARGET=new-host:3390, got:\n%s", content)
	}

	// Other vars preserved.
	if !strings.Contains(content, "TS_AUTHKEY=tskey-test-123") {
		t.Error("TS_AUTHKEY should be preserved")
	}
	if !strings.Contains(content, "TS_INSTANCE_NAME=mybridge") {
		t.Error("TS_INSTANCE_NAME should be preserved")
	}
}

func TestUpdateEnv_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	existing := `# Required
# Auth key
TS_AUTHKEY=tskey-test
# Target
TS_TARGET=old:3389
# Optional
# TS_VERBOSE=1
`
	if err := os.WriteFile(path, []byte(existing), 0600); err != nil {
		t.Fatalf("write initial .env: %v", err)
	}

	err := UpdateEnv(path, "new", 3389)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}

	content := string(data)
	// Comments should be preserved.
	if !strings.Contains(content, "# Required") {
		t.Error("comment '# Required' should be preserved")
	}
	if !strings.Contains(content, "# Auth key") {
		t.Error("comment '# Auth key' should be preserved")
	}
}

func TestUpdateEnv_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	// File with no trailing newline.
	if err := os.WriteFile(path, []byte("TS_AUTHKEY=tskey-test"), 0600); err != nil {
		t.Fatalf("write initial .env: %v", err)
	}

	err := UpdateEnv(path, "host", 3389)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}

	if !strings.Contains(string(data), "TS_AUTHKEY=tskey-test") {
		t.Error("TS_AUTHKEY should be preserved")
	}
	if !strings.Contains(string(data), "TS_TARGET=host:3389") {
		t.Error("TS_TARGET should be added")
	}
}
