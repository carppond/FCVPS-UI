package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

func TestWidgetTokenRepo_ReplaceLookupDelete(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-widget")
	repo := storage.NewWidgetTokenRepo(db, time.Now)
	ctx := context.Background()

	// No token yet.
	if exists, err := repo.ExistsForUser(ctx, user); err != nil || exists {
		t.Fatalf("ExistsForUser before mint = (%v,%v), want (false,nil)", exists, err)
	}
	if _, err := repo.Lookup(ctx, "deadbeef"); !errors.Is(err, storage.ErrWidgetTokenNotFound) {
		t.Fatalf("Lookup missing = %v, want ErrWidgetTokenNotFound", err)
	}

	// Mint.
	if err := repo.Replace(ctx, user, "hash-1"); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	got, err := repo.Lookup(ctx, "hash-1")
	if err != nil || got != user {
		t.Fatalf("Lookup hash-1 = (%q,%v), want (%q,nil)", got, err, user)
	}
	if exists, _ := repo.ExistsForUser(ctx, user); !exists {
		t.Fatal("ExistsForUser after mint = false, want true")
	}

	// Rotate: a second Replace invalidates the old hash (one token per user).
	if err := repo.Replace(ctx, user, "hash-2"); err != nil {
		t.Fatalf("Replace rotate: %v", err)
	}
	if _, err := repo.Lookup(ctx, "hash-1"); !errors.Is(err, storage.ErrWidgetTokenNotFound) {
		t.Fatalf("old hash after rotate = %v, want ErrWidgetTokenNotFound", err)
	}
	if got, _ := repo.Lookup(ctx, "hash-2"); got != user {
		t.Fatalf("new hash lookup = %q, want %q", got, user)
	}

	// Delete (disable widget).
	if err := repo.DeleteByUser(ctx, user); err != nil {
		t.Fatalf("DeleteByUser: %v", err)
	}
	if _, err := repo.Lookup(ctx, "hash-2"); !errors.Is(err, storage.ErrWidgetTokenNotFound) {
		t.Fatalf("after delete = %v, want ErrWidgetTokenNotFound", err)
	}
	// Delete again is a no-op.
	if err := repo.DeleteByUser(ctx, user); err != nil {
		t.Fatalf("DeleteByUser idempotent: %v", err)
	}
}
