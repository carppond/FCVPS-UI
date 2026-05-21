package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ProxyGroupCategoryRecord 是 proxy_group_categories 表的存储投影。
// member_proxies / member_groups 字段存储 JSON 文本（[]string 序列化结果）。
// repo 不解析 JSON，让上层 handler / 类型层处理 —— 保持存储层无领域逻辑。
type ProxyGroupCategoryRecord struct {
	ID            string
	UserID        string
	Name          string
	Type          string // "select" | "url-test" | "fallback" | "load-balance" | "relay"
	Icon          string
	SortOrder     int32
	TestURL       string
	TestInterval  int32
	Filter        string
	IncludeAll    bool
	MemberProxies string // JSON 数组文本，比如 `["DIRECT","REJECT"]`
	MemberGroups  string // JSON 数组文本，引用其它组的 id
	CreatedAt     int64
	UpdatedAt     int64
}

// ProxyGroupListOptions 是 List 查询的过滤项。Page<=0 时禁用分页（返回全集）。
type ProxyGroupListOptions struct {
	Page     int
	PageSize int
	Type     string // optional exact filter
	Keyword  string // matched against name (LIKE %kw%)
}

var (
	// ErrProxyGroupNotFound 是 proxy_group_categories 表的统一 not-found 哨兵。
	ErrProxyGroupNotFound = errors.New("storage: proxy group not found")
)

// ProxyGroupRepo 持有 proxy_group_categories 表的 CRUD 入口。
type ProxyGroupRepo struct {
	db  *DB
	now func() time.Time
}

// NewProxyGroupRepo 构造一个绑定到 db 的 repo。now 为 nil 时回退到 time.Now。
func NewProxyGroupRepo(db *DB, now func() time.Time) *ProxyGroupRepo {
	if now == nil {
		now = time.Now
	}
	return &ProxyGroupRepo{db: db, now: now}
}

// Create 插入一行 proxy_group_categories。零值时间戳会被自动填充。
func (r *ProxyGroupRepo) Create(ctx context.Context, rec ProxyGroupCategoryRecord) (*ProxyGroupCategoryRecord, error) {
	if rec.ID == "" {
		return nil, fmt.Errorf("proxy group create: empty id")
	}
	if rec.UserID == "" || rec.Name == "" || rec.Type == "" {
		return nil, fmt.Errorf("proxy group create: required field missing")
	}
	now := r.now().UnixMilli()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt == 0 {
		rec.UpdatedAt = now
	}
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO proxy_group_categories(id, user_id, name, type, icon, sort_order,
		                                   test_url, test_interval, filter, include_all,
		                                   member_proxies, member_groups,
		                                   created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.UserID, rec.Name, rec.Type,
		nullableString(rec.Icon), rec.SortOrder,
		nullableString(rec.TestURL), nullableInt32(rec.TestInterval),
		nullableString(rec.Filter), boolToInt(rec.IncludeAll),
		nullableString(rec.MemberProxies), nullableString(rec.MemberGroups),
		rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert proxy group: %w", err)
	}
	return &rec, nil
}

// GetByID 返回 (id, userID) 命中的行。跨用户访问回 ErrProxyGroupNotFound。
func (r *ProxyGroupRepo) GetByID(ctx context.Context, id, userID string) (*ProxyGroupCategoryRecord, error) {
	if id == "" || userID == "" {
		return nil, ErrProxyGroupNotFound
	}
	row := r.db.Read.QueryRowContext(ctx,
		selectProxyGroupSQL+" WHERE id = ? AND user_id = ?", id, userID)
	return scanProxyGroup(row)
}

// List 对用户的代理组做查询。默认按 sort_order ASC 排序。
func (r *ProxyGroupRepo) List(ctx context.Context, userID string, opts ProxyGroupListOptions) ([]ProxyGroupCategoryRecord, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("proxy group list: empty user_id")
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
		"SELECT COUNT(*) FROM proxy_group_categories WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count proxy groups: %w", err)
	}
	query := selectProxyGroupSQL + " WHERE " + where + " ORDER BY sort_order ASC, created_at ASC"
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
		return nil, 0, fmt.Errorf("list proxy groups: %w", err)
	}
	defer rows.Close()
	out := make([]ProxyGroupCategoryRecord, 0, 16)
	for rows.Next() {
		rec, err := scanProxyGroupRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate proxy groups: %w", err)
	}
	return out, total, nil
}

// ProxyGroupUpdate 表示部分更新。零值字段（空串 / nil 指针）表示"不变"。
// SortOrder 通过指针传递以区分 0 和"不变"。
type ProxyGroupUpdate struct {
	Name          string
	Type          string
	Icon          *string
	SortOrder     *int32
	TestURL       *string
	TestInterval  *int32
	Filter        *string
	IncludeAll    *bool
	MemberProxies *string
	MemberGroups  *string
}

// Update 覆盖可变字段。
func (r *ProxyGroupRepo) Update(ctx context.Context, id, userID string, upd ProxyGroupUpdate) error {
	if id == "" || userID == "" {
		return fmt.Errorf("proxy group update: empty id / user_id")
	}
	existing, err := r.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if upd.Name != "" {
		existing.Name = upd.Name
	}
	if upd.Type != "" {
		existing.Type = upd.Type
	}
	if upd.Icon != nil {
		existing.Icon = *upd.Icon
	}
	if upd.SortOrder != nil {
		existing.SortOrder = *upd.SortOrder
	}
	if upd.TestURL != nil {
		existing.TestURL = *upd.TestURL
	}
	if upd.TestInterval != nil {
		existing.TestInterval = *upd.TestInterval
	}
	if upd.Filter != nil {
		existing.Filter = *upd.Filter
	}
	if upd.IncludeAll != nil {
		existing.IncludeAll = *upd.IncludeAll
	}
	if upd.MemberProxies != nil {
		existing.MemberProxies = *upd.MemberProxies
	}
	if upd.MemberGroups != nil {
		existing.MemberGroups = *upd.MemberGroups
	}
	now := r.now().UnixMilli()
	res, err := r.db.Write.ExecContext(ctx, `
		UPDATE proxy_group_categories
		   SET name = ?, type = ?, icon = ?, sort_order = ?,
		       test_url = ?, test_interval = ?, filter = ?, include_all = ?,
		       member_proxies = ?, member_groups = ?, updated_at = ?
		 WHERE id = ? AND user_id = ?`,
		existing.Name, existing.Type,
		nullableString(existing.Icon), existing.SortOrder,
		nullableString(existing.TestURL), nullableInt32(existing.TestInterval),
		nullableString(existing.Filter), boolToInt(existing.IncludeAll),
		nullableString(existing.MemberProxies), nullableString(existing.MemberGroups),
		now, id, userID,
	)
	if err != nil {
		return fmt.Errorf("update proxy group: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrProxyGroupNotFound
	}
	return nil
}

// Delete 删除 (id, userID) 行。
func (r *ProxyGroupRepo) Delete(ctx context.Context, id, userID string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("proxy group delete: empty id / user_id")
	}
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM proxy_group_categories WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete proxy group: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrProxyGroupNotFound
	}
	return nil
}

// ProxyGroupReorderEntry 把组 id 和它的新 sort_order 配对。
type ProxyGroupReorderEntry struct {
	ID        string
	SortOrder int32
}

// Reorder 原子化批量更新 sort_order。引用了 userID 之外的 id 会被静默跳过
// （SQL 已带 user_id 过滤）。返回成功更新的行数；整个批次跑在一个写事务里。
func (r *ProxyGroupRepo) Reorder(ctx context.Context, userID string, entries []ProxyGroupReorderEntry) (int, error) {
	if userID == "" {
		return 0, fmt.Errorf("proxy group reorder: empty user_id")
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
			UPDATE proxy_group_categories SET sort_order = ?, updated_at = ?
			 WHERE id = ? AND user_id = ?`, e.SortOrder, now, e.ID, userID)
		if err != nil {
			return 0, fmt.Errorf("update sort_order: %w", err)
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

const selectProxyGroupSQL = `SELECT id, user_id, name, type, icon, sort_order,
		test_url, test_interval, filter, include_all,
		member_proxies, member_groups, created_at, updated_at
		FROM proxy_group_categories`

func scanProxyGroup(row *sql.Row) (*ProxyGroupCategoryRecord, error) {
	var (
		rec           ProxyGroupCategoryRecord
		icon          sql.NullString
		testURL       sql.NullString
		testInterval  sql.NullInt32
		filterStr     sql.NullString
		includeAll    int
		memberProxies sql.NullString
		memberGroups  sql.NullString
	)
	err := row.Scan(&rec.ID, &rec.UserID, &rec.Name, &rec.Type,
		&icon, &rec.SortOrder,
		&testURL, &testInterval, &filterStr, &includeAll,
		&memberProxies, &memberGroups,
		&rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProxyGroupNotFound
		}
		return nil, fmt.Errorf("scan proxy group: %w", err)
	}
	hydrateProxyGroup(&rec, icon, testURL, testInterval, filterStr, includeAll, memberProxies, memberGroups)
	return &rec, nil
}

func scanProxyGroupRow(rows *sql.Rows) (*ProxyGroupCategoryRecord, error) {
	var (
		rec           ProxyGroupCategoryRecord
		icon          sql.NullString
		testURL       sql.NullString
		testInterval  sql.NullInt32
		filterStr     sql.NullString
		includeAll    int
		memberProxies sql.NullString
		memberGroups  sql.NullString
	)
	if err := rows.Scan(&rec.ID, &rec.UserID, &rec.Name, &rec.Type,
		&icon, &rec.SortOrder,
		&testURL, &testInterval, &filterStr, &includeAll,
		&memberProxies, &memberGroups,
		&rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan proxy group: %w", err)
	}
	hydrateProxyGroup(&rec, icon, testURL, testInterval, filterStr, includeAll, memberProxies, memberGroups)
	return &rec, nil
}

func hydrateProxyGroup(rec *ProxyGroupCategoryRecord, icon, testURL sql.NullString,
	testInterval sql.NullInt32, filterStr sql.NullString, includeAll int,
	memberProxies, memberGroups sql.NullString) {
	if icon.Valid {
		rec.Icon = icon.String
	}
	if testURL.Valid {
		rec.TestURL = testURL.String
	}
	if testInterval.Valid {
		rec.TestInterval = testInterval.Int32
	}
	if filterStr.Valid {
		rec.Filter = filterStr.String
	}
	rec.IncludeAll = includeAll == 1
	if memberProxies.Valid {
		rec.MemberProxies = memberProxies.String
	}
	if memberGroups.Valid {
		rec.MemberGroups = memberGroups.String
	}
}

// nullableInt32 mirrors nullableInt64 for int32 columns. Zero => NULL.
func nullableInt32(v int32) any {
	if v == 0 {
		return nil
	}
	return v
}
