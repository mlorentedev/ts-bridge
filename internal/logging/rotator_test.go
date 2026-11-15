package logging

import (
	"compress/gzip"
	"errors"
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

// AC2: an archive that already exists is never truncated or overwritten; a
// later run of the same day gets an archive of its own.
func TestCompressDayNeverOverwritesAnArchive(t *testing.T) {
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
	if got := gunzip(t, dayFile(dir, day)+".gz"); got != "run1\n" {
		t.Fatalf("first archive = %q, want run1 untouched", got)
	}
	if got := archives(t, dir, day); len(got) != 2 || got[0]+got[1] != "run1\nrun2\n" && got[0]+got[1] != "run2\nrun1\n" {
		t.Fatalf("archives of the day = %q, want one per run", got)
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

// A compression that fails keeps the day's lines, even for a log past
// retention, and a later rotation archives them once the claim is stale.
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
	writeGzip = func(string, string) error { return errors.New("disk full") }
	t.Cleanup(func() { writeGzip = gzipFile })

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local) // 9 days later: past retention
	r := rotatorAt(dir, &now)
	write(t, r, "today\n")
	_ = r.Close()
	if remaining := claimedFiles(t, dir); len(remaining) != 1 {
		t.Fatalf("after a failed compression the lines must survive in one claim, found %v", remaining)
	}

	writeGzip = gzipFile
	now = now.Add(2 * staleClaimAge)
	r = rotatorAt(dir, &now)
	write(t, r, "later\n")
	_ = r.Close()
	if got := gunzip(t, dayFile(dir, day)+".gz"); got != "old lines\n" {
		t.Fatalf("archive after the retry = %q, want the claimed lines", got)
	}
	if remaining := claimedFiles(t, dir); len(remaining) != 0 {
		t.Fatalf("claims left after a successful retry: %v", remaining)
	}
}

// A claim younger than staleClaimAge may belong to a peer still compressing
// it, so no rotation touches it.
func TestFileRotator_LeavesAFreshClaimAlone(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 14, 0, 1, 0, 0, time.Local)
	claim := dayFile(dir, "2026-11-13") + ".99.1" + claimedSuffix
	if err := os.WriteFile(claim, []byte("peer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fresh := now.Add(-time.Minute)
	if err := os.Chtimes(claim, fresh, fresh); err != nil {
		t.Fatal(err)
	}
	r := rotatorAt(dir, &now)
	write(t, r, "x\n")
	_ = r.Close()
	if _, err := os.Stat(claim); err != nil {
		t.Fatalf("a fresh claim was taken: %v", err)
	}
	if got := archives(t, dir, "2026-11-13"); len(got) != 0 {
		t.Fatalf("a fresh claim was archived: %q", got)
	}
}

// Two rotators compressing the same past day never corrupt an archive and
// never lose the day: every archive holds it whole, and no claim is left.
func TestCompressDayConcurrentClaimsNeverCorrupt(t *testing.T) {
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
		got := archives(t, dir, day)
		if len(got) == 0 {
			t.Fatal("the day was lost: no archive")
		}
		for _, content := range got {
			if content != "line\n" {
				t.Fatalf("archive = %q, want the day whole", content)
			}
		}
		if remaining := claimedFiles(t, dir); len(remaining) != 0 {
			t.Fatalf("claims left: %v", remaining)
		}
	}
}

// A zero-value FileRotator still works (it reads time.Now).
func TestFileRotator_ZeroValueUsesTheWallClock(t *testing.T) {
	r := &FileRotator{dir: t.TempDir()}
	write(t, r, "x\n")
	_ = r.Close()
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

// archives returns the decompressed content of every archive of day.
func archives(t *testing.T, dir, day string) []string {
	t.Helper()
	matches, err := filepath.Glob(dayFile(dir, day) + "*.gz")
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, gunzip(t, m))
	}
	return out
}
