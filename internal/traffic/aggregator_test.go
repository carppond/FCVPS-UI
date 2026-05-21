package traffic_test

import (
	"context"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/traffic"
	"shiguang-vps/internal/util"
)

// newTestDB spins up a tmpdir SQLite DB with migrations applied. Mirrors the
// helper in internal/storage/*_test.go so this package can stay self-contained.
func newTestDB(t *testing.T) *storage.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(config.DatabaseConfig{
		DataDir: dir, Filename: "test.db", BusyTimeoutMs: 5000,
		MaxOpenWrite: 1, MaxOpenRead: 2,
	})
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return db
}

// seedUser + seedAgent are local helpers; keeping them next to the aggregator
// tests avoids a circular dep with internal/storage tests.
func seedUser(t *testing.T, db *storage.DB, id string) {
	t.Helper()
	users := storage.NewUserRepo(db, time.Now)
	if _, err := users.Create(context.Background(), storage.UserRecord{
		ID: id, Username: id, PasswordHash: "h", Role: "user", IsActive: true,
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func seedAgent(t *testing.T, db *storage.DB, userID, agentID string) {
	t.Helper()
	repo := storage.NewAgentRepo(db, time.Now)
	if _, err := repo.Create(context.Background(), storage.AgentRecord{
		ID: agentID, UserID: userID, Name: agentID, TokenHash: "h-" + agentID, Kind: "native",
	}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
}

func TestAggregatorRunForDateWritesPerAgentRows(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedAgent(t, db, "u1", "a1")
	seedAgent(t, db, "u1", "a2")
	recRepo := storage.NewAgentRecordRepo(db)
	tRepo := storage.NewTrafficRepo(db, time.Now)
	agRepo := storage.NewAgentRepo(db, time.Now)

	day, _ := time.Parse("2006-01-02", "2026-05-15")
	dayStartMs := day.UTC().UnixMilli()

	// Two samples per agent inside the day → deltas should match.
	recs := []storage.AgentMetricRecord{
		{AgentID: "a1", RecordedAt: dayStartMs + 1000, NetIn: 1000, NetOut: 500},
		{AgentID: "a1", RecordedAt: dayStartMs + 7200000, NetIn: 6000, NetOut: 3500},
		{AgentID: "a2", RecordedAt: dayStartMs + 1000, NetIn: 0, NetOut: 0},
		{AgentID: "a2", RecordedAt: dayStartMs + 3600000, NetIn: 200, NetOut: 100},
	}
	if err := recRepo.InsertBatch(context.Background(), recs); err != nil {
		t.Fatalf("insert: %v", err)
	}

	clock := util.NewFixedClock(day.Add(24 * time.Hour).Add(30 * time.Minute))
	agg, err := traffic.NewAggregator(traffic.AggregatorConfig{
		AgentRepo:       agRepo,
		AgentRecordRepo: recRepo,
		TrafficRepo:     tRepo,
		Clock:           clock,
	})
	if err != nil {
		t.Fatalf("NewAggregator: %v", err)
	}
	if err := agg.RunForDate(context.Background(), day); err != nil {
		t.Fatalf("RunForDate: %v", err)
	}

	from, _ := time.Parse("2006-01-02", "2026-05-15")
	to, _ := time.Parse("2006-01-02", "2026-05-15")
	rows, err := tRepo.ListByUser(context.Background(), "u1", from, to)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d (%+v)", len(rows), rows)
	}
	var a1, a2 *storage.TrafficRecord
	for i := range rows {
		switch rows[i].AgentID {
		case "a1":
			a1 = &rows[i]
		case "a2":
			a2 = &rows[i]
		}
	}
	if a1 == nil || a1.TotalIn != 5000 || a1.TotalOut != 3000 || a1.TotalUsed != 8000 {
		t.Fatalf("a1 deltas wrong: %+v", a1)
	}
	if a2 == nil || a2.TotalUsed != 300 {
		t.Fatalf("a2 deltas wrong: %+v", a2)
	}
}

func TestAggregatorRunDailyRollsYesterday(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedAgent(t, db, "u1", "a1")
	recRepo := storage.NewAgentRecordRepo(db)
	tRepo := storage.NewTrafficRepo(db, time.Now)
	agRepo := storage.NewAgentRepo(db, time.Now)

	// Anchor "now" at 2026-05-20 00:30 UTC. RunDaily must roll 2026-05-19.
	now := time.Date(2026, 5, 20, 0, 30, 0, 0, time.UTC)
	yesterday := now.AddDate(0, 0, -1)
	yStartMs := yesterday.UnixMilli()

	if err := recRepo.InsertBatch(context.Background(), []storage.AgentMetricRecord{
		{AgentID: "a1", RecordedAt: yStartMs + 1000, NetIn: 100, NetOut: 100},
		{AgentID: "a1", RecordedAt: yStartMs + 60000, NetIn: 500, NetOut: 700},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	clock := util.NewFixedClock(now)
	agg, _ := traffic.NewAggregator(traffic.AggregatorConfig{
		AgentRepo:       agRepo,
		AgentRecordRepo: recRepo,
		TrafficRepo:     tRepo,
		Clock:           clock,
	})
	if err := agg.RunDaily(context.Background()); err != nil {
		t.Fatalf("RunDaily: %v", err)
	}

	from, _ := time.Parse("2006-01-02", "2026-05-19")
	to, _ := time.Parse("2006-01-02", "2026-05-19")
	rows, _ := tRepo.ListByUser(context.Background(), "u1", from, to)
	if len(rows) != 1 || rows[0].TotalUsed != 1000 {
		t.Fatalf("expected one row for yesterday with used=1000, got %+v", rows)
	}
}

func TestAggregatorRunForDateIdempotent(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedAgent(t, db, "u1", "a1")
	recRepo := storage.NewAgentRecordRepo(db)
	tRepo := storage.NewTrafficRepo(db, time.Now)
	agRepo := storage.NewAgentRepo(db, time.Now)

	day, _ := time.Parse("2006-01-02", "2026-05-15")
	dayStartMs := day.UTC().UnixMilli()
	_ = recRepo.InsertBatch(context.Background(), []storage.AgentMetricRecord{
		{AgentID: "a1", RecordedAt: dayStartMs + 1000, NetIn: 0, NetOut: 0},
		{AgentID: "a1", RecordedAt: dayStartMs + 60000, NetIn: 100, NetOut: 100},
	})

	agg, _ := traffic.NewAggregator(traffic.AggregatorConfig{
		AgentRepo:       agRepo,
		AgentRecordRepo: recRepo,
		TrafficRepo:     tRepo,
		Clock:           util.NewFixedClock(day.Add(24 * time.Hour)),
	})
	if err := agg.RunForDate(context.Background(), day); err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if err := agg.RunForDate(context.Background(), day); err != nil {
		t.Fatalf("run 2: %v", err)
	}
	from, _ := time.Parse("2006-01-02", "2026-05-15")
	to, _ := time.Parse("2006-01-02", "2026-05-15")
	rows, _ := tRepo.ListByUser(context.Background(), "u1", from, to)
	if len(rows) != 1 {
		t.Fatalf("expected single row after two runs (upsert), got %d", len(rows))
	}
	if rows[0].TotalUsed != 200 {
		t.Fatalf("expected total_used=200, got %d", rows[0].TotalUsed)
	}
}

func TestAggregatorSkipsAgentsWithSingleSample(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedAgent(t, db, "u1", "a1")
	recRepo := storage.NewAgentRecordRepo(db)
	tRepo := storage.NewTrafficRepo(db, time.Now)
	agRepo := storage.NewAgentRepo(db, time.Now)

	day, _ := time.Parse("2006-01-02", "2026-05-15")
	dayStartMs := day.UTC().UnixMilli()
	// Single sample → can't compute a delta.
	_ = recRepo.InsertBatch(context.Background(), []storage.AgentMetricRecord{
		{AgentID: "a1", RecordedAt: dayStartMs + 1000, NetIn: 100, NetOut: 100},
	})

	agg, _ := traffic.NewAggregator(traffic.AggregatorConfig{
		AgentRepo: agRepo, AgentRecordRepo: recRepo, TrafficRepo: tRepo,
		Clock: util.NewFixedClock(day.Add(24 * time.Hour)),
	})
	if err := agg.RunForDate(context.Background(), day); err != nil {
		t.Fatalf("run: %v", err)
	}
	from, _ := time.Parse("2006-01-02", "2026-05-15")
	to, _ := time.Parse("2006-01-02", "2026-05-15")
	rows, _ := tRepo.ListByUser(context.Background(), "u1", from, to)
	if len(rows) != 0 {
		t.Fatalf("expected zero rows for single-sample agent, got %d", len(rows))
	}
}

func TestAggregatorHandlesCounterResetGracefully(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedAgent(t, db, "u1", "a1")
	recRepo := storage.NewAgentRecordRepo(db)
	tRepo := storage.NewTrafficRepo(db, time.Now)
	agRepo := storage.NewAgentRepo(db, time.Now)

	day, _ := time.Parse("2006-01-02", "2026-05-15")
	dayStartMs := day.UTC().UnixMilli()
	// Counter grows, then resets, then grows again.
	_ = recRepo.InsertBatch(context.Background(), []storage.AgentMetricRecord{
		{AgentID: "a1", RecordedAt: dayStartMs + 1000, NetIn: 5000, NetOut: 5000},
		{AgentID: "a1", RecordedAt: dayStartMs + 60000, NetIn: 100, NetOut: 100}, // reboot
		{AgentID: "a1", RecordedAt: dayStartMs + 120000, NetIn: 800, NetOut: 800},
	})

	agg, _ := traffic.NewAggregator(traffic.AggregatorConfig{
		AgentRepo: agRepo, AgentRecordRepo: recRepo, TrafficRepo: tRepo,
		Clock: util.NewFixedClock(day.Add(24 * time.Hour)),
	})
	if err := agg.RunForDate(context.Background(), day); err != nil {
		t.Fatalf("run: %v", err)
	}
	from, _ := time.Parse("2006-01-02", "2026-05-15")
	to, _ := time.Parse("2006-01-02", "2026-05-15")
	rows, _ := tRepo.ListByUser(context.Background(), "u1", from, to)
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	// max(in+out)=10000, min=200 → used=9800; positive number, never negative.
	if rows[0].TotalUsed < 0 {
		t.Fatalf("expected non-negative total_used, got %d", rows[0].TotalUsed)
	}
}
