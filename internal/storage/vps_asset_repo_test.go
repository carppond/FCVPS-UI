package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

func TestVpsAssetRepoCreateAndGet(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewVpsAssetRepo(db, time.Now)

	rec := storage.VpsAssetRecord{
		ID:           "v1",
		UserID:       "u1",
		Name:         "hk-vps-01",
		IP:           "1.2.3.4",
		SSHPort:      22,
		Provider:     "搬瓦工",
		Price:        49.99,
		Currency:     "USD",
		BillingCycle: "annual",
		ExpireAt:     time.Now().AddDate(0, 6, 0).Format("2006-01-02"),
		Tags:         `["prod","hk"]`,
	}
	created, err := repo.Create(context.Background(), rec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Fatalf("expected timestamps populated")
	}

	got, err := repo.GetByID(context.Background(), "v1", "u1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "hk-vps-01" {
		t.Fatalf("name mismatch: got %q", got.Name)
	}
	if got.Status != "normal" {
		t.Fatalf("expected status=normal for future expiry, got %q", got.Status)
	}
	tags := got.TagsSlice()
	if len(tags) != 2 || tags[0] != "prod" {
		t.Fatalf("tags mismatch: %v", tags)
	}
}

func TestVpsAssetRepoCrossUserHidden(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	newTestUser(t, db, "u2")
	repo := storage.NewVpsAssetRepo(db, time.Now)

	if _, err := repo.Create(context.Background(), storage.VpsAssetRecord{
		ID: "v1", UserID: "u1", Name: "n", Provider: "p",
		Price: 10, BillingCycle: "monthly", ExpireAt: "2099-01-01",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// u2 cannot see u1's asset.
	if _, err := repo.GetByID(context.Background(), "v1", "u2"); !errors.Is(err, storage.ErrVpsAssetNotFound) {
		t.Fatalf("expected ErrVpsAssetNotFound for cross-user, got %v", err)
	}
}

func TestVpsAssetRepoListFiltered(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewVpsAssetRepo(db, time.Now)

	for _, rec := range []storage.VpsAssetRecord{
		{ID: "v1", UserID: "u1", Name: "hk-01", Provider: "搬瓦工", Price: 10, BillingCycle: "monthly", ExpireAt: "2099-01-01", Location: "hk"},
		{ID: "v2", UserID: "u1", Name: "jp-01", Provider: "Hetzner", Price: 20, BillingCycle: "annual", ExpireAt: "2099-01-01", Location: "jp"},
		{ID: "v3", UserID: "u1", Name: "us-01", Provider: "搬瓦工", Price: 5, BillingCycle: "monthly", ExpireAt: time.Now().Add(-24 * time.Hour).Format("2006-01-02"), Location: "us"},
	} {
		if _, err := repo.Create(context.Background(), rec); err != nil {
			t.Fatalf("Create %s: %v", rec.ID, err)
		}
	}

	// Filter by provider.
	recs, total, err := repo.List(context.Background(), "u1", storage.VpsAssetListOptions{
		Page: 1, PageSize: 10, Provider: "搬瓦工",
	})
	if err != nil {
		t.Fatalf("List by provider: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 assets for provider 搬瓦工, got %d", total)
	}

	// Filter by status=expired.
	recs, total, err = repo.List(context.Background(), "u1", storage.VpsAssetListOptions{
		Page: 1, PageSize: 10, Status: "expired",
	})
	if err != nil {
		t.Fatalf("List by status: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 expired, got %d", total)
	}
	if recs[0].ID != "v3" {
		t.Fatalf("expected v3, got %s", recs[0].ID)
	}
	_ = recs
}

func TestVpsAssetRepoExpiryStatus(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewVpsAssetRepo(db, time.Now)

	now := time.Now()
	cases := []struct {
		id       string
		expireAt string
		wantStat string
	}{
		{"v-normal", now.AddDate(0, 0, 30).Format("2006-01-02"), "normal"},
		{"v-expiring", now.AddDate(0, 0, 3).Format("2006-01-02"), "expiring"},
		{"v-expired", now.AddDate(0, 0, -1).Format("2006-01-02"), "expired"},
	}
	for _, tc := range cases {
		if _, err := repo.Create(context.Background(), storage.VpsAssetRecord{
			ID: tc.id, UserID: "u1", Name: tc.id, Provider: "p",
			Price: 10, BillingCycle: "monthly", ExpireAt: tc.expireAt,
		}); err != nil {
			t.Fatalf("Create %s: %v", tc.id, err)
		}
	}
	for _, tc := range cases {
		got, err := repo.GetByID(context.Background(), tc.id, "u1")
		if err != nil {
			t.Fatalf("GetByID %s: %v", tc.id, err)
		}
		if got.Status != tc.wantStat {
			t.Errorf("%s: status=%q, want %q (days_until_expiry=%d)", tc.id, got.Status, tc.wantStat, got.DaysUntilExpiry)
		}
	}
}

func TestVpsAssetRepoListExpiring(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewVpsAssetRepo(db, time.Now)

	now := time.Now()
	if _, err := repo.Create(context.Background(), storage.VpsAssetRecord{
		ID: "v-soon", UserID: "u1", Name: "soon", Provider: "p",
		Price: 10, BillingCycle: "monthly", ExpireAt: now.AddDate(0, 0, 2).Format("2006-01-02"),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.Create(context.Background(), storage.VpsAssetRecord{
		ID: "v-far", UserID: "u1", Name: "far", Provider: "p",
		Price: 10, BillingCycle: "monthly", ExpireAt: now.AddDate(0, 6, 0).Format("2006-01-02"),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	recs, err := repo.ListAllExpiring(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListAllExpiring: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 expiring, got %d", len(recs))
	}
	if recs[0].ID != "v-soon" {
		t.Fatalf("expected v-soon, got %s", recs[0].ID)
	}
}

func TestVpsAssetRepoUpdateAndDelete(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewVpsAssetRepo(db, time.Now)

	if _, err := repo.Create(context.Background(), storage.VpsAssetRecord{
		ID: "v1", UserID: "u1", Name: "old-name", Provider: "p",
		Price: 10, BillingCycle: "monthly", ExpireAt: "2099-01-01",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := repo.Update(context.Background(), "v1", "u1", map[string]any{
		"name":  "new-name",
		"price": 99.0,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "new-name" {
		t.Fatalf("expected new-name, got %q", updated.Name)
	}
	if updated.Price != 99.0 {
		t.Fatalf("expected price=99, got %f", updated.Price)
	}

	if err := repo.Delete(context.Background(), "v1", "u1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(context.Background(), "v1", "u1"); !errors.Is(err, storage.ErrVpsAssetNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestVpsAssetRepoSummary(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewVpsAssetRepo(db, time.Now)

	now := time.Now()
	for _, rec := range []storage.VpsAssetRecord{
		{ID: "v1", UserID: "u1", Name: "a", Provider: "p", Price: 120, Currency: "CNY", BillingCycle: "annual", ExpireAt: "2099-01-01"},
		{ID: "v2", UserID: "u1", Name: "b", Provider: "p", Price: 5, Currency: "USD", BillingCycle: "monthly", ExpireAt: now.AddDate(0, 0, 3).Format("2006-01-02")},
		{ID: "v3", UserID: "u1", Name: "c", Provider: "p", Price: 30, Currency: "CNY", BillingCycle: "monthly", ExpireAt: now.AddDate(0, 0, -1).Format("2006-01-02")},
	} {
		if _, err := repo.Create(context.Background(), rec); err != nil {
			t.Fatalf("Create %s: %v", rec.ID, err)
		}
	}

	total, expiring, expired, err := repo.Summary(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if total != 3 {
		t.Fatalf("total: want 3, got %d", total)
	}
	if expiring != 1 {
		t.Fatalf("expiring: want 1, got %d", expiring)
	}
	if expired != 1 {
		t.Fatalf("expired: want 1, got %d", expired)
	}

	costMap, err := repo.MonthlyCostByUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("MonthlyCostByUser: %v", err)
	}
	// CNY: annual 120/12=10 + monthly 30 = 40
	if costMap["CNY"] != 40 {
		t.Fatalf("CNY monthly cost: want 40, got %f", costMap["CNY"])
	}
	if costMap["USD"] != 5 {
		t.Fatalf("USD monthly cost: want 5, got %f", costMap["USD"])
	}
}

// Empty table: SUM(...) over zero rows yields SQL NULL; the query must COALESCE
// it to 0, otherwise scanning NULL into int fails (regression for the
// "converting NULL to int is unsupported" 500 on fresh installs).
func TestVpsAssetRepoSummaryEmpty(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewVpsAssetRepo(db, time.Now)

	total, expiring, expired, err := repo.Summary(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Summary on empty table: %v", err)
	}
	if total != 0 || expiring != 0 || expired != 0 {
		t.Fatalf("empty summary: want 0/0/0, got %d/%d/%d", total, expiring, expired)
	}
}
