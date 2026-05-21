package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

// TestSettingsRepo_GetSetRoundTrip covers the happy path: Set persists a row
// then Get returns the same value. This is the single most-exercised path so
// breaking it would surface on the next test run.
func TestSettingsRepo_GetSetRoundTrip(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := storage.NewSettingsRepo(db, time.Now)
	ctx := context.Background()

	if err := repo.Set(ctx, "k1", "v1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := repo.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "v1" {
		t.Fatalf("Get returned %q want v1", got)
	}

	// Overwrite check — Set on an existing row updates the value rather than
	// erroring.
	if err := repo.Set(ctx, "k1", "v2"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	got2, _ := repo.Get(ctx, "k1")
	if got2 != "v2" {
		t.Fatalf("after overwrite got %q want v2", got2)
	}
}

// TestSettingsRepo_GetMissingReturnsSentinel ensures the missing-row case is
// distinguishable from a generic SQL error.
func TestSettingsRepo_GetMissingReturnsSentinel(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := storage.NewSettingsRepo(db, time.Now)
	_, err := repo.Get(context.Background(), "does_not_exist")
	if !errors.Is(err, storage.ErrSettingNotFound) {
		t.Fatalf("expected ErrSettingNotFound, got %v", err)
	}
}

// TestSettingsRepo_GetAllReturnsCompleteMap seeds a handful of keys and
// asserts GetAll returns them in a single response.
func TestSettingsRepo_GetAllReturnsCompleteMap(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := storage.NewSettingsRepo(db, time.Now)
	ctx := context.Background()

	seeds := map[string]string{
		"a": "1",
		"b": "two",
		"c": "x",
	}
	for k, v := range seeds {
		if err := repo.Set(ctx, k, v); err != nil {
			t.Fatalf("Set %q: %v", k, err)
		}
	}
	all, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	for k, want := range seeds {
		if got := all[k]; got != want {
			t.Fatalf("GetAll[%q] = %q, want %q", k, got, want)
		}
	}
}

// TestSettingsRepo_SetManyAtomicAndIdempotent verifies the batch upsert
// produces the expected end state and that an empty input is a no-op.
func TestSettingsRepo_SetManyAtomicAndIdempotent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := storage.NewSettingsRepo(db, time.Now)
	ctx := context.Background()

	if err := repo.SetMany(ctx, nil); err != nil {
		t.Fatalf("SetMany(nil): %v", err)
	}
	if err := repo.SetMany(ctx, map[string]string{}); err != nil {
		t.Fatalf("SetMany(empty): %v", err)
	}

	if err := repo.SetMany(ctx, map[string]string{
		"k1": "v1",
		"k2": "v2",
	}); err != nil {
		t.Fatalf("SetMany: %v", err)
	}
	// Second call updates one + adds one.
	if err := repo.SetMany(ctx, map[string]string{
		"k1": "v1-new",
		"k3": "v3",
	}); err != nil {
		t.Fatalf("SetMany 2: %v", err)
	}
	all, _ := repo.GetAll(ctx)
	if all["k1"] != "v1-new" || all["k2"] != "v2" || all["k3"] != "v3" {
		t.Fatalf("end state unexpected: %#v", all)
	}
}

// TestSettingsRepo_GetEmptyKeyRejected guards against accidental "" key
// inserts which would silently squat the PK.
func TestSettingsRepo_GetEmptyKeyRejected(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := storage.NewSettingsRepo(db, time.Now)
	if _, err := repo.Get(context.Background(), ""); err == nil {
		t.Fatalf("expected error for empty key")
	}
	if err := repo.Set(context.Background(), "", "x"); err == nil {
		t.Fatalf("expected error for empty key Set")
	}
}

// TestSettingsRepo_SensitiveKeysList ensures the published deny-list stays
// non-empty so a regression doesn't accidentally start leaking smtp_password
// to the GET response.
func TestSettingsRepo_SensitiveKeysList(t *testing.T) {
	t.Parallel()
	if _, ok := storage.SensitiveSettingKeys[storage.SettingSMTPPassword]; !ok {
		t.Fatalf("smtp_password missing from SensitiveSettingKeys")
	}
	if _, ok := storage.SensitiveSettingKeys[storage.SettingTelegramBotToken]; !ok {
		t.Fatalf("telegram_bot_token missing from SensitiveSettingKeys")
	}
}
