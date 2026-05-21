package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

func newScriptRec(id, userID, hook string) storage.ScriptRecord {
	return storage.ScriptRecord{
		ID:      id,
		UserID:  userID,
		Name:    "script-" + id,
		Hook:    hook,
		Code:    `__output = __input;`,
		Enabled: true,
	}
}

func TestScriptRepo_CRUD(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-script")
	repo := storage.NewScriptRepo(db, time.Now)

	created, err := repo.Create(context.Background(), newScriptRec("s1", uid, "pre_save_nodes"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.CreatedAt == 0 || created.UpdatedAt == 0 {
		t.Fatalf("timestamps not populated: %+v", created)
	}

	got, err := repo.GetByID(context.Background(), "s1", uid)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "script-s1" || got.Hook != "pre_save_nodes" || !got.Enabled {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	// Cross-user GET → NotFound
	if _, err := repo.GetByID(context.Background(), "s1", "other-user"); !errors.Is(err, storage.ErrScriptNotFound) {
		t.Fatalf("cross-user get: want ErrScriptNotFound, got %v", err)
	}

	// Patch: rename + disable
	newName := "renamed"
	disabled := false
	updated, err := repo.Update(context.Background(), "s1", uid, storage.ScriptUpdate{
		Name:    &newName,
		Enabled: &disabled,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "renamed" || updated.Enabled {
		t.Fatalf("update did not apply: %+v", updated)
	}

	// Empty code → rejected
	empty := ""
	if _, err := repo.Update(context.Background(), "s1", uid, storage.ScriptUpdate{Code: &empty}); err == nil {
		t.Fatalf("expected empty-code rejection")
	}

	if err := repo.Delete(context.Background(), "s1", uid); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(context.Background(), "s1", uid); !errors.Is(err, storage.ErrScriptNotFound) {
		t.Fatalf("after delete: want NotFound, got %v", err)
	}
}

func TestScriptRepo_List_FilterAndPagination(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-list")
	repo := storage.NewScriptRepo(db, time.Now)

	// 3 pre_save_nodes + 2 post_fetch
	for i := 0; i < 3; i++ {
		id := "pre-" + string(rune('a'+i))
		if _, err := repo.Create(context.Background(), newScriptRec(id, uid, "pre_save_nodes")); err != nil {
			t.Fatalf("seed pre %d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		id := "post-" + string(rune('a'+i))
		if _, err := repo.Create(context.Background(), newScriptRec(id, uid, "post_fetch")); err != nil {
			t.Fatalf("seed post %d: %v", i, err)
		}
	}

	all, total, err := repo.List(context.Background(), uid, storage.ScriptListOptions{})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if total != 5 || len(all) != 5 {
		t.Fatalf("total=%d len=%d, want 5", total, len(all))
	}

	preOnly, total, err := repo.List(context.Background(), uid, storage.ScriptListOptions{Hook: "pre_save_nodes"})
	if err != nil {
		t.Fatalf("List pre: %v", err)
	}
	if total != 3 || len(preOnly) != 3 {
		t.Fatalf("pre filter: total=%d len=%d", total, len(preOnly))
	}

	keyword, _, err := repo.List(context.Background(), uid, storage.ScriptListOptions{Keyword: "post-a"})
	if err != nil {
		t.Fatalf("List keyword: %v", err)
	}
	if len(keyword) != 1 || keyword[0].Name != "script-post-a" {
		t.Fatalf("keyword filter mismatch: %+v", keyword)
	}
}

func TestScriptRepo_ListEnabledByHook(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-hook")
	repo := storage.NewScriptRepo(db, time.Now)

	// Two enabled pre_save_nodes + one disabled
	for i := 0; i < 2; i++ {
		id := "on-" + string(rune('a'+i))
		if _, err := repo.Create(context.Background(), newScriptRec(id, uid, "pre_save_nodes")); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	rec := newScriptRec("off", uid, "pre_save_nodes")
	rec.Enabled = false
	if _, err := repo.Create(context.Background(), rec); err != nil {
		t.Fatalf("seed disabled: %v", err)
	}

	got, err := repo.ListEnabledByHook(context.Background(), uid, "pre_save_nodes")
	if err != nil {
		t.Fatalf("ListEnabledByHook: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 enabled scripts, got %d", len(got))
	}
	for _, s := range got {
		if !s.Enabled {
			t.Fatalf("unexpected disabled in result: %+v", s)
		}
	}

	if _, err := repo.ListEnabledByHook(context.Background(), uid, "bogus"); err == nil {
		t.Fatalf("expected error for invalid hook")
	}
}

func TestScriptRepo_RecordRun(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-run")
	repo := storage.NewScriptRepo(db, time.Now)

	if _, err := repo.Create(context.Background(), newScriptRec("s1", uid, "post_fetch")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Success path → clears any previous error
	if err := repo.RecordRun(context.Background(), "s1", time.Now().UnixMilli(), ""); err != nil {
		t.Fatalf("RecordRun success: %v", err)
	}
	got, _ := repo.GetByID(context.Background(), "s1", uid)
	if got.LastRunAt == 0 {
		t.Fatalf("last_run_at not stamped")
	}
	if got.LastError != "" {
		t.Fatalf("last_error should be empty, got %q", got.LastError)
	}

	// Failure path → error captured
	if err := repo.RecordRun(context.Background(), "s1", time.Now().UnixMilli(), "boom"); err != nil {
		t.Fatalf("RecordRun failure: %v", err)
	}
	got, _ = repo.GetByID(context.Background(), "s1", uid)
	if got.LastError != "boom" {
		t.Fatalf("last_error: %q", got.LastError)
	}

	// Unknown id → ErrScriptNotFound
	if err := repo.RecordRun(context.Background(), "missing", 0, ""); !errors.Is(err, storage.ErrScriptNotFound) {
		t.Fatalf("missing id: want NotFound, got %v", err)
	}
}

func TestScriptRepo_Create_Validation(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-val")
	repo := storage.NewScriptRepo(db, time.Now)

	cases := []struct {
		name string
		rec  storage.ScriptRecord
	}{
		{"missing id", storage.ScriptRecord{UserID: uid, Name: "x", Hook: "post_fetch", Code: "x"}},
		{"missing user", storage.ScriptRecord{ID: "x", Name: "x", Hook: "post_fetch", Code: "x"}},
		{"missing name", storage.ScriptRecord{ID: "x", UserID: uid, Hook: "post_fetch", Code: "x"}},
		{"missing code", storage.ScriptRecord{ID: "x", UserID: uid, Name: "x", Hook: "post_fetch"}},
		{"bad hook", storage.ScriptRecord{ID: "x", UserID: uid, Name: "x", Hook: "bogus", Code: "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := repo.Create(context.Background(), tc.rec); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}
