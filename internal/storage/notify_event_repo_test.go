package storage_test

import (
	"context"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

func TestNotificationEventRepo_InsertAndList(t *testing.T) {
	db := newTestDB(t)
	_ = seedUser(t, db, "u1")
	repo := storage.NewNotificationEventRepo(db, time.Now)
	for i := 0; i < 5; i++ {
		_, err := repo.Insert(context.Background(), storage.NotificationEventRecord{
			UserID:      "u1",
			EventType:   "node_offline",
			DedupeKey:   "k1",
			PayloadJSON: `{"foo":"bar"}`,
			Status:      "sent",
		})
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	recs, total, err := repo.ListByUser(context.Background(), "u1", storage.NotificationEventListOptions{
		Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if total != 5 || len(recs) != 5 {
		t.Fatalf("expected 5 events, got total=%d recs=%d", total, len(recs))
	}
}

func TestNotificationEventRepo_CountRecentByDedupeKey(t *testing.T) {
	db := newTestDB(t)
	_ = seedUser(t, db, "u1")
	repo := storage.NewNotificationEventRepo(db, time.Now)
	now := time.Now().UnixMilli()
	old := now - 10*60*1000 // 10 min ago

	if _, err := repo.Insert(context.Background(), storage.NotificationEventRecord{
		UserID: "u1", EventType: "t", DedupeKey: "k", PayloadJSON: "{}",
		Status: "sent", CreatedAt: old,
	}); err != nil {
		t.Fatalf("insert old: %v", err)
	}
	if _, err := repo.Insert(context.Background(), storage.NotificationEventRecord{
		UserID: "u1", EventType: "t", DedupeKey: "k", PayloadJSON: "{}",
		Status: "sent", CreatedAt: now,
	}); err != nil {
		t.Fatalf("insert now: %v", err)
	}
	since := now - 5*60*1000
	n, err := repo.CountRecentByDedupeKey(context.Background(), "k", since)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 recent (5m), got %d", n)
	}
}

func TestNotificationEventRepo_DeleteOlderThan(t *testing.T) {
	db := newTestDB(t)
	_ = seedUser(t, db, "u1")
	repo := storage.NewNotificationEventRepo(db, time.Now)
	now := time.Now().UnixMilli()
	if _, err := repo.Insert(context.Background(), storage.NotificationEventRecord{
		UserID: "u1", EventType: "t", PayloadJSON: "{}",
		Status: "sent", CreatedAt: now - 60*1000,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := repo.Insert(context.Background(), storage.NotificationEventRecord{
		UserID: "u1", EventType: "t", PayloadJSON: "{}",
		Status: "sent", CreatedAt: now,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	deleted, err := repo.DeleteOlderThan(context.Background(), now-30*1000)
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}
}
