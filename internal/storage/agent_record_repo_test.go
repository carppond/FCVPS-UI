package storage_test

import (
	"context"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

// seedAgent inserts a user + agent so agent_records FK is satisfied.
func seedAgent(t *testing.T, db *storage.DB, agentID string) {
	t.Helper()
	newTestUser(t, db, "u1")
	repo := storage.NewAgentRepo(db, time.Now)
	if _, err := repo.Create(context.Background(), storage.AgentRecord{
		ID: agentID, UserID: "u1", Name: "n", TokenHash: "h", Kind: "native",
	}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
}

func TestAgentRecordRepoInsertBatchAndListRecent(t *testing.T) {
	db := newTestDB(t)
	seedAgent(t, db, "a1")
	repo := storage.NewAgentRecordRepo(db)
	now := time.Now()
	recs := []storage.AgentMetricRecord{
		{AgentID: "a1", RecordedAt: now.Add(-3 * time.Minute).UnixMilli(), CPUPercent: 10},
		{AgentID: "a1", RecordedAt: now.Add(-2 * time.Minute).UnixMilli(), CPUPercent: 20},
		{AgentID: "a1", RecordedAt: now.Add(-1 * time.Minute).UnixMilli(), CPUPercent: 30},
	}
	if err := repo.InsertBatch(context.Background(), recs); err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	got, err := repo.ListRecent(context.Background(), "a1", now.Add(-5*time.Minute), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 records, got %d", len(got))
	}
	// Newest first.
	if got[0].CPUPercent != 30 {
		t.Fatalf("expected newest first, got %+v", got[0])
	}
}

func TestAgentRecordRepoListRecentSinceFilters(t *testing.T) {
	db := newTestDB(t)
	seedAgent(t, db, "a1")
	repo := storage.NewAgentRecordRepo(db)
	now := time.Now()
	recs := []storage.AgentMetricRecord{
		{AgentID: "a1", RecordedAt: now.Add(-10 * time.Minute).UnixMilli(), CPUPercent: 5},
		{AgentID: "a1", RecordedAt: now.Add(-2 * time.Minute).UnixMilli(), CPUPercent: 25},
	}
	_ = repo.InsertBatch(context.Background(), recs)
	got, _ := repo.ListRecent(context.Background(), "a1", now.Add(-5*time.Minute), 10)
	if len(got) != 1 || got[0].CPUPercent != 25 {
		t.Fatalf("since filter wrong: %+v", got)
	}
}

func TestAgentRecordRepoDeleteOlderThan(t *testing.T) {
	db := newTestDB(t)
	seedAgent(t, db, "a1")
	repo := storage.NewAgentRecordRepo(db)
	now := time.Now()
	recs := []storage.AgentMetricRecord{
		{AgentID: "a1", RecordedAt: now.Add(-8 * 24 * time.Hour).UnixMilli(), CPUPercent: 1},
		{AgentID: "a1", RecordedAt: now.Add(-1 * 24 * time.Hour).UnixMilli(), CPUPercent: 2},
	}
	_ = repo.InsertBatch(context.Background(), recs)
	n, err := repo.DeleteOlderThan(context.Background(), now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deletion, got %d", n)
	}
	remaining, _ := repo.ListRecent(context.Background(), "a1", time.Time{}, 10)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining row, got %d", len(remaining))
	}
}

func TestAgentRecordRepoInsertEmpty(t *testing.T) {
	db := newTestDB(t)
	repo := storage.NewAgentRecordRepo(db)
	if err := repo.InsertBatch(context.Background(), nil); err != nil {
		t.Fatalf("empty batch: %v", err)
	}
}

func TestAgentRecordRepoCascadeOnAgentDelete(t *testing.T) {
	db := newTestDB(t)
	seedAgent(t, db, "a1")
	repo := storage.NewAgentRecordRepo(db)
	now := time.Now()
	_ = repo.InsertBatch(context.Background(), []storage.AgentMetricRecord{
		{AgentID: "a1", RecordedAt: now.UnixMilli(), CPUPercent: 1},
	})
	agentRepo := storage.NewAgentRepo(db, time.Now)
	if err := agentRepo.Delete(context.Background(), "a1", "u1"); err != nil {
		t.Fatalf("delete parent agent: %v", err)
	}
	got, _ := repo.ListRecent(context.Background(), "a1", time.Time{}, 10)
	if len(got) != 0 {
		t.Fatalf("expected cascade delete, got %d rows", len(got))
	}
}
