package ota

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"shiguang-vps/internal/storage"
)

// WalCheckpointAttempts caps how many times we retry the TRUNCATE checkpoint
// before giving up. Each retry waits WalCheckpointBackoff. SQLite returns
// SQLITE_BUSY when other writers hold the lock; in practice a single retry
// covers the gap between the OTA dispatch and an in-flight ALTER TABLE.
const WalCheckpointAttempts = 3

// WalCheckpointBackoff is the pause between checkpoint retries.
const WalCheckpointBackoff = 250 * time.Millisecond

// WalCheckpoint runs `PRAGMA wal_checkpoint(TRUNCATE)` on db.Write with a
// short retry loop so the OTA applier can guarantee the WAL is empty before
// renaming the binary. Errors are wrapped with the attempt count so operators
// can decide whether to abort the upgrade.
//
// The PRAGMA returns three integers (busy, log, checkpointed). We do not parse
// them — modernc.org/sqlite forwards them through the same row, and any
// failure to flush is treated as a hard error rather than a partial success.
func WalCheckpoint(ctx context.Context, db *storage.DB, logger *slog.Logger) error {
	if db == nil || db.Write == nil {
		return fmt.Errorf("ota: wal checkpoint: nil db")
	}
	var lastErr error
	for attempt := 1; attempt <= WalCheckpointAttempts; attempt++ {
		// Use a per-attempt timeout so a hung writer doesn't deadlock the
		// graceful shutdown sequence (worst case = N × timeout).
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := db.Write.ExecContext(attemptCtx, "PRAGMA wal_checkpoint(TRUNCATE)")
		cancel()
		if err == nil {
			if logger != nil && attempt > 1 {
				logger.Info("ota wal checkpoint succeeded",
					slog.Int("attempt", attempt))
			}
			return nil
		}
		lastErr = err
		if logger != nil {
			logger.Warn("ota wal checkpoint retry",
				slog.Int("attempt", attempt),
				slog.String("err", err.Error()),
			)
		}
		// Honour ctx cancellation between retries so a SIGTERM during the
		// applier still exits promptly.
		select {
		case <-ctx.Done():
			return fmt.Errorf("ota: wal checkpoint cancelled: %w", ctx.Err())
		case <-time.After(WalCheckpointBackoff):
		}
	}
	return fmt.Errorf("ota: wal checkpoint failed after %d attempts: %w", WalCheckpointAttempts, lastErr)
}
