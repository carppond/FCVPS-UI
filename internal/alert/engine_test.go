package alert

import (
	"context"
	"testing"
	"time"

	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/storage"
)

// ── fakes ───────────────────────────────────────────────────────────────────

type fakeRules struct{ rules []storage.AlertRuleRecord }

func (f *fakeRules) ListEnabled(context.Context) ([]storage.AlertRuleRecord, error) {
	return f.rules, nil
}

type fakeAgents struct{ agents []storage.AgentRecord }

func (f *fakeAgents) GetByID(_ context.Context, id, userID string) (*storage.AgentRecord, error) {
	for i := range f.agents {
		if f.agents[i].ID == id && f.agents[i].UserID == userID {
			return &f.agents[i], nil
		}
	}
	return nil, storage.ErrAgentNotFound
}

func (f *fakeAgents) ListByUser(_ context.Context, userID string, _ storage.AgentListOptions) ([]storage.AgentRecord, int64, error) {
	var out []storage.AgentRecord
	for _, a := range f.agents {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, int64(len(out)), nil
}

type fakeRecords struct {
	byAgent map[string][]storage.AgentMetricRecord
}

func (f *fakeRecords) ListRecent(_ context.Context, agentID string, since time.Time, _ int) ([]storage.AgentMetricRecord, error) {
	var out []storage.AgentMetricRecord
	for _, r := range f.byAgent[agentID] {
		if r.RecordedAt >= since.UnixMilli() {
			out = append(out, r)
		}
	}
	return out, nil
}

type fakeSettings struct{ kv map[string]string }

func (f *fakeSettings) Get(_ context.Context, k string) (string, error) { return f.kv[k], nil }
func (f *fakeSettings) Set(_ context.Context, k, v string) error        { f.kv[k] = v; return nil }

type fakeNotify struct{ events []notify.Event }

func (f *fakeNotify) Emit(_ context.Context, e notify.Event) (int, error) {
	f.events = append(f.events, e)
	return 1, nil
}

// newEngine builds an engine with a fixed clock and the given fakes.
func newEngine(t *testing.T, now time.Time, r *fakeRules, a *fakeAgents, rec *fakeRecords, s *fakeSettings, n *fakeNotify) *Engine {
	t.Helper()
	e, err := NewEngine(Config{
		Rules: r, Agents: a, Records: rec, Settings: s, Notify: n,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return e
}

func sample(at time.Time, cpu float64) storage.AgentMetricRecord {
	return storage.AgentMetricRecord{RecordedAt: at.UnixMilli(), CPUPercent: cpu}
}

// ── tests ───────────────────────────────────────────────────────────────────

func TestInstantCPUBreachFires(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	r := &fakeRules{rules: []storage.AlertRuleRecord{{
		ID: "rule1", UserID: "u1", Name: "cpu", Enabled: true, AgentID: "a1",
		Metric: "cpu", Threshold: 80, DurationSec: 0, CooldownSec: 3600,
	}}}
	a := &fakeAgents{agents: []storage.AgentRecord{{ID: "a1", UserID: "u1", Name: "node1", Status: "online"}}}
	rec := &fakeRecords{byAgent: map[string][]storage.AgentMetricRecord{
		"a1": {sample(now.Add(-30*time.Second), 92)},
	}}
	s := &fakeSettings{kv: map[string]string{}}
	n := &fakeNotify{}
	newEngine(t, now, r, a, rec, s, n).EvaluateOnce(context.Background())

	if len(n.events) != 1 {
		t.Fatalf("want 1 alert, got %d", len(n.events))
	}
	if n.events[0].Type != notify.EventProbeAlert || n.events[0].UserID != "u1" {
		t.Errorf("unexpected event: %+v", n.events[0])
	}
}

func TestInstantBelowThresholdNoFire(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	r := &fakeRules{rules: []storage.AlertRuleRecord{{
		ID: "r", UserID: "u1", Enabled: true, AgentID: "a1", Metric: "cpu", Threshold: 80, CooldownSec: 3600,
	}}}
	a := &fakeAgents{agents: []storage.AgentRecord{{ID: "a1", UserID: "u1", Status: "online"}}}
	rec := &fakeRecords{byAgent: map[string][]storage.AgentMetricRecord{"a1": {sample(now, 50)}}}
	n := &fakeNotify{}
	newEngine(t, now, r, a, rec, &fakeSettings{kv: map[string]string{}}, n).EvaluateOnce(context.Background())
	if len(n.events) != 0 {
		t.Fatalf("below threshold must not fire, got %d", len(n.events))
	}
}

func TestSustainedRequiresAllSamplesOverThreshold(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	mk := func(samples []storage.AgentMetricRecord) (*fakeNotify, *fakeSettings) {
		r := &fakeRules{rules: []storage.AlertRuleRecord{{
			ID: "r", UserID: "u1", Enabled: true, AgentID: "a1",
			Metric: "cpu", Threshold: 80, DurationSec: 300, CooldownSec: 3600,
		}}}
		a := &fakeAgents{agents: []storage.AgentRecord{{ID: "a1", UserID: "u1", Status: "online"}}}
		rec := &fakeRecords{byAgent: map[string][]storage.AgentMetricRecord{"a1": samples}}
		n := &fakeNotify{}
		s := &fakeSettings{kv: map[string]string{}}
		newEngine(t, now, r, a, rec, s, n).EvaluateOnce(context.Background())
		return n, s
	}
	// One dip below 80 in the window → no fire.
	dip, _ := mk([]storage.AgentMetricRecord{
		sample(now.Add(-4*time.Minute), 95), sample(now.Add(-2*time.Minute), 70), sample(now, 90),
	})
	if len(dip.events) != 0 {
		t.Fatalf("a dip below threshold must prevent sustained fire, got %d", len(dip.events))
	}
	// All samples over 80 → fire.
	all, _ := mk([]storage.AgentMetricRecord{
		sample(now.Add(-4*time.Minute), 85), sample(now.Add(-2*time.Minute), 88), sample(now, 91),
	})
	if len(all.events) != 1 {
		t.Fatalf("all-over-threshold must fire, got %d", len(all.events))
	}
}

func TestOfflineRuleFiresAfterDuration(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	r := &fakeRules{rules: []storage.AlertRuleRecord{{
		ID: "r", UserID: "u1", Enabled: true, AgentID: "a1",
		Metric: "offline", DurationSec: 300, CooldownSec: 3600,
	}}}
	// offline for 10 min → past the 5-min duration.
	a := &fakeAgents{agents: []storage.AgentRecord{{
		ID: "a1", UserID: "u1", Name: "node1", Status: "offline",
		LastSeenAt: now.Add(-10 * time.Minute).UnixMilli(),
	}}}
	n := &fakeNotify{}
	newEngine(t, now, r, a, &fakeRecords{byAgent: map[string][]storage.AgentMetricRecord{}}, &fakeSettings{kv: map[string]string{}}, n).
		EvaluateOnce(context.Background())
	if len(n.events) != 1 {
		t.Fatalf("offline past duration must fire, got %d", len(n.events))
	}

	// Online agent → no fire.
	a.agents[0].Status = "online"
	n2 := &fakeNotify{}
	newEngine(t, now, r, a, &fakeRecords{byAgent: map[string][]storage.AgentMetricRecord{}}, &fakeSettings{kv: map[string]string{}}, n2).
		EvaluateOnce(context.Background())
	if len(n2.events) != 0 {
		t.Fatalf("online agent must not fire offline rule, got %d", len(n2.events))
	}
}

func TestCooldownSuppressesSecondFire(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	r := &fakeRules{rules: []storage.AlertRuleRecord{{
		ID: "r", UserID: "u1", Enabled: true, AgentID: "a1",
		Metric: "cpu", Threshold: 80, CooldownSec: 3600,
	}}}
	a := &fakeAgents{agents: []storage.AgentRecord{{ID: "a1", UserID: "u1", Status: "online"}}}
	rec := &fakeRecords{byAgent: map[string][]storage.AgentMetricRecord{"a1": {sample(now, 95)}}}
	s := &fakeSettings{kv: map[string]string{}}
	n := &fakeNotify{}

	// First pass fires + arms cooldown.
	newEngine(t, now, r, a, rec, s, n).EvaluateOnce(context.Background())
	// Second pass 1 min later: still in cooldown → no new fire.
	rec2 := &fakeRecords{byAgent: map[string][]storage.AgentMetricRecord{"a1": {sample(now.Add(time.Minute), 95)}}}
	newEngine(t, now.Add(time.Minute), r, a, rec2, s, n).EvaluateOnce(context.Background())
	if len(n.events) != 1 {
		t.Fatalf("cooldown must suppress the second fire, got %d events", len(n.events))
	}
	// After cooldown elapses → fires again.
	later := now.Add(2 * time.Hour)
	rec3 := &fakeRecords{byAgent: map[string][]storage.AgentMetricRecord{"a1": {sample(later, 95)}}}
	newEngine(t, later, r, a, rec3, s, n).EvaluateOnce(context.Background())
	if len(n.events) != 2 {
		t.Fatalf("after cooldown a new fire is expected, got %d events", len(n.events))
	}
}
