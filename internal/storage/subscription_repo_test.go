package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// seedUser inserts a minimal user row so the FK on subscriptions(user_id)
// is satisfied. The returned ID matches the user's row id.
func seedUser(t *testing.T, db *storage.DB, id string) string {
	t.Helper()
	repo := storage.NewUserRepo(db, time.Now)
	_, err := repo.Create(context.Background(), storage.UserRecord{
		ID: id, Username: id, PasswordHash: "x",
		Role: string(types.RoleUser), IsActive: true,
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func TestSubscriptionRepoCreateAndGet(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-create")
	repo := storage.NewSubscriptionRepo(db, time.Now)

	created, err := repo.Create(context.Background(), storage.SubscriptionRecord{
		ID:        "sub-1",
		UserID:    user,
		Name:      "primary",
		Type:      string(types.SubTypeURL),
		SourceURL: "https://example.com/sub",
		Tags:      []string{"foo", "bar"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ShareToken == "" {
		t.Fatalf("expected share_token to be generated")
	}
	if created.SyncInterval <= 0 {
		t.Fatalf("expected default sync_interval")
	}

	got, err := repo.GetByID(context.Background(), "sub-1", user)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "primary" || got.SourceURL != "https://example.com/sub" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "foo" {
		t.Fatalf("tags round-trip: %+v", got.Tags)
	}
}

func TestSubscriptionRepoCrossUserIsolation(t *testing.T) {
	db := newTestDB(t)
	owner := seedUser(t, db, "u-owner")
	intruder := seedUser(t, db, "u-intruder")
	repo := storage.NewSubscriptionRepo(db, time.Now)

	_, err := repo.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-iso", UserID: owner, Name: "private",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Intruder cannot read by id (filtered out by user_id clause).
	_, err = repo.GetByID(context.Background(), "sub-iso", intruder)
	if !errors.Is(err, storage.ErrSubscriptionNotFound) {
		t.Fatalf("expected ErrSubscriptionNotFound for intruder, got %v", err)
	}
	// Intruder cannot delete it.
	if err := repo.Delete(context.Background(), "sub-iso", intruder); !errors.Is(err, storage.ErrSubscriptionNotFound) {
		t.Fatalf("expected ErrSubscriptionNotFound on intruder delete, got %v", err)
	}
	// Owner still sees it.
	if _, err := repo.GetByID(context.Background(), "sub-iso", owner); err != nil {
		t.Fatalf("owner read failed: %v", err)
	}
	// Admin (userID == "") sees it.
	if _, err := repo.GetByID(context.Background(), "sub-iso", ""); err != nil {
		t.Fatalf("admin read failed: %v", err)
	}
}

func TestSubscriptionRepoListAndUpdate(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-list")
	repo := storage.NewSubscriptionRepo(db, time.Now)

	for i, name := range []string{"alpha", "beta", "gamma"} {
		_, err := repo.Create(context.Background(), storage.SubscriptionRecord{
			ID:        "sub-list-" + string(rune('a'+i)),
			UserID:    user,
			Name:      name,
			Type:      string(types.SubTypeManual),
			CreatedAt: time.Now().UnixMilli() + int64(i),
		})
		if err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}

	items, total, err := repo.List(context.Background(), user, storage.SubscriptionListOptions{PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total=3, got %d", total)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Update remark + tags then re-read.
	newName := "alpha-renamed"
	newTags := []string{"prod"}
	if err := repo.Update(context.Background(), "sub-list-a", user, storage.SubscriptionUpdate{
		Name: &newName, Tags: &newTags,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := repo.GetByID(context.Background(), "sub-list-a", user)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if got.Name != "alpha-renamed" || got.Tags[0] != "prod" {
		t.Fatalf("update did not persist: %+v", got)
	}
}

func TestSubscriptionRepoRotateShareToken(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-rot")
	repo := storage.NewSubscriptionRepo(db, time.Now)

	created, err := repo.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-rot", UserID: user, Name: "rotme",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	old := created.ShareToken

	newToken, err := repo.RotateShareToken(context.Background(), "sub-rot", user)
	if err != nil {
		t.Fatalf("RotateShareToken: %v", err)
	}
	if newToken == "" || newToken == old {
		t.Fatalf("rotation did not produce a new token (old=%q new=%q)", old, newToken)
	}
	// Old token no longer resolves to a subscription.
	if _, err := repo.GetByShareToken(context.Background(), old); !errors.Is(err, storage.ErrSubscriptionNotFound) {
		t.Fatalf("expected old token to be invalid, got %v", err)
	}
	if _, err := repo.GetByShareToken(context.Background(), newToken); err != nil {
		t.Fatalf("new token lookup failed: %v", err)
	}
}

func TestSubscriptionRepoUpdateSyncState(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-sync")
	repo := storage.NewSubscriptionRepo(db, time.Now)
	_, err := repo.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-sync", UserID: user, Name: "syncme",
		Type: string(types.SubTypeURL), SourceURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	now := time.Now()
	if err := repo.UpdateSyncState(context.Background(), "sub-sync",
		string(types.SyncStatusOK), now, ""); err != nil {
		t.Fatalf("UpdateSyncState ok: %v", err)
	}
	got, err := repo.GetByID(context.Background(), "sub-sync", user)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if got.LastSyncStatus != string(types.SyncStatusOK) {
		t.Fatalf("expected status=ok, got %q", got.LastSyncStatus)
	}
	if got.LastSyncedAt == 0 {
		t.Fatalf("expected last_synced_at to be populated")
	}

	if err := repo.UpdateSyncState(context.Background(), "sub-sync",
		string(types.SyncStatusError), now, "boom"); err != nil {
		t.Fatalf("UpdateSyncState error: %v", err)
	}
	got, _ = repo.GetByID(context.Background(), "sub-sync", user)
	if got.LastSyncStatus != string(types.SyncStatusError) || got.LastSyncError != "boom" {
		t.Fatalf("error state did not persist: %+v", got)
	}
}

func TestSubscriptionRepoDeleteCascadesNodes(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-cascade")
	repo := storage.NewSubscriptionRepo(db, time.Now)
	_, err := repo.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-cascade", UserID: user, Name: "cascading",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Insert a dummy node directly to exercise the cascade.
	_, err = db.Write.Exec(`INSERT INTO nodes(id, subscription_id, raw_uri, parsed_config_json,
		protocol, server, port, tag, position, created_at, updated_at)
		VALUES (?, ?, ?, '{}', 'vmess', '1.2.3.4', 443, 'n1', 0, 1, 1)`,
		"node-cascade", "sub-cascade", "vmess://abc")
	if err != nil {
		t.Fatalf("insert node: %v", err)
	}

	if err := repo.Delete(context.Background(), "sub-cascade", user); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	var n int
	if err := db.Read.QueryRow("SELECT COUNT(*) FROM nodes WHERE subscription_id = ?", "sub-cascade").Scan(&n); err != nil {
		t.Fatalf("count nodes: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected nodes to be cascade-deleted, got %d", n)
	}
}

func TestSubscriptionRepoGetByName(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-name")
	repo := storage.NewSubscriptionRepo(db, time.Now)
	_, err := repo.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-name", UserID: user, Name: "lookup-by-name",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetByName(context.Background(), user, "lookup-by-name")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.ID != "sub-name" {
		t.Fatalf("expected sub-name, got %s", got.ID)
	}
}
