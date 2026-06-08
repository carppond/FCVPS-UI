// Package alert evaluates user-defined agent alert rules (CPU/MEM/Disk/offline)
// on a periodic ticker and fires notifications through the notify manager when a
// rule's condition is met and its per-(rule,agent) cooldown has elapsed.
//
// It deliberately mirrors internal/traffic.Threshold: stateless evaluation +
// cooldown state stored in system_settings (no extra table), notify failures
// logged not propagated.
package alert

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// alertStatePrefix keys the last-fired timestamp (unix ms) per (rule, agent) in
// system_settings, used for cooldown suppression.
const alertStatePrefix = "alert_state:"

// metricSampleWindow caps how far back ListRecent looks when DurationSec is 0
// (instant rules) — the most recent sample within this window is "current".
const metricSampleWindow = 3 * time.Minute

// AlertRepo is the subset of storage.AlertRuleRepo the engine needs.
type AlertRepo interface {
	ListEnabled(ctx context.Context) ([]storage.AlertRuleRecord, error)
}

// AgentRepo resolves a rule's target agents.
type AgentRepo interface {
	GetByID(ctx context.Context, id, userID string) (*storage.AgentRecord, error)
	ListByUser(ctx context.Context, userID string, opts storage.AgentListOptions) ([]storage.AgentRecord, int64, error)
}

// RecordRepo reads recent metric samples.
type RecordRepo interface {
	ListRecent(ctx context.Context, agentID string, since time.Time, limit int) ([]storage.AgentMetricRecord, error)
}

// SettingsRepo persists cooldown state.
type SettingsRepo interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// Notifier emits alerts.
type Notifier interface {
	Emit(ctx context.Context, event notify.Event) (int, error)
}

// Config wires the engine.
type Config struct {
	Rules    AlertRepo
	Agents   AgentRepo
	Records  RecordRepo
	Settings SettingsRepo
	Notify   Notifier // may be nil (alerts skipped)
	Now      func() time.Time
	Logger   *slog.Logger
}

// Engine evaluates alert rules.
type Engine struct {
	cfg Config
	now func() time.Time
	log *slog.Logger
}

// NewEngine validates and constructs the engine.
func NewEngine(cfg Config) (*Engine, error) {
	if cfg.Rules == nil || cfg.Agents == nil || cfg.Records == nil || cfg.Settings == nil {
		return nil, fmt.Errorf("alert engine: rules/agents/records/settings required")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Engine{cfg: cfg, now: now, log: log}, nil
}

// EvaluateOnce runs a single evaluation pass over all enabled rules.
func (e *Engine) EvaluateOnce(ctx context.Context) {
	rules, err := e.cfg.Rules.ListEnabled(ctx)
	if err != nil {
		e.log.Warn("alert: list enabled rules", slog.String("err", err.Error()))
		return
	}
	for _, rule := range rules {
		e.evalRule(ctx, rule)
	}
}

func (e *Engine) evalRule(ctx context.Context, rule storage.AlertRuleRecord) {
	agents, err := e.targetAgents(ctx, rule)
	if err != nil {
		e.log.Warn("alert: resolve agents", slog.String("rule", rule.ID), slog.String("err", err.Error()))
		return
	}
	for _, ag := range agents {
		breached, value, detail := e.evalAgent(ctx, rule, ag)
		if !breached {
			continue
		}
		if !e.cooldownElapsed(ctx, rule, ag.ID) {
			continue
		}
		e.fire(ctx, rule, ag, value, detail)
	}
}

// targetAgents returns the agents a rule applies to: a single agent when
// AgentID is set, otherwise all of the owner's agents.
func (e *Engine) targetAgents(ctx context.Context, rule storage.AlertRuleRecord) ([]storage.AgentRecord, error) {
	if rule.AgentID != "" {
		ag, err := e.cfg.Agents.GetByID(ctx, rule.AgentID, rule.UserID)
		if err != nil {
			return nil, nil //nolint:nilerr // agent gone / not owned → skip, not an error
		}
		return []storage.AgentRecord{*ag}, nil
	}
	list, _, err := e.cfg.Agents.ListByUser(ctx, rule.UserID, storage.AgentListOptions{Page: 1, PageSize: 1000})
	return list, err
}

// evalAgent reports whether the rule is breached for ag, plus the observed
// value (% for metrics) and a human-readable detail string.
func (e *Engine) evalAgent(ctx context.Context, rule storage.AlertRuleRecord, ag storage.AgentRecord) (bool, float64, string) {
	if types.AlertMetric(rule.Metric) == types.AlertMetricOffline {
		return e.evalOffline(rule, ag)
	}
	return e.evalMetric(ctx, rule, ag)
}

func (e *Engine) evalOffline(rule storage.AlertRuleRecord, ag storage.AgentRecord) (bool, float64, string) {
	if ag.Status == "online" {
		return false, 0, ""
	}
	offlineFor := time.Duration(0)
	if ag.LastSeenAt > 0 {
		offlineFor = e.now().Sub(time.UnixMilli(ag.LastSeenAt))
	}
	if rule.DurationSec > 0 && offlineFor < time.Duration(rule.DurationSec)*time.Second {
		return false, 0, "" // not offline long enough yet
	}
	detail := "已离线"
	if offlineFor > 0 {
		detail = fmt.Sprintf("已离线 %s", offlineFor.Round(time.Minute))
	}
	return true, 0, detail
}

func (e *Engine) evalMetric(ctx context.Context, rule storage.AlertRuleRecord, ag storage.AgentRecord) (bool, float64, string) {
	window := metricSampleWindow
	if rule.DurationSec > 0 {
		window = time.Duration(rule.DurationSec) * time.Second
	}
	since := e.now().Add(-window)
	recs, err := e.cfg.Records.ListRecent(ctx, ag.ID, since, 5000)
	if err != nil || len(recs) == 0 {
		return false, 0, "" // no data → can't breach
	}
	metric := types.AlertMetric(rule.Metric)
	// Instant rule (DurationSec==0): use the single most recent sample.
	if rule.DurationSec <= 0 {
		v := metricValue(metric, recs[0])
		if v >= rule.Threshold {
			return true, v, ""
		}
		return false, v, ""
	}
	// Sustained rule: every sample in the window must exceed the threshold,
	// and the window must actually contain samples spanning ~its length.
	minV := 100.0
	for _, r := range recs {
		v := metricValue(metric, r)
		if v < rule.Threshold {
			return false, v, ""
		}
		if v < minV {
			minV = v
		}
	}
	return true, minV, fmt.Sprintf("持续 %s", window.Round(time.Minute))
}

// metricValue extracts the percentage for the given metric from a sample.
func metricValue(metric types.AlertMetric, r storage.AgentMetricRecord) float64 {
	switch metric {
	case types.AlertMetricCPU:
		return r.CPUPercent
	case types.AlertMetricMem:
		return pct(r.MemUsed, r.MemTotal)
	case types.AlertMetricDisk:
		return pct(r.DiskUsed, r.DiskTotal)
	default:
		return 0
	}
}

func pct(used, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

func (e *Engine) cooldownElapsed(ctx context.Context, rule storage.AlertRuleRecord, agentID string) bool {
	key := alertStatePrefix + rule.ID + ":" + agentID
	raw, _ := e.cfg.Settings.Get(ctx, key)
	if raw == "" {
		return true
	}
	last, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return true
	}
	cooldown := time.Duration(rule.CooldownSec) * time.Second
	return e.now().Sub(time.UnixMilli(last)) >= cooldown
}

func (e *Engine) fire(ctx context.Context, rule storage.AlertRuleRecord, ag storage.AgentRecord, value float64, detail string) {
	// Record the fire time first so a notify failure still arms the cooldown
	// (avoids a tight retry storm against a broken channel).
	key := alertStatePrefix + rule.ID + ":" + ag.ID
	if err := e.cfg.Settings.Set(ctx, key, strconv.FormatInt(e.now().UnixMilli(), 10)); err != nil {
		e.log.Warn("alert: persist cooldown", slog.String("rule", rule.ID), slog.String("err", err.Error()))
	}
	if e.cfg.Notify == nil {
		return
	}
	event := notify.Event{
		Type:       notify.EventProbeAlert,
		UserID:     rule.UserID,
		ResourceID: rule.ID + ":" + ag.ID,
		Subject:    fmt.Sprintf("[shiguang-vps] probe alert: %s %s", ag.Name, rule.Metric),
		Payload: notify.ProbeAlertPayload{
			RuleID:    rule.ID,
			RuleName:  rule.Name,
			AgentID:   ag.ID,
			AgentName: ag.Name,
			Metric:    rule.Metric,
			Value:     value,
			Threshold: rule.Threshold,
			Detail:    detail,
		},
	}
	if _, err := e.cfg.Notify.Emit(ctx, event); err != nil {
		e.log.Warn("alert: notify emit", slog.String("rule", rule.ID), slog.String("err", err.Error()))
	}
}

// StartLoop runs EvaluateOnce every interval until ctx is cancelled. Returns a
// stop func. interval <= 0 defaults to 60s.
func (e *Engine) StartLoop(ctx context.Context, interval time.Duration) func() {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				e.EvaluateOnce(ctx)
			}
		}
	}()
	return cancel
}
