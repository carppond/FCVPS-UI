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

	// Rotate now requires silent mode to be enabled first.
	if _, err := sm.Enable(context.Background()); err != nil {
		t.Fatalf("Enable: %v", err)
	}
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
	if _, err := sm.Enable(context.Background()); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	newPrefix, err := sm.Rotate(context.Background())
	if err != nil {
		t.Fatalf("Rotate should still succeed: %v", err)
	}
	persisted, _ := repo.Get(context.Background(), storage.SettingSilentModePrefix)
	if persisted != newPrefix {
		t.Fatalf("Rotate did not persist when revoker failed")
	}
}

// TestSilentMode_EnsureInitialDoesNotGeneratePrefix asserts the new opt-in
// behaviour: EnsureInitial seeds the silent_mode_enabled=false row but does
// NOT mint a prefix. This is the key fix that lets dev restarts preserve the
// "no entry URL needed" state without regenerating a hidden one on every boot.
func TestSilentMode_EnsureInitialDoesNotGeneratePrefix(t *testing.T) {
	t.Parallel()
	repo := newSettingsRepo(t)
	sm, _ := ops.NewSilentMode(ops.SilentModeConfig{Repo: repo})
	prefix, err := sm.EnsureInitial(context.Background())
	if err != nil {
		t.Fatalf("EnsureInitial: %v", err)
	}
	if prefix != "" {
		t.Fatalf("EnsureInitial generated a prefix when it should not: %q", prefix)
	}
	// Calling again is idempotent (no error, still empty).
	again, err := sm.EnsureInitial(context.Background())
	if err != nil {
		t.Fatalf("EnsureInitial 2: %v", err)
	}
	if again != "" {
		t.Fatalf("EnsureInitial generated a prefix on second call: %q", again)
	}
	// Verify the enabled row defaulted to false.
	on, err := sm.IsEnabled(context.Background())
	if err != nil {
		t.Fatalf("IsEnabled: %v", err)
	}
	if on {
		t.Fatalf("EnsureInitial should leave silent mode disabled, got enabled=true")
	}
}

// TestSilentMode_EnableThenDisableRoundTrip exercises the new opt-in flow:
// Enable mints a prefix + flips the flag; Disable flips the flag back BUT
// retains the prefix; re-Enable reuses the same prefix (so saved URLs keep
// working across enable/disable cycles).
func TestSilentMode_EnableThenDisableRoundTrip(t *testing.T) {
	t.Parallel()
	repo := newSettingsRepo(t)
	var appliedPrefix string
	var enabledFlips []bool
	sm, _ := ops.NewSilentMode(ops.SilentModeConfig{
		Repo:           repo,
		Applier:        func(p string) { appliedPrefix = p },
		EnabledApplier: func(b bool) { enabledFlips = append(enabledFlips, b) },
	})

	// Start disabled.
	if on, _ := sm.IsEnabled(context.Background()); on {
		t.Fatalf("initial state should be disabled")
	}

	// Enable — prefix is generated, flag flips to true.
	first, err := sm.Enable(context.Background())
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !sm.Validate(first) {
		t.Fatalf("Enable returned invalid prefix %q", first)
	}
	if appliedPrefix != first {
		t.Fatalf("PrefixApplier got %q want %q", appliedPrefix, first)
	}
	if len(enabledFlips) != 1 || !enabledFlips[0] {
		t.Fatalf("EnabledApplier should have been called with true, got %v", enabledFlips)
	}
	on, _ := sm.IsEnabled(context.Background())
	if !on {
		t.Fatalf("IsEnabled should be true after Enable")
	}

	// Disable — flag flips back; prefix is preserved.
	if err := sm.Disable(context.Background()); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if len(enabledFlips) != 2 || enabledFlips[1] {
		t.Fatalf("EnabledApplier should have been called with false, got %v", enabledFlips)
	}
	on, _ = sm.IsEnabled(context.Background())
	if on {
		t.Fatalf("IsEnabled should be false after Disable")
	}
	persisted, _ := sm.Current(context.Background())
	if persisted != first {
		t.Fatalf("prefix lost on Disable: got %q want %q", persisted, first)
	}

	// Re-Enable — same prefix is reused.
	second, err := sm.Enable(context.Background())
	if err != nil {
		t.Fatalf("Re-Enable: %v", err)
	}
	if second != first {
		t.Fatalf("Re-Enable produced different prefix: %q vs %q", second, first)
	}
}

// TestSilentMode_RotateFailsWhenDisabled guards the invariant that rotation
// is only meaningful when silent mode is currently active. Asking to rotate
// while disabled is almost certainly an operator mistake (the new prefix
// would not be enforced anyway).
func TestSilentMode_RotateFailsWhenDisabled(t *testing.T) {
	t.Parallel()
	sm, _ := ops.NewSilentMode(ops.SilentModeConfig{Repo: newSettingsRepo(t)})
	if _, err := sm.Rotate(context.Background()); err == nil {
		t.Fatalf("Rotate should fail when silent mode is disabled")
	}
}

// TestSilentMode_NewRequiresRepo guards the constructor's nil-repo guard.
func TestSilentMode_NewRequiresRepo(t *testing.T) {
	t.Parallel()
	if _, err := ops.NewSilentMode(ops.SilentModeConfig{}); err == nil {
		t.Fatalf("expected error when Repo is nil")
	}
}
