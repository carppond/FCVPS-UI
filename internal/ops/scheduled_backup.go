package ops

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// backupFilePrefix / backupFileSuffix bound the names ScheduledBackup writes
// and prunes, so it never touches unrelated files in the target directory.
const (
	backupFilePrefix = "shiguang-backup-"
	backupFileSuffix = ".tar.gz"
)

// archiver is the subset of *Backup the scheduler needs (lets tests stub it).
type archiver interface {
	Create(ctx context.Context) (string, error)
}

// ScheduledBackupConfig wires the scheduler. Enabled only when Dir is set.
type ScheduledBackupConfig struct {
	Backup archiver
	// Dir is the destination directory for nightly archives. Empty disables
	// the scheduler entirely.
	Dir string
	// Keep is how many newest archives to retain; older ones are pruned.
	Keep int
	// Hour is the UTC hour (0-23) to run at.
	Hour int
	Now  func() time.Time
	Log  *slog.Logger
}

// ScheduledBackup writes a nightly backup archive into a directory and prunes
// older archives beyond the retention count. It reuses ops.Backup.Create and
// the same daily-ticker pattern as the traffic sweeps.
type ScheduledBackup struct {
	cfg ScheduledBackupConfig
	now func() time.Time
	log *slog.Logger
}

// NewScheduledBackup constructs the scheduler. Returns nil, nil when Dir is
// empty (feature disabled) so callers can `if sb != nil { sb.StartDaily(...) }`.
func NewScheduledBackup(cfg ScheduledBackupConfig) (*ScheduledBackup, error) {
	if cfg.Dir == "" {
		return nil, nil
	}
	if cfg.Backup == nil {
		return nil, fmt.Errorf("scheduled backup: archiver required")
	}
	if cfg.Keep <= 0 {
		cfg.Keep = 7
	}
	if cfg.Hour < 0 || cfg.Hour > 23 {
		cfg.Hour = 4
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &ScheduledBackup{cfg: cfg, now: cfg.Now, log: cfg.Log}, nil
}

// RunOnce creates one archive into Dir and prunes old ones. Exposed for tests
// and a potential manual trigger.
func (s *ScheduledBackup) RunOnce(ctx context.Context) error {
	if err := os.MkdirAll(s.cfg.Dir, 0o750); err != nil {
		return fmt.Errorf("scheduled backup: mkdir: %w", err)
	}
	tmp, err := s.cfg.Backup.Create(ctx)
	if err != nil {
		return fmt.Errorf("scheduled backup: create: %w", err)
	}
	defer os.Remove(tmp) // best-effort cleanup if the move fell back to copy

	dest := filepath.Join(s.cfg.Dir, SuggestedFilename(s.now()))
	if err := moveFile(tmp, dest); err != nil {
		return fmt.Errorf("scheduled backup: move: %w", err)
	}
	s.log.Info("scheduled backup written", slog.String("path", dest))
	if pruned := s.prune(); pruned > 0 {
		s.log.Info("scheduled backup pruned old archives", slog.Int("removed", pruned))
	}
	return nil
}

// prune deletes archives beyond Keep (newest by filename, which is timestamped
// and therefore lexicographically sortable). Returns the count removed.
func (s *ScheduledBackup) prune() int {
	entries, err := os.ReadDir(s.cfg.Dir)
	if err != nil {
		return 0
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasPrefix(n, backupFilePrefix) && strings.HasSuffix(n, backupFileSuffix) {
			names = append(names, n)
		}
	}
	if len(names) <= s.cfg.Keep {
		return 0
	}
	sort.Strings(names) // ascending; oldest first
	removed := 0
	for _, n := range names[:len(names)-s.cfg.Keep] {
		if err := os.Remove(filepath.Join(s.cfg.Dir, n)); err == nil {
			removed++
		}
	}
	return removed
}

// StartDaily launches the once-a-day loop at the configured UTC hour. The
// returned func cancels the worker.
func (s *ScheduledBackup) StartDaily(ctx context.Context) func() {
	subCtx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			next := nextDailyRun(s.now().UTC(), s.cfg.Hour)
			d := next.Sub(s.now().UTC())
			if d <= 0 {
				d = time.Second
			}
			timer := time.NewTimer(d)
			select {
			case <-subCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			if err := s.RunOnce(subCtx); err != nil {
				s.log.Warn("scheduled backup failed", slog.String("err", err.Error()))
			}
		}
	}()
	return cancel
}

// nextDailyRun returns the next occurrence of hour:00 UTC strictly after now.
func nextDailyRun(now time.Time, hour int) time.Time {
	now = now.UTC()
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, time.UTC)
	if !target.After(now) {
		target = target.Add(24 * time.Hour)
	}
	return target
}

// moveFile renames src→dst, falling back to copy+remove across filesystems
// (the temp dir and the backup dir may be on different mounts).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}
