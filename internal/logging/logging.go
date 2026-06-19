// Package logging provides SRE-grade structured logging with dual output:
// human-readable text on console and JSON to a rotating log file.
package logging

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Config holds logging configuration.
type Config struct {
	// Verbose enables DEBUG-level output on console.
	Verbose bool
	// LogFormat is "text" (console) or "json" (file).
	LogFormat string
	// LogDir is the directory for log files. Empty = no file output.
	LogDir string
}

// Logger wraps the dual slog.Logger instances.
type Logger struct {
	console *slog.Logger
	file    *slog.Logger
	fileRot *FileRotator
}

// New creates a dual-output logger: text on console, JSON to rotating file.
// If logDir is empty, file output is disabled.
func New(cfg Config) *Logger {
	var consoleHandler slog.Handler
	if cfg.Verbose {
		consoleHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		consoleHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})
	}

	console := slog.New(consoleHandler)

	if cfg.LogDir == "" {
		return &Logger{console: console}
	}

	// Ensure log directory exists.
	// #nosec G301 // log directory permissions 0700 restrict access to the owner.
	if err := os.MkdirAll(cfg.LogDir, 0700); err != nil {
		console.Warn("failed to create log directory", "dir", cfg.LogDir, "error", err)
		return &Logger{console: console}
	}

	// Create the rotating file handler.
	rot := NewFileRotator(cfg.LogDir)

	// Open the current log file.
	if _, err := rot.Open(); err != nil {
		console.Warn("failed to open log file", "error", err)
		return &Logger{console: console}
	}

	fileHandler := slog.NewJSONHandler(rot, &slog.HandlerOptions{Level: slog.LevelDebug})
	file := slog.New(fileHandler)

	return &Logger{
		console: console,
		file:    file,
		fileRot: rot,
	}
}

// Console returns the console logger.
func (l *Logger) Console() *slog.Logger {
	return l.console
}

// File returns the file logger.
func (l *Logger) File() *slog.Logger {
	return l.file
}

// Combined returns a logger that writes to both console and file.
// Use this for logs that should appear in both places.
func (l *Logger) Combined() *slog.Logger {
	if l.file != nil {
		return slog.New(NewMultiHandler(l.console.Handler(), l.file.Handler()))
	}
	return l.console
}

// Close flushes and closes the file logger.
func (l *Logger) Close() error {
	if l.fileRot != nil {
		return l.fileRot.Close()
	}
	return nil
}

// LogPath returns the current log file path, or empty string if file logging is disabled.
func (l *Logger) LogPath() string {
	if l.fileRot == nil {
		return ""
	}
	return l.fileRot.CurrentPath()
}

// --- MultiHandler: writes to multiple slog handlers ---

type multiHandler struct {
	handlers []slog.Handler
}

func NewMultiHandler(hs ...slog.Handler) slog.Handler {
	return &multiHandler{handlers: hs}
}

//nolint:contextcheck // slog.Handler interface requires context but we don't need one here.
func (m *multiHandler) Enabled(_ context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(context.TODO(), level) {
			return true
		}
	}
	return false
}

//nolint:contextcheck // slog.Handler interface requires context but we don't need one here.
func (m *multiHandler) Handle(_ context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m.handlers {
		if h.Enabled(context.TODO(), r.Level) {
			if err := h.Handle(context.TODO(), r); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}



// --- File Rotator: daily rotation with gzip compression ---

// FileRotator manages daily log file rotation.
type FileRotator struct {
	mu         sync.Mutex
	dir        string
	current    *os.File
	currentDay string
}

// NewFileRotator creates a rotator that writes to dir/ts-bridge-YYYY-MM-DD.log.
func NewFileRotator(dir string) *FileRotator {
	return &FileRotator{dir: dir}
}

// CurrentPath returns the path of the current log file.
func (r *FileRotator) CurrentPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil {
		return ""
	}
	return r.current.Name()
}

// Write implements io.Writer so FileRotator can be passed directly to
// slog handlers. This enables dynamic rotation: every log write checks
// if the day has changed and rotates if needed.
func (r *FileRotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	today := time.Now().Format("2006-01-02")

	// Rotate if the day has changed.
	if r.current == nil || r.currentDay != today {
		if r.current != nil {
			// #nosec G104 // cleanup: ignore close errors.
			r.current.Close()
			r.currentDay = "" // Force re-open below.
		}

		if r.currentDay != today {
			path := filepath.Join(r.dir, fmt.Sprintf("ts-bridge-%s.log", today))
			// #nosec G302,G304 // filename is date-derived, not user-controlled.
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				return 0, fmt.Errorf("open log file: %w", err)
			}
			r.current = f
			r.currentDay = today
		}
	}

	return r.current.Write(p)
}

// Open opens (or creates) the log file for today.
// If the current file is from a different day, it closes it,
// compresses it, and opens a new one.
func (r *FileRotator) Open() (*os.File, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	today := time.Now().Format("2006-01-02")

	// If we already have today's file open, return it.
	if r.current != nil && r.currentDay == today {
		return r.current, nil
	}

	// Close and compress yesterday's file.
	if r.current != nil {
		// #nosec G104 // cleanup: ignore close errors.
		r.current.Close()
		r.compress(r.currentDay)
	}

	// Open today's file.
	path := filepath.Join(r.dir, fmt.Sprintf("ts-bridge-%s.log", today))
	// #nosec G302,G304 // filename is date-derived, not user-controlled.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	r.current = f
	r.currentDay = today

	// Clean up old files.
	r.cleanup()

	return f, nil
}

// Close closes the current file and compresses it.
func (r *FileRotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.current != nil {
		// #nosec G104 // cleanup: ignore close errors.
		r.current.Close()
		r.compress(r.currentDay)
		r.current = nil
	}
	return nil
}

// compress gzips the log file for the given day.
func (r *FileRotator) compress(day string) {
	srcPath := filepath.Join(r.dir, fmt.Sprintf("ts-bridge-%s.log", day))
	dstPath := srcPath + ".gz"

	// #nosec G304 // filename is date-derived, not user-controlled.
	src, err := os.Open(srcPath)
	if err != nil {
		return // File already gone or unreadable.
	}
	defer src.Close()
	// #nosec G304 // filename is date-derived, not user-controlled.
	dst, err := os.Create(dstPath)
	if err != nil {
		return
	}
	defer dst.Close()

	gw := gzip.NewWriter(dst)
	if _, err := io.Copy(gw, src); err != nil {
		return
	}
	if err := gw.Close(); err != nil {
		return
	}
	if err := dst.Close(); err != nil {
		return
	}

	// Remove the uncompressed file.
	// #nosec G104 // cleanup: ignore remove errors.
	os.Remove(srcPath)
}

// cleanup removes log files older than maxDays and keeps at most maxFiles.
func (r *FileRotator) cleanup() {
	const maxDays = 7
	const maxFiles = 10

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return
	}

	type logFile struct {
		name    string
		modTime time.Time
	}

	logs := make([]logFile, 0, len(entries))
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "ts-bridge-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		logs = append(logs, logFile{
			name:    e.Name(),
			modTime: info.ModTime(),
		})
	}

	// Sort by modTime descending (newest first).
	for i := 0; i < len(logs); i++ {
		for j := i + 1; j < len(logs); j++ {
			if logs[j].modTime.After(logs[i].modTime) {
				logs[i], logs[j] = logs[j], logs[i]
			}
		}
	}

	now := time.Now()
	gzCount := 0
	for _, lf := range logs {
		age := now.Sub(lf.modTime)

		if strings.HasSuffix(lf.name, ".gz") {
			gzCount++
		}

		// Remove files older than maxDays.
		if age > maxDays*24*time.Hour {
			// #nosec G104 // cleanup: ignore remove errors.
			os.Remove(filepath.Join(r.dir, lf.name))
			continue
		}

		// Remove excess .gz files (keep newest maxFiles).
		if strings.HasSuffix(lf.name, ".gz") && gzCount > maxFiles {
			// #nosec G104 // cleanup: ignore remove errors.
			os.Remove(filepath.Join(r.dir, lf.name))
		}
	}
}

// --- Utility: parse log directory from config ---

// LogDirForPlatform returns the platform-appropriate log directory.
func LogDirForPlatform() string {
	home := os.Getenv("HOME")
	appData := os.Getenv("LOCALAPPDATA")

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(appData, "ts-bridge", "logs")
	case "darwin":
		return filepath.Join(home, "Library", "Logs", "ts-bridge")
	default:
		// Linux and others
		return filepath.Join(home, ".local", "share", "ts-bridge", "logs")
	}
}

// --- slog Level string helpers ---

// LevelString returns the slog level as a short string.
func LevelString(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "DBG"
	case slog.LevelInfo:
		return "INF"
	case slog.LevelWarn:
		return "WRN"
	case slog.LevelError:
		return "ERR"
	default:
		return "???"
	}
}
