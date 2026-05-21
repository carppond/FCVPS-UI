package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ShortLinkRecord is the storage projection of a short_links row.
// PrimaryKey is the composite (FileCode, UserCode); FileCode is globally
// monotonic (base62), UserCode is monotonic per UserID (base62).
type ShortLinkRecord struct {
	FileCode  string
	UserCode  string
	UserID    string
	TargetURL string
	ExpiresAt int64 // unix ms; 0 = permanent
	CreatedAt int64
}

// ErrShortLinkNotFound is the canonical not-found sentinel returned by
// Resolve / Delete when the (fileCode, userCode) tuple is absent or expired.
var ErrShortLinkNotFound = errors.New("storage: short link not found")

// ShortLinkRepo encapsulates SQL access to the short_links table.
type ShortLinkRepo struct {
	db  *DB
	now func() time.Time
}

// NewShortLinkRepo wires a repo to db. nil now defaults to time.Now.
func NewShortLinkRepo(db *DB, now func() time.Time) *ShortLinkRepo {
	if now == nil {
		now = time.Now
	}
	return &ShortLinkRepo{db: db, now: now}
}

// Create inserts a new short_links row. Both codes must be pre-computed by
// the caller (typically the shortlink.Service which serialises the two
// counters). Conflicts on the (file_code, user_code) PK surface as an error
// — the caller is expected to retry with a fresh tuple.
func (r *ShortLinkRepo) Create(ctx context.Context, rec ShortLinkRecord) (*ShortLinkRecord, error) {
	if rec.FileCode == "" || rec.UserCode == "" {
		return nil, fmt.Errorf("shortlink create: empty codes")
	}
	if rec.UserID == "" {
		return nil, fmt.Errorf("shortlink create: empty user_id")
	}
	if rec.TargetURL == "" {
		return nil, fmt.Errorf("shortlink create: empty target_url")
	}
	if rec.CreatedAt == 0 {
		rec.CreatedAt = r.now().UnixMilli()
	}
	var expiresArg any
	if rec.ExpiresAt > 0 {
		expiresArg = rec.ExpiresAt
	}
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO short_links(file_code, user_code, user_id, target_url,
			expires_at, created_at)
		VALUES(?,?,?,?,?,?)`,
		rec.FileCode, rec.UserCode, rec.UserID, rec.TargetURL,
		expiresArg, rec.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert short link: %w", err)
	}
	return &rec, nil
}

// Resolve looks up the (fileCode, userCode) tuple and returns the row.
// Expired rows return ErrShortLinkNotFound (callers don't need to track
// expiry themselves). Used by the public GET /s/<code> redirect handler.
func (r *ShortLinkRepo) Resolve(ctx context.Context, fileCode, userCode string) (*ShortLinkRecord, error) {
	if fileCode == "" || userCode == "" {
		return nil, ErrShortLinkNotFound
	}
	row := r.db.Read.QueryRowContext(ctx, `
		SELECT file_code, user_code, user_id, target_url,
		       COALESCE(expires_at, 0), created_at
		  FROM short_links
		 WHERE file_code = ? AND user_code = ?`,
		fileCode, userCode,
	)
	var rec ShortLinkRecord
	if err := row.Scan(&rec.FileCode, &rec.UserCode, &rec.UserID,
		&rec.TargetURL, &rec.ExpiresAt, &rec.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrShortLinkNotFound
		}
		return nil, fmt.Errorf("scan short link: %w", err)
	}
	if rec.ExpiresAt > 0 && rec.ExpiresAt <= r.now().UnixMilli() {
		return nil, ErrShortLinkNotFound
	}
	return &rec, nil
}

// ListByUser returns every (non-expired) short link owned by userID, newest
// first. Expired rows are filtered server-side so the UI never displays a
// dead link the user cannot use.
func (r *ShortLinkRepo) ListByUser(ctx context.Context, userID string) ([]ShortLinkRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("shortlink list-by-user: empty user_id")
	}
	now := r.now().UnixMilli()
	rows, err := r.db.Read.QueryContext(ctx, `
		SELECT file_code, user_code, user_id, target_url,
		       COALESCE(expires_at, 0), created_at
		  FROM short_links
		 WHERE user_id = ?
		   AND (expires_at IS NULL OR expires_at > ?)
		 ORDER BY created_at DESC`,
		userID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("list short links: %w", err)
	}
	defer rows.Close()
	out := make([]ShortLinkRecord, 0, 16)
	for rows.Next() {
		var rec ShortLinkRecord
		if err := rows.Scan(&rec.FileCode, &rec.UserCode, &rec.UserID,
			&rec.TargetURL, &rec.ExpiresAt, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan short link: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate short links: %w", err)
	}
	return out, nil
}

// Delete removes the (fileCode, userCode) row owned by userID. The userID
// guard prevents a malicious caller from deleting another user's link by
// guessing the public code. Returns ErrShortLinkNotFound when no row matches.
func (r *ShortLinkRepo) Delete(ctx context.Context, fileCode, userCode, userID string) error {
	if fileCode == "" || userCode == "" || userID == "" {
		return ErrShortLinkNotFound
	}
	res, err := r.db.Write.ExecContext(ctx, `
		DELETE FROM short_links
		 WHERE file_code = ? AND user_code = ? AND user_id = ?`,
		fileCode, userCode, userID,
	)
	if err != nil {
		return fmt.Errorf("delete short link: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrShortLinkNotFound
	}
	return nil
}

// MaxFileCode returns the lexicographically-greatest existing file_code, or
// the empty string when the table is empty. base62 codes share the property
// that lexical ordering matches the integer they represent only when codes
// have equal length — the service layer enforces that by left-padding via
// EncodeBase62MinWidth.
//
// We use this instead of a separate counter table because it keeps the
// schema flat. The cost of the MAX scan is negligible (a single index seek
// thanks to the PK).
func (r *ShortLinkRepo) MaxFileCode(ctx context.Context) (string, error) {
	row := r.db.Read.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(file_code), '') FROM short_links`,
	)
	var code string
	if err := row.Scan(&code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("scan max file code: %w", err)
	}
	return code, nil
}

// MaxUserCode returns the lexicographically-greatest user_code owned by
// userID. Used by the service layer to compute the next per-user
// monotonic code without a dedicated counter row.
func (r *ShortLinkRepo) MaxUserCode(ctx context.Context, userID string) (string, error) {
	if userID == "" {
		return "", nil
	}
	row := r.db.Read.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(user_code), '') FROM short_links WHERE user_id = ?`,
		userID,
	)
	var code string
	if err := row.Scan(&code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("scan max user code: %w", err)
	}
	return code, nil
}

// DeleteExpired removes every row whose expires_at is <= cutoffMs. Used by a
// future background cleaner; safe to call unsupervised.
func (r *ShortLinkRepo) DeleteExpired(ctx context.Context, cutoffMs int64) (int64, error) {
	if cutoffMs <= 0 {
		return 0, nil
	}
	res, err := r.db.Write.ExecContext(ctx, `
		DELETE FROM short_links WHERE expires_at IS NOT NULL AND expires_at <= ?`,
		cutoffMs,
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired short links: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
