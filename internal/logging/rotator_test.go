package logging

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func dayFile(dir, day string) string { return filepath.Join(dir, "ts-bridge-"+day+".log") }

func rotatorAt(dir string, now *time.Time) *FileRotator {
	r := NewFileRotator(dir)
	r.now = func() time.Time { return *now }
	return r
}

func gunzip(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path) // #nosec G304 -- test fixture under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func write(t *testing.T, r *FileRotator, line string) {
	t.Helper()
	if _, err := r.Write([]byte(line)); err != nil {
		t.Fatal(err)
	}
}

// AC1: one session exiting must not take the shared file from another (#788).
func TestFileRotator_CloseLeavesAPeersFileInPlace(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.Local)
	a, b := rotatorAt(dir, &now), rotatorAt(dir, &now)
	write(t, a, "a1\n")
	write(t, b, "b1\n")
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	write(t, b, "b2\n")

	got, err := os.ReadFile(dayFile(dir, "2026-11-14"))
	if err != nil {
		t.Fatalf("the shared day file is gone after one rotator closed: %v", err)
	}
	if string(got) != "a1\nb1\nb2\n" {
		t.Fatalf("day file = %q, want every line from both rotators", got)
	}
	if _, err := os.Stat(dayFile(dir, "2026-11-14") + ".gz"); err == nil {
		t.Fatal("closing compressed a day another rotator is still writing")
	}
	_ = b.Close()
}

// AC2: an archive that already exists is extended, never truncated.
func TestCompressDayAppendsToAnExistingArchive(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.Local)
	r := rotatorAt(dir, &now)
	day := "2026-11-13"
	for _, content := range []string{"run1\n", "run2\n"} {
		if err := os.WriteFile(dayFile(dir, day), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		r.compressDay(day)
	}
	if got := gunzip(t, dayFile(dir, day)+".gz"); got != "run1\nrun2\n" {
		t.Fatalf("archive = %q, want both runs", got)
	}
	if _, err := os.Stat(dayFile(dir, day)); !os.IsNotExist(err) {
		t.Fatalf("the compressed .log is still there: %v", err)
	}
}

// AC3 + AC4: crossing midnight compresses yesterday and never today.
func TestFileRotator_MidnightRolloverCompressesThePastDayOnly(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 13, 23, 59, 0, 0, time.Local)
	r := rotatorAt(dir, &now)
	write(t, r, "before midnight\n")
	now = now.Add(2 * time.Minute)
	write(t, r, "after midnight\n")

	if got := gunzip(t, dayFile(dir, "2026-11-13")+".gz"); got != "before midnight\n" {
		t.Fatalf("yesterday's archive = %q", got)
	}
	got, err := os.ReadFile(dayFile(dir, "2026-11-14"))
	if err != nil || string(got) != "after midnight\n" {
		t.Fatalf("today's file = %q (%v), want it open and uncompressed", got, err)
	}
	if _, err := os.Stat(dayFile(dir, "2026-11-14") + ".gz"); err == nil {
		t.Fatal("rotation compressed today's file")
	}
	_ = r.Close()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), claimedSuffix) {
			t.Fatalf("a claimed file was left behind: %s", e.Name())
		}
	}
}

// A compression that fails must keep the day's lines, even for a log old
// enough for retention, and a later rotation must archive them (review F1).
func TestFileRotator_FailedCompressionIsRetriedNotLost(t *testing.T) {
	dir := t.TempDir()
	day := "2026-11-01"
	if err := os.WriteFile(dayFile(dir, day), []byte("old lines\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2026, 11, 1, 23, 0, 0, 0, time.Local)
	if err := os.Chtimes(dayFile(dir, day), old, old); err != nil {
		t.Fatal(err)
	}
	blocked := dayFile(dir, day) + ".gz"
	if err := os.Mkdir(blocked, 0o700); err != nil { // an archive that cannot be opened
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local) // 9 days later: past retention
	r := rotatorAt(dir, &now)
	write(t, r, "today\n")
	_ = r.Close()
	if remaining := claimedFiles(t, dir); len(remaining) != 1 {
		t.Fatalf("after a failed compression the lines must survive in one claim, found %v", remaining)
	}

	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	now = now.Add(24 * time.Hour)
	r = rotatorAt(dir, &now)
	write(t, r, "next day\n")
	_ = r.Close()
	if got := gunzip(t, blocked); got != "old lines\n" {
		t.Fatalf("archive after the retry = %q, want the claimed lines", got)
	}
	if remaining := claimedFiles(t, dir); len(remaining) != 0 {
		t.Fatalf("claims left after a successful retry: %v", remaining)
	}
}

// Two rotators compressing the same past day archive it exactly once.
func TestCompressDayConcurrentClaimsArchiveOnce(t *testing.T) {
	for range 20 {
		dir := t.TempDir()
		day := "2026-11-13"
		if err := os.WriteFile(dayFile(dir, day), []byte("line\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 14, 0, 0, 1, 0, time.Local)
		a, b := rotatorAt(dir, &now), rotatorAt(dir, &now)
		done := make(chan struct{})
		go func() { a.compressDay(day); close(done) }()
		b.compressDay(day)
		<-done
		if got := gunzip(t, dayFile(dir, day)+".gz"); got != "line\n" {
			t.Fatalf("archive = %q, want the day exactly once", got)
		}
	}
}

// A directory that happens to carry a log's name is not a log.
func TestFileRotator_LeavesADirectoryNamedLikeALog(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(dayFile(dir, "2026-11-13"), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.Local)
	r := rotatorAt(dir, &now)
	write(t, r, "x\n")
	_ = r.Close()
	if info, err := os.Stat(dayFile(dir, "2026-11-13")); err != nil || !info.IsDir() {
		t.Fatalf("the directory was moved: %v", err)
	}
	if _, err := os.Stat(dayFile(dir, "2026-11-13") + ".gz"); err == nil {
		t.Fatal("an archive was made for a directory")
	}
}

func claimedFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), claimedSuffix) {
			out = append(out, e.Name())
		}
	}
	return out
}
