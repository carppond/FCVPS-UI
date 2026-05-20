package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// rotateOptions configures size/age based rotation. Values are sanitised by
// the caller (see build()); they are required to be positive.
type rotateOptions struct {
	MaxSizeMB  int
	MaxAgeDays int
	MaxBackups int
}

// fileRotator is a minimal size+age log file writer used in place of
// lumberjack. It avoids an external dependency while keeping the behaviour
// observable (rotated files are renamed with a timestamp suffix).
//
// Concurrency: Write/Close are guarded by a mutex. Writes that would push the
// file past MaxSizeMB rotate first.
type fileRotator struct {
	path string
	opts rotateOptions

	mu   sync.Mutex
	file *os.File
	size int64
}

// ErrRotatorClosed indicates a Write attempt on a closed rotator.
var ErrRotatorClosed = errors.New("log rotator closed")

// newRotator opens (or creates) the active log file and returns a writer that
// rotates on size/age. The parent directory is created when missing.
func newRotator(path string, opts rotateOptions) (*fileRotator, error) {
	if path == "" {
		return nil, fmt.Errorf("log rotator: path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	r := &fileRotator{path: path, opts: opts}
	if err := r.open(); err != nil {
		return nil, err
	}
	// Best-effort prune on startup (caller is fine with errors here).
	r.prune()
	return r, nil
}

// Write satisfies io.Writer. It rotates the file if the next write would
// exceed MaxSizeMB.
func (r *fileRotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return 0, ErrRotatorClosed
	}
	maxBytes := int64(r.opts.MaxSizeMB) * 1024 * 1024
	if maxBytes > 0 && r.size+int64(len(p)) > maxBytes {
		if err := r.rotateLocked(); err != nil {
			return 0, err
		}
	}
	n, err := r.file.Write(p)
	r.size += int64(n)
	if err != nil {
		return n, fmt.Errorf("log file write: %w", err)
	}
	return n, nil
}

// Close flushes and closes the active file. Safe to call multiple times.
func (r *fileRotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	if err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	return nil
}

// open creates / re-opens the active file in append mode and seeds size.
func (r *fileRotator) open() error {
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open log file %q: %w", r.path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("stat log file: %w", err)
	}
	r.file = f
	r.size = info.Size()
	return nil
}

// rotateLocked renames the active file with a timestamp suffix and opens a
// new active file. Caller must hold r.mu.
func (r *fileRotator) rotateLocked() error {
	if err := r.file.Close(); err != nil {
		return fmt.Errorf("close before rotate: %w", err)
	}
	r.file = nil
	rotated := r.rotatedName(time.Now())
	if err := os.Rename(r.path, rotated); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rename log file: %w", err)
	}
	if err := r.open(); err != nil {
		return err
	}
	r.prune()
	return nil
}

// rotatedName builds the historical filename for the current rotation point.
func (r *fileRotator) rotatedName(at time.Time) string {
	dir := filepath.Dir(r.path)
	base := filepath.Base(r.path)
	stem := base
	ext := ""
	if i := strings.LastIndex(base, "."); i > 0 {
		stem = base[:i]
		ext = base[i:]
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%s%s", stem, at.UTC().Format("20060102-150405"), ext))
}

// prune deletes rotated files older than MaxAgeDays or beyond MaxBackups.
// Errors are silently ignored; logger should never break the caller.
func (r *fileRotator) prune() {
	dir := filepath.Dir(r.path)
	base := filepath.Base(r.path)
	stem := base
	ext := ""
	if i := strings.LastIndex(base, "."); i > 0 {
		stem = base[:i]
		ext = base[i:]
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := stem + "-"
	type rotated struct {
		path    string
		modTime time.Time
	}
	var matched []rotated
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ext) {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		matched = append(matched, rotated{
			path:    filepath.Join(dir, name),
			modTime: info.ModTime(),
		})
	}
	if len(matched) == 0 {
		return
	}
	// Newest first.
	sort.Slice(matched, func(i, j int) bool { return matched[i].modTime.After(matched[j].modTime) })
	cutoff := time.Now().Add(-time.Duration(r.opts.MaxAgeDays) * 24 * time.Hour)
	for idx, m := range matched {
		shouldRemove := false
		if r.opts.MaxBackups > 0 && idx >= r.opts.MaxBackups {
			shouldRemove = true
		}
		if r.opts.MaxAgeDays > 0 && m.modTime.Before(cutoff) {
			shouldRemove = true
		}
		if shouldRemove {
			_ = os.Remove(m.path)
		}
	}
}
