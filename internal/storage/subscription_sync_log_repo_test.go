package storage_test

import (
	"context"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

// seedSyncLogFixture creates the FK prerequisites (foreign_keys is ON) and returns
// the subscription id + user id.
func seedSyncLogFixture(t *testing.T, db *storage.DB) (subID, userID string) {
	t.Helper()
	users := storage.NewUserRepo(db, time.Now)
	if _, err := users.Create(context.Background(), storage.UserRecord{
		ID: "u1", Username: "alice", PasswordHash: "h", Role: "user", IsActive: true,
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	subs := storage.NewSubscriptionRepo(db, time.Now)
	rec, err := subs.Create(context.Background(), storage.SubscriptionRecord{
		ID: "s1", UserID: "u1", Name: "sub", Type: "manual",
	})
	if err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return rec.ID, "u1"
}

func TestSyncLogRecordAndList(t *testing.T) {
	db := newTestDB(t)
	subID, userID := seedSyncLogFixture(t, db)
	repo := storage.NewSubscriptionSyncLogRepo(db, time.Now)
	ctx := context.Background()

	if err := repo.Record(ctx, storage.SubscriptionSyncLogRecord{
		SubscriptionID: subID, UserID: userID, Status: "ok", NodeCount: 12, CreatedAt: 1000,
	}); err != nil {
		t.Fatalf("Record ok: %v", err)
	}
	if err := repo.Record(ctx, storage.SubscriptionSyncLogRecord{
		SubscriptionID: subID, UserID: userID, Status: "error", Error: "boom", CreatedAt: 2000,
	}); err != nil {
		t.Fatalf("Record error: %v", err)
	}

	logs, err := repo.ListBySubscription(ctx, subID, userID, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("want 2 logs, got %d", len(logs))
	}
	// Newest first.
	if logs[0].Status != "error" || logs[0].Error != "boom" {
		t.Errorf("newest should be the error row: %+v", logs[0])
	}
	if logs[1].Status != "ok" || logs[1].NodeCount != 12 {
		t.Errorf("oldest should be the ok row: %+v", logs[1])
	}
}

func TestSyncLogPrunesBeyondCap(t *testing.T) {
	db := newTestDB(t)
	subID, userID := seedSyncLogFixture(t, db)
	repo := storage.NewSubscriptionSyncLogRepo(db, time.Now)
	ctx := context.Background()

	// Insert 60 rows; the repo keeps the newest 50.
	for i := 1; i <= 60; i++ {
		if err := repo.Record(ctx, storage.SubscriptionSyncLogRecord{
			SubscriptionID: subID, UserID: userID, Status: "ok", NodeCount: i, CreatedAt: int64(i * 1000),
		}); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}
	logs, err := repo.ListBySubscription(ctx, subID, userID, 1000)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(logs) != 50 {
		t.Fatalf("expected prune to 50, got %d", len(logs))
	}
	// The newest (createdAt=60000, NodeCount=60) survives; the oldest kept is #11.
	if logs[0].NodeCount != 60 {
		t.Errorf("newest kept should be #60, got %d", logs[0].NodeCount)
	}
	if logs[len(logs)-1].NodeCount != 11 {
		t.Errorf("oldest kept should be #11, got %d", logs[len(logs)-1].NodeCount)
	}
}

func TestSyncLogCrossUserIsolation(t *testing.T) {
	db := newTestDB(t)
	subID, userID := seedSyncLogFixture(t, db)
	repo := storage.NewSubscriptionSyncLogRepo(db, time.Now)
	ctx := context.Background()
	_ = repo.Record(ctx, storage.SubscriptionSyncLogRecord{
		SubscriptionID: subID, UserID: userID, Status: "ok", CreatedAt: 1000,
	})
	// A different user must not see another user's logs.
	logs, err := repo.ListBySubscription(ctx, subID, "other-user", 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("cross-user must see no logs, got %d", len(logs))
	}
}
