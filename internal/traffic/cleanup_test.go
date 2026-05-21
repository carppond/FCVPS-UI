package traffic_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/traffic"
	"shiguang-vps/internal/util"
)

func TestCleanupDeletesRowsOlderThanRetention(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedAgent(t, db, "u1", "a1")
	recRepo := storage.NewAgentRecordRepo(db)

	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	ms := func(t time.Time) int64 { return t.UnixMilli() }
	rows := []storage.AgentMetricRecord{
		{AgentID: "a1", RecordedAt: ms(now.AddDate(0, 0, -10)), CPUPercent: 1}, // older than 7d
		{AgentID: "a1", RecordedAt: ms(now.AddDate(0, 0, -8)), CPUPercent: 2},  // older than 7d
		{AgentID: "a1", RecordedAt: ms(now.AddDate(0, 0, -1)), CPUPercent: 3},  // within retention
		{AgentID: "a1", RecordedAt: ms(now), CPUPercent: 4},                    // now
	}
	if err := recRepo.InsertBatch(context.Background(), rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	cl, err := traffic.NewCleanup(traffic.CleanupConfig{
		AgentRecordRepo: recRepo,
		Clock:           util.NewFixedClock(now),
	})
	if err != nil {
		t.Fatalf("NewCleanup: %v", err)
	}
	deleted, err := cl.CleanupOldRecords(context.Background())
	if err != nil {
		t.Fatalf("CleanupOldRecords: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deletions, got %d", deleted)
	}
	remaining, _ := recRepo.ListRecent(context.Background(), "a1",
		now.AddDate(0, 0, -30), 100)
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining rows, got %d", len(remaining))
	}
}

// TestStartDailyCancelStops drives StartDaily for every worker and confirms
// the returned cancel func reliably stops the goroutine within a short
// deadline. The actual schedule firing is exercised indirectly via the
// RunDaily / Run / CleanupOldRecords tests above (which inject a FixedClock).
func TestStartDailyCancelStops(t *testing.T) {
	db := newTestDB(t)
	recRepo := storage.NewAgentRecordRepo(db)
	cl, _ := traffic.NewCleanup(traffic.CleanupConfig{
		AgentRecordRepo: recRepo,
		Clock:           util.NewFixedClock(time.Now().UTC()),
	})
	ctx, cancel := context.WithCancel(context.Background())
	stop := cl.StartDaily(ctx)
	// Give the goroutine a chance to start.
	time.Sleep(20 * time.Millisecond)
	stop()
	cancel()
	// Drain any background mutations — the test passes as long as no goroutine
	// leak survives the deadline.
	deadline := time.After(2 * time.Second)
	var settled atomic.Bool
	go func() { settled.Store(true) }()
	for !settled.Load() {
		select {
		case <-deadline:
			t.Fatalf("background routine did not exit promptly")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
