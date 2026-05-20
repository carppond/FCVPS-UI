package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
)

func newTestDB(t *testing.T) *storage.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(config.DatabaseConfig{
		DataDir: dir, Filename: "test.db", BusyTimeoutMs: 5000,
		MaxOpenWrite: 1, MaxOpenRead: 2,
	})
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return db
}

func TestUserRepoCreateAndGetByUsername(t *testing.T) {
	db := newTestDB(t)
	repo := storage.NewUserRepo(db, time.Now)
	rec := storage.UserRecord{
		ID: "u1", Username: "alice", PasswordHash: "hash",
		Role: "user", IsActive: true,
	}
	created, err := repo.Create(context.Background(), rec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.CreatedAt == 0 || created.UpdatedAt == 0 {
		t.Fatalf("expected timestamps to be populated")
	}
	got, err := repo.GetByUsername(context.Background(), "alice")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if got.ID != "u1" || got.PasswordHash != "hash" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestUserRepoGetByUsernameNotFound(t *testing.T) {
	db := newTestDB(t)
	repo := storage.NewUserRepo(db, time.Now)
	_, err := repo.GetByUsername(context.Background(), "nobody")
	if !errors.Is(err, storage.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepoCreateDuplicateUsername(t *testing.T) {
	db := newTestDB(t)
	repo := storage.NewUserRepo(db, time.Now)
	rec := storage.UserRecord{ID: "u1", Username: "alice", PasswordHash: "h", Role: "user", IsActive: true}
	if _, err := repo.Create(context.Background(), rec); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	rec.ID = "u2"
	if _, err := repo.Create(context.Background(), rec); !errors.Is(err, storage.ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
}

func TestUserRepoCountAdmins(t *testing.T) {
	db := newTestDB(t)
	repo := storage.NewUserRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), storage.UserRecord{ID: "a", Username: "a", PasswordHash: "h", Role: "admin", IsActive: true})
	_, _ = repo.Create(context.Background(), storage.UserRecord{ID: "b", Username: "b", PasswordHash: "h", Role: "admin", IsActive: true})
	_, _ = repo.Create(context.Background(), storage.UserRecord{ID: "c", Username: "c", PasswordHash: "h", Role: "user", IsActive: true})
	count, err := repo.CountAdmins(context.Background())
	if err != nil {
		t.Fatalf("CountAdmins: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 admins, got %d", count)
	}
}

func TestUserRepoUpdatePassword(t *testing.T) {
	db := newTestDB(t)
	repo := storage.NewUserRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), storage.UserRecord{ID: "u1", Username: "alice", PasswordHash: "old", Role: "user", IsActive: true})
	if err := repo.UpdatePassword(context.Background(), "u1", "new"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	got, _ := repo.GetByID(context.Background(), "u1")
	if got.PasswordHash != "new" {
		t.Fatalf("expected hash 'new', got %q", got.PasswordHash)
	}
}

func TestUserRepoDeleteCascades(t *testing.T) {
	db := newTestDB(t)
	repo := storage.NewUserRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), storage.UserRecord{ID: "u1", Username: "alice", PasswordHash: "h", Role: "user", IsActive: true})
	if err := repo.Delete(context.Background(), "u1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(context.Background(), "u1"); !errors.Is(err, storage.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound on second delete, got %v", err)
	}
}

func TestSessionRepoCRUD(t *testing.T) {
	db := newTestDB(t)
	users := storage.NewUserRepo(db, time.Now)
	_, _ = users.Create(context.Background(), storage.UserRecord{ID: "u1", Username: "alice", PasswordHash: "h", Role: "user", IsActive: true})

	sessions := storage.NewSessionRepo(db, time.Now)
	rec := storage.SessionRecord{
		ID: "s1", UserID: "u1", TokenHash: "hash-1",
		ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
	}
	if err := sessions.Create(context.Background(), rec); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := sessions.GetByTokenHash(context.Background(), "hash-1")
	if err != nil {
		t.Fatalf("GetByTokenHash: %v", err)
	}
	if got.ID != "s1" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if err := sessions.Delete(context.Background(), "hash-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := sessions.GetByTokenHash(context.Background(), "hash-1"); !errors.Is(err, storage.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound after delete, got %v", err)
	}
}

func TestSessionRepoCleanupExpired(t *testing.T) {
	db := newTestDB(t)
	users := storage.NewUserRepo(db, time.Now)
	_, _ = users.Create(context.Background(), storage.UserRecord{ID: "u1", Username: "alice", PasswordHash: "h", Role: "user", IsActive: true})

	sessions := storage.NewSessionRepo(db, time.Now)
	past := time.Now().Add(-time.Hour).UnixMilli()
	future := time.Now().Add(time.Hour).UnixMilli()
	_ = sessions.Create(context.Background(), storage.SessionRecord{ID: "expired", UserID: "u1", TokenHash: "h1", ExpiresAt: past})
	_ = sessions.Create(context.Background(), storage.SessionRecord{ID: "live", UserID: "u1", TokenHash: "h2", ExpiresAt: future})
	n, err := sessions.CleanupExpired(context.Background())
	if err != nil {
		t.Fatalf("CleanupExpired: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 expired row deleted, got %d", n)
	}
}
