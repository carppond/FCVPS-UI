package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

func newRuleRec(id, uid string, sort int32, enabled bool) storage.CustomRuleRecord {
	return storage.CustomRuleRecord{
		ID: id, UserID: uid, Name: "r-" + id, Type: "rules", Mode: "append",
		Content: "MATCH,Proxy\n", Enabled: enabled, Sort: sort,
	}
}

func TestCustomRuleRepo_CRUD(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-rule-1")
	repo := storage.NewCustomRuleRepo(db, time.Now)

	created, err := repo.Create(context.Background(), newRuleRec("r1", uid, 100, true))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.CreatedAt == 0 || created.UpdatedAt == 0 {
		t.Fatalf("timestamps not populated: %+v", created)
	}

	got, err := repo.GetByID(context.Background(), "r1", uid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "r-r1" || !got.Enabled {
		t.Fatalf("round trip: %+v", got)
	}

	// Update name + disable.
	upd := storage.CustomRuleRecord{
		ID: "r1", UserID: uid, Name: "renamed", Enabled: false,
	}
	if err := repo.Update(context.Background(), upd, true, false); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := repo.GetByID(context.Background(), "r1", uid)
	if got2.Name != "renamed" {
		t.Fatalf("name not updated: %q", got2.Name)
	}
	if got2.Enabled {
		t.Fatalf("enabled not flipped")
	}

	if err := repo.Delete(context.Background(), "r1", uid); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = repo.GetByID(context.Background(), "r1", uid)
	if !errors.Is(err, storage.ErrCustomRuleNotFound) {
		t.Fatalf("want ErrCustomRuleNotFound, got %v", err)
	}
}

func TestCustomRuleRepo_CrossUserIsolation(t *testing.T) {
	db := newTestDB(t)
	uidA := seedUser(t, db, "u-rule-a")
	uidB := seedUser(t, db, "u-rule-b")
	repo := storage.NewCustomRuleRepo(db, time.Now)

	_, _ = repo.Create(context.Background(), newRuleRec("r-shared", uidA, 100, true))

	// Cross-user GET returns not-found.
	_, err := repo.GetByID(context.Background(), "r-shared", uidB)
	if !errors.Is(err, storage.ErrCustomRuleNotFound) {
		t.Fatalf("cross-user GET should be not-found; got %v", err)
	}

	// Cross-user delete also returns not-found.
	if err := repo.Delete(context.Background(), "r-shared", uidB); !errors.Is(err, storage.ErrCustomRuleNotFound) {
		t.Fatalf("cross-user DELETE should be not-found; got %v", err)
	}
}

func TestCustomRuleRepo_ListEnabledOrder(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-rule-list")
	repo := storage.NewCustomRuleRepo(db, time.Now)

	// Mixed enabled / disabled / sort orders.
	_, _ = repo.Create(context.Background(), newRuleRec("r-300", uid, 300, true))
	_, _ = repo.Create(context.Background(), newRuleRec("r-100", uid, 100, true))
	_, _ = repo.Create(context.Background(), newRuleRec("r-200-off", uid, 200, false))
	_, _ = repo.Create(context.Background(), newRuleRec("r-200", uid, 200, true))

	enabled, err := repo.ListEnabled(context.Background(), uid)
	if err != nil {
		t.Fatalf("list enabled: %v", err)
	}
	if len(enabled) != 3 {
		t.Fatalf("want 3 enabled rules, got %d", len(enabled))
	}
	wantOrder := []string{"r-100", "r-200", "r-300"}
	for i, e := range enabled {
		if e.ID != wantOrder[i] {
			t.Fatalf("position %d: want %s, got %s", i, wantOrder[i], e.ID)
		}
	}
}

func TestCustomRuleRepo_Reorder(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-rule-reorder")
	repo := storage.NewCustomRuleRepo(db, time.Now)

	_, _ = repo.Create(context.Background(), newRuleRec("r-a", uid, 100, true))
	_, _ = repo.Create(context.Background(), newRuleRec("r-b", uid, 200, true))
	_, _ = repo.Create(context.Background(), newRuleRec("r-c", uid, 300, true))

	updated, err := repo.Reorder(context.Background(), uid, []storage.ReorderEntry{
		{ID: "r-c", Sort: 50},
		{ID: "r-a", Sort: 150},
		{ID: "missing", Sort: 999},
	})
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if updated != 2 {
		t.Fatalf("want 2 affected rows (missing id skipped), got %d", updated)
	}
	rules, _ := repo.ListEnabled(context.Background(), uid)
	// expected order by sort: c(50), a(150), b(200)
	got := []string{rules[0].ID, rules[1].ID, rules[2].ID}
	want := []string{"r-c", "r-a", "r-b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("reorder mismatch: want %v, got %v", want, got)
		}
	}
}

func TestCustomRuleRepo_List_PaginationAndFilter(t *testing.T) {
	db := newTestDB(t)
	uid := seedUser(t, db, "u-rule-page")
	repo := storage.NewCustomRuleRepo(db, time.Now)

	for i := 0; i < 5; i++ {
		rec := newRuleRec("p"+string(rune('a'+i)), uid, int32(i*100), true)
		_, _ = repo.Create(context.Background(), rec)
	}
	// Insert one of dns type to test filter.
	dnsRec := newRuleRec("p-dns", uid, 999, true)
	dnsRec.Type = "dns"
	dnsRec.Content = "nameservers: [1.1.1.1]\n"
	_, _ = repo.Create(context.Background(), dnsRec)

	all, total, err := repo.List(context.Background(), uid, storage.CustomRuleListOptions{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 6 {
		t.Fatalf("total: %d", total)
	}
	if len(all) != 6 {
		t.Fatalf("len(all): %d", len(all))
	}
	dnsOnly, total2, _ := repo.List(context.Background(), uid, storage.CustomRuleListOptions{Type: "dns"})
	if total2 != 1 {
		t.Fatalf("dns total: %d", total2)
	}
	if len(dnsOnly) != 1 || dnsOnly[0].ID != "p-dns" {
		t.Fatalf("dns filter wrong: %+v", dnsOnly)
	}
	// Pagination.
	page, totalP, _ := repo.List(context.Background(), uid, storage.CustomRuleListOptions{Page: 1, PageSize: 2})
	if totalP != 6 {
		t.Fatalf("paged total: %d", totalP)
	}
	if len(page) != 2 {
		t.Fatalf("page size not honoured: %d", len(page))
	}
}
