package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// VpsAssetRecord is the storage projection of a vps_assets row.
type VpsAssetRecord struct {
	ID             string
	UserID         string
	Name           string
	IP             string
	SSHPort        int
	SSHUser        string
	SSHPassword    string
	SSHPrivateKey  string
	OS             string
	Location       string
	Provider       string
	Price          float64
	Currency       string
	BillingCycle   string
	Bandwidth      string
	MonthlyTraffic int
	CPU            string
	Memory         string
	Disk           string
	ExpireAt       string // YYYY-MM-DD
	Notes          string
	AgentID        string
	Tags           string // JSON array
	CreatedAt      string
	UpdatedAt      string
	// Dynamic — computed at query time, not stored.
	DaysUntilExpiry int
	Status          string // "normal" | "expiring" | "expired"
}

// TagsSlice returns the Tags field decoded as a string slice.
func (r *VpsAssetRecord) TagsSlice() []string {
	if r.Tags == "" || r.Tags == "null" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(r.Tags), &tags); err != nil {
		return []string{}
	}
	return tags
}

// VpsAssetListOptions narrows the List query.
type VpsAssetListOptions struct {
	Page     int
	PageSize int
	Provider string
	Status   string // "normal" | "expiring" | "expired"
	Location string
	Keyword  string
}

// Errors surfaced by VpsAssetRepo.
var (
	ErrVpsAssetNotFound = errors.New("storage: vps asset not found")
)

// VpsAssetRepo encapsulates SQL access to the vps_assets table.
type VpsAssetRepo struct {
	db  *DB
	now func() time.Time
}

// NewVpsAssetRepo wires a repo to db.
func NewVpsAssetRepo(db *DB, now func() time.Time) *VpsAssetRepo {
	if now == nil {
		now = time.Now
	}
	return &VpsAssetRepo{db: db, now: now}
}

// Create inserts a new vps_assets row.
func (r *VpsAssetRepo) Create(ctx context.Context, rec VpsAssetRecord) (*VpsAssetRecord, error) {
	if rec.ID == "" || rec.UserID == "" || rec.Name == "" || rec.Provider == "" || rec.ExpireAt == "" {
		return nil, fmt.Errorf("vps asset create: required field missing")
	}
	if rec.Currency == "" {
		rec.Currency = "CNY"
	}
	if rec.SSHPort == 0 {
		rec.SSHPort = 22
	}
	now := r.now().UTC().Format(time.RFC3339)
	if rec.CreatedAt == "" {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt == "" {
		rec.UpdatedAt = now
	}
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO vps_assets(id, user_id, name, ip, ssh_port, ssh_user, ssh_password, ssh_private_key, os, location,
		                       provider, price, currency, billing_cycle, bandwidth,
		                       monthly_traffic, cpu, memory, disk, expire_at, notes,
		                       agent_id, tags, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.UserID, rec.Name,
		nullableString(rec.IP), rec.SSHPort, nullableString(rec.SSHUser),
		rec.SSHPassword, rec.SSHPrivateKey,
		nullableString(rec.OS), nullableString(rec.Location),
		rec.Provider, rec.Price, rec.Currency, rec.BillingCycle,
		nullableString(rec.Bandwidth), rec.MonthlyTraffic,
		nullableString(rec.CPU), nullableString(rec.Memory), nullableString(rec.Disk),
		rec.ExpireAt, nullableString(rec.Notes),
		nullableString(rec.AgentID), nullableString(rec.Tags),
		rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert vps asset: %w", err)
	}
	return &rec, nil
}

// selectVpsAssetSQL is the shared SELECT prefix with dynamic fields.
const selectVpsAssetSQL = `SELECT id, user_id, name,
	COALESCE(ip,''), ssh_port, COALESCE(ssh_user,''),
	COALESCE(ssh_password,''), COALESCE(ssh_private_key,''),
	COALESCE(os,''), COALESCE(location,''),
	provider, price, currency, billing_cycle,
	COALESCE(bandwidth,''), monthly_traffic,
	COALESCE(cpu,''), COALESCE(memory,''), COALESCE(disk,''),
	expire_at, COALESCE(notes,''),
	COALESCE(agent_id,''), COALESCE(tags,''),
	created_at, updated_at,
	CAST(julianday(expire_at) - julianday('now') AS INTEGER) AS days_until_expiry,
	CASE
		WHEN julianday(expire_at) - julianday('now') <= 0 THEN 'expired'
		WHEN julianday(expire_at) - julianday('now') <= 7 THEN 'expiring'
		ELSE 'normal'
	END AS status
	FROM vps_assets`

// GetByID returns the vps_assets row identified by id, scoped to userID.
func (r *VpsAssetRepo) GetByID(ctx context.Context, id, userID string) (*VpsAssetRecord, error) {
	if id == "" {
		return nil, ErrVpsAssetNotFound
	}
	q := selectVpsAssetSQL + " WHERE id = ?"
	args := []any{id}
	if userID != "" {
		q += " AND user_id = ?"
		args = append(args, userID)
	}
	row := r.db.Read.QueryRowContext(ctx, q, args...)
	return scanVpsAssetRow(row)
}

// List paginates vps_assets owned by userID with optional filters.
func (r *VpsAssetRepo) List(ctx context.Context, userID string, opts VpsAssetListOptions) ([]VpsAssetRecord, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("vps asset list: empty user id")
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

	where := []string{"user_id = ?"}
	args := []any{userID}

	if opts.Provider != "" {
		where = append(where, "provider = ?")
		args = append(args, opts.Provider)
	}
	if opts.Location != "" {
		where = append(where, "location = ?")
		args = append(args, opts.Location)
	}
	if opts.Keyword != "" {
		where = append(where, "(name LIKE ? OR provider LIKE ? OR ip LIKE ?)")
		kw := "%" + opts.Keyword + "%"
		args = append(args, kw, kw, kw)
	}

	clause := " WHERE " + strings.Join(where, " AND ")

	// For status filtering we need a HAVING-like clause on the computed field.
	// Since SQLite allows referencing computed columns in WHERE via subquery,
	// we use a CTE approach. However, for simplicity, we'll filter in a
	// wrapping query.
	if opts.Status != "" {
		switch opts.Status {
		case "expired":
			clause += " AND julianday(expire_at) - julianday('now') <= 0"
		case "expiring":
			clause += " AND julianday(expire_at) - julianday('now') > 0 AND julianday(expire_at) - julianday('now') <= 7"
		case "normal":
			clause += " AND julianday(expire_at) - julianday('now') > 7"
		}
	}

	var total int64
	if err := r.db.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM vps_assets"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count vps assets: %w", err)
	}

	offset := (opts.Page - 1) * opts.PageSize
	rows, err := r.db.Read.QueryContext(ctx,
		selectVpsAssetSQL+clause+" ORDER BY expire_at ASC LIMIT ? OFFSET ?",
		append(args, opts.PageSize, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list vps assets: %w", err)
	}
	defer rows.Close()
	out := make([]VpsAssetRecord, 0, opts.PageSize)
	for rows.Next() {
		a, err := scanVpsAssetRowMulti(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate vps assets: %w", err)
	}
	return out, total, nil
}

// ListExpiring returns all VPS that expire within daysThreshold days for a
// specific user. When userID is empty, returns across all users.
func (r *VpsAssetRepo) ListExpiring(ctx context.Context, userID string, daysThreshold int) ([]VpsAssetRecord, error) {
	q := selectVpsAssetSQL + " WHERE julianday(expire_at) - julianday('now') <= ? AND julianday(expire_at) - julianday('now') > -1"
	args := []any{daysThreshold}
	if userID != "" {
		q += " AND user_id = ?"
		args = append(args, userID)
	}
	q += " ORDER BY expire_at ASC"
	rows, err := r.db.Read.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list expiring vps: %w", err)
	}
	defer rows.Close()
	var out []VpsAssetRecord
	for rows.Next() {
		a, err := scanVpsAssetRowMulti(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expiring vps: %w", err)
	}
	return out, nil
}

// ListAllExpiring returns all VPS assets across all users expiring within
// daysThreshold days (including already expired up to 1 day past).
func (r *VpsAssetRepo) ListAllExpiring(ctx context.Context, daysThreshold int) ([]VpsAssetRecord, error) {
	return r.ListExpiring(ctx, "", daysThreshold)
}

// Update modifies an existing vps_assets row. Only non-zero fields are updated.
func (r *VpsAssetRepo) Update(ctx context.Context, id, userID string, sets map[string]any) (*VpsAssetRecord, error) {
	if id == "" || userID == "" {
		return nil, fmt.Errorf("update vps asset: empty id/user")
	}
	if len(sets) == 0 {
		return r.GetByID(ctx, id, userID)
	}

	setClauses := make([]string, 0, len(sets)+1)
	args := make([]any, 0, len(sets)+3)
	for col, val := range sets {
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}
	now := r.now().UTC().Format(time.RFC3339)
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now)
	args = append(args, id, userID)

	stmt := "UPDATE vps_assets SET " + strings.Join(setClauses, ", ") + " WHERE id = ? AND user_id = ?"
	res, err := r.db.Write.ExecContext(ctx, stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("update vps asset: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrVpsAssetNotFound
	}
	return r.GetByID(ctx, id, userID)
}

// Delete removes a vps_assets row. Cross-user attempts return ErrVpsAssetNotFound.
func (r *VpsAssetRepo) Delete(ctx context.Context, id, userID string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("delete vps asset: empty id/user")
	}
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM vps_assets WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete vps asset: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrVpsAssetNotFound
	}
	return nil
}

// Summary returns aggregate VPS statistics for a user.
func (r *VpsAssetRepo) Summary(ctx context.Context, userID string) (total, expiring, expired int, err error) {
	if userID == "" {
		return 0, 0, 0, fmt.Errorf("summary: empty user id")
	}
	err = r.db.Read.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			SUM(CASE WHEN julianday(expire_at) - julianday('now') > 0
			          AND julianday(expire_at) - julianday('now') <= 7 THEN 1 ELSE 0 END),
			SUM(CASE WHEN julianday(expire_at) - julianday('now') <= 0 THEN 1 ELSE 0 END)
		FROM vps_assets WHERE user_id = ?`, userID).Scan(&total, &expiring, &expired)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("summary: %w", err)
	}
	return
}

// MonthlyCostByUser returns per-currency monthly cost for the user.
func (r *VpsAssetRepo) MonthlyCostByUser(ctx context.Context, userID string) (map[string]float64, error) {
	if userID == "" {
		return nil, fmt.Errorf("monthly cost: empty user id")
	}
	rows, err := r.db.Read.QueryContext(ctx, `
		SELECT currency, billing_cycle, price
		FROM vps_assets WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("monthly cost query: %w", err)
	}
	defer rows.Close()
	costMap := make(map[string]float64)
	for rows.Next() {
		var currency, cycle string
		var price float64
		if err := rows.Scan(&currency, &cycle, &price); err != nil {
			return nil, fmt.Errorf("scan monthly cost: %w", err)
		}
		months := billingCycleMonths(cycle)
		costMap[currency] += price / float64(months)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monthly cost: %w", err)
	}
	return costMap, nil
}

func billingCycleMonths(cycle string) int {
	switch cycle {
	case "monthly":
		return 1
	case "quarterly":
		return 3
	case "semi_annual":
		return 6
	case "annual":
		return 12
	case "biennial":
		return 24
	case "triennial":
		return 36
	default:
		return 1
	}
}

func scanVpsAssetRow(row *sql.Row) (*VpsAssetRecord, error) {
	var a VpsAssetRecord
	err := row.Scan(
		&a.ID, &a.UserID, &a.Name,
		&a.IP, &a.SSHPort, &a.SSHUser,
		&a.SSHPassword, &a.SSHPrivateKey,
		&a.OS, &a.Location,
		&a.Provider, &a.Price, &a.Currency, &a.BillingCycle,
		&a.Bandwidth, &a.MonthlyTraffic,
		&a.CPU, &a.Memory, &a.Disk,
		&a.ExpireAt, &a.Notes,
		&a.AgentID, &a.Tags,
		&a.CreatedAt, &a.UpdatedAt,
		&a.DaysUntilExpiry, &a.Status,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVpsAssetNotFound
		}
		return nil, fmt.Errorf("scan vps asset: %w", err)
	}
	return &a, nil
}

func scanVpsAssetRowMulti(rows *sql.Rows) (*VpsAssetRecord, error) {
	var a VpsAssetRecord
	if err := rows.Scan(
		&a.ID, &a.UserID, &a.Name,
		&a.IP, &a.SSHPort, &a.SSHUser,
		&a.SSHPassword, &a.SSHPrivateKey,
		&a.OS, &a.Location,
		&a.Provider, &a.Price, &a.Currency, &a.BillingCycle,
		&a.Bandwidth, &a.MonthlyTraffic,
		&a.CPU, &a.Memory, &a.Disk,
		&a.ExpireAt, &a.Notes,
		&a.AgentID, &a.Tags,
		&a.CreatedAt, &a.UpdatedAt,
		&a.DaysUntilExpiry, &a.Status,
	); err != nil {
		return nil, fmt.Errorf("scan vps asset: %w", err)
	}
	return &a, nil
}
