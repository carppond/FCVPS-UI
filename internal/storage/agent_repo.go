package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// AgentRecord is the storage projection of an agents row.
//
// PublicIP / Version / OS / Arch / LastSeenAt are nullable in the schema; we
// surface them as empty/zero values for ergonomic consumers. Token is never
// stored in plaintext — only TokenHash (sha256 hex of the raw bearer token).
type AgentRecord struct {
	ID         string
	UserID     string
	Name       string
	TokenHash  string
	Kind       string // "native" | "nezha_compat"
	Version    string
	OS         string
	Arch       string
	PublicIP   string
	LastSeenAt int64
	Status     string // "online" | "offline" | "degraded"
	CreatedAt  int64
	UpdatedAt  int64

	// Per-agent monthly traffic quota (0009). TrafficLimit is operator-entered
	// (bytes); the Bwg* fields are BandwagonHost auto-fetch config + cache.
	TrafficLimit int64
	BwgVeid      string
	BwgAPIKey    string
	BwgUsed      int64
	BwgLimit     int64
	BwgResetAt   int64
	BwgSyncedAt  int64
}

// AgentListOptions narrows the List query.
type AgentListOptions struct {
	Page     int
	PageSize int
	Keyword  string // matched against name (LIKE %kw%)
}

// Errors surfaced by AgentRepo.
var (
	// ErrAgentNotFound is returned when no row matches the lookup or when a
	// cross-user lookup is attempted.
	ErrAgentNotFound = errors.New("storage: agent not found")
)

// AgentRepo encapsulates SQL access to the agents table.
type AgentRepo struct {
	db  *DB
	now func() time.Time
}

// NewAgentRepo wires a repo to db. When now is nil, time.Now is used.
func NewAgentRepo(db *DB, now func() time.Time) *AgentRepo {
	if now == nil {
		now = time.Now
	}
	return &AgentRepo{db: db, now: now}
}

// Create inserts a new agents row. The supplied record's ID, UserID, Name,
// TokenHash and Kind must be populated; CreatedAt / UpdatedAt are normalised
// to the current wall clock when zero. Status defaults to "offline".
func (r *AgentRepo) Create(ctx context.Context, rec AgentRecord) (*AgentRecord, error) {
	if rec.ID == "" || rec.UserID == "" || rec.Name == "" || rec.TokenHash == "" {
		return nil, fmt.Errorf("agent create: required field missing")
	}
	if rec.Kind == "" {
		rec.Kind = "native"
	}
	if rec.Kind != "native" && rec.Kind != "nezha_compat" {
		return nil, fmt.Errorf("agent create: invalid kind %q", rec.Kind)
	}
	if rec.Status == "" {
		rec.Status = "offline"
	}
	now := r.now().UnixMilli()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt == 0 {
		rec.UpdatedAt = now
	}
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO agents(id, user_id, name, token_hash, kind, version, os, arch,
		                   public_ip, last_seen_at, status, created_at, updated_at,
		                   traffic_limit, bwg_veid, bwg_api_key)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.UserID, rec.Name, rec.TokenHash, rec.Kind,
		nullableString(rec.Version), nullableString(rec.OS), nullableString(rec.Arch),
		nullableString(rec.PublicIP), nullableInt64(rec.LastSeenAt),
		rec.Status, rec.CreatedAt, rec.UpdatedAt,
		rec.TrafficLimit, rec.BwgVeid, rec.BwgAPIKey,
	)
	if err != nil {
		return nil, fmt.Errorf("insert agent: %w", err)
	}
	return &rec, nil
}

// GetByID returns the agents row identified by id, but only when it belongs to
// the supplied userID. Cross-user access yields ErrAgentNotFound to avoid
// information disclosure.
//
// userID may be empty when the caller is an admin / hub-internal flow (e.g.
// the WebSocket hub does not yet have a user context). Pass "" to skip the
// user-scope check.
func (r *AgentRepo) GetByID(ctx context.Context, id, userID string) (*AgentRecord, error) {
	if id == "" {
		return nil, ErrAgentNotFound
	}
	q := selectAgentSQL + " WHERE id = ?"
	args := []any{id}
	if userID != "" {
		q += " AND user_id = ?"
		args = append(args, userID)
	}
	row := r.db.Read.QueryRowContext(ctx, q, args...)
	return scanAgentRow(row)
}

// GetByTokenHash resolves an agent by sha256(token) hex. Used by the WS hub at
// handshake time. Returns ErrAgentNotFound when no row matches.
func (r *AgentRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*AgentRecord, error) {
	if tokenHash == "" {
		return nil, ErrAgentNotFound
	}
	row := r.db.Read.QueryRowContext(ctx,
		selectAgentSQL+" WHERE token_hash = ?", tokenHash)
	return scanAgentRow(row)
}

// ListByUser paginates agents owned by userID.
func (r *AgentRepo) ListByUser(ctx context.Context, userID string, opts AgentListOptions) ([]AgentRecord, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("agent list: empty user id")
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
	if opts.Keyword != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+opts.Keyword+"%")
	}
	clause := " WHERE " + strings.Join(where, " AND ")
	var total int64
	if err := r.db.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM agents"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count agents: %w", err)
	}
	offset := (opts.Page - 1) * opts.PageSize
	rows, err := r.db.Read.QueryContext(ctx,
		selectAgentSQL+clause+" ORDER BY created_at DESC LIMIT ? OFFSET ?",
		append(args, opts.PageSize, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()
	out := make([]AgentRecord, 0, opts.PageSize)
	for rows.Next() {
		a, err := scanAgentRowMulti(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate agents: %w", err)
	}
	return out, total, nil
}

// ListAll returns every agent row regardless of owner. Used by /api/admin/agents
// and by the hub on startup (to reset stale "online" rows to "offline").
func (r *AgentRepo) ListAll(ctx context.Context) ([]AgentRecord, error) {
	rows, err := r.db.Read.QueryContext(ctx,
		selectAgentSQL+" ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list all agents: %w", err)
	}
	defer rows.Close()
	out := make([]AgentRecord, 0)
	for rows.Next() {
		a, err := scanAgentRowMulti(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agents: %w", err)
	}
	return out, nil
}

// UpdateLastSeen stamps last_seen_at + status (online/offline/degraded) and
// optionally refreshes version / os / arch (omitted when empty so heartbeat
// updates do not clobber the original handshake values).
func (r *AgentRepo) UpdateLastSeen(ctx context.Context, id, status, version, osName, arch string) error {
	if id == "" {
		return fmt.Errorf("update last seen: empty id")
	}
	if status != "" && status != "online" && status != "offline" && status != "degraded" {
		return fmt.Errorf("update last seen: invalid status %q", status)
	}
	now := r.now().UnixMilli()
	sets := []string{"last_seen_at = ?"}
	args := []any{now}
	if status != "" {
		sets = append(sets, "status = ?")
		args = append(args, status)
	}
	if version != "" {
		sets = append(sets, "version = ?")
		args = append(args, version)
	}
	if osName != "" {
		sets = append(sets, "os = ?")
		args = append(args, osName)
	}
	if arch != "" {
		sets = append(sets, "arch = ?")
		args = append(args, arch)
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, now, id)
	stmt := "UPDATE agents SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	res, err := r.db.Write.ExecContext(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("update last seen: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrAgentNotFound
	}
	return nil
}

// UpdateStatus changes only the status column + last_seen_at (used by the
// offline detector and graceful shutdown).
func (r *AgentRepo) UpdateStatus(ctx context.Context, id, status string) error {
	return r.UpdateLastSeen(ctx, id, status, "", "", "")
}

// UpdateProfile changes name (Tech Lead task §C: PUT /api/agents/:id supports
// name updates).
func (r *AgentRepo) UpdateProfile(ctx context.Context, id, userID, name string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("update profile: empty id/user")
	}
	if name == "" {
		return nil
	}
	now := r.now().UnixMilli()
	res, err := r.db.Write.ExecContext(ctx,
		"UPDATE agents SET name = ?, updated_at = ? WHERE id = ? AND user_id = ?",
		name, now, id, userID)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrAgentNotFound
	}
	return nil
}

// UpdateTrafficConfig sets the per-agent traffic quota config. traffic_limit and
// bwg_veid are written verbatim; apiKey is only written when non-empty so the UI
// can leave the field blank to keep the stored key. Clearing bwg_veid also
// resets the cached provider figures so stale data does not linger.
// Returns ErrAgentNotFound when no (id, userID) row matches.
func (r *AgentRepo) UpdateTrafficConfig(ctx context.Context, id, userID string, limit int64, veid, apiKey string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("update traffic config: empty id/user")
	}
	now := r.now().UnixMilli()
	sets := []string{"traffic_limit = ?", "bwg_veid = ?", "updated_at = ?"}
	args := []any{limit, veid, now}
	if apiKey != "" {
		sets = append(sets, "bwg_api_key = ?")
		args = append(args, apiKey)
	}
	if veid == "" {
		sets = append(sets, "bwg_api_key = ''", "bwg_used = 0", "bwg_limit = 0",
			"bwg_reset_at = 0", "bwg_synced_at = 0")
	}
	args = append(args, id, userID)
	res, err := r.db.Write.ExecContext(ctx,
		"UPDATE agents SET "+strings.Join(sets, ", ")+" WHERE id = ? AND user_id = ?",
		args...)
	if err != nil {
		return fmt.Errorf("update traffic config: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrAgentNotFound
	}
	return nil
}

// RotateToken atomically updates token_hash + updated_at. Caller is responsible
// for generating + hashing the new token.
func (r *AgentRepo) RotateToken(ctx context.Context, id, userID, newTokenHash string) error {
	if id == "" || userID == "" || newTokenHash == "" {
		return fmt.Errorf("rotate token: required field missing")
	}
	now := r.now().UnixMilli()
	res, err := r.db.Write.ExecContext(ctx,
		"UPDATE agents SET token_hash = ?, updated_at = ? WHERE id = ? AND user_id = ?",
		newTokenHash, now, id, userID)
	if err != nil {
		return fmt.Errorf("rotate token: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrAgentNotFound
	}
	return nil
}

// Delete removes an agent row (foreign keys cascade to agent_records). Cross-
// user attempts return ErrAgentNotFound (information hiding).
func (r *AgentRepo) Delete(ctx context.Context, id, userID string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("delete agent: empty id/user")
	}
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM agents WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrAgentNotFound
	}
	return nil
}

// MarkAllOffline sets status='offline' for every row. Called on hub startup so
// stale "online" entries left over from a crashed process do not falsely report
// availability.
func (r *AgentRepo) MarkAllOffline(ctx context.Context) error {
	now := r.now().UnixMilli()
	_, err := r.db.Write.ExecContext(ctx,
		"UPDATE agents SET status = 'offline', updated_at = ? WHERE status != 'offline'",
		now)
	if err != nil {
		return fmt.Errorf("mark agents offline: %w", err)
	}
	return nil
}

// selectAgentSQL is the shared SELECT prefix.
const selectAgentSQL = `SELECT id, user_id, name, token_hash, kind,
		COALESCE(version,''), COALESCE(os,''), COALESCE(arch,''),
		COALESCE(public_ip,''), COALESCE(last_seen_at,0),
		status, created_at, updated_at,
		COALESCE(traffic_limit,0), COALESCE(bwg_veid,''), COALESCE(bwg_api_key,''),
		COALESCE(bwg_used,0), COALESCE(bwg_limit,0),
		COALESCE(bwg_reset_at,0), COALESCE(bwg_synced_at,0) FROM agents`

func scanAgentRow(row *sql.Row) (*AgentRecord, error) {
	var a AgentRecord
	err := row.Scan(&a.ID, &a.UserID, &a.Name, &a.TokenHash, &a.Kind,
		&a.Version, &a.OS, &a.Arch, &a.PublicIP, &a.LastSeenAt,
		&a.Status, &a.CreatedAt, &a.UpdatedAt,
		&a.TrafficLimit, &a.BwgVeid, &a.BwgAPIKey,
		&a.BwgUsed, &a.BwgLimit, &a.BwgResetAt, &a.BwgSyncedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("scan agent: %w", err)
	}
	return &a, nil
}

func scanAgentRowMulti(rows *sql.Rows) (*AgentRecord, error) {
	var a AgentRecord
	if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.TokenHash, &a.Kind,
		&a.Version, &a.OS, &a.Arch, &a.PublicIP, &a.LastSeenAt,
		&a.Status, &a.CreatedAt, &a.UpdatedAt,
		&a.TrafficLimit, &a.BwgVeid, &a.BwgAPIKey,
		&a.BwgUsed, &a.BwgLimit, &a.BwgResetAt, &a.BwgSyncedAt); err != nil {
		return nil, fmt.Errorf("scan agent: %w", err)
	}
	return &a, nil
}

// nullableInt64 lives in subscription_repo.go; this file reuses the
// package-level helper rather than redeclaring it.
