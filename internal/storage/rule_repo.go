package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CustomRuleRecord is the storage projection of a custom_rules row. Handler
// code translates this into types.CustomRule.
//
// Per migration 0001 the `type` column is constrained to one of
// "dns" / "rules" / "rule-providers" and `mode` to "replace" / "prepend" /
// "append". The repo does not re-validate the value strings; that is the
// handler's job (it owns the contract DTO).
type CustomRuleRecord struct {
	ID        string
	UserID    string
	Name      string
	Type      string
	Mode      string
	Content   string
	Enabled   bool
	Sort      int32
	CreatedAt int64
	UpdatedAt int64
}

// CustomRuleListOptions narrows / paginates a List query. When Page <= 0 the
// repo returns every rule (without pagination) — used by the injector path
// where the full enabled set is needed.
type CustomRuleListOptions struct {
	Page     int
	PageSize int
	Type     string // optional exact filter
	Keyword  string // matched against name (LIKE %kw%)
}

// Errors surfaced by CustomRuleRepo.
var (
	// ErrCustomRuleNotFound is the canonical not-found sentinel for the
	// custom_rules table.
	ErrCustomRuleNotFound = errors.New("storage: custom rule not found")
)

// CustomRuleRepo encapsulates SQL access to the custom_rules table. All
// queries scope by user_id so cross-user reads collapse to "not found"
// (information hiding, mirrors the pipeline repo).
type CustomRuleRepo struct {
	db  *DB
	now func() time.Time
}

// NewCustomRuleRepo wires a repo to db. When now is nil, time.Now is used.
func NewCustomRuleRepo(db *DB, now func() time.Time) *CustomRuleRepo {
	if now == nil {
		now = time.Now
	}
	return &CustomRuleRepo{db: db, now: now}
}

// Create inserts a new custom_rules row. CreatedAt / UpdatedAt are populated
// when zero. The supplied rec.ID must be non-empty (UUID v7 in production).
func (r *CustomRuleRepo) Create(ctx context.Context, rec CustomRuleRecord) (*CustomRuleRecord, error) {
	if rec.ID == "" {
		return nil, fmt.Errorf("custom rule create: empty id")
	}
	if rec.UserID == "" || rec.Name == "" || rec.Type == "" || rec.Mode == "" {
		return nil, fmt.Errorf("custom rule create: required field missing")
	}
	now := r.now().UnixMilli()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt == 0 {
		rec.UpdatedAt = now
	}
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO custom_rules(id, user_id, name, type, mode, content,
		                        enabled, sort, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.UserID, rec.Name, rec.Type, rec.Mode, rec.Content,
		boolToInt(rec.Enabled), rec.Sort, rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert custom rule: %w", err)
	}
	return &rec, nil
}

// GetByID returns a custom rule row scoped to userID. The userID filter is
// mandatory — cross-user GETs surface as ErrCustomRuleNotFound.
func (r *CustomRuleRepo) GetByID(ctx context.Context, id, userID string) (*CustomRuleRecord, error) {
	if id == "" || userID == "" {
		return nil, fmt.Errorf("custom rule get: empty id / user_id")
	}
	row := r.db.Read.QueryRowContext(ctx, selectCustomRuleSQL+
		` WHERE id = ? AND user_id = ?`, id, userID)
	return scanCustomRule(row)
}

// List paginates custom rules for a user. When opts.Page <= 0 the entire
// (filtered) set is returned in a single page; this is the path used by the
// injector when assembling the active rule list.
func (r *CustomRuleRepo) List(ctx context.Context, userID string, opts CustomRuleListOptions) ([]CustomRuleRecord, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("custom rule list: empty user_id")
	}
	args := []any{userID}
	where := "user_id = ?"
	if opts.Type != "" {
		where += " AND type = ?"
		args = append(args, opts.Type)
	}
	if opts.Keyword != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+opts.Keyword+"%")
	}
	var total int64
	if err := r.db.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM custom_rules WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count custom rules: %w", err)
	}
	query := selectCustomRuleSQL + " WHERE " + where + " ORDER BY sort ASC, created_at ASC"
	if opts.Page > 0 {
		if opts.PageSize <= 0 {
			opts.PageSize = 50
		}
		if opts.PageSize > 200 {
			opts.PageSize = 200
		}
		offset := (opts.Page - 1) * opts.PageSize
		query += " LIMIT ? OFFSET ?"
		args = append(args, opts.PageSize, offset)
	}
	rows, err := r.db.Read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list custom rules: %w", err)
	}
	defer rows.Close()
	out := make([]CustomRuleRecord, 0, 16)
	for rows.Next() {
		rec, err := scanCustomRuleMulti(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate custom rules: %w", err)
	}
	return out, total, nil
}

// ListEnabled returns every enabled rule for userID, ordered by sort ASC.
// This is the canonical input to rule_injector.Inject — callers should not
// re-sort the slice.
func (r *CustomRuleRepo) ListEnabled(ctx context.Context, userID string) ([]CustomRuleRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("custom rule list-enabled: empty user_id")
	}
	rows, err := r.db.Read.QueryContext(ctx,
		selectCustomRuleSQL+` WHERE user_id = ? AND enabled = 1 ORDER BY sort ASC, created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list enabled custom rules: %w", err)
	}
	defer rows.Close()
	out := make([]CustomRuleRecord, 0, 8)
	for rows.Next() {
		rec, err := scanCustomRuleMulti(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled custom rules: %w", err)
	}
	return out, nil
}

// Update overwrites mutable fields of the rule identified by (id, user_id).
// Empty string fields ("name", "mode", "content") are treated as "leave
// unchanged"; Enabled and Sort are pointers so the caller can distinguish
// "set to false / 0" from "leave alone".
func (r *CustomRuleRepo) Update(ctx context.Context, rec CustomRuleRecord, updateEnabled bool, updateSort bool) error {
	if rec.ID == "" || rec.UserID == "" {
		return fmt.Errorf("custom rule update: empty id / user_id")
	}
	existing, err := r.GetByID(ctx, rec.ID, rec.UserID)
	if err != nil {
		return err
	}
	if rec.Name != "" {
		existing.Name = rec.Name
	}
	if rec.Mode != "" {
		existing.Mode = rec.Mode
	}
	if rec.Content != "" {
		existing.Content = rec.Content
	}
	if updateEnabled {
		existing.Enabled = rec.Enabled
	}
	if updateSort {
		existing.Sort = rec.Sort
	}
	now := r.now().UnixMilli()
	res, err := r.db.Write.ExecContext(ctx, `
		UPDATE custom_rules
		   SET name = ?, mode = ?, content = ?, enabled = ?, sort = ?, updated_at = ?
		 WHERE id = ? AND user_id = ?`,
		existing.Name, existing.Mode, existing.Content,
		boolToInt(existing.Enabled), existing.Sort, now,
		rec.ID, rec.UserID,
	)
	if err != nil {
		return fmt.Errorf("update custom rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrCustomRuleNotFound
	}
	return nil
}

// Delete removes the custom_rules row scoped to userID.
func (r *CustomRuleRepo) Delete(ctx context.Context, id, userID string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("custom rule delete: empty id / user_id")
	}
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM custom_rules WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete custom rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrCustomRuleNotFound
	}
	return nil
}

// ReorderEntry pairs a rule id with its new sort value. Used by Reorder.
type ReorderEntry struct {
	ID   string
	Sort int32
}

// Reorder atomically updates the sort field of every supplied entry. Entries
// that reference an id which does not belong to userID are silently skipped
// (no row affected); the caller can detect this via the returned `updated`
// count.
//
// The whole batch runs inside a single write transaction so partial failures
// roll back.
func (r *CustomRuleRepo) Reorder(ctx context.Context, userID string, entries []ReorderEntry) (int, error) {
	if userID == "" {
		return 0, fmt.Errorf("custom rule reorder: empty user_id")
	}
	if len(entries) == 0 {
		return 0, nil
	}
	tx, err := r.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := r.now().UnixMilli()
	updated := 0
	for _, e := range entries {
		if e.ID == "" {
			continue
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE custom_rules SET sort = ?, updated_at = ?
			 WHERE id = ? AND user_id = ?`, e.Sort, now, e.ID, userID)
		if err != nil {
			return 0, fmt.Errorf("update sort: %w", err)
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			updated++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit reorder: %w", err)
	}
	return updated, nil
}

const selectCustomRuleSQL = `SELECT id, user_id, name, type, mode, content,
		enabled, sort, created_at, updated_at FROM custom_rules`

// scanCustomRule drains a QueryRow result.
func scanCustomRule(row *sql.Row) (*CustomRuleRecord, error) {
	var rec CustomRuleRecord
	var enabled int
	err := row.Scan(&rec.ID, &rec.UserID, &rec.Name, &rec.Type, &rec.Mode,
		&rec.Content, &enabled, &rec.Sort, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCustomRuleNotFound
		}
		return nil, fmt.Errorf("scan custom rule: %w", err)
	}
	rec.Enabled = enabled == 1
	return &rec, nil
}

// scanCustomRuleMulti drains a rows.Next result.
func scanCustomRuleMulti(rows *sql.Rows) (*CustomRuleRecord, error) {
	var rec CustomRuleRecord
	var enabled int
	if err := rows.Scan(&rec.ID, &rec.UserID, &rec.Name, &rec.Type, &rec.Mode,
		&rec.Content, &enabled, &rec.Sort, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan custom rule: %w", err)
	}
	rec.Enabled = enabled == 1
	return &rec, nil
}
