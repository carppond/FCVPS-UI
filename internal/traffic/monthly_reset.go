package traffic

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/util"
)

// MonthlyResetConfig wires the monthly reset worker. Notify is optional —
// when nil the worker still clears threshold state but skips the recap mail.
type MonthlyResetConfig struct {
	TrafficRepo  *storage.TrafficRepo
	SettingsRepo *storage.SettingsRepo
	UserRepo     *storage.UserRepo
	Threshold    *Threshold
	Notify       *notify.Manager
	Clock        util.Clock
	Logger       *slog.Logger
}

// MonthlyReset runs on a daily ticker and triggers per-user reset workflows
// when "today" matches `system_settings.monthly_reset_day`.
//
// "Reset" means:
//
//  1. Emit a "last month summary" notification for each user with traffic
//     activity in the previous billing cycle.
//  2. Clear the user's threshold state so the new month's 80/90/100 alerts
//     re-fire as expected.
//  3. Seed the new month's first traffic_records row with total_used=0 so
//     the chart axis has a non-empty starting anchor.
//
// Idempotence: a per-user "last reset YYYY-MM" key is stored in
// system_settings; reruns of Run on the same day are no-ops once the key
// matches.
type MonthlyReset struct {
	cfg    MonthlyResetConfig
	clock  util.Clock
	logger *slog.Logger
}

// NewMonthlyReset validates dependencies. TrafficRepo + SettingsRepo +
// UserRepo are required; Notify / Threshold / Clock / Logger fall back to
// safe defaults.
func NewMonthlyReset(cfg MonthlyResetConfig) (*MonthlyReset, error) {
	if cfg.TrafficRepo == nil || cfg.SettingsRepo == nil || cfg.UserRepo == nil {
		return nil, fmt.Errorf("monthly reset: TrafficRepo + SettingsRepo + UserRepo are required")
	}
	if cfg.Clock == nil {
		cfg.Clock = util.RealClock{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &MonthlyReset{cfg: cfg, clock: cfg.Clock, logger: cfg.Logger}, nil
}

// Run performs the daily check. If today is not the configured reset day, the
// function is a no-op (returns nil). When the reset day matches, every user
// is iterated and the per-user reset is fanned out.
func (m *MonthlyReset) Run(ctx context.Context) error {
	now := m.clock.Now().UTC()
	resetDay := m.loadResetDay(ctx)
	if now.Day() != resetDay {
		return nil
	}
	users, _, err := m.cfg.UserRepo.List(ctx, storage.UserListOptions{
		Page: 1, PageSize: 10_000,
	})
	if err != nil {
		return fmt.Errorf("monthly reset: list users: %w", err)
	}
	currentMonthKey := now.Format("2006-01")
	processed := 0
	for i := range users {
		u := users[i]
		stateKey := settingTrafficLastResetPrefix + u.ID
		prev, _ := m.cfg.SettingsRepo.Get(ctx, stateKey)
		if prev == currentMonthKey {
			continue
		}
		if err := m.resetUser(ctx, u.ID, now); err != nil {
			m.logger.Warn("monthly reset: user",
				slog.String("user_id", u.ID), slog.String("err", err.Error()))
			continue
		}
		if err := m.cfg.SettingsRepo.Set(ctx, stateKey, currentMonthKey); err != nil {
			m.logger.Warn("monthly reset: save state",
				slog.String("user_id", u.ID), slog.String("err", err.Error()))
		}
		processed++
	}
	m.logger.Info("monthly reset: run complete",
		slog.Int("reset_day", resetDay),
		slog.Int("users", len(users)),
		slog.Int("processed", processed),
	)
	return nil
}

// resetUser performs the per-user reset workflow. Errors propagate to Run so
// the caller can log them with the user context.
func (m *MonthlyReset) resetUser(ctx context.Context, userID string, now time.Time) error {
	// 1. Summarise the previous billing cycle (last month).
	prevMonth := now.AddDate(0, -1, 0)
	limit := m.loadMonthlyLimit(ctx)
	summary, err := m.cfg.TrafficRepo.GetMonthSummary(ctx, userID,
		prevMonth.Year(), prevMonth.Month(), limit)
	if err != nil {
		return fmt.Errorf("get summary: %w", err)
	}
	if summary.TotalUsed > 0 && m.cfg.Notify != nil {
		percent := float64(0)
		if limit > 0 {
			percent = float64(summary.TotalUsed) / float64(limit) * 100
		}
		event := notify.Event{
			Type:       notify.EventTrafficThreshold,
			UserID:     userID,
			ResourceID: "traffic-monthly-recap-" + summary.PeriodStart,
			Subject:    "[shiguang-vps] previous month traffic summary",
			Payload: notify.TrafficThresholdPayload{
				UserID:       userID,
				PeriodStart:  summary.PeriodStart,
				PeriodEnd:    summary.PeriodEnd,
				TotalUsed:    summary.TotalUsed,
				TotalLimit:   summary.TotalLimit,
				UsagePercent: percent,
				ThresholdPct: 0, // 0 → "monthly recap", not a threshold trip
			},
		}
		if _, err := m.cfg.Notify.Emit(ctx, event); err != nil {
			m.logger.Warn("monthly reset: notify",
				slog.String("user_id", userID), slog.String("err", err.Error()))
		}
	}

	// 2. Clear threshold state for the previous month so the new month re-fires.
	if m.cfg.Threshold != nil {
		if err := m.cfg.Threshold.ResetForUser(ctx, userID, prevMonth); err != nil {
			m.logger.Warn("monthly reset: clear threshold",
				slog.String("user_id", userID), slog.String("err", err.Error()))
		}
	}

	// 3. Seed the new month's first row so the chart has an anchor. The
	//    aggregator will overwrite this row at the next 00:30 sweep once real
	//    samples exist.
	if err := m.cfg.TrafficRepo.UpsertDaily(ctx, storage.TrafficRecord{
		Date:       now.Format("2006-01-02"),
		UserID:     userID,
		AgentID:    "",
		TotalLimit: limit,
		TotalUsed:  0, TotalIn: 0, TotalOut: 0,
	}); err != nil {
		return fmt.Errorf("seed first row: %w", err)
	}
	return nil
}

func (m *MonthlyReset) loadResetDay(ctx context.Context) int {
	raw, err := m.cfg.SettingsRepo.Get(ctx, SettingMonthlyResetDay)
	if err != nil || raw == "" {
		return DefaultMonthlyResetDay
	}
	v, ok := parseInt64(raw)
	if !ok || v < 1 || v > 28 {
		return DefaultMonthlyResetDay
	}
	return int(v)
}

func (m *MonthlyReset) loadMonthlyLimit(ctx context.Context) int64 {
	raw, err := m.cfg.SettingsRepo.Get(ctx, SettingMonthlyTrafficLimit)
	if err != nil || raw == "" {
		return 0
	}
	v, ok := parseInt64(raw)
	if !ok {
		return 0
	}
	return v
}

// StartDaily launches the 00:00 UTC daily ticker that drives Run. The returned
// stop func cancels the goroutine. Safe to call multiple times — each call
// returns its own cancel.
func (m *MonthlyReset) StartDaily(ctx context.Context) func() {
	subCtx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			next := nextRunAt(m.clock.Now(), monthlyResetHour, monthlyResetMinute)
			d := next.Sub(m.clock.Now())
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
			if err := m.Run(subCtx); err != nil {
				m.logger.Warn("monthly reset: tick", slog.String("err", err.Error()))
			}
		}
	}()
	return cancel
}

// monthlyResetHour/Minute is 00:00 UTC per T-18.
const (
	monthlyResetHour   = 0
	monthlyResetMinute = 0
)
