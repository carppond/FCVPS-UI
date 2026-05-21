package traffic

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/util"
)

// AgentRecordsRetention is the 7-day default per PRD M-AGENT.5. Tests may
// override the constant by configuring CleanupConfig.Retention directly; the
// production cron uses this default unless system_settings overrides it
// (left to a future task).
const AgentRecordsRetention = 7 * 24 * time.Hour

// CleanupConfig wires the cleanup worker. Retention defaults to 7 days when
// zero.
type CleanupConfig struct {
	AgentRecordRepo *storage.AgentRecordRepo
	Retention       time.Duration
	Clock           util.Clock
	Logger          *slog.Logger
}

// Cleanup deletes agent_records older than the retention window. It is safe
// to call repeatedly — the underlying DELETE is idempotent. The 03:00 UTC
// schedule chosen by T-18 sits between the 00:30 aggregator sweep and the
// morning operator window, minimising contention with metrics writes.
type Cleanup struct {
	cfg    CleanupConfig
	clock  util.Clock
	logger *slog.Logger
}

// NewCleanup validates dependencies and returns a ready worker.
func NewCleanup(cfg CleanupConfig) (*Cleanup, error) {
	if cfg.AgentRecordRepo == nil {
		return nil, fmt.Errorf("cleanup: AgentRecordRepo is required")
	}
	if cfg.Retention <= 0 {
		cfg.Retention = AgentRecordsRetention
	}
	if cfg.Clock == nil {
		cfg.Clock = util.RealClock{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Cleanup{cfg: cfg, clock: cfg.Clock, logger: cfg.Logger}, nil
}

// CleanupOldRecords runs the DELETE once. Returns the number of rows removed.
func (c *Cleanup) CleanupOldRecords(ctx context.Context) (int64, error) {
	cutoff := c.clock.Now().Add(-c.cfg.Retention)
	n, err := c.cfg.AgentRecordRepo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup: delete: %w", err)
	}
	if n > 0 {
		c.logger.Info("cleanup: deleted old agent_records",
			slog.Int64("rows", n),
			slog.Time("cutoff", cutoff),
		)
	}
	return n, nil
}

// StartDaily launches the 03:00 UTC sweep loop. The returned stop func
// cancels the worker. Safe to call multiple times.
func (c *Cleanup) StartDaily(ctx context.Context) func() {
	subCtx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			next := nextRunAt(c.clock.Now(), cleanupHour, cleanupMinute)
			d := next.Sub(c.clock.Now())
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
			if _, err := c.CleanupOldRecords(subCtx); err != nil {
				c.logger.Warn("cleanup: sweep", slog.String("err", err.Error()))
			}
		}
	}()
	return cancel
}

// cleanupHour/Minute is 03:00 UTC per T-18.
const (
	cleanupHour   = 3
	cleanupMinute = 0
)
