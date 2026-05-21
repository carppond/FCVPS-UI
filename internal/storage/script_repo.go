package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ScriptRecord is the storage-layer projection of a `scripts` row.
// Field naming mirrors types.Script so the handler layer can map 1:1.
type ScriptRecord struct {
	ID        string
	UserID    string
	Name      string
	Hook      string // "pre_save_nodes" | "post_fetch"
	Code      string
	Enabled   bool
	LastRunAt int64  // unix ms; 0 means "never executed"
	LastError string // last failure message; empty when last run succeeded
	CreatedAt int64
	UpdatedAt int64
}

// ScriptListOptions narrows the List query.
type ScriptListOptions struct {
	Page     int
	PageSize int
	Hook     string // optional: "pre_save_nodes" | "post_fetch"
	Keyword  string // matched against name (LIKE %kw%)
}

// Sentinels.
var (
	// ErrScriptNotFound is the canonical not-found sentinel for the
	// `scripts` table. Cross-user lookups also yield this so the handler
	// can reply 404 without leaking existence.
	ErrScriptNotFound = errors.New("storage: script not found")
)

// ScriptRepo owns CRUD access to the `scripts` table plus the small set of
// queries the script-engine hook integrator needs (ListEnabledByHook).
type ScriptRepo struct {
	db  *DB
	now func() time.Time
}

// NewScriptRepo wires a repo to db. When now is nil, time.Now is used.
func NewScriptRepo(db *DB, now func() time.Time) *ScriptRepo {
	if now == nil {
		now = time.Now
	}
	return &ScriptRepo{db: db, now: now}
}

// Create inserts a new scripts row. Required fields: ID, UserID, Name, Hook,
// Code. The Enabled flag defaults to true when zero-valued, matching the
// "user creates a script and immediately wants it active" UX.
func (r *ScriptRepo) Create(ctx context.Context, rec ScriptRecord) (*ScriptRecord, error) {
	if rec.ID == "" || rec.UserID == "" || rec.Name == "" {
		return nil, fmt.Errorf("script create: id/user_id/name required")
	}
	if rec.Hook != "pre_save_nodes" && rec.Hook != "post_fetch" {
		return nil, fmt.Errorf("script create: invalid hook %q", rec.Hook)
	}
	if rec.Code == "" {
		return nil, fmt.Errorf("script create: code required")
	}
	now := r.now().UnixMilli()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt == 0 {
		rec.UpdatedAt = now
	}
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO scripts(id, user_id, name, hook, code, enabled,
		                    last_run_at, last_error, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.UserID, rec.Name, rec.Hook, rec.Code, boolToInt(rec.Enabled),
		nullableInt64(rec.LastRunAt), nullableString(rec.LastError),
		rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert script: %w", err)
	}
	return &rec, nil
}

// GetByID returns the row identified by id, scoped to userID. Cross-user
// access yields ErrScriptNotFound (information hiding).
func (r *ScriptRepo) GetByID(ctx context.Context, id, userID string) (*ScriptRecord, error) {
	if id == "" || userID == "" {
		return nil, ErrScriptNotFound
	}
	row := r.db.Read.QueryRowContext(ctx,
		selectScriptSQL+" WHERE id = ? AND user_id = ?", id, userID)
	return scanScriptRow(row)
}

// List paginates the caller's scripts. Hook + Keyword are optional filters.
// Newest-first by updated_at to match the UI ordering.
func (r *ScriptRepo) List(ctx context.Context, userID string, opts ScriptListOptions) ([]ScriptRecord, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("script list: empty user_id")
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.PageSize > 100 {
		opts.PageSize = 100
	}
	args := []any{userID}
	where := []string{"user_id = ?"}
	if opts.Hook != "" {
		where = append(where, "hook = ?")
		args = append(args, opts.Hook)
	}
	if opts.Keyword != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+opts.Keyword+"%")
	}
	clause := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM scripts"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count scripts: %w", err)
	}
	offset := (opts.Page - 1) * opts.PageSize
	rows, err := r.db.Read.QueryContext(ctx,
		selectScriptSQL+clause+" ORDER BY updated_at DESC LIMIT ? OFFSET ?",
		append(args, opts.PageSize, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list scripts: %w", err)
	}
	defer rows.Close()
	out := make([]ScriptRecord, 0, opts.PageSize)
	for rows.Next() {
		s, err := scanScriptRowMulti(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate scripts: %w", err)
	}
	return out, total, nil
}

// ListEnabledByHook returns every enabled script for the given user matching
// hook. The substore sync pipeline calls this on every fetch / save to
// gather the active hook chain. Ordered by created_at so user-visible
// execution order is stable across re-saves.
func (r *ScriptRepo) ListEnabledByHook(ctx context.Context, userID, hook string) ([]ScriptRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("script ListEnabledByHook: empty user_id")
	}
	if hook != "pre_save_nodes" && hook != "post_fetch" {
		return nil, fmt.Errorf("script ListEnabledByHook: invalid hook %q", hook)
	}
	rows, err := r.db.Read.QueryContext(ctx,
		selectScriptSQL+" WHERE user_id = ? AND hook = ? AND enabled = 1 ORDER BY created_at ASC",
		userID, hook,
	)
	if err != nil {
		return nil, fmt.Errorf("list enabled scripts: %w", err)
	}
	defer rows.Close()
	out := make([]ScriptRecord, 0)
	for rows.Next() {
		s, err := scanScriptRowMulti(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scripts: %w", err)
	}
	return out, nil
}

// ScriptUpdate is the partial-update payload accepted by Update. Pointer
// fields opt out when nil; the empty string for Name is treated as "skip"
// so PATCH-style requests do not accidentally wipe a script's name.
type ScriptUpdate struct {
	Name    *string
	Code    *string
	Enabled *bool
}

// Update patches the row in-place. updated_at is always refreshed when at
// least one field is being changed.
func (r *ScriptRepo) Update(ctx context.Context, id, userID string, upd ScriptUpdate) (*ScriptRecord, error) {
	if id == "" || userID == "" {
		return nil, ErrScriptNotFound
	}
	sets := make([]string, 0, 4)
	args := make([]any, 0, 5)
	if upd.Name != nil && *upd.Name != "" {
		sets = append(sets, "name = ?")
		args = append(args, *upd.Name)
	}
	if upd.Code != nil {
		if *upd.Code == "" {
			return nil, fmt.Errorf("script update: code cannot be empty")
		}
		sets = append(sets, "code = ?")
		args = append(args, *upd.Code)
	}
	if upd.Enabled != nil {
		sets = append(sets, "enabled = ?")
		args = append(args, boolToInt(*upd.Enabled))
	}
	if len(sets) == 0 {
		// Nothing to update — return current row so the handler can echo
		// it back without a wasted write.
		return r.GetByID(ctx, id, userID)
	}
	now := r.now().UnixMilli()
	sets = append(sets, "updated_at = ?")
	args = append(args, now)

	args = append(args, id, userID)
	res, err := r.db.Write.ExecContext(ctx,
		"UPDATE scripts SET "+strings.Join(sets, ", ")+" WHERE id = ? AND user_id = ?",
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("update script: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrScriptNotFound
	}
	return r.GetByID(ctx, id, userID)
}

// RecordRun stamps last_run_at + last_error after a hook invocation. errMsg
// may be empty for a successful run; an empty string clears any prior error.
func (r *ScriptRepo) RecordRun(ctx context.Context, id string, ranAt int64, errMsg string) error {
	if id == "" {
		return ErrScriptNotFound
	}
	if ranAt == 0 {
		ranAt = r.now().UnixMilli()
	}
	res, err := r.db.Write.ExecContext(ctx, `
		UPDATE scripts
		   SET last_run_at = ?, last_error = ?, updated_at = ?
		 WHERE id = ?`,
		ranAt, nullableString(errMsg), r.now().UnixMilli(), id,
	)
	if err != nil {
		return fmt.Errorf("record script run: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrScriptNotFound
	}
	return nil
}

// Delete removes the row. Returns ErrScriptNotFound when no row matched.
func (r *ScriptRepo) Delete(ctx context.Context, id, userID string) error {
	if id == "" || userID == "" {
		return ErrScriptNotFound
	}
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM scripts WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete script: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrScriptNotFound
	}
	return nil
}

const selectScriptSQL = `SELECT id, user_id, name, hook, code, enabled,
		last_run_at, last_error, created_at, updated_at FROM scripts`

func scanScriptRow(row *sql.Row) (*ScriptRecord, error) {
	var s ScriptRecord
	var enabledInt int
	var lastRunAt sql.NullInt64
	var lastError sql.NullString
	err := row.Scan(&s.ID, &s.UserID, &s.Name, &s.Hook, &s.Code, &enabledInt,
		&lastRunAt, &lastError, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrScriptNotFound
		}
		return nil, fmt.Errorf("scan script: %w", err)
	}
	s.Enabled = enabledInt != 0
	if lastRunAt.Valid {
		s.LastRunAt = lastRunAt.Int64
	}
	if lastError.Valid {
		s.LastError = lastError.String
	}
	return &s, nil
}

func scanScriptRowMulti(rows *sql.Rows) (*ScriptRecord, error) {
	var s ScriptRecord
	var enabledInt int
	var lastRunAt sql.NullInt64
	var lastError sql.NullString
	if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.Hook, &s.Code, &enabledInt,
		&lastRunAt, &lastError, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan script: %w", err)
	}
	s.Enabled = enabledInt != 0
	if lastRunAt.Valid {
		s.LastRunAt = lastRunAt.Int64
	}
	if lastError.Valid {
		s.LastError = lastError.String
	}
	return &s, nil
}
