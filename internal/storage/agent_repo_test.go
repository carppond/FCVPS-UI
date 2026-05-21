package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

// newTestUser inserts a user row used as the FK parent for agents tests.
// agents.user_id has ON DELETE CASCADE → users(id), so the FK constraint must
// be satisfied.
func newTestUser(t *testing.T, db *storage.DB, id string) {
	t.Helper()
	users := storage.NewUserRepo(db, time.Now)
	if _, err := users.Create(context.Background(), storage.UserRecord{
		ID: id, Username: id, PasswordHash: "h", Role: "user", IsActive: true,
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func TestAgentRepoCreateAndGetByTokenHash(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewAgentRepo(db, time.Now)
	rec := storage.AgentRecord{
		ID: "a1", UserID: "u1", Name: "nyc-1",
		TokenHash: "deadbeef", Kind: "native",
	}
	created, err := repo.Create(context.Background(), rec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.CreatedAt == 0 || created.UpdatedAt == 0 {
		t.Fatalf("expected timestamps populated")
	}
	if created.Status != "offline" {
		t.Fatalf("expected default status=offline, got %q", created.Status)
	}
	got, err := repo.GetByTokenHash(context.Background(), "deadbeef")
	if err != nil {
		t.Fatalf("GetByTokenHash: %v", err)
	}
	if got.ID != "a1" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestAgentRepoGetByIDCrossUserHidden(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	newTestUser(t, db, "u2")
	repo := storage.NewAgentRepo(db, time.Now)
	if _, err := repo.Create(context.Background(), storage.AgentRecord{
		ID: "a1", UserID: "u1", Name: "n", TokenHash: "h1", Kind: "native",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// u2 cannot see u1's agent.
	if _, err := repo.GetByID(context.Background(), "a1", "u2"); !errors.Is(err, storage.ErrAgentNotFound) {
		t.Fatalf("expected ErrAgentNotFound for cross-user, got %v", err)
	}
	// userID="" (hub-internal path) succeeds.
	if _, err := repo.GetByID(context.Background(), "a1", ""); err != nil {
		t.Fatalf("internal GetByID: %v", err)
	}
}

func TestAgentRepoListByUserIsolated(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	newTestUser(t, db, "u2")
	repo := storage.NewAgentRepo(db, time.Now)
	for i, owner := range []string{"u1", "u1", "u2"} {
		if _, err := repo.Create(context.Background(), storage.AgentRecord{
			ID: "a" + string(rune('1'+i)), UserID: owner,
			Name: "n", TokenHash: "h" + string(rune('1'+i)), Kind: "native",
		}); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}
	recs, total, err := repo.ListByUser(context.Background(), "u1", storage.AgentListOptions{})
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if total != 2 || len(recs) != 2 {
		t.Fatalf("expected 2 rows for u1, got total=%d len=%d", total, len(recs))
	}
	for _, r := range recs {
		if r.UserID != "u1" {
			t.Fatalf("unexpected owner %q in u1 listing", r.UserID)
		}
	}
}

func TestAgentRepoUpdateLastSeenStatusAndProfile(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewAgentRepo(db, time.Now)
	if _, err := repo.Create(context.Background(), storage.AgentRecord{
		ID: "a1", UserID: "u1", Name: "n", TokenHash: "h", Kind: "native",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.UpdateLastSeen(context.Background(), "a1", "online", "1.0.0", "linux", "amd64"); err != nil {
		t.Fatalf("UpdateLastSeen: %v", err)
	}
	got, _ := repo.GetByID(context.Background(), "a1", "u1")
	if got.Status != "online" || got.Version != "1.0.0" || got.OS != "linux" || got.Arch != "amd64" {
		t.Fatalf("update did not stick: %+v", got)
	}
	if got.LastSeenAt == 0 {
		t.Fatalf("expected last_seen_at populated")
	}
	// Heartbeat path (omit version/os/arch) must not clobber existing values.
	if err := repo.UpdateLastSeen(context.Background(), "a1", "online", "", "", ""); err != nil {
		t.Fatalf("heartbeat update: %v", err)
	}
	got2, _ := repo.GetByID(context.Background(), "a1", "u1")
	if got2.Version != "1.0.0" || got2.OS != "linux" || got2.Arch != "amd64" {
		t.Fatalf("heartbeat update clobbered handshake fields: %+v", got2)
	}
}

func TestAgentRepoUpdateLastSeenRejectsInvalidStatus(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewAgentRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), storage.AgentRecord{
		ID: "a1", UserID: "u1", Name: "n", TokenHash: "h", Kind: "native",
	})
	if err := repo.UpdateLastSeen(context.Background(), "a1", "bogus", "", "", ""); err == nil {
		t.Fatalf("expected error for invalid status")
	}
}

func TestAgentRepoRotateToken(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewAgentRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), storage.AgentRecord{
		ID: "a1", UserID: "u1", Name: "n", TokenHash: "h_old", Kind: "native",
	})
	if err := repo.RotateToken(context.Background(), "a1", "u1", "h_new"); err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	if _, err := repo.GetByTokenHash(context.Background(), "h_old"); !errors.Is(err, storage.ErrAgentNotFound) {
		t.Fatalf("old token still resolves: %v", err)
	}
	if _, err := repo.GetByTokenHash(context.Background(), "h_new"); err != nil {
		t.Fatalf("new token does not resolve: %v", err)
	}
}

func TestAgentRepoDeleteCrossUser(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	newTestUser(t, db, "u2")
	repo := storage.NewAgentRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), storage.AgentRecord{
		ID: "a1", UserID: "u1", Name: "n", TokenHash: "h", Kind: "native",
	})
	if err := repo.Delete(context.Background(), "a1", "u2"); !errors.Is(err, storage.ErrAgentNotFound) {
		t.Fatalf("cross-user delete must fail: %v", err)
	}
	if err := repo.Delete(context.Background(), "a1", "u1"); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if _, err := repo.GetByID(context.Background(), "a1", "u1"); !errors.Is(err, storage.ErrAgentNotFound) {
		t.Fatalf("agent still present after delete: %v", err)
	}
}

func TestAgentRepoMarkAllOffline(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewAgentRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), storage.AgentRecord{
		ID: "a1", UserID: "u1", Name: "n", TokenHash: "h", Kind: "native",
		Status: "online",
	})
	_, _ = repo.Create(context.Background(), storage.AgentRecord{
		ID: "a2", UserID: "u1", Name: "n", TokenHash: "h2", Kind: "native",
		Status: "degraded",
	})
	if err := repo.MarkAllOffline(context.Background()); err != nil {
		t.Fatalf("MarkAllOffline: %v", err)
	}
	for _, id := range []string{"a1", "a2"} {
		got, _ := repo.GetByID(context.Background(), id, "u1")
		if got.Status != "offline" {
			t.Fatalf("%s status = %q after MarkAllOffline", id, got.Status)
		}
	}
}

func TestAgentRepoCreateValidatesKind(t *testing.T) {
	db := newTestDB(t)
	newTestUser(t, db, "u1")
	repo := storage.NewAgentRepo(db, time.Now)
	_, err := repo.Create(context.Background(), storage.AgentRecord{
		ID: "a1", UserID: "u1", Name: "n", TokenHash: "h", Kind: "bogus",
	})
	if err == nil {
		t.Fatalf("expected error for invalid kind")
	}
}
