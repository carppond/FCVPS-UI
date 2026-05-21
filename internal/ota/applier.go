package ota

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"shiguang-vps/internal/storage"
)

// ApplierConfig wires the Applier dependencies.
type ApplierConfig struct {
	// DB is required: the applier checkpoints the WAL before replacing the
	// binary so the new process starts against a clean main file.
	DB *storage.DB
	// BinaryPath points at the running executable that should be overwritten
	// at the end of Apply. Defaults to os.Executable() when empty.
	BinaryPath string
	// Shutdown is invoked once the new binary is staged + renamed; it should
	// trigger the graceful-shutdown path so an external supervisor restarts
	// the process with the new binary. nil disables the trigger (used by
	// tests so the test runner does not get torn down).
	Shutdown func()
	// Logger receives structured progress / failure events. Optional.
	Logger *slog.Logger
	// Mode is the permission bits applied to the new binary post-rename
	// (defaults to 0o755 so systemd can re-exec it).
	Mode os.FileMode
}

// Applier coordinates the final stage of an OTA: WAL checkpoint, atomic
// rename, chmod, then trigger graceful shutdown. The methods are NOT
// goroutine-safe — at most one Apply must be in flight per process.
type Applier struct {
	db         *storage.DB
	binaryPath string
	shutdown   func()
	logger     *slog.Logger
	mode       os.FileMode
}

// NewApplier builds an Applier. Returns an error when BinaryPath is empty AND
// os.Executable() fails to resolve.
func NewApplier(cfg ApplierConfig) (*Applier, error) {
	bin := cfg.BinaryPath
	if bin == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("ota: resolve executable: %w", err)
		}
		// Follow symlinks so the rename targets the real file (systemd
		// frequently points /usr/local/bin/foo at /opt/foo-1.2.3/bin/foo).
		resolved, err := filepath.EvalSymlinks(exe)
		if err == nil {
			exe = resolved
		}
		bin = exe
	}
	mode := cfg.Mode
	if mode == 0 {
		mode = 0o755
	}
	return &Applier{
		db:         cfg.DB,
		binaryPath: bin,
		shutdown:   cfg.Shutdown,
		logger:     cfg.Logger,
		mode:       mode,
	}, nil
}

// BinaryPath returns the configured target for visibility (handler logs).
func (a *Applier) BinaryPath() string {
	if a == nil {
		return ""
	}
	return a.binaryPath
}

// Apply performs the atomic replacement:
//
//  1. WAL checkpoint (TRUNCATE) so the new process sees a clean main DB.
//  2. Chmod the new binary to 0o755 before rename — Linux allows replacing a
//     running text segment via rename, but only when the destination is
//     already +x (otherwise the supervisor's exec will EACCES).
//  3. Rename the running binary into `<bin>.bak` (best-effort; ignored when
//     the file is unlinked by something racing with us).
//  4. Rename newBinaryPath → binaryPath atomically (same-filesystem).
//  5. Fire the shutdown callback so systemd / docker restart=always brings
//     the new binary online.
//
// On rename failure the .bak is moved back so the supervisor never finds a
// missing executable. Errors carry the failing step so handler logs are
// actionable.
func (a *Applier) Apply(ctx context.Context, newBinaryPath string) error {
	if a == nil {
		return fmt.Errorf("ota: nil applier")
	}
	if a.binaryPath == "" {
		return fmt.Errorf("ota: apply: binary path unresolved")
	}
	if newBinaryPath == "" {
		return fmt.Errorf("ota: apply: empty new binary path")
	}
	if !sameDir(a.binaryPath, newBinaryPath) {
		// Cross-directory rename works only when both paths sit on the same
		// filesystem. We restrict to the same directory because the handler
		// always stages the download as `<bin>.new` next to the running
		// binary — a different parent here usually means a misconfiguration.
		return fmt.Errorf("ota: apply: new binary must live alongside %s (got %s)", a.binaryPath, newBinaryPath)
	}
	if info, err := os.Stat(newBinaryPath); err != nil {
		return fmt.Errorf("ota: apply: stat new binary: %w", err)
	} else if info.Size() == 0 {
		return fmt.Errorf("ota: apply: new binary is empty")
	}

	// 1. Checkpoint WAL so the new process starts cleanly.
	if err := WalCheckpoint(ctx, a.db, a.logger); err != nil {
		return fmt.Errorf("ota: apply: wal checkpoint: %w", err)
	}

	// 2. Ensure the new file is executable BEFORE rename. Required because
	// some filesystems (FAT-on-USB, NFS w/ root-squash) drop mode bits on
	// rename and the supervisor's exec would EACCES.
	if err := os.Chmod(newBinaryPath, a.mode); err != nil {
		return fmt.Errorf("ota: apply: chmod new binary: %w", err)
	}

	// 3. Stash the running binary as <bin>.bak so a failed rename can be
	// rolled back. On filesystems that lack hard-link support (rare) the
	// rename below is the only safety net.
	bakPath := a.binaryPath + ".bak"
	bakStashed := false
	if _, err := os.Stat(a.binaryPath); err == nil {
		// Best-effort: ignore the failure but record it for the post-mortem.
		if err := os.Rename(a.binaryPath, bakPath); err != nil {
			if a.logger != nil {
				a.logger.Warn("ota: failed to stash backup",
					slog.String("err", err.Error()),
					slog.String("path", bakPath),
				)
			}
		} else {
			bakStashed = true
		}
	}

	// 4. Atomic swap. If this fails we restore the backup so the supervisor
	// keeps using the previous binary; the operator can inspect <bin>.new.
	if err := os.Rename(newBinaryPath, a.binaryPath); err != nil {
		if bakStashed {
			if rbErr := os.Rename(bakPath, a.binaryPath); rbErr != nil && a.logger != nil {
				a.logger.Error("ota: rollback rename failed",
					slog.String("err", rbErr.Error()))
			}
		}
		return fmt.Errorf("ota: apply: rename: %w", err)
	}
	if a.logger != nil {
		a.logger.Info("ota apply succeeded",
			slog.String("binary", a.binaryPath),
			slog.String("bak", bakPath),
			slog.String("goos", runtime.GOOS),
		)
	}

	// 5. Schedule the shutdown trigger AFTER a short delay so the HTTP
	// handler has time to drain the response back to the admin UI ("OK,
	// restarting…") before the listener stops accepting connections. The
	// supervisor (systemd, docker) is responsible for bringing the new
	// binary back up.
	if a.shutdown != nil {
		go func(cb func()) {
			time.Sleep(ShutdownGrace)
			cb()
		}(a.shutdown)
	}
	return nil
}

// ShutdownGrace is the gap between Apply returning success and the shutdown
// trigger firing. Long enough for the SSE "done" event + HTTP 200 response to
// reach the browser; short enough that an operator notices the restart.
const ShutdownGrace = 1500 * time.Millisecond

// sameDir checks that two paths share the same parent directory; used as a
// cheap "same filesystem" heuristic for atomic rename.
func sameDir(a, b string) bool {
	return filepath.Dir(a) == filepath.Dir(b)
}

// StageTo opens (truncating) `<bin>.new` next to the binary and returns the
// path + an *os.File the downloader can stream into. The caller closes the
// file; the path can then be handed to Apply.
//
// Centralising the path computation here means the handler and the applier
// agree on the convention without each maintaining a hard-coded suffix.
func (a *Applier) StageTo() (string, *os.File, error) {
	if a == nil || a.binaryPath == "" {
		return "", nil, fmt.Errorf("ota: stage: binary path unresolved")
	}
	stagePath := a.binaryPath + ".new"
	// Truncate any stale staging file from an earlier failed run; otherwise
	// the SHA-256 check would compare against unrelated bytes.
	if err := os.Remove(stagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", nil, fmt.Errorf("ota: stage: clear old: %w", err)
	}
	f, err := os.OpenFile(stagePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", nil, fmt.Errorf("ota: stage: open: %w", err)
	}
	return stagePath, f, nil
}

// drainFile is a small helper used by tests: copies r to a newly-created file
// at path and returns the byte count. Kept package-private so tests can build
// fake binaries without dragging in an os.WriteFile dependency.
func drainFile(path string, r io.Reader) (int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, r)
}
