package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

func newRuleSetRec(id, uid string, enabled bool) storage.RuleSetProviderRecord {
	return storage.RuleSetProviderRecord{
		ID: id, UserID: uid,
		Name:            "rs-" + id,
		Behavior:        "domain",
		Format:          "mrs",
		URL:             "https://example.com/" + id + ".mrs",
		IntervalSeconds: 86400,
		Enabled:         enabled,
	}
}

func TestRuleSetRepo_CRUD(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-rs-1")
	repo := storage.NewRuleSetProviderRepo(db, time.Now)

	created, err := repo.Create(context.Background(), newRuleSetRec("rs1", uid, true))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.CreatedAt == 0 || created.UpdatedAt == 0 {
		t.Fatalf("timestamps not populated: %+v", created)
	}

	got, err := repo.GetByID(context.Background(), "rs1", uid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "rs-rs1" || !got.Enabled || got.Behavior != "domain" {
		t.Fatalf("round trip: %+v", got)
	}

	// Update: switch behavior + disable.
	enabled := false
	upd := storage.RuleSetProviderUpdate{
		Behavior: "classical",
		Enabled:  &enabled,
	}
	if err := repo.Update(context.Background(), "rs1", uid, upd); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := repo.GetByID(context.Background(), "rs1", uid)
	if got2.Behavior != "classical" {
		t.Fatalf("behavior not updated: %q", got2.Behavior)
	}
	if got2.Enabled {
		t.Fatalf("enabled not flipped")
	}

	// Sync status.
	if err := repo.UpdateSyncStatus(context.Background(), "rs1", uid, "ok", ""); err != nil {
		t.Fatalf("update sync status: %v", err)
	}
	got3, _ := repo.GetByID(context.Background(), "rs1", uid)
	if got3.LastSyncedAt == 0 || got3.LastSyncStatus != "ok" {
		t.Fatalf("sync status not persisted: %+v", got3)
	}

	if err := repo.Delete(context.Background(), "rs1", uid); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = repo.GetByID(context.Background(), "rs1", uid)
	if !errors.Is(err, storage.ErrRuleSetProviderNotFound) {
		t.Fatalf("want ErrRuleSetProviderNotFound, got %v", err)
	}
}

func TestRuleSetRepo_CrossUserIsolation(t *testing.T) {
	db := newTestDB(t)
	uidA := seedUser(t, db, "u-rs-a")
	uidB := seedUser(t, db, "u-rs-b")
	repo := storage.NewRuleSetProviderRepo(db, time.Now)

	_, _ = repo.Create(context.Background(), newRuleSetRec("rs-shared", uidA, true))

	_, err := repo.GetByID(context.Background(), "rs-shared", uidB)
	if !errors.Is(err, storage.ErrRuleSetProviderNotFound) {
		t.Fatalf("cross-user GET should be not-found; got %v", err)
	}

	if err := repo.Delete(context.Background(), "rs-shared", uidB); !errors.Is(err, storage.ErrRuleSetProviderNotFound) {
		t.Fatalf("cross-user DELETE should be not-found; got %v", err)
	}

	// uidA can still see + delete it.
	if _, err := repo.GetByID(context.Background(), "rs-shared", uidA); err != nil {
		t.Fatalf("owner GET: %v", err)
	}
}

func TestRuleSetRepo_ListEnabled(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-rs-list")
	repo := storage.NewRuleSetProviderRepo(db, time.Now)

	_, _ = repo.Create(context.Background(), newRuleSetRec("a", uid, true))
	_, _ = repo.Create(context.Background(), newRuleSetRec("b", uid, false))
	_, _ = repo.Create(context.Background(), newRuleSetRec("c", uid, true))

	enabled, err := repo.ListEnabled(context.Background(), uid)
	if err != nil {
		t.Fatalf("list enabled: %v", err)
	}
	if len(enabled) != 2 {
		t.Fatalf("want 2 enabled, got %d", len(enabled))
	}
}

func TestRuleSetRepo_List_PaginationAndFilter(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-rs-page")
	repo := storage.NewRuleSetProviderRepo(db, time.Now)

	for i := 0; i < 5; i++ {
		rec := newRuleSetRec("p"+string(rune('a'+i)), uid, true)
		_, _ = repo.Create(context.Background(), rec)
	}
	// One row whose name matches keyword "special".
	special := newRuleSetRec("p-sp", uid, true)
	special.Name = "special-one"
	_, _ = repo.Create(context.Background(), special)

	all, total, err := repo.List(context.Background(), uid, storage.RuleSetProviderListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 6 || len(all) != 6 {
		t.Fatalf("totals: total=%d, len=%d", total, len(all))
	}
	hits, totalK, _ := repo.List(context.Background(), uid, storage.RuleSetProviderListOptions{Keyword: "special"})
	if totalK != 1 || len(hits) != 1 {
		t.Fatalf("keyword: total=%d, hits=%d", totalK, len(hits))
	}
	page, totalP, _ := repo.List(context.Background(), uid, storage.RuleSetProviderListOptions{Page: 1, PageSize: 2})
	if totalP != 6 || len(page) != 2 {
		t.Fatalf("paged: total=%d, len=%d", totalP, len(page))
	}
}
