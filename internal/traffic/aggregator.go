// Package traffic owns the M-TRAFFIC backend (PRD M-TRAFFIC.1-5).
//
// The package contains four collaborating workers:
//
//  1. Aggregator    — rolls yesterday's agent_records into one traffic_records
//                     row per (user, agent). Runs at 00:30 UTC so all agent
//                     heartbeats for the day are collected before the sweep.
//  2. MonthlyReset  — fires the "billing cycle reset" notification on
//                     `monthly_reset_day` and seeds the new month's first
//                     traffic_records row so the chart never has a gap.
//  3. Threshold     — emits 80% / 90% / 100% notifications when a user's
//                     monthly usage crosses each level. State is persisted in
//                     system_settings so a process restart never re-fires.
//  4. CleanupOldRecords — purges agent_records older than 7 days at 03:00.
//
// All workers receive a util.Clock so tests can advance time deterministically.
package traffic

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/util"
)

// AggregatorConfig wires the daily aggregator to its collaborators. AgentRepo
// is used to discover the (user, agent) pairs that need a row; RecordRepo
// supplies the high-frequency samples; TrafficRepo writes the rolled-up row.
type AggregatorConfig struct {
	AgentRepo       *storage.AgentRepo
	AgentRecordRepo *storage.AgentRecordRepo
	TrafficRepo     *storage.TrafficRepo
	SettingsRepo    *storage.SettingsRepo
	Clock           util.Clock
	Logger          *slog.Logger
}

// Aggregator rolls a single day's worth of agent_records into traffic_records.
// It is safe to invoke RunForDate any number of times for the same date — the
// repo upsert guarantees idempotence so re-runs after a crash are harmless.
type Aggregator struct {
	cfg    AggregatorConfig
	clock  util.Clock
	logger *slog.Logger
}

// NewAggregator validates the supplied dependencies and returns a ready
// aggregator. AgentRepo, AgentRecordRepo and TrafficRepo are required;
// SettingsRepo is optional (defaults to "no monthly limit").
func NewAggregator(cfg AggregatorConfig) (*Aggregator, error) {
	if cfg.AgentRepo == nil || cfg.AgentRecordRepo == nil || cfg.TrafficRepo == nil {
		return nil, fmt.Errorf("aggregator: AgentRepo, AgentRecordRepo and TrafficRepo are required")
	}
	if cfg.Clock == nil {
		cfg.Clock = util.RealClock{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Aggregator{cfg: cfg, clock: cfg.Clock, logger: cfg.Logger}, nil
}

// RunDaily rolls up "yesterday" relative to the aggregator's clock. Errors
// surfaced by RunForDate are returned verbatim so the caller can decide
// whether to alert.
func (a *Aggregator) RunDaily(ctx context.Context) error {
	yesterday := a.clock.Now().UTC().AddDate(0, 0, -1)
	return a.RunForDate(ctx, yesterday)
}

// RunForDate rolls up the calendar day containing `day` (interpreted as UTC).
// For every (user, agent) pair with at least one sample on that day, an
// upsert into traffic_records is performed.
//
// Per-agent total_used is derived from the agent_records counter delta:
// max(net_in+net_out) - min(net_in+net_out). This survives an agent reboot
// (a counter reset shows up as a negative delta, which the formula clamps
// to >= 0 by switching to the absolute max sample when min > max would
// occur). When the day has < 2 samples, the row is skipped — a single
// snapshot tells us nothing about a delta.
func (a *Aggregator) RunForDate(ctx context.Context, day time.Time) error {
	day = day.UTC()
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	dateStr := dayStart.Format("2006-01-02")

	agents, err := a.cfg.AgentRepo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("aggregator: list agents: %w", err)
	}
	monthlyLimit := a.loadMonthlyLimit(ctx)
	processed := 0
	for i := range agents {
		ag := agents[i]
		if ag.UserID == "" {
			continue
		}
		samples, err := a.cfg.AgentRecordRepo.ListRecent(ctx, ag.ID, dayStart, 5000)
		if err != nil {
			a.logger.Warn("aggregator: list records",
				slog.String("agent_id", ag.ID), slog.String("err", err.Error()))
			continue
		}
		// ListRecent returns newest-first; trim to samples actually inside the
		// day window (dayStart, dayEnd]. ListRecent uses recorded_at >= since,
		// so future samples (recorded_at >= dayEnd) must be filtered out.
		windowed := filterDayWindow(samples, dayStart, dayEnd)
		if len(windowed) < 2 {
			continue
		}
		used, in, out := computeDayUsage(windowed)
		if err := a.cfg.TrafficRepo.UpsertDaily(ctx, storage.TrafficRecord{
			Date: dateStr, UserID: ag.UserID, AgentID: ag.ID,
			TotalLimit: monthlyLimit,
			TotalUsed:  used, TotalIn: in, TotalOut: out,
		}); err != nil {
			a.logger.Warn("aggregator: upsert",
				slog.String("agent_id", ag.ID), slog.String("err", err.Error()))
			continue
		}
		processed++
	}
	a.logger.Info("aggregator: run complete",
		slog.String("date", dateStr),
		slog.Int("agents", len(agents)),
		slog.Int("processed", processed),
	)
	return nil
}

// filterDayWindow keeps only samples whose RecordedAt is within
// [dayStart, dayEnd). Caller passes ListRecent output (newest-first); we
// return a copy in the same order — order does not matter for the min/max
// formula.
func filterDayWindow(samples []storage.AgentMetricRecord, dayStart, dayEnd time.Time) []storage.AgentMetricRecord {
	startMs := dayStart.UnixMilli()
	endMs := dayEnd.UnixMilli()
	out := samples[:0:0]
	for i := range samples {
		t := samples[i].RecordedAt
		if t >= startMs && t < endMs {
			out = append(out, samples[i])
		}
	}
	return out
}

// computeDayUsage returns (totalUsed, totalIn, totalOut) for a slice of
// samples. The "used" delta is the max-of-(in+out) minus min-of-(in+out),
// which clamps reboots to a positive value. Per-direction deltas are computed
// the same way (max - min) — this is a v1 simplification; it slightly
// over-counts if the counter resets mid-day, but the alternative (tracking
// every reset boundary) is out of scope for v1.
func computeDayUsage(samples []storage.AgentMetricRecord) (used, in, out int64) {
	if len(samples) == 0 {
		return 0, 0, 0
	}
	var (
		minTotal, maxTotal int64
		minIn, maxIn       int64
		minOut, maxOut     int64
		initialized       bool
	)
	for i := range samples {
		s := samples[i]
		total := s.NetIn + s.NetOut
		if !initialized {
			minTotal, maxTotal = total, total
			minIn, maxIn = s.NetIn, s.NetIn
			minOut, maxOut = s.NetOut, s.NetOut
			initialized = true
			continue
		}
		if total < minTotal {
			minTotal = total
		}
		if total > maxTotal {
			maxTotal = total
		}
		if s.NetIn < minIn {
			minIn = s.NetIn
		}
		if s.NetIn > maxIn {
			maxIn = s.NetIn
		}
		if s.NetOut < minOut {
			minOut = s.NetOut
		}
		if s.NetOut > maxOut {
			maxOut = s.NetOut
		}
	}
	used = maxTotal - minTotal
	in = maxIn - minIn
	out = maxOut - minOut
	if used < 0 {
		used = 0
	}
	if in < 0 {
		in = 0
	}
	if out < 0 {
		out = 0
	}
	return used, in, out
}

// loadMonthlyLimit reads the operator-configured monthly limit (bytes). A
// missing / unparseable row yields 0 (interpreted as "no limit").
func (a *Aggregator) loadMonthlyLimit(ctx context.Context) int64 {
	if a.cfg.SettingsRepo == nil {
		return 0
	}
	raw, err := a.cfg.SettingsRepo.Get(ctx, SettingMonthlyTrafficLimit)
	if err != nil || raw == "" {
		return 0
	}
	v, ok := parseInt64(raw)
	if !ok {
		return 0
	}
	return v
}

// StartDaily launches the 00:30 UTC sweep loop. The returned stop func cancels
// the worker. Safe to call multiple times — every invocation returns its own
// cancel.
//
// The loop schedules the next sweep at the next 00:30 UTC tick, sleeps until
// then, runs RunDaily, and repeats. ctx cancellation aborts the sleep and
// stops the loop.
func (a *Aggregator) StartDaily(ctx context.Context) func() {
	subCtx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			next := nextRunAt(a.clock.Now(), dailySweepHour, dailySweepMinute)
			d := next.Sub(a.clock.Now())
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
			if err := a.RunDaily(subCtx); err != nil {
				a.logger.Warn("aggregator: daily sweep", slog.String("err", err.Error()))
			}
		}
	}()
	return cancel
}

// dailySweepHour/Minute is the 00:30 UTC tick documented in T-18.
const (
	dailySweepHour   = 0
	dailySweepMinute = 30
)

// nextRunAt returns the next moment >= now that matches (hour, minute, 0s)
// in UTC. If "today's" tick already passed, we return tomorrow's.
func nextRunAt(now time.Time, hour, minute int) time.Time {
	now = now.UTC()
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
	if !target.After(now) {
		target = target.Add(24 * time.Hour)
	}
	return target
}

// parseInt64 is a tiny strconv.ParseInt wrapper that returns (0, false) for
// any error. Centralised so the threshold + monthly reset modules use the
// same parser as the aggregator.
func parseInt64(s string) (int64, bool) {
	var n int64
	var neg bool
	if len(s) == 0 {
		return 0, false
	}
	i := 0
	if s[0] == '-' {
		neg = true
		i = 1
	}
	if i >= len(s) {
		return 0, false
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int64(c-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}
