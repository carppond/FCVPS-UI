package shortlink_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/shortlink"
	"shiguang-vps/internal/storage"
)

// newTestService spins up an in-memory SQLite + ShortLinkRepo + Service.
// Tests rely on RunMigrations to create short_links and users.
func newTestService(t *testing.T) (*shortlink.Service, *storage.ShortLinkRepo, *storage.DB) {
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
	// Seed two users so the FK on short_links.user_id is happy.
	userRepo := storage.NewUserRepo(db, time.Now)
	for _, id := range []string{"u-alice", "u-bob"} {
		_, _ = userRepo.Create(context.Background(), storage.UserRecord{
			ID: id, Username: id, PasswordHash: "h", Role: "user", IsActive: true,
		})
	}
	repo := storage.NewShortLinkRepo(db, time.Now)
	svc := shortlink.New(repo, nil, time.Now)
	return svc, repo, db
}

func TestServiceGenerateThenResolve(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	rec, err := svc.Generate(ctx, "u-alice", "https://example.com/a", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if rec.FileCode == "" || rec.UserCode == "" {
		t.Fatalf("expected non-empty codes, got %+v", rec)
	}
	if rec.ExpiresAt != 0 {
		t.Fatalf("expected permanent link, expires_at=%d", rec.ExpiresAt)
	}

	combined := rec.FileCode + rec.UserCode
	target, err := svc.Resolve(ctx, combined)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", combined, err)
	}
	if target != "https://example.com/a" {
		t.Fatalf("Resolve returned %q", target)
	}
}

func TestServiceGenerateMonotonic(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	a, err := svc.Generate(ctx, "u-alice", "https://a", nil)
	if err != nil {
		t.Fatalf("Generate#1: %v", err)
	}
	b, err := svc.Generate(ctx, "u-alice", "https://b", nil)
	if err != nil {
		t.Fatalf("Generate#2: %v", err)
	}
	if a.FileCode == b.FileCode {
		t.Fatalf("expected distinct file codes, got both %q", a.FileCode)
	}
	if a.UserCode == b.UserCode {
		t.Fatalf("expected distinct user codes, got both %q", a.UserCode)
	}
	// Both file_code and user_code should advance strictly.
	aFC, _ := shortlink.DecodeBase62(a.FileCode)
	bFC, _ := shortlink.DecodeBase62(b.FileCode)
	if bFC <= aFC {
		t.Fatalf("file_code not monotonic: %q -> %q", a.FileCode, b.FileCode)
	}
	aUC, _ := shortlink.DecodeBase62(a.UserCode)
	bUC, _ := shortlink.DecodeBase62(b.UserCode)
	if bUC <= aUC {
		t.Fatalf("user_code not monotonic: %q -> %q", a.UserCode, b.UserCode)
	}
}

func TestServiceUserCodesIsolated(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	a, err := svc.Generate(ctx, "u-alice", "https://a", nil)
	if err != nil {
		t.Fatalf("alice: %v", err)
	}
	b, err := svc.Generate(ctx, "u-bob", "https://b", nil)
	if err != nil {
		t.Fatalf("bob: %v", err)
	}
	// file_code shares the global counter so they MUST differ.
	if a.FileCode == b.FileCode {
		t.Fatalf("expected global file_code to be unique across users")
	}
	// user_code is per-user; both fresh users should both start at "1".
	if a.UserCode != b.UserCode {
		t.Fatalf("expected per-user user_code to start identically; got %q / %q",
			a.UserCode, b.UserCode)
	}
}

func TestServiceResolveExpired(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	past := time.Now().Add(-time.Hour)
	rec, err := svc.Generate(ctx, "u-alice", "https://x", &past)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	_, err = svc.Resolve(ctx, rec.FileCode+rec.UserCode)
	if !errors.Is(err, storage.ErrShortLinkNotFound) {
		t.Fatalf("expected ErrShortLinkNotFound, got %v", err)
	}
}

func TestServiceResolveUnknown(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.Resolve(context.Background(), "zzZZ99")
	if !errors.Is(err, storage.ErrShortLinkNotFound) {
		t.Fatalf("expected ErrShortLinkNotFound, got %v", err)
	}
}

func TestBase62Roundtrip(t *testing.T) {
	cases := []uint64{0, 1, 61, 62, 63, 3843, 100000, 9999999}
	for _, n := range cases {
		s := shortlink.EncodeBase62(n)
		back, err := shortlink.DecodeBase62(s)
		if err != nil {
			t.Fatalf("decode %q: %v", s, err)
		}
		if back != n {
			t.Fatalf("roundtrip %d → %q → %d", n, s, back)
		}
	}
}

func TestServiceDelete(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	rec, err := svc.Generate(ctx, "u-alice", "https://to-delete", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := svc.Delete(ctx, rec.FileCode, rec.UserCode, "u-alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := svc.Delete(ctx, rec.FileCode, rec.UserCode, "u-alice"); !errors.Is(err, storage.ErrShortLinkNotFound) {
		t.Fatalf("second delete: expected ErrShortLinkNotFound, got %v", err)
	}
}

func TestServiceDeleteRejectsWrongOwner(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	rec, err := svc.Generate(ctx, "u-alice", "https://owned", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := svc.Delete(ctx, rec.FileCode, rec.UserCode, "u-bob"); !errors.Is(err, storage.ErrShortLinkNotFound) {
		t.Fatalf("expected ErrShortLinkNotFound for foreign delete, got %v", err)
	}
}
