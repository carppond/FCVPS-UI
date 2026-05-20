package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SessionRecord is the storage-layer projection of a sessions row.
type SessionRecord struct {
	ID         string
	UserID     string
	TokenHash  string
	Pending2FA bool
	ExpiresAt  int64
	LastUsedAt int64
	IP         string
	UserAgent  string
	CreatedAt  int64
}

// ErrSessionNotFound is returned when a token hash does not resolve to a row.
var ErrSessionNotFound = errors.New("storage: session not found")

// SessionRepo encapsulates SQL access to the sessions table.
type SessionRepo struct {
	db  *DB
	now func() time.Time
}

// NewSessionRepo wires a repo to db. When now is nil time.Now is used.
func NewSessionRepo(db *DB, now func() time.Time) *SessionRepo {
	if now == nil {
		now = time.Now
	}
	return &SessionRepo{db: db, now: now}
}

// Create inserts a sessions row. Caller supplies all fields including the
// pre-computed TokenHash (sha256 of the plaintext token).
func (r *SessionRepo) Create(ctx context.Context, rec SessionRecord) error {
	if rec.ID == "" || rec.UserID == "" || rec.TokenHash == "" {
		return fmt.Errorf("session create: required field missing")
	}
	now := r.now().UnixMilli()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	if rec.LastUsedAt == 0 {
		rec.LastUsedAt = now
	}
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO sessions(id, user_id, token_hash, pending_2fa, expires_at,
		                     last_used_at, ip, user_agent, created_at)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.UserID, rec.TokenHash, boolToInt(rec.Pending2FA),
		rec.ExpiresAt, rec.LastUsedAt,
		nullableString(rec.IP), nullableString(rec.UserAgent),
		rec.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// GetByTokenHash resolves a session by its hashed token. Returns
// ErrSessionNotFound when no row matches OR when the matched row is expired.
func (r *SessionRepo) GetByTokenHash(ctx context.Context, hash string) (*SessionRecord, error) {
	row := r.db.Read.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, pending_2fa, expires_at, last_used_at,
		       COALESCE(ip,''), COALESCE(user_agent,''), created_at
		FROM sessions WHERE token_hash = ?`, hash)
	var rec SessionRecord
	var pending int
	if err := row.Scan(&rec.ID, &rec.UserID, &rec.TokenHash, &pending,
		&rec.ExpiresAt, &rec.LastUsedAt, &rec.IP, &rec.UserAgent, &rec.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("scan session: %w", err)
	}
	rec.Pending2FA = pending == 1
	if rec.ExpiresAt <= r.now().UnixMilli() {
		return nil, ErrSessionNotFound
	}
	return &rec, nil
}

// Touch refreshes last_used_at (and optionally extends expires_at). Called by
// TokenStore.Lookup to implement sliding expiry.
func (r *SessionRepo) Touch(ctx context.Context, hash string, newLastUsed, newExpiresAt int64) error {
	if newExpiresAt > 0 {
		_, err := r.db.Write.ExecContext(ctx,
			"UPDATE sessions SET last_used_at = ?, expires_at = ? WHERE token_hash = ?",
			newLastUsed, newExpiresAt, hash)
		if err != nil {
			return fmt.Errorf("touch session: %w", err)
		}
		return nil
	}
	_, err := r.db.Write.ExecContext(ctx,
		"UPDATE sessions SET last_used_at = ? WHERE token_hash = ?",
		newLastUsed, hash)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// Delete removes one session row by its hashed token.
func (r *SessionRepo) Delete(ctx context.Context, hash string) error {
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM sessions WHERE token_hash = ?", hash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// DeleteAllForUser revokes every session for the given user. Used on password
// change / disable-2fa / admin revoke-sessions.
func (r *SessionRepo) DeleteAllForUser(ctx context.Context, userID string) error {
	_, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM sessions WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("delete sessions for user: %w", err)
	}
	return nil
}

// ListByUser returns every active session for userID, newest first.
func (r *SessionRepo) ListByUser(ctx context.Context, userID string) ([]SessionRecord, error) {
	rows, err := r.db.Read.QueryContext(ctx, `
		SELECT id, user_id, token_hash, pending_2fa, expires_at, last_used_at,
		       COALESCE(ip,''), COALESCE(user_agent,''), created_at
		FROM sessions WHERE user_id = ? AND expires_at > ?
		ORDER BY last_used_at DESC`, userID, r.now().UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	out := make([]SessionRecord, 0, 4)
	for rows.Next() {
		var rec SessionRecord
		var pending int
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.TokenHash, &pending,
			&rec.ExpiresAt, &rec.LastUsedAt, &rec.IP, &rec.UserAgent, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		rec.Pending2FA = pending == 1
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}
	return out, nil
}

// CleanupExpired deletes every sessions row whose expires_at is in the past.
// Returns the number of rows removed.
func (r *SessionRepo) CleanupExpired(ctx context.Context) (int64, error) {
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM sessions WHERE expires_at <= ?", r.now().UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("cleanup expired sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
