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
