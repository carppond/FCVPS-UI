package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RuleSetProviderRecord 是 rule_set_providers 表的存储投影。Handler 层把
// 它翻译成 types.RuleSetProvider DTO。
//
// behavior / format 的取值由 migration 中的 CHECK 约束保证；repo 不再重复
// 校验（让违规值直接撞 DB constraint 反馈给调用方）。
type RuleSetProviderRecord struct {
	ID              string
	UserID          string
	Name            string
	Behavior        string // "domain" | "ipcidr" | "classical"
	Format          string // "yaml" | "text" | "mrs"
	URL             string
	IntervalSeconds int32
	Enabled         bool
	LastSyncedAt    int64
	LastSyncStatus  string
	LastSyncError   string
	CreatedAt       int64
	UpdatedAt       int64
}

// RuleSetProviderListOptions 是 List 查询的可选过滤项。Page<=0 时禁用分页，
// 返回全集 —— 内部 SyncAll 走这个路径。
type RuleSetProviderListOptions struct {
	Page     int
	PageSize int
	Keyword  string // matched against name (LIKE %kw%)
}

var (
	// ErrRuleSetProviderNotFound 是规则集表的统一 not-found 哨兵。
	// 跨用户访问也走这个 sentinel 以避免泄漏存在性。
	ErrRuleSetProviderNotFound = errors.New("storage: rule set provider not found")
)

// RuleSetProviderRepo 持有 rule_set_providers 表的 CRUD 入口。每个查询都用
// user_id 限定作用域，跨用户访问回 ErrRuleSetProviderNotFound（信息隐藏）。
type RuleSetProviderRepo struct {
	db  *DB
	now func() time.Time
}

// NewRuleSetProviderRepo 构造一个绑定到 db 的 repo。now 为 nil 时回退到 time.Now。
func NewRuleSetProviderRepo(db *DB, now func() time.Time) *RuleSetProviderRepo {
	if now == nil {
		now = time.Now
	}
	return &RuleSetProviderRepo{db: db, now: now}
}

// Create 插入一行 rule_set_providers。零值 CreatedAt / UpdatedAt 会被自动填充
// 为当前时间。rec.ID 必须非空（生产代码用 UUIDv7）。
func (r *RuleSetProviderRepo) Create(ctx context.Context, rec RuleSetProviderRecord) (*RuleSetProviderRecord, error) {
	if rec.ID == "" {
		return nil, fmt.Errorf("rule set provider create: empty id")
	}
	if rec.UserID == "" || rec.Name == "" || rec.Behavior == "" || rec.Format == "" || rec.URL == "" {
		return nil, fmt.Errorf("rule set provider create: required field missing")
	}
	if rec.IntervalSeconds <= 0 {
		rec.IntervalSeconds = 86400
	}
	now := r.now().UnixMilli()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt == 0 {
		rec.UpdatedAt = now
	}
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO rule_set_providers(id, user_id, name, behavior, format, url,
		                              interval_seconds, enabled,
		                              last_synced_at, last_sync_status, last_sync_error,
		                              created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.UserID, rec.Name, rec.Behavior, rec.Format, rec.URL,
		rec.IntervalSeconds, boolToInt(rec.Enabled),
		nullableInt64(rec.LastSyncedAt), nullableString(rec.LastSyncStatus),
		nullableString(rec.LastSyncError),
		rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert rule set provider: %w", err)
	}
	return &rec, nil
}

// GetByID 返回 (id, userID) 命中的行。跨用户访问回 ErrRuleSetProviderNotFound。
func (r *RuleSetProviderRepo) GetByID(ctx context.Context, id, userID string) (*RuleSetProviderRecord, error) {
	if id == "" || userID == "" {
		return nil, ErrRuleSetProviderNotFound
	}
	row := r.db.Read.QueryRowContext(ctx,
		selectRuleSetSQL+" WHERE id = ? AND user_id = ?", id, userID)
	return scanRuleSet(row)
}

// List 对用户的规则集做分页 / 关键字过滤。Page<=0 时返回全集（用于内部
// SyncAll 扫描）。
func (r *RuleSetProviderRepo) List(ctx context.Context, userID string, opts RuleSetProviderListOptions) ([]RuleSetProviderRecord, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("rule set provider list: empty user_id")
	}
	args := []any{userID}
	where := "user_id = ?"
	if opts.Keyword != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+opts.Keyword+"%")
	}
	var total int64
	if err := r.db.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM rule_set_providers WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count rule set providers: %w", err)
	}
	query := selectRuleSetSQL + " WHERE " + where + " ORDER BY created_at ASC"
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
		return nil, 0, fmt.Errorf("list rule set providers: %w", err)
	}
	defer rows.Close()
	out := make([]RuleSetProviderRecord, 0, 16)
	for rows.Next() {
		rec, err := scanRuleSetRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate rule set providers: %w", err)
	}
	return out, total, nil
}

// ListEnabled 返回该用户所有 enabled=1 的规则集，按 created_at 升序。用于
// 同步任务遍历。
func (r *RuleSetProviderRepo) ListEnabled(ctx context.Context, userID string) ([]RuleSetProviderRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("rule set provider list-enabled: empty user_id")
	}
	rows, err := r.db.Read.QueryContext(ctx,
		selectRuleSetSQL+" WHERE user_id = ? AND enabled = 1 ORDER BY created_at ASC",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list enabled rule set providers: %w", err)
	}
	defer rows.Close()
	out := make([]RuleSetProviderRecord, 0, 8)
	for rows.Next() {
		rec, err := scanRuleSetRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled rule set providers: %w", err)
	}
	return out, nil
}

// RuleSetProviderUpdate 表示一次部分更新。指针字段为 nil 时表示"不变"，其他
// 字段为零值（空串 / 0）时也表示"不变" —— 与 CustomRuleRepo.Update 的约定一致。
type RuleSetProviderUpdate struct {
	Name            string
	Behavior        string
	Format          string
	URL             string
	IntervalSeconds int32
	Enabled         *bool
}

// Update 覆盖可变字段。空字符串字段（Name/Behavior/Format/URL）以及 0 的
// IntervalSeconds 都视为"保持原值"；Enabled 通过指针传递以区分 false 和"不变"。
func (r *RuleSetProviderRepo) Update(ctx context.Context, id, userID string, upd RuleSetProviderUpdate) error {
	if id == "" || userID == "" {
		return fmt.Errorf("rule set provider update: empty id / user_id")
	}
	existing, err := r.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if upd.Name != "" {
		existing.Name = upd.Name
	}
	if upd.Behavior != "" {
		existing.Behavior = upd.Behavior
	}
	if upd.Format != "" {
		existing.Format = upd.Format
	}
	if upd.URL != "" {
		existing.URL = upd.URL
	}
	if upd.IntervalSeconds > 0 {
		existing.IntervalSeconds = upd.IntervalSeconds
	}
	if upd.Enabled != nil {
		existing.Enabled = *upd.Enabled
	}
	now := r.now().UnixMilli()
	res, err := r.db.Write.ExecContext(ctx, `
		UPDATE rule_set_providers
		   SET name = ?, behavior = ?, format = ?, url = ?,
		       interval_seconds = ?, enabled = ?, updated_at = ?
		 WHERE id = ? AND user_id = ?`,
		existing.Name, existing.Behavior, existing.Format, existing.URL,
		existing.IntervalSeconds, boolToInt(existing.Enabled), now,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("update rule set provider: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrRuleSetProviderNotFound
	}
	return nil
}

// UpdateSyncStatus 记录一次同步结果（成功 / 失败）。Status 取值 "ok" 或
// "error"；err 为空时清空 last_sync_error。
func (r *RuleSetProviderRepo) UpdateSyncStatus(ctx context.Context, id, userID, status, syncErr string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("rule set provider sync-status: empty id / user_id")
	}
	now := r.now().UnixMilli()
	res, err := r.db.Write.ExecContext(ctx, `
		UPDATE rule_set_providers
		   SET last_synced_at = ?, last_sync_status = ?, last_sync_error = ?, updated_at = ?
		 WHERE id = ? AND user_id = ?`,
		now, nullableString(status), nullableString(syncErr), now, id, userID,
	)
	if err != nil {
		return fmt.Errorf("update sync status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrRuleSetProviderNotFound
	}
	return nil
}

// Delete 删除 (id, userID) 行。跨用户调用回 ErrRuleSetProviderNotFound。
func (r *RuleSetProviderRepo) Delete(ctx context.Context, id, userID string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("rule set provider delete: empty id / user_id")
	}
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM rule_set_providers WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete rule set provider: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrRuleSetProviderNotFound
	}
	return nil
}

const selectRuleSetSQL = `SELECT id, user_id, name, behavior, format, url,
		interval_seconds, enabled, last_synced_at, last_sync_status,
		last_sync_error, created_at, updated_at FROM rule_set_providers`

func scanRuleSet(row *sql.Row) (*RuleSetProviderRecord, error) {
	var (
		rec            RuleSetProviderRecord
		enabled        int
		syncedAt       sql.NullInt64
		syncStatus     sql.NullString
		syncError      sql.NullString
	)
	err := row.Scan(&rec.ID, &rec.UserID, &rec.Name, &rec.Behavior, &rec.Format,
		&rec.URL, &rec.IntervalSeconds, &enabled,
		&syncedAt, &syncStatus, &syncError,
		&rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRuleSetProviderNotFound
		}
		return nil, fmt.Errorf("scan rule set provider: %w", err)
	}
	rec.Enabled = enabled == 1
	if syncedAt.Valid {
		rec.LastSyncedAt = syncedAt.Int64
	}
	if syncStatus.Valid {
		rec.LastSyncStatus = syncStatus.String
	}
	if syncError.Valid {
		rec.LastSyncError = syncError.String
	}
	return &rec, nil
}

func scanRuleSetRow(rows *sql.Rows) (*RuleSetProviderRecord, error) {
	var (
		rec        RuleSetProviderRecord
		enabled    int
		syncedAt   sql.NullInt64
		syncStatus sql.NullString
		syncError  sql.NullString
	)
	if err := rows.Scan(&rec.ID, &rec.UserID, &rec.Name, &rec.Behavior, &rec.Format,
		&rec.URL, &rec.IntervalSeconds, &enabled,
		&syncedAt, &syncStatus, &syncError,
		&rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan rule set provider: %w", err)
	}
	rec.Enabled = enabled == 1
	if syncedAt.Valid {
		rec.LastSyncedAt = syncedAt.Int64
	}
	if syncStatus.Valid {
		rec.LastSyncStatus = syncStatus.String
	}
	if syncError.Valid {
		rec.LastSyncError = syncError.String
	}
	return &rec, nil
}
