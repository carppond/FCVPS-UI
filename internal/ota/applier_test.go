package ota_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/ota"
	"shiguang-vps/internal/storage"
)

// newTestDB spins up a temp SQLite database with the project schema applied so
// the WAL checkpoint helper has a real connection to act against.
func newTestDB(t *testing.T) *storage.DB {
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
	return db
}

// stageBinary writes a non-empty file at <bin>.new so the applier passes its
// stat / size checks. Returns the path.
func stageBinary(t *testing.T, binPath string, contents string) string {
	t.Helper()
	stage := binPath + ".new"
	if err := os.WriteFile(stage, []byte(contents), 0o600); err != nil {
		t.Fatalf("write stage: %v", err)
	}
	return stage
}

func TestApplier_Apply_SwapsBinaryAndChmod(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	binPath := filepath.Join(dir, "shiguang-vps")
	if err := os.WriteFile(binPath, []byte("OLD_BINARY"), 0o755); err != nil {
		t.Fatalf("seed binary: %v", err)
	}
	stage := stageBinary(t, binPath, "NEW_BINARY_BYTES")

	db := newTestDB(t)
	var triggered sync.WaitGroup
	triggered.Add(1)
	applier, err := ota.NewApplier(ota.ApplierConfig{
		DB:         db,
		BinaryPath: binPath,
		Shutdown:   func() { triggered.Done() },
	})
	if err != nil {
		t.Fatalf("new applier: %v", err)
	}
	if err := applier.Apply(context.Background(), stage); err != nil {
		t.Fatalf("apply: %v", err)
	}

	got, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read swapped binary: %v", err)
	}
	if string(got) != "NEW_BINARY_BYTES" {
		t.Fatalf("binary not swapped: %q", got)
	}
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("permission = %v, want 0755", info.Mode().Perm())
	}
	bak, err := os.ReadFile(binPath + ".bak")
	if err != nil {
		t.Fatalf("read bak: %v", err)
	}
	if string(bak) != "OLD_BINARY" {
		t.Fatalf("bak = %q", bak)
	}

	// Shutdown is invoked after a short delay; wait briefly.
	done := make(chan struct{})
	go func() { triggered.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("shutdown not triggered")
	}
}

func TestApplier_Apply_RejectsCrossDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	otherDir := t.TempDir()
	binPath := filepath.Join(dir, "shiguang-vps")
	if err := os.WriteFile(binPath, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stage := filepath.Join(otherDir, "shiguang-vps.new")
	if err := os.WriteFile(stage, []byte("NEW"), 0o600); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	db := newTestDB(t)
	applier, err := ota.NewApplier(ota.ApplierConfig{
		DB:         db,
		BinaryPath: binPath,
		Shutdown:   func() {},
	})
	if err != nil {
		t.Fatalf("new applier: %v", err)
	}
	if err := applier.Apply(context.Background(), stage); err == nil {
		t.Fatalf("expected cross-directory rejection")
	}
}

func TestApplier_Apply_EmptyNewBinaryFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	binPath := filepath.Join(dir, "shiguang-vps")
	if err := os.WriteFile(binPath, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("seed binary: %v", err)
	}
	// Stage is zero bytes; applier should refuse.
	stage := binPath + ".new"
	if err := os.WriteFile(stage, nil, 0o600); err != nil {
		t.Fatalf("seed empty: %v", err)
	}
	db := newTestDB(t)
	applier, err := ota.NewApplier(ota.ApplierConfig{
		DB:         db,
		BinaryPath: binPath,
		Shutdown:   func() {},
	})
	if err != nil {
		t.Fatalf("new applier: %v", err)
	}
	if err := applier.Apply(context.Background(), stage); err == nil {
		t.Fatalf("expected empty-binary rejection")
	}
	// Original binary must remain intact.
	got, _ := os.ReadFile(binPath)
	if string(got) != "OLD" {
		t.Fatalf("binary modified despite failure: %q", got)
	}
}

func TestApplier_StageTo_TruncatesStale(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	binPath := filepath.Join(dir, "shiguang-vps")
	stale := binPath + ".new"
	if err := os.WriteFile(stale, []byte("LEFTOVER"), 0o600); err != nil {
		t.Fatalf("seed stale: %v", err)
	}
	db := newTestDB(t)
	applier, err := ota.NewApplier(ota.ApplierConfig{
		DB:         db,
		BinaryPath: binPath,
		Shutdown:   func() {},
	})
	if err != nil {
		t.Fatalf("new applier: %v", err)
	}
	path, f, err := applier.StageTo()
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	defer f.Close()
	if path != stale {
		t.Fatalf("unexpected stage path %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("stale not truncated: size = %d", info.Size())
	}
}

func TestWalCheckpoint_RunsSuccessfully(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := ota.WalCheckpoint(context.Background(), db, nil); err != nil {
		t.Fatalf("wal checkpoint: %v", err)
	}
}
