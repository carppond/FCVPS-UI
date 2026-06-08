// Package storage owns the SQLite connection lifecycle and the repo layer.
//
// Two physical connection pools are used (per architecture §4.1):
//
//   - write: SetMaxOpenConns(1) — SQLite serialises writers anyway, so a
//     single connection lets us push transactions through with no contention.
//   - read:  SetMaxOpenConns(8)  — concurrent read replicas via WAL.
//
// The DSN forces WAL mode, busy_timeout=5000ms, foreign_keys=on and
// synchronous=NORMAL (see architecture §6.1).
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	// modernc.org/sqlite is a pure-Go driver; registers itself as "sqlite".
	_ "modernc.org/sqlite"

	"shiguang-vps/internal/config"
)

// DB owns the read and write connection pools. Always close via Close().
type DB struct {
	cfg config.DatabaseConfig

	// Write is the writer-only pool (max open = 1).
	Write *sql.DB
	// Read is the reader pool (max open = cfg.MaxOpenRead).
	Read *sql.DB

	closeOnce sync.Once
}

// Errors surfaced by this package.
var (
	// ErrDBClosed is returned when an operation is attempted on a closed DB.
	ErrDBClosed = errors.New("storage: db closed")
)

// Open creates the data directory if missing, opens both pools and returns a
// ready-to-use *DB. The caller is responsible for invoking Close on shutdown.
func Open(cfg config.DatabaseConfig) (*DB, error) {
	if cfg.Filename == "" {
		return nil, fmt.Errorf("storage.Open: empty filename")
	}
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("storage.Open: empty data dir")
	}
	if cfg.MaxOpenRead <= 0 {
		cfg.MaxOpenRead = config.DefaultDBMaxOpenRead
	}
	if cfg.MaxOpenWrite <= 0 {
		cfg.MaxOpenWrite = config.DefaultDBMaxOpenWrite
	}
	if cfg.BusyTimeoutMs <= 0 {
		cfg.BusyTimeoutMs = config.DefaultDBBusyTimeoutMs
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dsn := buildDSN(filepath.Join(cfg.DataDir, cfg.Filename), cfg.BusyTimeoutMs)

	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open writer pool: %w", err)
	}
	writer.SetMaxOpenConns(cfg.MaxOpenWrite)
	writer.SetMaxIdleConns(cfg.MaxOpenWrite)

	reader, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("open reader pool: %w", err)
	}
	reader.SetMaxOpenConns(cfg.MaxOpenRead)
	reader.SetMaxIdleConns(cfg.MaxOpenRead)

	if err := verifyPragmas(context.Background(), writer); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, fmt.Errorf("verify pragmas on writer: %w", err)
	}
	if err := verifyPragmas(context.Background(), reader); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, fmt.Errorf("verify pragmas on reader: %w", err)
	}

	return &DB{cfg: cfg, Write: writer, Read: reader}, nil
}

// Close releases both pools. It additionally runs `PRAGMA wal_checkpoint(TRUNCATE)`
// so the WAL/SHM sidecar files are emptied — graceful shutdown leaves a tidy
// database file behind. Safe to call multiple times.
func (db *DB) Close() error {
	var closeErr error
	db.closeOnce.Do(func() {
		if db.Write != nil {
			// Best-effort WAL checkpoint; ignore error so we still close the
			// underlying connection.
			ctx, cancel := context.WithCancel(context.Background())
			_, _ = db.Write.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
			cancel()
		}
		if db.Read != nil {
			if err := db.Read.Close(); err != nil {
				closeErr = errors.Join(closeErr, fmt.Errorf("close reader: %w", err))
			}
		}
		if db.Write != nil {
			if err := db.Write.Close(); err != nil {
				closeErr = errors.Join(closeErr, fmt.Errorf("close writer: %w", err))
			}
		}
	})
	return closeErr
}

// Path returns the resolved database file path.
func (db *DB) Path() string {
	return filepath.Join(db.cfg.DataDir, db.cfg.Filename)
}

// buildDSN composes the modernc sqlite DSN with project-mandated pragmas.
// modernc.org/sqlite supports per-connection PRAGMA via `_pragma=` repeated
// query parameters.
func buildDSN(path string, busyTimeoutMs int) string {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMs))
	q.Add("_pragma", "foreign_keys(on)")
	q.Add("_pragma", "synchronous(NORMAL)")
	return "file:" + path + "?" + q.Encode()
}

// verifyPragmas runs read-only PRAGMA queries to confirm the DSN was honoured.
// Returns an error if WAL is not active or foreign keys are disabled.
func verifyPragmas(ctx context.Context, db *sql.DB) error {
	var journal string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil {
		return fmt.Errorf("read journal_mode: %w", err)
	}
	if journal != "wal" {
		return fmt.Errorf("expected journal_mode=wal, got %q", journal)
	}
	var fk int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		return fmt.Errorf("read foreign_keys: %w", err)
	}
	if fk != 1 {
		return fmt.Errorf("expected foreign_keys=on, got %d", fk)
	}
	return nil
}

// EnsureColumn adds `col` (with the supplied DDL fragment, e.g. "TEXT NOT NULL DEFAULT ''")
// to table when the column is absent. Used for incremental schema patches
// outside of the explicit migration files (see migrations/README.md).
//
// Safe to call repeatedly; subsequent invocations after the column exists
// are no-ops. ALTER TABLE in SQLite cannot drop or rename columns, so this
// helper only supports addition.
func (db *DB) EnsureColumn(ctx context.Context, table, col, ddl string) error {
	if db == nil || db.Write == nil {
		return ErrDBClosed
	}
	if table == "" || col == "" || ddl == "" {
		return fmt.Errorf("ensure column: table/col/ddl must all be set")
	}
	// PRAGMA table_info(table) — the table parameter is interpolated because
	// PRAGMA does not accept positional binding in SQLite. We validate the
	// identifier manually to defeat injection.
	if !isIdentifier(table) || !isIdentifier(col) {
		return fmt.Errorf("ensure column: invalid identifier")
	}
	rows, err := db.Write.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("table_info(%s): %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info: %w", err)
		}
		if name == col {
			return nil // already present
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table_info: %w", err)
	}
	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col, ddl)
	if _, err := db.Write.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("add column: %w", err)
	}
	return nil
}

// isIdentifier returns true for ASCII alphanumeric + underscore strings, which
// is the safe-subset we allow for dynamically interpolated table/column names.
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
