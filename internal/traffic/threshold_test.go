package traffic_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/traffic"
	"shiguang-vps/internal/util"
)

// seedTrafficRow inserts a single agent's daily roll-up so SumWindow returns
// a known value. Caller controls the date / used bytes.
func seedTrafficRow(t *testing.T, repo *storage.TrafficRepo, date, userID, agentID string, used int64) {
	t.Helper()
	if err := repo.UpsertDaily(context.Background(), storage.TrafficRecord{
		Date: date, UserID: userID, AgentID: agentID,
		TotalUsed: used, TotalIn: used / 2, TotalOut: used - used/2,
	}); err != nil {
		t.Fatalf("seed traffic row: %v", err)
	}
}

func TestThresholdNoLimitNoFire(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedAgent(t, db, "u1", "a1")
	tRepo := storage.NewTrafficRepo(db, time.Now)
	sRepo := storage.NewSettingsRepo(db, time.Now)
	// limit is unset → CheckAndAlert is a no-op.
	th, err := traffic.NewThreshold(traffic.ThresholdConfig{
		TrafficRepo: tRepo, SettingsRepo: sRepo,
		Clock: util.NewFixedClock(time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("NewThreshold: %v", err)
	}
	fired, err := th.CheckAndAlert(context.Background(), "u1")
	if err != nil {
		t.Fatalf("CheckAndAlert: %v", err)
	}
	if len(fired) != 0 {
		t.Fatalf("expected no fires without limit, got %v", fired)
	}
}

func TestThresholdCrossesLevelAndPersistsState(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedAgent(t, db, "u1", "a1")
	tRepo := storage.NewTrafficRepo(db, time.Now)
	sRepo := storage.NewSettingsRepo(db, time.Now)
	ctx := context.Background()

	// 1 GB limit, current usage 0.79 GB → no 80% trip yet.
	if err := sRepo.Set(ctx, traffic.SettingMonthlyTrafficLimit, strconv.FormatInt(1_000_000_000, 10)); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	seedTrafficRow(t, tRepo, "2026-05-10", "u1", "a1", 790_000_000)

	th, _ := traffic.NewThreshold(traffic.ThresholdConfig{
		TrafficRepo: tRepo, SettingsRepo: sRepo,
		Clock: util.NewFixedClock(time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)),
	})
	fired, _ := th.CheckAndAlert(ctx, "u1")
	if len(fired) != 0 {
		t.Fatalf("expected no fire at 79%%, got %v", fired)
	}

	// Bump to 0.85 GB → 80% trips.
	seedTrafficRow(t, tRepo, "2026-05-11", "u1", "a1", 60_000_000)
	fired, _ = th.CheckAndAlert(ctx, "u1")
	if len(fired) != 1 || fired[0] != 80 {
		t.Fatalf("expected [80] fire at 85%%, got %v", fired)
	}

	// Re-running the same check must NOT re-fire 80%.
	fired, _ = th.CheckAndAlert(ctx, "u1")
	if len(fired) != 0 {
		t.Fatalf("expected dedupe of already-fired level, got %v", fired)
	}

	// Bump past 90 + 100 → both new levels fire.
	seedTrafficRow(t, tRepo, "2026-05-12", "u1", "a1", 200_000_000)
	fired, _ = th.CheckAndAlert(ctx, "u1")
	if len(fired) != 2 || fired[0] != 90 || fired[1] != 100 {
		t.Fatalf("expected [90 100] at >=100%%, got %v", fired)
	}
}

func TestThresholdResetForUserClearsState(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedAgent(t, db, "u1", "a1")
	tRepo := storage.NewTrafficRepo(db, time.Now)
	sRepo := storage.NewSettingsRepo(db, time.Now)
	ctx := context.Background()

	_ = sRepo.Set(ctx, traffic.SettingMonthlyTrafficLimit, "1000")
	seedTrafficRow(t, tRepo, "2026-05-10", "u1", "a1", 900) // 90%

	now := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	th, _ := traffic.NewThreshold(traffic.ThresholdConfig{
		TrafficRepo: tRepo, SettingsRepo: sRepo,
		Clock: util.NewFixedClock(now),
	})
	fired, _ := th.CheckAndAlert(ctx, "u1")
	if len(fired) != 2 || fired[0] != 80 || fired[1] != 90 {
		t.Fatalf("expected [80 90] at 90%%, got %v", fired)
	}
	// Reset for May → next CheckAndAlert in May re-fires both.
	if err := th.ResetForUser(ctx, "u1", now); err != nil {
		t.Fatalf("ResetForUser: %v", err)
	}
	fired, _ = th.CheckAndAlert(ctx, "u1")
	if len(fired) != 2 {
		t.Fatalf("expected re-fire after reset, got %v", fired)
	}
}

func TestThresholdCrossMonthRefires(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedAgent(t, db, "u1", "a1")
	tRepo := storage.NewTrafficRepo(db, time.Now)
	sRepo := storage.NewSettingsRepo(db, time.Now)
	ctx := context.Background()

	_ = sRepo.Set(ctx, traffic.SettingMonthlyTrafficLimit, "1000")
	// May data → 900 used = 90%.
	seedTrafficRow(t, tRepo, "2026-05-10", "u1", "a1", 900)

	clock := util.NewFixedClock(time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC))
	th, _ := traffic.NewThreshold(traffic.ThresholdConfig{
		TrafficRepo: tRepo, SettingsRepo: sRepo, Clock: clock,
	})
	if fired, _ := th.CheckAndAlert(ctx, "u1"); len(fired) != 2 {
		t.Fatalf("May expected [80 90] fire, got %v", fired)
	}

	// Advance to June. The state key includes YYYY-MM so the new month has
	// an empty state slot; with June data the thresholds fire fresh.
	clock.Advance(20 * 24 * time.Hour) // 2026-06-04
	seedTrafficRow(t, tRepo, "2026-06-02", "u1", "a1", 900)
	fired, _ := th.CheckAndAlert(ctx, "u1")
	if len(fired) != 2 || fired[0] != 80 || fired[1] != 90 {
		t.Fatalf("June expected [80 90] fresh fire, got %v", fired)
	}
}
