package cmd

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ts-bridge/internal/config"
)

// connect must refuse a non-loopback health listener before anything starts,
// whichever layer supplied it, unless the operator opted in (#633).
func TestConnectHealthAddrRefusesNonLoopback(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		yaml    string
		args    []string
		wantErr bool
	}{
		{"flag loopback", nil, "", []string{"--health-addr", "127.0.0.1:9090"}, false},
		{"flag remote", nil, "", []string{"--health-addr", "0.0.0.0:9090"}, true},
		{"flag remote with opt-in", nil, "", []string{"--health-addr", "0.0.0.0:9090", "--health-allow-remote"}, false},
		{"env remote", map[string]string{"TS_HEALTH_ADDR": "0.0.0.0:9090"}, "", nil, true},
		{"env remote with env opt-in", map[string]string{"TS_HEALTH_ADDR": "0.0.0.0:9090", "TS_HEALTH_ALLOW_REMOTE": "true"}, "", nil, false},
		{"env remote with flag opt-out over env opt-in", map[string]string{"TS_HEALTH_ADDR": "0.0.0.0:9090", "TS_HEALTH_ALLOW_REMOTE": "1"}, "", []string{"--health-allow-remote=false"}, true},
		{"yaml remote", nil, "health_addr: 10.0.0.5:9090\n", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := setupProfileRuntimeTest(t)
			writeProfileRuntimeFixture(t, path)
			t.Setenv("TS_HEALTH_ALLOW_REMOTE", "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			ran := false
			Runner = func(config.Config) error { ran = true; return nil }
			args := append([]string{"connect", "--profile", "office"}, tt.args...)
			if tt.yaml != "" {
				configPath := filepath.Join(t.TempDir(), "override.yaml")
				if err := os.WriteFile(configPath, []byte(tt.yaml), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--config", configPath)
			}
			command := NewRootCmd()
			command.SetArgs(args)
			command.SilenceErrors, command.SilenceUsage = true, true

			err := command.Execute()

			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "--health-allow-remote") {
					t.Fatalf("error = %v, want a refusal naming --health-allow-remote", err)
				}
				if ran {
					t.Fatal("runner started despite the refusal")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ran {
				t.Fatal("runner was not started")
			}
		})
	}
}

func TestWarnHealthExposureNamesTheExposedEndpoints(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	warnHealthExposure(log, config.Config{HealthAddr: "0.0.0.0:9090", HealthAllowRemote: true})

	for _, want := range []string{"0.0.0.0:9090", "/metrics", "/health/ready", "level=WARN"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("warning %q should mention %q", buf.String(), want)
		}
	}

	buf.Reset()
	warnHealthExposure(log, config.Config{HealthAddr: "127.0.0.1:9090"})
	if buf.Len() != 0 {
		t.Errorf("a loopback listener must not warn, got %q", buf.String())
	}
}
