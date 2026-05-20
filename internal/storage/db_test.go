package storage_test

import (
	"context"
	"path/filepath"
	"testing"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
)

// testConfig builds a fresh DatabaseConfig pointing at t.TempDir().
func testConfig(t *testing.T) config.DatabaseConfig {
	t.Helper()
	return config.DatabaseConfig{
		DataDir:       t.TempDir(),
		Filename:      "test.db",
		BusyTimeoutMs: config.DefaultDBBusyTimeoutMs,
		MaxOpenWrite:  config.DefaultDBMaxOpenWrite,
		MaxOpenRead:   config.DefaultDBMaxOpenRead,
	}
}

func TestOpenSucceedsAndWALEnabled(t *testing.T) {
	cfg := testConfig(t)
	db, err := storage.Open(cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var journal string
	if err := db.Read.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journal != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journal)
	}

	var fk int
	if err := db.Read.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d, want 1", fk)
	}

	var busy int
	if err := db.Read.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busy != cfg.BusyTimeoutMs {
		t.Fatalf("busy_timeout = %d, want %d", busy, cfg.BusyTimeoutMs)
	}
}

func TestOpenCreatesDataDir(t *testing.T) {
	cfg := testConfig(t)
	cfg.DataDir = filepath.Join(cfg.DataDir, "nested", "dir")
	db, err := storage.Open(cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if got := db.Path(); got == "" {
		t.Fatal("expected non-empty db path")
	}
}

func TestOpenSimpleSelect(t *testing.T) {
	db, err := storage.Open(testConfig(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var one int
	if err := db.Read.QueryRowContext(context.Background(), "SELECT 1").Scan(&one); err != nil {
		t.Fatalf("select 1: %v", err)
	}
	if one != 1 {
		t.Fatalf("select 1 = %d, want 1", one)
	}
}

func TestCloseIdempotent(t *testing.T) {
	db, err := storage.Open(testConfig(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestEnsureColumnRejectsBadIdent(t *testing.T) {
	db, err := storage.Open(testConfig(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.EnsureColumn(context.Background(), "users; DROP TABLE users", "x", "TEXT"); err == nil {
		t.Fatal("expected error for invalid identifier")
	}
}

func TestEnsureColumnAddsThenIdempotent(t *testing.T) {
	db, err := storage.Open(testConfig(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if _, err := db.Write.ExecContext(ctx, "CREATE TABLE t1(id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.EnsureColumn(ctx, "t1", "extra", "TEXT NOT NULL DEFAULT ''"); err != nil {
		t.Fatalf("ensure add: %v", err)
	}
	if err := db.EnsureColumn(ctx, "t1", "extra", "TEXT NOT NULL DEFAULT ''"); err != nil {
		t.Fatalf("ensure idempotent: %v", err)
	}
	// Verify the column exists by inserting and reading back.
	if _, err := db.Write.ExecContext(ctx, "INSERT INTO t1(id, extra) VALUES (1, 'ok')"); err != nil {
		t.Fatalf("insert with extra: %v", err)
	}
	var val string
	if err := db.Read.QueryRowContext(ctx, "SELECT extra FROM t1 WHERE id = 1").Scan(&val); err != nil {
		t.Fatalf("read extra: %v", err)
	}
	if val != "ok" {
		t.Fatalf("extra = %q, want ok", val)
	}
}
