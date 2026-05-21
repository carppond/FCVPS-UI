package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// AuditLogRecord is the storage projection of an audit_logs row. Payload is
// stored as a TEXT blob (JSON in practice); the repo treats it opaquely.
type AuditLogRecord struct {
	ID           int64
	UserID       string
	Action       string
	ResourceType string
	ResourceID   string
	IP           string
	UserAgent    string
	Payload      string
	Success      bool
	CreatedAt    int64
}

// AuditLogFilter narrows / paginates a List query. Empty fields mean "no
// filter on this dimension".
type AuditLogFilter struct {
	UserID     string
	Action     string
	From       int64 // unix ms inclusive
	To         int64 // unix ms exclusive
	Page       int
	PageSize   int
	SuccessOnly bool // true → only success=1 rows
}

// ErrAuditLogNotFound is reserved for future single-row lookups. List always
// returns a (possibly empty) slice; insert never fails with not-found.
var ErrAuditLogNotFound = errors.New("storage: audit log not found")

// AuditRepo encapsulates SQL access to audit_logs.
type AuditRepo struct {
	db  *DB
	now func() time.Time
}

// NewAuditRepo wires a repo to db. nil now defaults to time.Now.
func NewAuditRepo(db *DB, now func() time.Time) *AuditRepo {
	if now == nil {
		now = time.Now
	}
	return &AuditRepo{db: db, now: now}
}

// Insert appends a single row. The returned record has the AUTOINCREMENT id
// populated. The audit.Logger calls this from a worker goroutine — direct
// callers (rare) should expect single-row latency.
func (r *AuditRepo) Insert(ctx context.Context, rec AuditLogRecord) (*AuditLogRecord, error) {
	if rec.Action == "" {
		return nil, fmt.Errorf("audit insert: empty action")
	}
	if rec.CreatedAt == 0 {
		rec.CreatedAt = r.now().UnixMilli()
	}
	successInt := 0
	if rec.Success {
		successInt = 1
	}
	var userArg, typeArg, resArg, ipArg, uaArg, payloadArg any
	if rec.UserID != "" {
		userArg = rec.UserID
	}
	if rec.ResourceType != "" {
		typeArg = rec.ResourceType
	}
	if rec.ResourceID != "" {
		resArg = rec.ResourceID
	}
	if rec.IP != "" {
		ipArg = rec.IP
	}
	if rec.UserAgent != "" {
		uaArg = rec.UserAgent
	}
	if rec.Payload != "" {
		payloadArg = rec.Payload
	}
	res, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO audit_logs(user_id, action, resource_type, resource_id,
			ip, user_agent, payload, success, created_at)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		userArg, rec.Action, typeArg, resArg, ipArg, uaArg, payloadArg,
		successInt, rec.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert audit log: %w", err)
	}
	id, _ := res.LastInsertId()
	rec.ID = id
	return &rec, nil
}

// List paginates rows newest-first. Returns (rows, total, err). An empty
// filter returns the most recent page across all users.
func (r *AuditRepo) List(ctx context.Context, filter AuditLogFilter) ([]AuditLogRecord, int64, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 50
	}
	if filter.PageSize > 500 {
		filter.PageSize = 500
	}
	where := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if filter.UserID != "" {
		where = append(where, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if filter.Action != "" {
		where = append(where, "action = ?")
		args = append(args, filter.Action)
	}
	if filter.From > 0 {
		where = append(where, "created_at >= ?")
		args = append(args, filter.From)
	}
	if filter.To > 0 {
		where = append(where, "created_at < ?")
		args = append(args, filter.To)
	}
	if filter.SuccessOnly {
		where = append(where, "success = 1")
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.db.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM audit_logs"+clause, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}
	offset := (filter.Page - 1) * filter.PageSize
	rows, err := r.db.Read.QueryContext(ctx, `
		SELECT id, COALESCE(user_id, ''), action,
		       COALESCE(resource_type,''), COALESCE(resource_id,''),
		       COALESCE(ip,''), COALESCE(user_agent,''),
		       COALESCE(payload,''), success, created_at
		  FROM audit_logs`+clause+`
		 ORDER BY created_at DESC, id DESC
		 LIMIT ? OFFSET ?`,
		append(args, filter.PageSize, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()
	out := make([]AuditLogRecord, 0, filter.PageSize)
	for rows.Next() {
		var rec AuditLogRecord
		var success int
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.Action,
			&rec.ResourceType, &rec.ResourceID, &rec.IP, &rec.UserAgent,
			&rec.Payload, &success, &rec.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		rec.Success = success != 0
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit logs: %w", err)
	}
	return out, total, nil
}

// DeleteOlderThan removes rows whose created_at < cutoffMs. Used by the
// background retention cleaner (default 180 days, see §4.4).
func (r *AuditRepo) DeleteOlderThan(ctx context.Context, cutoffMs int64) (int64, error) {
	if cutoffMs <= 0 {
		return 0, nil
	}
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM audit_logs WHERE created_at < ?", cutoffMs,
	)
	if err != nil {
		return 0, fmt.Errorf("delete old audit logs: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// CountByAction returns a quick action→count breakdown over the [from, to)
// window. Used by the admin dashboard tile that surfaces the busiest audit
// events at a glance.
func (r *AuditRepo) CountByAction(ctx context.Context, fromMs, toMs int64) (map[string]int64, error) {
	args := []any{}
	clause := ""
	if fromMs > 0 || toMs > 0 {
		conds := []string{}
		if fromMs > 0 {
			conds = append(conds, "created_at >= ?")
			args = append(args, fromMs)
		}
		if toMs > 0 {
			conds = append(conds, "created_at < ?")
			args = append(args, toMs)
		}
		clause = " WHERE " + strings.Join(conds, " AND ")
	}
	rows, err := r.db.Read.QueryContext(ctx,
		`SELECT action, COUNT(*) FROM audit_logs`+clause+` GROUP BY action`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("count audit by action: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var action string
		var n int64
		if err := rows.Scan(&action, &n); err != nil {
			return nil, fmt.Errorf("scan action count: %w", err)
		}
		out[action] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate action counts: %w", err)
	}
	return out, nil
}

// suppress unused-import warning for database/sql; the symbol is exported via
// the rows-based loops above.
var _ = sql.ErrNoRows
