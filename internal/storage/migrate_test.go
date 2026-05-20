package storage_test

import (
	"context"
	"testing"

	"shiguang-vps/internal/storage"
)

// expectedTables enumerates every table created by migrations/0001_initial.sql.
// `schema_migrations` is included because it is also a table in the DB.
var expectedTables = []string{
	"agent_records",
	"agents",
	"audit_logs",
	"custom_rules",
	"nodes",
	"notification_channels",
	"notification_events",
	"pipeline_bindings",
	"pipelines",
	"schema_migrations",
	"scripts",
	"sessions",
	"short_links",
	"subscriptions",
	"system_settings",
	"traffic_records",
	"users",
}

func TestRunMigrationsCreatesAllTables(t *testing.T) {
	db, err := storage.Open(testConfig(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	got := listTables(t, db)
	for _, name := range expectedTables {
		if _, ok := got[name]; !ok {
			t.Errorf("missing table %q after migration; got=%v", name, got)
		}
	}
}

func TestRunMigrationsIsIdempotent(t *testing.T) {
	db, err := storage.Open(testConfig(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first := listTables(t, db)
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second := listTables(t, db)
	if len(first) != len(second) {
		t.Fatalf("table count drifted: first=%d second=%d", len(first), len(second))
	}
	for name := range first {
		if _, ok := second[name]; !ok {
			t.Errorf("table %q lost after re-run", name)
		}
	}

	// Verify schema_migrations has exactly one row per applied file.
	var count int
	if err := db.Read.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count == 0 {
		t.Fatal("schema_migrations empty after RunMigrations")
	}
}

func TestForeignKeyCascadeUserDelete(t *testing.T) {
	db, err := storage.Open(testConfig(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Insert a user and a session, then delete the user; the session should
	// vanish (ON DELETE CASCADE).
	if _, err := db.Write.ExecContext(ctx,
		`INSERT INTO users(id, username, password_hash, role, created_at, updated_at)
		 VALUES ('u1', 'alice', 'x', 'admin', 1, 1)`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Write.ExecContext(ctx,
		`INSERT INTO sessions(id, user_id, token_hash, expires_at, last_used_at, created_at)
		 VALUES ('s1', 'u1', 'h', 99999999, 1, 1)`); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if _, err := db.Write.ExecContext(ctx, `DELETE FROM users WHERE id = 'u1'`); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	var sessions int
	if err := db.Read.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE user_id = 'u1'").Scan(&sessions); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("expected 0 sessions after cascade, got %d", sessions)
	}
}

// listTables returns the set of user/system tables in the DB. SQLite stores
// these in sqlite_master.
func listTables(t *testing.T, db *storage.DB) map[string]struct{} {
	t.Helper()
	rows, err := db.Read.QueryContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iter: %v", err)
	}
	return out
}
