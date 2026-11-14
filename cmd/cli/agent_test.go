//go:build !windows

package cmd

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ts-bridge/internal/agent"
)

func TestAgentCommandServesUntilInterrupted(t *testing.T) {
	root, err := os.MkdirTemp("", "tsb") // short: a socket path is limited to ~104 bytes
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	paths := agent.Paths{Base: filepath.Join(root, "b"), Run: filepath.Join(root, "r")}

	ctx, interrupt := context.WithCancel(context.Background())
	defer interrupt()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- runAgent(ctx, paths, &out) }()

	client := agent.HTTPClient(paths)
	client.Timeout = time.Second
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := client.Get("http://agent/v1/status")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the agent never answered: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	interrupt()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runAgent after an interrupt: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("runAgent did not return after an interrupt")
	}
	if !strings.Contains(out.String(), filepath.Join(paths.Run, "agent.sock")) {
		t.Fatalf("output %q does not name the socket", out.String())
	}
	if _, err := os.Lstat(filepath.Join(paths.Run, "agent.sock")); !os.IsNotExist(err) {
		t.Fatalf("socket left behind after a clean stop: %v", err)
	}
}
