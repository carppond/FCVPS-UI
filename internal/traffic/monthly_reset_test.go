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

func TestMonthlyResetNoOpWhenWrongDay(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	tRepo := storage.NewTrafficRepo(db, time.Now)
	sRepo := storage.NewSettingsRepo(db, time.Now)
	uRepo := storage.NewUserRepo(db, time.Now)
	// monthly_reset_day defaults to 1 — anchor on day 5.
	now := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	mr, err := traffic.NewMonthlyReset(traffic.MonthlyResetConfig{
		TrafficRepo: tRepo, SettingsRepo: sRepo, UserRepo: uRepo,
		Clock: util.NewFixedClock(now),
	})
	if err != nil {
		t.Fatalf("NewMonthlyReset: %v", err)
	}
	if err := mr.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	from, _ := time.Parse("2006-01-02", "2026-05-01")
	to, _ := time.Parse("2006-01-02", "2026-05-31")
	rows, _ := tRepo.ListByUser(context.Background(), "u1", from, to)
	if len(rows) != 0 {
		t.Fatalf("expected no seed rows on wrong day, got %d", len(rows))
	}
}

func TestMonthlyResetSeedsRowOnConfiguredDay(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedAgent(t, db, "u1", "a1")
	tRepo := storage.NewTrafficRepo(db, time.Now)
	sRepo := storage.NewSettingsRepo(db, time.Now)
	uRepo := storage.NewUserRepo(db, time.Now)
	ctx := context.Background()
	_ = sRepo.Set(ctx, traffic.SettingMonthlyResetDay, "1")
	_ = sRepo.Set(ctx, traffic.SettingMonthlyTrafficLimit, "1000")

	// April activity that the recap notification should observe.
	seedTrafficRow(t, tRepo, "2026-04-10", "u1", "a1", 500)

	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	mr, _ := traffic.NewMonthlyReset(traffic.MonthlyResetConfig{
		TrafficRepo: tRepo, SettingsRepo: sRepo, UserRepo: uRepo,
		Clock: util.NewFixedClock(now),
	})
	if err := mr.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// New month: 2026-05-01 anchor row exists with total_used=0.
	from, _ := time.Parse("2006-01-02", "2026-05-01")
	to, _ := time.Parse("2006-01-02", "2026-05-01")
	rows, _ := tRepo.ListByUser(ctx, "u1", from, to)
	if len(rows) != 1 {
		t.Fatalf("expected one anchor row on May 1, got %d", len(rows))
	}
	if rows[0].TotalUsed != 0 || rows[0].TotalLimit != 1000 {
		t.Fatalf("anchor row wrong: %+v", rows[0])
	}

	// State key was written so re-running the same day is a no-op.
	stateKey := "traffic_last_reset:u1"
	if v, _ := sRepo.Get(ctx, stateKey); v != "2026-05" {
		t.Fatalf("expected state key value 2026-05, got %q", v)
	}
}

func TestMonthlyResetCustomDayBoundary(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	tRepo := storage.NewTrafficRepo(db, time.Now)
	sRepo := storage.NewSettingsRepo(db, time.Now)
	uRepo := storage.NewUserRepo(db, time.Now)
	ctx := context.Background()
	_ = sRepo.Set(ctx, traffic.SettingMonthlyResetDay, strconv.Itoa(15))

	// Day 14 → no-op.
	noop := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)
	mr, _ := traffic.NewMonthlyReset(traffic.MonthlyResetConfig{
		TrafficRepo: tRepo, SettingsRepo: sRepo, UserRepo: uRepo,
		Clock: util.NewFixedClock(noop),
	})
	if err := mr.Run(ctx); err != nil {
		t.Fatalf("Run day 14: %v", err)
	}
	if v, _ := sRepo.Get(ctx, "traffic_last_reset:u1"); v != "" {
		t.Fatalf("expected no state on day 14, got %q", v)
	}

	// Day 15 → fires.
	fire := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	mr2, _ := traffic.NewMonthlyReset(traffic.MonthlyResetConfig{
		TrafficRepo: tRepo, SettingsRepo: sRepo, UserRepo: uRepo,
		Clock: util.NewFixedClock(fire),
	})
	if err := mr2.Run(ctx); err != nil {
		t.Fatalf("Run day 15: %v", err)
	}
	if v, _ := sRepo.Get(ctx, "traffic_last_reset:u1"); v != "2026-05" {
		t.Fatalf("expected state 2026-05 on day 15, got %q", v)
	}
}

func TestMonthlyResetIdempotentSameDay(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	tRepo := storage.NewTrafficRepo(db, time.Now)
	sRepo := storage.NewSettingsRepo(db, time.Now)
	uRepo := storage.NewUserRepo(db, time.Now)
	ctx := context.Background()
	_ = sRepo.Set(ctx, traffic.SettingMonthlyResetDay, "1")

	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	mr, _ := traffic.NewMonthlyReset(traffic.MonthlyResetConfig{
		TrafficRepo: tRepo, SettingsRepo: sRepo, UserRepo: uRepo,
		Clock: util.NewFixedClock(now),
	})
	if err := mr.Run(ctx); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// Second run on the same day must short-circuit.
	if err := mr.Run(ctx); err != nil {
		t.Fatalf("second run: %v", err)
	}
	// The anchor row remains a single row (idempotent UpsertDaily).
	from, _ := time.Parse("2006-01-02", "2026-05-01")
	to, _ := time.Parse("2006-01-02", "2026-05-01")
	rows, _ := tRepo.ListByUser(ctx, "u1", from, to)
	if len(rows) != 1 {
		t.Fatalf("expected one anchor row after two runs, got %d", len(rows))
	}
}
