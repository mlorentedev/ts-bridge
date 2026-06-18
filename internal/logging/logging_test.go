package logging

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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

func TestNew_ConsoleLevel_SuppressesInfoWhenNotVerbose(t *testing.T) {
	dir := t.TempDir()

	// Capture stdout BEFORE creating the logger (slog captures os.Stdout at construction).
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Non-verbose: console should only show WARN+.
	cfg := Config{Verbose: false, LogFormat: "text", LogDir: dir}
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

	// INFO and DEBUG should NOT appear on console.
	if strings.Contains(output, "info message") {
		t.Error("INFO message should be suppressed on console when verbose=false")
	}
	if strings.Contains(output, "debug message") {
		t.Error("DEBUG message should be suppressed on console when verbose=false")
	}
	// WARN and ERROR should appear.
	if !strings.Contains(output, "warn message") {
		t.Error("WARN message should appear on console")
	}
	if !strings.Contains(output, "error message") {
		t.Error("ERROR message should appear on console")
	}
}

func TestNew_ConsoleLevel_ShowsInfoWhenVerbose(t *testing.T) {
	dir := t.TempDir()

	// Capture stdout BEFORE creating the logger (slog captures os.Stdout at construction).
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Verbose: console should show INFO+.
	cfg := Config{Verbose: true, LogFormat: "text", LogDir: dir}
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

	// All levels should appear on console when verbose.
	for _, msg := range []string{"debug message", "info message", "warn message", "error message"} {
		if !strings.Contains(output, msg) {
			t.Errorf("%q should appear on console when verbose=true", msg)
		}
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

func TestNew_VerboseConsole(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Verbose: true, LogFormat: "text", LogDir: dir}
	l := New(cfg)
	defer l.Close()

	// DEBUG should appear on console when verbose.
	l.console.Debug("debug message")
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

	// Windows path.
	os.Setenv("LOCALAPPDATA", "C:\\Users\\test")
	os.Setenv("HOME", "/home/test")
	dir := LogDirForPlatform()
	if !strings.Contains(dir, "ts-bridge") || !strings.Contains(dir, "logs") {
		t.Errorf("expected Windows log dir, got %s", dir)
	}

	// Linux path.
	os.Setenv("LOCALAPPDATA", "")
	os.Setenv("HOME", "/home/test")
	dir = LogDirForPlatform()
	if !strings.Contains(dir, "ts-bridge") || !strings.Contains(dir, "logs") {
		t.Errorf("expected Linux log dir, got %s", dir)
	}
	if !strings.Contains(dir, filepath.Join(".local", "share")) {
		t.Errorf("expected .local/share in path, got %s", dir)
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
	// LogDir set to a path that can't be created (e.g., /proc/invalid on Linux).
	cfg := Config{Verbose: true, LogDir: "/proc/invalid_tsbridge_nonexistent"}
	l := New(cfg)
	defer l.Close()

	// Should still return a valid logger with console output.
	if l.console == nil {
		t.Fatal("console logger should still be created even if file dir fails")
	}
}
