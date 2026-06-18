package logging

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNew_DisabledFileOutput(t *testing.T) {
	cfg := Config{Verbose: true, LogFormat: "text", LogDir: ""}
	l := New(cfg)
	if l.console == nil {
		t.Fatal("console logger should not be nil")
	}
	if l.file != nil {
		t.Fatal("file logger should be nil when LogDir is empty")
	}
	if l.fileRot != nil {
		t.Fatal("fileRotator should be nil when LogDir is empty")
	}
	if l.LogPath() != "" {
		t.Fatal("LogPath should be empty when file logging is disabled")
	}
}

func TestNew_CreatesLogDirectory(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Verbose: true, LogFormat: "text", LogDir: dir}
	l := New(cfg)
	defer l.Close()

	if l.console == nil {
		t.Fatal("console logger should not be nil")
	}
	if l.file == nil {
		t.Fatal("file logger should not be nil")
	}
	if l.fileRot == nil {
		t.Fatal("fileRotator should not be nil")
	}

	// Log directory should exist.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("log directory should have been created")
	}

	// Log path should be set.
	path := l.LogPath()
	if path == "" {
		t.Fatal("LogPath should not be empty")
	}
	if !strings.Contains(path, "ts-bridge-") {
		t.Fatalf("LogPath should contain ts-bridge- prefix, got %s", path)
	}
}

func TestNew_FileWritesJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Verbose: false, LogFormat: "json", LogDir: dir}
	l := New(cfg)
	defer l.Close()

	// Write a log entry.
	l.file.Info("test message", "key", "value")

	// Read the file and verify JSON format.
	f, err := os.Open(filepath.Join(dir, "ts-bridge-"+time.Now().Format("2006-01-02")+".log"))
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	line, err := reader.ReadString('\n')
	if err != io.EOF && err != nil {
		t.Fatalf("failed to read log line: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nline: %s", err, line)
	}

	if entry["msg"] != "test message" {
		t.Errorf("expected msg=test message, got %v", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Errorf("expected key=value, got %v", entry["key"])
	}
}

func TestNew_ConsoleLevel(t *testing.T) {
	tests := []struct {
		name       string
		verbose    bool
		wantMsgs   []string
		notWantMsg []string
	}{
		{
			name:       "suppress_info_when_not_verbose",
			verbose:    false,
			wantMsgs:   []string{"warn message", "error message"},
			notWantMsg: []string{"info message", "debug message"},
		},
		{
			name:       "show_all_when_verbose",
			verbose:    true,
			wantMsgs:   []string{"debug message", "info message", "warn message", "error message"},
			notWantMsg: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Capture stdout BEFORE creating the logger (slog captures os.Stdout at construction).
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cfg := Config{Verbose: tt.verbose, LogFormat: "text", LogDir: dir}
			l := New(cfg)
			defer l.Close()

			// Log at different levels.
			l.console.Debug("debug message")
			l.console.Info("info message")
			l.console.Warn("warn message")
			l.console.Error("error message")

			w.Close()
			os.Stdout = oldStdout

			var buf strings.Builder
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			for _, msg := range tt.wantMsgs {
				if !strings.Contains(output, msg) {
					t.Errorf("%q should appear on console when verbose=%v", msg, tt.verbose)
				}
			}
			for _, msg := range tt.notWantMsg {
				if strings.Contains(output, msg) {
					t.Errorf("%q should NOT appear on console when verbose=%v", msg, tt.verbose)
				}
			}
		})
	}
}

func TestNew_FileReceivesAllLevels(t *testing.T) {
	dir := t.TempDir()

	// Non-verbose: file should receive ALL levels.
	cfg := Config{Verbose: false, LogFormat: "json", LogDir: dir}
	l := New(cfg)
	defer l.Close()

	// Log at all levels.
	l.file.Debug("debug message")
	l.file.Info("info message")
	l.file.Warn("warn message")
	l.file.Error("error message")

	// Read the file and verify all entries are present.
	f, err := os.Open(filepath.Join(dir, "ts-bridge-"+time.Now().Format("2006-01-02")+".log"))
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var msgs []string
	for scanner.Scan() {
		var entry map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if msg, ok := entry["msg"].(string); ok {
			msgs = append(msgs, msg)
		}
	}

	for _, want := range []string{"debug message", "info message", "warn message", "error message"} {
		found := false
		for _, got := range msgs {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("file should contain %q, got: %v", want, msgs)
		}
	}
}



func TestFileRotator_Open(t *testing.T) {
	dir := t.TempDir()
	rot := NewFileRotator(dir)
	defer rot.Close()

	f, err := rot.Open()
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	expectedName := filepath.Join(dir, "ts-bridge-"+time.Now().Format("2006-01-02")+".log")
	if f.Name() != expectedName {
		t.Errorf("expected %s, got %s", expectedName, f.Name())
	}

	// Second Open should return the same file.
	f2, err := rot.Open()
	if err != nil {
		t.Fatalf("second Open() failed: %v", err)
	}
	if f2.Name() != f.Name() {
		t.Error("second Open() should return the same file handle")
	}
}

func TestFileRotator_Close(t *testing.T) {
	dir := t.TempDir()
	rot := NewFileRotator(dir)

	f, err := rot.Open()
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	_ = f

	err = rot.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// After Close, current file should be nil.
	if rot.CurrentPath() != "" {
		t.Error("CurrentPath should be empty after Close")
	}
}

func TestLogDirForPlatform(t *testing.T) {
	// Save and restore env vars.
	origHome := os.Getenv("HOME")
	origAppData := os.Getenv("LOCALAPPDATA")
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("LOCALAPPDATA", origAppData)
	}()

	// Set up env vars for all platforms.
	os.Setenv("LOCALAPPDATA", "C:\\Users\\test")
	os.Setenv("HOME", "/home/test")

	// The function uses runtime.GOOS, so the result depends on the build target.
	// We just verify it returns a valid path containing ts-bridge and logs.
	dir := LogDirForPlatform()
	if !strings.Contains(dir, "ts-bridge") || !strings.Contains(dir, "logs") {
		t.Errorf("expected path containing ts-bridge and logs, got %s", dir)
	}

	// Verify the path structure is correct for the detected platform.
	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(dir, filepath.Join("ts-bridge", "logs")) {
			t.Errorf("expected Windows path, got %s", dir)
		}
	default:
		// Linux/darwin: verify it contains the expected segments.
		if !strings.Contains(dir, "ts-bridge") {
			t.Errorf("expected ts-bridge in path, got %s", dir)
		}
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelDebug, "DBG"},
		{slog.LevelInfo, "INF"},
		{slog.LevelWarn, "WRN"},
		{slog.LevelError, "ERR"},
	}
	for _, tt := range tests {
		if got := LevelString(tt.level); got != tt.want {
			t.Errorf("LevelString(%v) = %s, want %s", tt.level, got, tt.want)
		}
	}
}

func TestNew_FailedDirectoryCreation(t *testing.T) {
	// Create a temp file, then try to create a directory at that file's path.
	// This deterministically fails on all platforms.
	tmpFile := t.TempDir() + "/not_a_dir"
	// #nosec G306 // test temp file, permissions don't matter.
	if err := os.WriteFile(tmpFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{Verbose: true, LogDir: tmpFile}
	l := New(cfg)
	defer l.Close()

	// Should still return a valid logger with console output.
	if l.console == nil {
		t.Fatal("console logger should still be created even if file dir fails")
	}
}
