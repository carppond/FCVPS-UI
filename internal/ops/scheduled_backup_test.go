package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// stubArchiver writes a tiny temp file each call, mimicking Backup.Create
// (which returns a path the scheduler then moves into Dir).
type stubArchiver struct {
	tmpDir string
	n      int
	err    error
}

func (s *stubArchiver) Create(context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	s.n++
	p := filepath.Join(s.tmpDir, fmt.Sprintf("src-%d.tmp", s.n))
	if err := os.WriteFile(p, []byte("snapshot"), 0o600); err != nil {
		return "", err
	}
	return p, nil
}

func listArchives(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestScheduledBackupDisabledWhenNoDir(t *testing.T) {
	sb, err := NewScheduledBackup(ScheduledBackupConfig{})
	if err != nil {
		t.Fatalf("NewScheduledBackup: %v", err)
	}
	if sb != nil {
		t.Fatalf("empty Dir must disable the scheduler (got %v)", sb)
	}
}

func TestScheduledBackupRunOnceWritesArchive(t *testing.T) {
	dir := t.TempDir()
	src := t.TempDir()
	now := time.Date(2026, 6, 1, 4, 0, 0, 0, time.UTC)
	sb, err := NewScheduledBackup(ScheduledBackupConfig{
		Backup: &stubArchiver{tmpDir: src},
		Dir:    dir,
		Now:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewScheduledBackup: %v", err)
	}
	if err := sb.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	got := listArchives(t, dir)
	if len(got) != 1 {
		t.Fatalf("want 1 archive, got %v", got)
	}
	if got[0] != SuggestedFilename(now) {
		t.Errorf("archive name = %q, want %q", got[0], SuggestedFilename(now))
	}
	// The source temp file must have been consumed by the move.
	if leftovers := listArchives(t, src); len(leftovers) != 0 {
		t.Errorf("source temp not cleaned: %v", leftovers)
	}
}

func TestScheduledBackupPrunesToKeep(t *testing.T) {
	dir := t.TempDir()
	// Pre-seed 5 older archives with sortable timestamped names.
	for i := 1; i <= 5; i++ {
		name := fmt.Sprintf("%s2026010%d-040000%s", backupFilePrefix, i, backupFileSuffix)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// An unrelated file the pruner must never touch.
	keepMe := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(keepMe, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 1, 9, 4, 0, 0, 0, time.UTC)
	sb, _ := NewScheduledBackup(ScheduledBackupConfig{
		Backup: &stubArchiver{tmpDir: t.TempDir()},
		Dir:    dir, Keep: 3,
		Now: func() time.Time { return now },
	})
	if err := sb.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	got := listArchives(t, dir)
	// 5 seeded + 1 new = 6 archives → pruned to newest 3, plus notes.txt.
	var archives, others []string
	for _, n := range got {
		if n == "notes.txt" {
			others = append(others, n)
			continue
		}
		archives = append(archives, n)
	}
	if len(archives) != 3 {
		t.Fatalf("want 3 archives after prune, got %v", archives)
	}
	if len(others) != 1 {
		t.Errorf("unrelated files must survive prune, got others=%v", others)
	}
	// Newest kept must include today's archive; oldest seeds gone.
	if archives[len(archives)-1] != SuggestedFilename(now) {
		t.Errorf("newest archive should be today's: %v", archives)
	}
	for _, gone := range []string{"20260101", "20260102", "20260103"} {
		for _, a := range archives {
			if filepath.Base(a) == backupFilePrefix+gone+"-040000"+backupFileSuffix {
				t.Errorf("%s should have been pruned, survivors=%v", gone, archives)
			}
		}
	}
}

func TestScheduledBackupCreateErrorPropagates(t *testing.T) {
	sb, _ := NewScheduledBackup(ScheduledBackupConfig{
		Backup: &stubArchiver{err: fmt.Errorf("boom")},
		Dir:    t.TempDir(),
	})
	if err := sb.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce should surface the archiver error")
	}
}

func TestNextDailyRun(t *testing.T) {
	// 03:00 → same-day 04:00.
	if got := nextDailyRun(time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC), 4); got.Hour() != 4 || got.Day() != 1 {
		t.Errorf("03:00→%v, want same-day 04:00", got)
	}
	// 05:00 → next-day 04:00.
	if got := nextDailyRun(time.Date(2026, 6, 1, 5, 0, 0, 0, time.UTC), 4); got.Hour() != 4 || got.Day() != 2 {
		t.Errorf("05:00→%v, want next-day 04:00", got)
	}
}
