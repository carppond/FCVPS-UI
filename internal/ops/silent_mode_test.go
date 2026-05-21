package ops_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/ops"
	"shiguang-vps/internal/storage"
)

// newSettingsRepo opens a fresh DB inside a temp dir and returns a settings
// repo bound to it. The DB closes when the test ends via t.Cleanup.
func newSettingsRepo(t *testing.T) *storage.SettingsRepo {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(config.DatabaseConfig{
		DataDir: dir, Filename: "test.db", BusyTimeoutMs: 5000,
		MaxOpenWrite: 1, MaxOpenRead: 2,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return storage.NewSettingsRepo(db, time.Now)
}

// recordingRevoker captures RevokeAll calls so tests can assert the rotation
// flow purged sessions exactly once.
type recordingRevoker struct {
	mu    sync.Mutex
	calls int
	count int64
	err   error
}

func (r *recordingRevoker) RevokeAll(ctx context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.count, r.err
}

// TestSilentMode_GenerateProducesValidPrefix asserts the generator's output
// satisfies the regex used by the middleware.
func TestSilentMode_GenerateProducesValidPrefix(t *testing.T) {
	t.Parallel()
	sm, err := ops.NewSilentMode(ops.SilentModeConfig{Repo: newSettingsRepo(t)})
	if err != nil {
		t.Fatalf("NewSilentMode: %v", err)
	}
	for i := 0; i < 32; i++ {
		got := sm.Generate()
		if len(got) != 32 {
			t.Fatalf("prefix length = %d", len(got))
		}
		if !sm.Validate(got) {
			t.Fatalf("Validate(%q) = false", got)
		}
	}
}

// TestSilentMode_ValidateRejectsInvalidShapes guards against regressions in
// the regex (e.g. accidentally allowing uppercase or trailing whitespace).
func TestSilentMode_ValidateRejectsInvalidShapes(t *testing.T) {
	t.Parallel()
	sm, _ := ops.NewSilentMode(ops.SilentModeConfig{Repo: newSettingsRepo(t)})
	cases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"too_short", "abc"},
		{"too_long", strings.Repeat("a", 33)},
		{"uppercase", strings.ToUpper(sm.Generate())},
		{"non_hex", strings.Repeat("g", 32)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if sm.Validate(tc.input) {
				t.Fatalf("expected invalid, got valid for %q", tc.input)
			}
		})
	}
}

// TestSilentMode_RotatePersistsAndAppliesAndRevokes is the integration check:
// after Rotate the DB row matches the returned prefix, the applier callback
// was fired with the same value, and the revoker was invoked once.
func TestSilentMode_RotatePersistsAndAppliesAndRevokes(t *testing.T) {
	t.Parallel()
	repo := newSettingsRepo(t)
	var applied string
	rev := &recordingRevoker{count: 7}
	sm, _ := ops.NewSilentMode(ops.SilentModeConfig{
		Repo:    repo,
		Applier: func(p string) { applied = p },
		Revoker: rev,
	})

	newPrefix, err := sm.Rotate(context.Background())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if !sm.Validate(newPrefix) {
		t.Fatalf("Rotate returned invalid prefix %q", newPrefix)
	}
	if applied != newPrefix {
		t.Fatalf("applier got %q want %q", applied, newPrefix)
	}
	if rev.calls != 1 {
		t.Fatalf("revoker calls = %d, want 1", rev.calls)
	}
	persisted, err := repo.Get(context.Background(), storage.SettingSilentModePrefix)
	if err != nil {
		t.Fatalf("Get persisted prefix: %v", err)
	}
	if persisted != newPrefix {
		t.Fatalf("DB prefix %q != rotated %q", persisted, newPrefix)
	}
}

// TestSilentMode_RotateContinuesWhenRevokerFails covers the "rotation must
// not get stuck if session purge errors" branch — the prefix is already live
// at that point so propagating the error would leave the DB and middleware in
// an inconsistent state.
func TestSilentMode_RotateContinuesWhenRevokerFails(t *testing.T) {
	t.Parallel()
	repo := newSettingsRepo(t)
	rev := &recordingRevoker{err: context.Canceled}
	sm, _ := ops.NewSilentMode(ops.SilentModeConfig{
		Repo:    repo,
		Revoker: rev,
	})
	newPrefix, err := sm.Rotate(context.Background())
	if err != nil {
		t.Fatalf("Rotate should still succeed: %v", err)
	}
	persisted, _ := repo.Get(context.Background(), storage.SettingSilentModePrefix)
	if persisted != newPrefix {
		t.Fatalf("Rotate did not persist when revoker failed")
	}
}

// TestSilentMode_EnsureInitialIdempotent verifies the boot-time helper only
// generates a prefix once.
func TestSilentMode_EnsureInitialIdempotent(t *testing.T) {
	t.Parallel()
	sm, _ := ops.NewSilentMode(ops.SilentModeConfig{Repo: newSettingsRepo(t)})
	first, err := sm.EnsureInitial(context.Background())
	if err != nil {
		t.Fatalf("EnsureInitial 1: %v", err)
	}
	second, err := sm.EnsureInitial(context.Background())
	if err != nil {
		t.Fatalf("EnsureInitial 2: %v", err)
	}
	if first != second {
		t.Fatalf("EnsureInitial not idempotent: %q vs %q", first, second)
	}
}

// TestSilentMode_NewRequiresRepo guards the constructor's nil-repo guard.
func TestSilentMode_NewRequiresRepo(t *testing.T) {
	t.Parallel()
	if _, err := ops.NewSilentMode(ops.SilentModeConfig{}); err == nil {
		t.Fatalf("expected error when Repo is nil")
	}
}
