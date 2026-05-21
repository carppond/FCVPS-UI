package traffic

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/util"
)

// ThresholdConfig wires the threshold checker to its collaborators. Notify is
// optional — when nil the checker still tracks state but skips the emit (used
// by tests that only care about the state machine).
type ThresholdConfig struct {
	TrafficRepo  *storage.TrafficRepo
	SettingsRepo *storage.SettingsRepo
	Notify       *notify.Manager
	Clock        util.Clock
	Logger       *slog.Logger
}

// Threshold fires 80% / 90% / 100% notifications when a user's monthly usage
// crosses each level. Dedupe is twofold:
//
//   - In-memory: the notify.Manager's 5-minute deduper collapses rapid-fire
//     checks for the same user.
//   - On-disk:   the per-user state key in system_settings records which
//     percentage levels have already fired for the current month so a process
//     restart never re-emits a level the user has already received.
//
// State is cleared automatically at month boundaries — the key includes the
// YYYY-MM so old months simply stop being looked up. (A future janitor can
// purge stale keys; v1 leaves them in place because the row count is bounded
// by users × months.)
type Threshold struct {
	cfg    ThresholdConfig
	clock  util.Clock
	logger *slog.Logger
}

// NewThreshold validates dependencies. TrafficRepo + SettingsRepo are
// required. Notify, Clock and Logger fall back to sensible defaults.
func NewThreshold(cfg ThresholdConfig) (*Threshold, error) {
	if cfg.TrafficRepo == nil || cfg.SettingsRepo == nil {
		return nil, fmt.Errorf("threshold: TrafficRepo and SettingsRepo are required")
	}
	if cfg.Clock == nil {
		cfg.Clock = util.RealClock{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Threshold{cfg: cfg, clock: cfg.Clock, logger: cfg.Logger}, nil
}

// CheckAndAlert evaluates the current usage for userID against every
// configured threshold percent (default 80 / 90 / 100). For each level the
// user has crossed (and has not already been notified about this month), a
// notify.Emit is invoked and the state is persisted.
//
// Returns the slice of percentage levels that fired in this invocation (may
// be empty). Errors from the notifier are logged and not propagated — a
// failed alert must not block subsequent ones.
func (t *Threshold) CheckAndAlert(ctx context.Context, userID string) ([]int, error) {
	if userID == "" {
		return nil, fmt.Errorf("threshold: empty user_id")
	}
	limit, levels, err := t.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || len(levels) == 0 {
		return nil, nil
	}
	now := t.clock.Now().UTC()
	period := monthBounds(now)
	used, _, _, err := t.cfg.TrafficRepo.SumWindow(ctx, userID, period.start, period.end)
	if err != nil {
		return nil, fmt.Errorf("threshold: sum window: %w", err)
	}
	percent := float64(used) / float64(limit) * 100
	monthKey := now.Format("2006-01")
	stateKey := settingTrafficLastThresholdPrefix + userID + ":" + monthKey
	already, _ := t.loadState(ctx, stateKey)
	alreadyMap := make(map[int]bool, len(already))
	for _, l := range already {
		alreadyMap[l] = true
	}

	fired := make([]int, 0, len(levels))
	for _, lvl := range levels {
		if percent < float64(lvl) {
			continue
		}
		if alreadyMap[lvl] {
			continue
		}
		t.emit(ctx, userID, lvl, used, limit, percent, period)
		fired = append(fired, lvl)
		alreadyMap[lvl] = true
	}
	if len(fired) > 0 {
		if err := t.saveState(ctx, stateKey, alreadyMap); err != nil {
			t.logger.Warn("threshold: save state",
				slog.String("user_id", userID), slog.String("err", err.Error()))
		}
	}
	return fired, nil
}

// ResetForUser clears the recorded state for userID. The MonthlyReset worker
// calls this when the new billing cycle starts so the next month's
// thresholds re-fire as expected.
func (t *Threshold) ResetForUser(ctx context.Context, userID string, monthAnchor time.Time) error {
	if userID == "" {
		return fmt.Errorf("threshold: empty user_id")
	}
	monthKey := monthAnchor.UTC().Format("2006-01")
	stateKey := settingTrafficLastThresholdPrefix + userID + ":" + monthKey
	return t.cfg.SettingsRepo.Set(ctx, stateKey, "")
}

func (t *Threshold) emit(ctx context.Context, userID string, level int, used, limit int64, percent float64, period monthRange) {
	if t.cfg.Notify == nil {
		return
	}
	subject := fmt.Sprintf("[shiguang-vps] traffic usage reached %d%%", level)
	event := notify.Event{
		Type:       notify.EventTrafficThreshold,
		UserID:     userID,
		ResourceID: fmt.Sprintf("traffic-%s-%d", period.start.Format("2006-01"), level),
		Subject:    subject,
		Payload: notify.TrafficThresholdPayload{
			UserID:       userID,
			PeriodStart:  period.start.Format("2006-01-02"),
			PeriodEnd:    period.end.Format("2006-01-02"),
			TotalUsed:    used,
			TotalLimit:   limit,
			UsagePercent: percent,
			ThresholdPct: int32(level),
		},
	}
	if _, err := t.cfg.Notify.Emit(ctx, event); err != nil {
		t.logger.Warn("threshold: notify emit",
			slog.String("user_id", userID),
			slog.Int("level", level),
			slog.String("err", err.Error()))
	}
}

// loadConfig reads (monthlyLimit, thresholdPercents). Defaults apply when the
// rows are missing.
func (t *Threshold) loadConfig(ctx context.Context) (int64, []int, error) {
	limit := int64(0)
	rawLimit, err := t.cfg.SettingsRepo.Get(ctx, SettingMonthlyTrafficLimit)
	if err == nil && rawLimit != "" {
		if v, ok := parseInt64(rawLimit); ok {
			limit = v
		}
	}
	levels := append([]int(nil), DefaultThresholdPercents...)
	rawLevels, err := t.cfg.SettingsRepo.Get(ctx, SettingTrafficThresholdPercents)
	if err == nil && rawLevels != "" {
		parsed := parseLevels(rawLevels)
		if len(parsed) > 0 {
			levels = parsed
		}
	}
	sort.Ints(levels)
	return limit, levels, nil
}

func (t *Threshold) loadState(ctx context.Context, key string) ([]int, error) {
	raw, err := t.cfg.SettingsRepo.Get(ctx, key)
	if err != nil {
		return nil, nil // missing row → empty state
	}
	return parseLevels(raw), nil
}

func (t *Threshold) saveState(ctx context.Context, key string, levels map[int]bool) error {
	out := make([]int, 0, len(levels))
	for k := range levels {
		out = append(out, k)
	}
	sort.Ints(out)
	parts := make([]string, 0, len(out))
	for _, l := range out {
		parts = append(parts, strconv.Itoa(l))
	}
	return t.cfg.SettingsRepo.Set(ctx, key, strings.Join(parts, ","))
}

// parseLevels splits "80,90,100" into [80,90,100]. Whitespace and duplicates
// are tolerated; non-numeric segments are skipped.
func parseLevels(s string) []int {
	parts := strings.Split(s, ",")
	seen := make(map[int]struct{}, len(parts))
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		if n < 0 || n > 1000 {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

// monthRange is a (start, end) tuple covering a calendar month (UTC).
type monthRange struct {
	start time.Time
	end   time.Time
}

// monthBounds returns the [first-of-month, last-of-month] window in UTC
// containing t.
func monthBounds(t time.Time) monthRange {
	t = t.UTC()
	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	return monthRange{start: start, end: end}
}
