package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"time"

	"shiguang-vps/migrations"
)

// migrationsFS aliases the embedded FS exported by the migrations package.
// It is a variable (not a constant) so tests can substitute a custom FS.
var migrationsFS fs.FS = migrations.FS

// MigrationFile is a single migration as resolved from the embedded FS.
type MigrationFile struct {
	Name string
	Body []byte
}

// ErrNoMigrations is returned by RunMigrations when no SQL files were found.
// This is treated as a configuration bug (the embed directive missed the
// files) rather than a runtime condition.
var ErrNoMigrations = errors.New("storage: no migrations found in embedded FS")

// RunMigrations applies every embedded SQL file whose name is not already in
// schema_migrations. It is safe to invoke repeatedly; already-applied files
// are skipped.
func (db *DB) RunMigrations(ctx context.Context) error {
	if db == nil || db.Write == nil {
		return ErrDBClosed
	}
	files, err := loadMigrations()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return ErrNoMigrations
	}
	if err := ensureMigrationsTable(ctx, db); err != nil {
		return err
	}
	applied, err := loadAppliedSet(ctx, db)
	if err != nil {
		return err
	}
	for _, f := range files {
		if _, ok := applied[f.Name]; ok {
			continue
		}
		if err := applyMigration(ctx, db, f); err != nil {
			return fmt.Errorf("apply migration %s: %w", f.Name, err)
		}
	}
	return nil
}

// loadMigrations returns the embedded *.sql files sorted by file name.
func loadMigrations() ([]MigrationFile, error) {
	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		if path.Ext(ent.Name()) != ".sql" {
			continue
		}
		names = append(names, ent.Name())
	}
	sort.Strings(names)
	out := make([]MigrationFile, 0, len(names))
	for _, name := range names {
		body, err := fs.ReadFile(migrationsFS, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		out = append(out, MigrationFile{Name: name, Body: body})
	}
	return out, nil
}

// ensureMigrationsTable creates schema_migrations if it does not exist. The
// table is also created by 0001_initial.sql, but we provision it eagerly so
// the bookkeeping query below works on a fresh database.
func ensureMigrationsTable(ctx context.Context, db *DB) error {
	const ddl = `CREATE TABLE IF NOT EXISTS schema_migrations (
        filename   TEXT PRIMARY KEY,
        applied_at INTEGER NOT NULL
    )`
	if _, err := db.Write.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

// loadAppliedSet reads every recorded migration name into a lookup set.
func loadAppliedSet(ctx context.Context, db *DB) (map[string]struct{}, error) {
	rows, err := db.Write.QueryContext(ctx, "SELECT filename FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		out[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_migrations: %w", err)
	}
	return out, nil
}

// applyMigration executes a migration body in a single transaction.
// modernc.org/sqlite's driver accepts multiple statements per ExecContext call.
func applyMigration(ctx context.Context, db *DB, f MigrationFile) error {
	tx, err := db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, string(f.Body)); err != nil {
		return fmt.Errorf("exec body: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT OR REPLACE INTO schema_migrations(filename, applied_at) VALUES (?, ?)",
		f.Name, time.Now().UnixMilli(),
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
