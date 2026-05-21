package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

func newProxyGroupRec(id, uid string, sort int32) storage.ProxyGroupCategoryRecord {
	return storage.ProxyGroupCategoryRecord{
		ID: id, UserID: uid,
		Name:      "g-" + id,
		Type:      "select",
		Icon:      "🚀",
		SortOrder: sort,
	}
}

func TestProxyGroupRepo_CRUD(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-pg-1")
	repo := storage.NewProxyGroupRepo(db, time.Now)

	created, err := repo.Create(context.Background(), newProxyGroupRec("g1", uid, 100))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.CreatedAt == 0 {
		t.Fatalf("timestamps not populated: %+v", created)
	}

	got, err := repo.GetByID(context.Background(), "g1", uid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "g-g1" || got.Type != "select" || got.SortOrder != 100 {
		t.Fatalf("round trip: %+v", got)
	}

	// Update: rename + change type + set members.
	includeAll := true
	newSort := int32(50)
	memberProxies := `["DIRECT","REJECT"]`
	upd := storage.ProxyGroupUpdate{
		Name:          "renamed",
		Type:          "url-test",
		SortOrder:     &newSort,
		IncludeAll:    &includeAll,
		MemberProxies: &memberProxies,
	}
	if err := repo.Update(context.Background(), "g1", uid, upd); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := repo.GetByID(context.Background(), "g1", uid)
	if got2.Name != "renamed" || got2.Type != "url-test" || got2.SortOrder != 50 || !got2.IncludeAll {
		t.Fatalf("update not applied: %+v", got2)
	}
	if got2.MemberProxies != memberProxies {
		t.Fatalf("members not persisted: %q", got2.MemberProxies)
	}

	if err := repo.Delete(context.Background(), "g1", uid); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = repo.GetByID(context.Background(), "g1", uid)
	if !errors.Is(err, storage.ErrProxyGroupNotFound) {
		t.Fatalf("want ErrProxyGroupNotFound, got %v", err)
	}
}

func TestProxyGroupRepo_CrossUserIsolation(t *testing.T) {
	db := newTestDB(t)
	uidA := seedUser(t, db, "u-pg-a")
	uidB := seedUser(t, db, "u-pg-b")
	repo := storage.NewProxyGroupRepo(db, time.Now)

	_, _ = repo.Create(context.Background(), newProxyGroupRec("g-shared", uidA, 100))

	_, err := repo.GetByID(context.Background(), "g-shared", uidB)
	if !errors.Is(err, storage.ErrProxyGroupNotFound) {
		t.Fatalf("cross-user GET should be not-found; got %v", err)
	}

	if err := repo.Delete(context.Background(), "g-shared", uidB); !errors.Is(err, storage.ErrProxyGroupNotFound) {
		t.Fatalf("cross-user DELETE should be not-found; got %v", err)
	}
}

func TestProxyGroupRepo_ListOrderedBySort(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-pg-list")
	repo := storage.NewProxyGroupRepo(db, time.Now)

	_, _ = repo.Create(context.Background(), newProxyGroupRec("z", uid, 300))
	_, _ = repo.Create(context.Background(), newProxyGroupRec("a", uid, 100))
	_, _ = repo.Create(context.Background(), newProxyGroupRec("m", uid, 200))

	all, total, err := repo.List(context.Background(), uid, storage.ProxyGroupListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 {
		t.Fatalf("total: %d", total)
	}
	want := []string{"a", "m", "z"}
	for i, w := range want {
		if all[i].ID != w {
			t.Fatalf("position %d: want %q, got %q", i, w, all[i].ID)
		}
	}
}

func TestProxyGroupRepo_Reorder(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-pg-reorder")
	repo := storage.NewProxyGroupRepo(db, time.Now)

	_, _ = repo.Create(context.Background(), newProxyGroupRec("a", uid, 100))
	_, _ = repo.Create(context.Background(), newProxyGroupRec("b", uid, 200))
	_, _ = repo.Create(context.Background(), newProxyGroupRec("c", uid, 300))

	updated, err := repo.Reorder(context.Background(), uid, []storage.ProxyGroupReorderEntry{
		{ID: "c", SortOrder: 50},
		{ID: "a", SortOrder: 150},
		{ID: "missing", SortOrder: 999},
	})
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if updated != 2 {
		t.Fatalf("want 2 affected (missing skipped), got %d", updated)
	}
	groups, _, _ := repo.List(context.Background(), uid, storage.ProxyGroupListOptions{})
	// expected order by sort_order: c(50), a(150), b(200)
	want := []string{"c", "a", "b"}
	for i, w := range want {
		if groups[i].ID != w {
			t.Fatalf("reorder mismatch position %d: want %q, got %q",
				i, w, groups[i].ID)
		}
	}
}
