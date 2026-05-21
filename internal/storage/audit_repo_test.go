package storage_test

import (
	"context"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

func TestAuditRepoInsertAndList(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	repo := storage.NewAuditRepo(db, time.Now)
	for i, action := range []string{"login", "create_subscription", "delete_pipeline"} {
		_, err := repo.Insert(context.Background(), storage.AuditLogRecord{
			Action: action, UserID: "u1", Success: i%2 == 0,
		})
		if err != nil {
			t.Fatalf("Insert %s: %v", action, err)
		}
	}
	rows, total, err := repo.List(context.Background(), storage.AuditLogFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 || len(rows) != 3 {
		t.Fatalf("expected 3 rows, got total=%d len=%d", total, len(rows))
	}
}

func TestAuditRepoFilterByAction(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	repo := storage.NewAuditRepo(db, time.Now)
	for _, a := range []string{"login", "logout", "login"} {
		_, _ = repo.Insert(context.Background(), storage.AuditLogRecord{
			Action: a, UserID: "u1", Success: true,
		})
	}
	_, total, err := repo.List(context.Background(), storage.AuditLogFilter{
		Action: "login", Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 login rows, got %d", total)
	}
}

func TestAuditRepoDeleteOlderThan(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	now := time.Now()
	repo := storage.NewAuditRepo(db, func() time.Time { return now })
	old := now.Add(-48 * time.Hour).UnixMilli()
	fresh := now.UnixMilli()
	for _, ts := range []int64{old, old, fresh} {
		_, _ = repo.Insert(context.Background(), storage.AuditLogRecord{
			Action: "login", UserID: "u1", Success: true, CreatedAt: ts,
		})
	}
	cutoff := now.Add(-24 * time.Hour).UnixMilli()
	n, err := repo.DeleteOlderThan(context.Background(), cutoff)
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 rows pruned, got %d", n)
	}
}
