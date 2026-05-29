package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"shiguang-vps/internal/util"
)

// ErrWidgetTokenNotFound is returned by Lookup when no row matches the hash.
var ErrWidgetTokenNotFound = errors.New("storage: widget token not found")

// WidgetTokenRepo manages the widget_tokens table: one scoped, read-only
// token per user, used by the mobile home-screen traffic widget. Only the
// sha256 hash is stored.
type WidgetTokenRepo struct {
	db  *DB
	now func() time.Time
}

// NewWidgetTokenRepo wires a repo to db. nil now falls back to time.Now.
func NewWidgetTokenRepo(db *DB, now func() time.Time) *WidgetTokenRepo {
	if now == nil {
		now = time.Now
	}
	return &WidgetTokenRepo{db: db, now: now}
}

// Replace upserts the single widget token for userID: it deletes any existing
// row for the user and inserts the new hash, all in one transaction. This
// makes mint and rotate identical operations.
func (r *WidgetTokenRepo) Replace(ctx context.Context, userID, tokenHash string) error {
	if userID == "" || tokenHash == "" {
		return fmt.Errorf("widget token replace: empty userID or hash")
	}
	tx, err := r.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM widget_tokens WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("delete existing: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO widget_tokens (id, user_id, token_hash, created_at)
		 VALUES (?, ?, ?, ?)`,
		util.UUIDv7(), userID, tokenHash, r.now().UnixMilli(),
	); err != nil {
		return fmt.Errorf("insert: %w", err)
	}
	return tx.Commit()
}

// Lookup resolves a token hash to its owning user ID, returning
// ErrWidgetTokenNotFound when absent.
func (r *WidgetTokenRepo) Lookup(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.db.Read.QueryRowContext(ctx,
		"SELECT user_id FROM widget_tokens WHERE token_hash = ?", tokenHash,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrWidgetTokenNotFound
		}
		return "", fmt.Errorf("widget token lookup: %w", err)
	}
	return userID, nil
}

// DeleteByUser removes the user's widget token (disable widget). Deleting a
// non-existent token is a no-op (no error).
func (r *WidgetTokenRepo) DeleteByUser(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("widget token delete: empty userID")
	}
	if _, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM widget_tokens WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("widget token delete: %w", err)
	}
	return nil
}

// ExistsForUser reports whether the user currently has a widget token (used by
// the settings toggle to show enabled/disabled state without exposing the
// token itself).
func (r *WidgetTokenRepo) ExistsForUser(ctx context.Context, userID string) (bool, error) {
	var one int
	err := r.db.Read.QueryRowContext(ctx,
		"SELECT 1 FROM widget_tokens WHERE user_id = ? LIMIT 1", userID,
	).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("widget token exists: %w", err)
	}
	return true, nil
}

// TouchLastUsed best-effort updates last_used_at. Errors are ignored by
// callers (it is observability, not correctness).
func (r *WidgetTokenRepo) TouchLastUsed(ctx context.Context, tokenHash string) error {
	_, err := r.db.Write.ExecContext(ctx,
		"UPDATE widget_tokens SET last_used_at = ? WHERE token_hash = ?",
		r.now().UnixMilli(), tokenHash)
	return err
}
