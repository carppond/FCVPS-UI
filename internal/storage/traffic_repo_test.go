package storage_test

import (
	"context"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

// seedTrafficAgents writes two agents under user "u1" so traffic_records rows
// with agent_id satisfy the FK.
func seedTrafficAgents(t *testing.T, db *storage.DB, userID string, agentIDs ...string) {
	t.Helper()
	newTestUser(t, db, userID)
	repo := storage.NewAgentRepo(db, time.Now)
	for _, id := range agentIDs {
		if _, err := repo.Create(context.Background(), storage.AgentRecord{
			ID: id, UserID: userID, Name: id, TokenHash: "h-" + id, Kind: "native",
		}); err != nil {
			t.Fatalf("seed agent: %v", err)
		}
	}
}

func TestTrafficRepoUpsertDailyAndListByUser(t *testing.T) {
	db := newTestDB(t)
	seedTrafficAgents(t, db, "u1", "a1", "a2")
	repo := storage.NewTrafficRepo(db, time.Now)
	ctx := context.Background()

	if err := repo.UpsertDaily(ctx, storage.TrafficRecord{
		Date: "2026-05-01", UserID: "u1", AgentID: "a1",
		TotalUsed: 100, TotalIn: 40, TotalOut: 60,
	}); err != nil {
		t.Fatalf("upsert a1: %v", err)
	}
	if err := repo.UpsertDaily(ctx, storage.TrafficRecord{
		Date: "2026-05-01", UserID: "u1", AgentID: "a2",
		TotalUsed: 200, TotalIn: 100, TotalOut: 100,
	}); err != nil {
		t.Fatalf("upsert a2: %v", err)
	}
	// Overwrite a1: total_used grows.
	if err := repo.UpsertDaily(ctx, storage.TrafficRecord{
		Date: "2026-05-01", UserID: "u1", AgentID: "a1",
		TotalUsed: 150, TotalIn: 60, TotalOut: 90,
	}); err != nil {
		t.Fatalf("upsert a1 again: %v", err)
	}

	from, _ := time.Parse("2006-01-02", "2026-05-01")
	to, _ := time.Parse("2006-01-02", "2026-05-31")
	rows, err := repo.ListByUser(ctx, "u1", from, to)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows after upsert, got %d", len(rows))
	}
	// a1 row must reflect the second upsert.
	var a1 *storage.TrafficRecord
	for i := range rows {
		if rows[i].AgentID == "a1" {
			a1 = &rows[i]
		}
	}
	if a1 == nil || a1.TotalUsed != 150 || a1.TotalOut != 90 {
		t.Fatalf("a1 row not overwritten: %+v", a1)
	}
}

func TestTrafficRepoGetMonthSummary(t *testing.T) {
	db := newTestDB(t)
	seedTrafficAgents(t, db, "u1", "a1", "a2")
	repo := storage.NewTrafficRepo(db, time.Now)
	ctx := context.Background()

	// Two days, two agents.
	for _, rec := range []storage.TrafficRecord{
		{Date: "2026-05-01", UserID: "u1", AgentID: "a1", TotalUsed: 100, TotalIn: 50, TotalOut: 50},
		{Date: "2026-05-01", UserID: "u1", AgentID: "a2", TotalUsed: 200, TotalIn: 100, TotalOut: 100},
		{Date: "2026-05-02", UserID: "u1", AgentID: "a1", TotalUsed: 50, TotalIn: 25, TotalOut: 25},
		// Different month — must be excluded.
		{Date: "2026-06-01", UserID: "u1", AgentID: "a1", TotalUsed: 999, TotalIn: 500, TotalOut: 499},
	} {
		if err := repo.UpsertDaily(ctx, rec); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}

	summary, err := repo.GetMonthSummary(ctx, "u1", 2026, time.May, 1_000_000)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.PeriodStart != "2026-05-01" || summary.PeriodEnd != "2026-05-31" {
		t.Fatalf("unexpected period: %s..%s", summary.PeriodStart, summary.PeriodEnd)
	}
	if summary.TotalUsed != 350 || summary.TotalIn != 175 || summary.TotalOut != 175 {
		t.Fatalf("unexpected totals: %+v", summary)
	}
	if summary.TotalLimit != 1_000_000 {
		t.Fatalf("unexpected limit: %d", summary.TotalLimit)
	}
	if len(summary.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(summary.Agents))
	}
}

func TestTrafficRepoListDailyAndMonthlyTotals(t *testing.T) {
	db := newTestDB(t)
	seedTrafficAgents(t, db, "u1", "a1", "a2")
	repo := storage.NewTrafficRepo(db, time.Now)
	ctx := context.Background()

	// Three days, two months.
	rows := []storage.TrafficRecord{
		{Date: "2026-04-30", UserID: "u1", AgentID: "a1", TotalUsed: 10, TotalIn: 5, TotalOut: 5},
		{Date: "2026-05-01", UserID: "u1", AgentID: "a1", TotalUsed: 20, TotalIn: 10, TotalOut: 10},
		{Date: "2026-05-01", UserID: "u1", AgentID: "a2", TotalUsed: 30, TotalIn: 15, TotalOut: 15},
		{Date: "2026-05-02", UserID: "u1", AgentID: "a1", TotalUsed: 40, TotalIn: 20, TotalOut: 20},
	}
	for _, r := range rows {
		if err := repo.UpsertDaily(ctx, r); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}

	from, _ := time.Parse("2006-01-02", "2026-04-01")
	to, _ := time.Parse("2006-01-02", "2026-05-31")

	daily, err := repo.ListDailyTotals(ctx, "u1", from, to)
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	if len(daily) != 3 {
		t.Fatalf("expected 3 daily totals, got %d", len(daily))
	}
	if daily[0].Date != "2026-04-30" || daily[0].TotalUsed != 10 {
		t.Fatalf("first day wrong: %+v", daily[0])
	}
	if daily[1].Date != "2026-05-01" || daily[1].TotalUsed != 50 {
		t.Fatalf("second day must sum agents: %+v", daily[1])
	}

	monthly, err := repo.ListMonthlyTotals(ctx, "u1", from, to)
	if err != nil {
		t.Fatalf("monthly: %v", err)
	}
	if len(monthly) != 2 {
		t.Fatalf("expected 2 months, got %d", len(monthly))
	}
	if monthly[1].Date != "2026-05-01" || monthly[1].TotalUsed != 90 {
		t.Fatalf("may total wrong: %+v", monthly[1])
	}
}

func TestTrafficRepoSumWindow(t *testing.T) {
	db := newTestDB(t)
	seedTrafficAgents(t, db, "u1", "a1")
	repo := storage.NewTrafficRepo(db, time.Now)
	ctx := context.Background()

	if err := repo.UpsertDaily(ctx, storage.TrafficRecord{
		Date: "2026-05-15", UserID: "u1", AgentID: "a1",
		TotalUsed: 700, TotalIn: 350, TotalOut: 350,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	from, _ := time.Parse("2006-01-02", "2026-05-01")
	to, _ := time.Parse("2006-01-02", "2026-05-31")
	used, in, out, err := repo.SumWindow(ctx, "u1", from, to)
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if used != 700 || in != 350 || out != 350 {
		t.Fatalf("unexpected sums: used=%d in=%d out=%d", used, in, out)
	}

	// Out-of-window query returns zeros (no error).
	earlier, _ := time.Parse("2006-01-02", "2026-04-01")
	earlierEnd, _ := time.Parse("2006-01-02", "2026-04-30")
	used2, _, _, _ := repo.SumWindow(ctx, "u1", earlier, earlierEnd)
	if used2 != 0 {
		t.Fatalf("expected 0 outside window, got %d", used2)
	}
}
