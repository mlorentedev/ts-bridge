// Package logging provides SRE-grade structured logging with dual output:
// human-readable text on console and JSON to a rotating log file.
package logging

import (
	"compress/gzip"
	"errors"
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

// File returns the file logger.
func (l *Logger) File() *slog.Logger {
	return l.file
}

// Combined returns a logger that writes to both console and file.
// Use this for logs that should appear in both places.
func (l *Logger) Combined() *slog.Logger {
	if l.file != nil {
		return slog.New(slog.NewMultiHandler(l.console.Handler(), l.file.Handler()))
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

// --- File Rotator: daily rotation with gzip compression ---

const (
	logPrefix = "ts-bridge-"
	dayLayout = "2006-01-02"
	// claimedSuffix marks a past day's log a process has taken to compress.
	claimedSuffix = ".claimed"
	// partialSuffix marks an archive still being written.
	partialSuffix = ".partial"
	// staleClaimAge is how old a claim must be before a rotation retries it.
	staleClaimAge = time.Hour
)

// FileRotator manages daily log file rotation.
type FileRotator struct {
	mu         sync.Mutex
	dir        string
	current    *os.File
	currentDay string
	now        func() time.Time
}

// NewFileRotator creates a rotator that writes to dir/ts-bridge-YYYY-MM-DD.log.
func NewFileRotator(dir string) *FileRotator {
	return &FileRotator{dir: dir, now: time.Now}
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
// slog handlers. Every write checks the day and rotates through the same path
// Open uses, so a process that crosses midnight compresses yesterday and
// enforces retention without a restart (#788).
func (r *FileRotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.rotateLocked(); err != nil {
		return 0, err
	}
	return r.current.Write(p)
}

// Open opens (or creates) today's log file, compressing past days and
// applying retention when it starts a new day.
func (r *FileRotator) Open() (*os.File, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.rotateLocked(); err != nil {
		return nil, err
	}
	return r.current, nil
}

// Close closes the current file. It never compresses it: other processes
// may still be writing the same day (#788). The day is compressed by the
// first rotation, in any process, after it has ended.
func (r *FileRotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current != nil {
		// #nosec G104 // cleanup: ignore close errors.
		r.current.Close()
		r.current = nil
	}
	return nil
}

// rotateLocked makes today's file current. On a day change it opens today's
// file, compresses every past day still left as a .log, and applies
// retention. Callers hold r.mu.
func (r *FileRotator) rotateLocked() error {
	today := r.clock().Format(dayLayout)
	if r.current != nil && r.currentDay == today {
		return nil
	}
	if r.current != nil {
		// #nosec G104 // cleanup: ignore close errors.
		r.current.Close()
		r.current = nil
	}
	// #nosec G302,G304 // filename is date-derived, not user-controlled.
	f, err := os.OpenFile(r.dayPath(today), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	r.current, r.currentDay = f, today
	r.compressPastDays(today)
	r.cleanup()
	return nil
}

// clock is the rotator's time source; a zero-value FileRotator uses time.Now.
func (r *FileRotator) clock() time.Time {
	if r.now == nil {
		return time.Now()
	}
	return r.now()
}

func (r *FileRotator) dayPath(day string) string {
	return filepath.Join(r.dir, logPrefix+day+".log")
}

// compressPastDays archives each ts-bridge-<day>.log whose day is before
// today, and retries any claim a failed or interrupted compression left behind
// once it is older than staleClaimAge. Today, any day after it (a clock set
// back), and anything that is not a regular file are left alone.
func (r *FileRotator) compressPastDays(today string) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		day, claim, ok := pastDayEntry(e, today)
		if !ok || (claim && !r.staleClaim(e)) {
			continue
		}
		r.archive(filepath.Join(r.dir, e.Name()), day)
	}
}

// staleClaim reports whether a claim is old enough that no live process can
// still be compressing it. A compression takes milliseconds; a claim is
// touched when it is made, so a peer's in-flight claim is never taken.
func (r *FileRotator) staleClaim(e os.DirEntry) bool {
	info, err := e.Info()
	return err == nil && r.clock().Sub(info.ModTime()) >= staleClaimAge
}

// pastDayEntry reports the day of a regular file named ts-bridge-<day>.log
// or ts-bridge-<day>.log.<...>.claimed whose day is before today.
func pastDayEntry(e os.DirEntry, today string) (day string, claim, ok bool) {
	rest, found := strings.CutPrefix(e.Name(), logPrefix)
	if !found || !e.Type().IsRegular() || len(rest) < len(dayLayout) {
		return "", false, false
	}
	day, rest = rest[:len(dayLayout)], rest[len(dayLayout):]
	if _, err := time.Parse(dayLayout, day); err != nil || day >= today {
		return "", false, false
	}
	switch {
	case rest == ".log":
		return day, false, true
	case strings.HasPrefix(rest, ".log.") && strings.HasSuffix(rest, claimedSuffix):
		return day, true, true
	}
	return "", false, false
}

// compressDay archives a past day's log.
func (r *FileRotator) compressDay(day string) {
	r.archive(r.dayPath(day), day)
}

// archive claims path under a unique name by an atomic rename, so two
// processes never compress the same file at once (the loser's rename fails),
// then writes it to an archive of its own and drops the claim. No archive is
// ever appended to or overwritten. If anything fails the claim keeps the
// lines, and a later rotation retries it once it is stale.
func (r *FileRotator) archive(path, day string) {
	claimed := r.uniqueName(day, claimedSuffix)
	if err := os.Rename(path, claimed); err != nil {
		return
	}
	now := r.clock()
	_ = os.Chtimes(claimed, now, now) // the claim's age starts now, not at the log's last write
	if err := r.writeArchive(claimed, day); err != nil {
		return
	}
	// #nosec G104 // cleanup: ignore remove errors.
	os.Remove(claimed)
}

// uniqueName is a per-compression name next to the day's log.
func (r *FileRotator) uniqueName(day, suffix string) string {
	return fmt.Sprintf("%s.%d.%d%s", r.dayPath(day), os.Getpid(), time.Now().UnixNano(), suffix)
}

// writeArchive gzips src into a private temp file, then publishes it without
// clobbering: as ts-bridge-<day>.log.gz when no archive of that day exists
// (os.Link fails on an existing name), otherwise under a unique name. A
// second run of the same day therefore gets its own archive.
func (r *FileRotator) writeArchive(src, day string) error {
	tmp := r.uniqueName(day, ".gz"+partialSuffix)
	if err := writeGzip(src, tmp); err != nil {
		// #nosec G104 // cleanup: ignore remove errors.
		os.Remove(tmp)
		return err
	}
	if err := os.Link(tmp, r.dayPath(day)+".gz"); err == nil {
		// #nosec G104 // cleanup: ignore remove errors.
		os.Remove(tmp)
		return nil
	}
	if err := os.Rename(tmp, strings.TrimSuffix(tmp, partialSuffix)); err != nil {
		// #nosec G104 // cleanup: ignore remove errors.
		os.Remove(tmp)
		return err
	}
	return nil
}

// writeGzip is gzipFile; tests replace it to make a compression fail.
var writeGzip = gzipFile

// gzipFile writes srcPath gzipped to a new file at dstPath.
func gzipFile(srcPath, dstPath string) error {
	// #nosec G304 // filename is date-derived, not user-controlled.
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	// #nosec G302,G304 // filename is date-derived, not user-controlled.
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	gw := gzip.NewWriter(dst)
	_, err = io.Copy(gw, src)
	if err == nil {
		err = gw.Close()
	}
	return errors.Join(err, dst.Close())
}

// cleanup removes log files older than maxDays and keeps at most maxFiles.
func (r *FileRotator) cleanup() { //nolint:funlen,gocyclo // baseline #629
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
		// Retention removes archives only. A .log or a claim holds lines not
		// yet archived, so it is never aged out.
		if !strings.HasPrefix(e.Name(), logPrefix) || !strings.HasSuffix(e.Name(), ".gz") {
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

	now := r.clock()
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
	return logDirFor(runtime.GOOS, os.Getenv("HOME"), os.Getenv("LOCALAPPDATA"))
}

// logDirFor takes the OS as an input so every branch is testable on any runner.
func logDirFor(goos, home, appData string) string {
	switch goos {
	case "windows":
		return filepath.Join(appData, "ts-bridge", "logs")
	case "darwin":
		return filepath.Join(home, "Library", "Logs", "ts-bridge")
	default:
		// Linux and others
		return filepath.Join(home, ".local", "share", "ts-bridge", "logs")
	}
}
