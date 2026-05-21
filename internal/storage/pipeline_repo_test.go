package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

// seedUserAndSub inserts a users row + subscriptions row so foreign keys hold
// for pipeline + binding inserts. Returns (userID, subscriptionID).
func seedUserAndSub(t *testing.T, db *storage.DB) (string, string) {
	t.Helper()
	uid := seedUser(t, db, "u-pipe")
	now := time.Now().UnixMilli()
	if _, err := db.Write.ExecContext(context.Background(), `
		INSERT INTO subscriptions(id, user_id, name, type, sync_interval, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?)`,
		"s-pipe", uid, "sub", "manual", 21600, now, now); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return uid, "s-pipe"
}

func newPipelineRec(id, userID string) storage.PipelineRecord {
	return storage.PipelineRecord{
		ID: id, UserID: userID, Name: "p-" + id,
		YAMLContent: "apiVersion: shiguang/v1\noperators: []\n",
		ASTJSON:     `{"api_version":"shiguang/v1","operators":[]}`,
	}
}

func TestPipelineRepo_CRUD(t *testing.T) {
	db := newTestDB(t)
	uid, _ := seedUserAndSub(t, db)
	repo := storage.NewPipelineRepo(db, time.Now)

	created, err := repo.Create(context.Background(), newPipelineRec("p1", uid))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Version != 1 {
		t.Fatalf("want version=1, got %d", created.Version)
	}
	if created.SchemaVersion != "shiguang/v1" {
		t.Fatalf("schema_version default: %q", created.SchemaVersion)
	}

	got, err := repo.GetByID(context.Background(), "p1", uid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "p-p1" {
		t.Fatalf("name: %q", got.Name)
	}

	// Update bumps version.
	got.Name = "renamed"
	got.YAMLContent = "apiVersion: shiguang/v1\noperators:\n  - kind: output\n    args:\n      format: clash\n"
	got.ASTJSON = `{"api_version":"shiguang/v1","operators":[{"kind":"output","args":{"format":"clash"},"enabled":true}]}`
	if err := repo.Update(context.Background(), *got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := repo.GetByID(context.Background(), "p1", uid)
	if got2.Name != "renamed" {
		t.Fatalf("rename failed: %q", got2.Name)
	}
	if got2.Version != 2 {
		t.Fatalf("version not bumped: %d", got2.Version)
	}

	if err := repo.Delete(context.Background(), "p1", uid); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(context.Background(), "p1", uid); !errors.Is(err, storage.ErrPipelineNotFound) {
		t.Fatalf("want ErrPipelineNotFound after delete, got %v", err)
	}
}

func TestPipelineRepo_UpdateOptimisticLock(t *testing.T) {
	db := newTestDB(t)
	uid, _ := seedUserAndSub(t, db)
	repo := storage.NewPipelineRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), newPipelineRec("p2", uid))

	// Stale version → conflict.
	stale := newPipelineRec("p2", uid)
	stale.Version = 99
	err := repo.Update(context.Background(), stale)
	if !errors.Is(err, storage.ErrPipelineVersionConflict) {
		t.Fatalf("want ErrPipelineVersionConflict, got %v", err)
	}

	// Missing row → not found.
	missing := newPipelineRec("nope", uid)
	missing.Version = 1
	err = repo.Update(context.Background(), missing)
	if !errors.Is(err, storage.ErrPipelineNotFound) {
		t.Fatalf("want ErrPipelineNotFound, got %v", err)
	}
}

func TestPipelineRepo_CrossUserIsolation(t *testing.T) {
	db := newTestDB(t)
	uid, _ := seedUserAndSub(t, db)
	repo := storage.NewPipelineRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), newPipelineRec("p3", uid))

	if _, err := repo.GetByID(context.Background(), "p3", "other-user"); !errors.Is(err, storage.ErrPipelineNotFound) {
		t.Fatalf("cross-user read should 404, got %v", err)
	}
	if err := repo.Delete(context.Background(), "p3", "other-user"); !errors.Is(err, storage.ErrPipelineNotFound) {
		t.Fatalf("cross-user delete should 404, got %v", err)
	}
}

func TestPipelineRepo_ListPaginates(t *testing.T) {
	db := newTestDB(t)
	uid, _ := seedUserAndSub(t, db)
	repo := storage.NewPipelineRepo(db, time.Now)
	for _, id := range []string{"p-a", "p-b", "p-c"} {
		_, _ = repo.Create(context.Background(), newPipelineRec(id, uid))
	}
	items, total, err := repo.List(context.Background(), uid, storage.PipelineListOptions{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 || len(items) != 2 {
		t.Fatalf("want total=3 size=2, got total=%d items=%d", total, len(items))
	}
}

func TestPipelineRepo_BindUnbindList(t *testing.T) {
	db := newTestDB(t)
	uid, sid := seedUserAndSub(t, db)
	repo := storage.NewPipelineRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), newPipelineRec("p-1", uid))
	_, _ = repo.Create(context.Background(), newPipelineRec("p-2", uid))

	if err := repo.Bind(context.Background(), sid, "p-1", 0, true); err != nil {
		t.Fatalf("bind 1: %v", err)
	}
	if err := repo.Bind(context.Background(), sid, "p-2", 1, true); err != nil {
		t.Fatalf("bind 2: %v", err)
	}
	bindings, err := repo.ListBindings(context.Background(), sid)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != 2 || bindings[0].PipelineID != "p-1" || bindings[1].PipelineID != "p-2" {
		t.Fatalf("bindings wrong: %+v", bindings)
	}

	// Re-Bind with new position is idempotent.
	if err := repo.Bind(context.Background(), sid, "p-1", 5, false); err != nil {
		t.Fatalf("rebind: %v", err)
	}
	bindings, _ = repo.ListBindings(context.Background(), sid)
	for _, b := range bindings {
		if b.PipelineID == "p-1" && (b.Position != 5 || b.Enabled) {
			t.Fatalf("rebind state wrong: %+v", b)
		}
	}

	// Unbind missing pipeline → sentinel.
	if err := repo.Unbind(context.Background(), sid, "p-missing"); !errors.Is(err, storage.ErrPipelineBindingNotFound) {
		t.Fatalf("want ErrPipelineBindingNotFound, got %v", err)
	}

	if err := repo.Unbind(context.Background(), sid, "p-1"); err != nil {
		t.Fatalf("unbind: %v", err)
	}
	bindings, _ = repo.ListBindings(context.Background(), sid)
	if len(bindings) != 1 {
		t.Fatalf("want 1 remaining, got %d", len(bindings))
	}
}

func TestPipelineRepo_ReplaceBindingsAtomic(t *testing.T) {
	db := newTestDB(t)
	uid, sid := seedUserAndSub(t, db)
	repo := storage.NewPipelineRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), newPipelineRec("p-x", uid))
	_, _ = repo.Create(context.Background(), newPipelineRec("p-y", uid))
	_ = repo.Bind(context.Background(), sid, "p-x", 0, true)
	_ = repo.Bind(context.Background(), sid, "p-y", 1, true)

	// Replace with a single binding.
	err := repo.ReplaceBindings(context.Background(), sid, []storage.PipelineBindingRecord{
		{SubscriptionID: sid, PipelineID: "p-y", Position: 0, Enabled: true},
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	bindings, _ := repo.ListBindings(context.Background(), sid)
	if len(bindings) != 1 || bindings[0].PipelineID != "p-y" {
		t.Fatalf("replace failed: %+v", bindings)
	}

	// Replacement carrying empty pipeline_id is rejected; pre-existing bindings remain
	// untouched (transaction rolled back).
	err = repo.ReplaceBindings(context.Background(), sid, []storage.PipelineBindingRecord{
		{SubscriptionID: sid, PipelineID: "", Position: 0, Enabled: true},
	})
	if err == nil {
		t.Fatalf("expected error on empty pipeline_id")
	}
	bindings, _ = repo.ListBindings(context.Background(), sid)
	if len(bindings) != 1 || bindings[0].PipelineID != "p-y" {
		t.Fatalf("rollback failed: %+v", bindings)
	}
}
