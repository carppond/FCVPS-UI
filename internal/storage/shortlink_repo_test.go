package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

func TestShortLinkRepoCreateAndResolve(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	repo := storage.NewShortLinkRepo(db, time.Now)
	_, err := repo.Create(context.Background(), storage.ShortLinkRecord{
		FileCode: "1", UserCode: "1", UserID: "u1",
		TargetURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.Resolve(context.Background(), "1", "1")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.TargetURL != "https://example.com" {
		t.Fatalf("unexpected target: %q", got.TargetURL)
	}
}

func TestShortLinkRepoResolveExpired(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	repo := storage.NewShortLinkRepo(db, time.Now)
	past := time.Now().Add(-time.Hour).UnixMilli()
	_, err := repo.Create(context.Background(), storage.ShortLinkRecord{
		FileCode: "1", UserCode: "1", UserID: "u1",
		TargetURL: "https://example.com", ExpiresAt: past,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = repo.Resolve(context.Background(), "1", "1")
	if !errors.Is(err, storage.ErrShortLinkNotFound) {
		t.Fatalf("expected ErrShortLinkNotFound, got %v", err)
	}
}

func TestShortLinkRepoMaxFileCode(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	repo := storage.NewShortLinkRepo(db, time.Now)
	got, err := repo.MaxFileCode(context.Background())
	if err != nil {
		t.Fatalf("MaxFileCode empty: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty MaxFileCode on empty table, got %q", got)
	}
	for _, code := range []string{"1", "2", "A"} {
		_, err := repo.Create(context.Background(), storage.ShortLinkRecord{
			FileCode: code, UserCode: "1", UserID: "u1",
			TargetURL: "https://x",
		})
		if err != nil {
			t.Fatalf("Create %s: %v", code, err)
		}
	}
	got, err = repo.MaxFileCode(context.Background())
	if err != nil {
		t.Fatalf("MaxFileCode: %v", err)
	}
	// lex MAX of {"1","2","A"} is "A" because uppercase < lowercase but A > 2 in ASCII.
	if got != "A" {
		t.Fatalf("expected MAX 'A', got %q", got)
	}
}

func TestShortLinkRepoListByUserSkipsExpired(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	repo := storage.NewShortLinkRepo(db, time.Now)
	past := time.Now().Add(-time.Hour).UnixMilli()
	_, _ = repo.Create(context.Background(), storage.ShortLinkRecord{
		FileCode: "1", UserCode: "1", UserID: "u1", TargetURL: "https://x",
	})
	_, _ = repo.Create(context.Background(), storage.ShortLinkRecord{
		FileCode: "2", UserCode: "1", UserID: "u1", TargetURL: "https://y",
		ExpiresAt: past,
	})
	list, err := repo.ListByUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 active link, got %d", len(list))
	}
}

func TestShortLinkRepoDeleteGuardsUserID(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedUser(t, db, "u2")
	repo := storage.NewShortLinkRepo(db, time.Now)
	_, _ = repo.Create(context.Background(), storage.ShortLinkRecord{
		FileCode: "1", UserCode: "1", UserID: "u1", TargetURL: "https://x",
	})
	if err := repo.Delete(context.Background(), "1", "1", "u2"); !errors.Is(err, storage.ErrShortLinkNotFound) {
		t.Fatalf("expected ErrShortLinkNotFound from wrong owner, got %v", err)
	}
	if err := repo.Delete(context.Background(), "1", "1", "u1"); err != nil {
		t.Fatalf("Delete by owner: %v", err)
	}
}
