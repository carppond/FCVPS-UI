package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"shiguang-vps/internal/types"
)

// SubscriptionRecord is the storage projection of a subscriptions row.
//
// Tags is decoded from the TEXT JSON column into a []string for ergonomic
// callers; persistence is in canonical JSON-array form. ShareToken is the
// per-subscription bearer that fronts the sub-store v2 compat path
// (/download/:name?token=...) — see docs/05-tech-lead-plan.md §1.3.
type SubscriptionRecord struct {
	ID             string
	UserID         string
	Name           string
	Type           string
	SourceURL      string
	RawContent     []byte
	UA             string
	SyncInterval   int32
	LastSyncedAt   int64
	LastSyncStatus string
	LastSyncError  string
	ExpireAt       int64
	TrafficTotal   int64
	TrafficUsed    int64
	Tags           []string
	Remark         string
	ShareToken     string
	AllowInsecure  bool
	CreatedAt      int64
	UpdatedAt      int64
	// NodeCount is the live count of rows in nodes joined on subscription_id.
	// It is NOT a column in the subscriptions table — every read path computes
	// it via LEFT JOIN so list/detail responses reflect the real count (avoids
	// the drift / incremental-sync bookkeeping cost of caching it).
	NodeCount int32
}

// SubscriptionListOptions narrows / paginates a List query.
type SubscriptionListOptions struct {
	Page     int
	PageSize int
	Keyword  string // matched against name (LIKE %kw%)
	Type     string // optional exact filter
}

// Errors surfaced by SubscriptionRepo.
var (
	// ErrSubscriptionNotFound is returned when no row matches the lookup.
	ErrSubscriptionNotFound = errors.New("storage: subscription not found")
)

// SubscriptionRepo encapsulates SQL access to the subscriptions table.
type SubscriptionRepo struct {
	db  *DB
	now func() time.Time
}

// NewSubscriptionRepo wires a repo to db. When now is nil, time.Now is used.
func NewSubscriptionRepo(db *DB, now func() time.Time) *SubscriptionRepo {
	if now == nil {
		now = time.Now
	}
	return &SubscriptionRepo{db: db, now: now}
}

// Create inserts a new subscriptions row. ShareToken is generated when empty.
// Returns the persisted record (with id/timestamps/token filled in).
func (r *SubscriptionRepo) Create(ctx context.Context, rec SubscriptionRecord) (*SubscriptionRecord, error) {
	if rec.ID == "" {
		return nil, fmt.Errorf("subscription create: empty id")
	}
	if rec.UserID == "" || rec.Name == "" || rec.Type == "" {
		return nil, fmt.Errorf("subscription create: required field missing")
	}
	if !isValidSubType(rec.Type) {
		return nil, fmt.Errorf("subscription create: invalid type %q", rec.Type)
	}
	now := r.now().UnixMilli()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt == 0 {
		rec.UpdatedAt = now
	}
	if rec.SyncInterval <= 0 {
		rec.SyncInterval = 21600
	}
	if rec.ShareToken == "" {
		token, err := generateShareToken()
		if err != nil {
			return nil, fmt.Errorf("subscription create: %w", err)
		}
		rec.ShareToken = token
	}
	tagsJSON, err := encodeTags(rec.Tags)
	if err != nil {
		return nil, fmt.Errorf("subscription create: %w", err)
	}
	_, err = r.db.Write.ExecContext(ctx, `
		INSERT INTO subscriptions(
			id, user_id, name, type, source_url, raw_content, ua,
			sync_interval, last_synced_at, last_sync_status, last_sync_error,
			expire_at, traffic_total, traffic_used, tags, remark,
			share_token, created_at, updated_at, allow_insecure
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.UserID, rec.Name, rec.Type,
		nullableString(rec.SourceURL), nullableBytes(rec.RawContent), nullableString(rec.UA),
		rec.SyncInterval, nullableInt64(rec.LastSyncedAt),
		nullableString(rec.LastSyncStatus), nullableString(rec.LastSyncError),
		nullableInt64(rec.ExpireAt), nullableInt64(rec.TrafficTotal), nullableInt64(rec.TrafficUsed),
		tagsJSON, nullableString(rec.Remark),
		rec.ShareToken, rec.CreatedAt, rec.UpdatedAt, rec.AllowInsecure,
	)
	if err != nil {
		return nil, fmt.Errorf("insert subscription: %w", err)
	}
	return &rec, nil
}

// GetByID returns the subscription identified by id.
//
// When userID is non-empty the row is filtered by user_id so cross-user reads
// 404 instead of leaking data; callers acting as admin should pass "" to
// bypass the check.
func (r *SubscriptionRepo) GetByID(ctx context.Context, id, userID string) (*SubscriptionRecord, error) {
	if id == "" {
		return nil, ErrSubscriptionNotFound
	}
	query := selectSubscriptionSQL + " WHERE id = ?"
	args := []any{id}
	if userID != "" {
		query += " AND user_id = ?"
		args = append(args, userID)
	}
	row := r.db.Read.QueryRowContext(ctx, query, args...)
	return scanSubscriptionRow(row)
}

// GetByName resolves a subscription by (user_id, name). Used by the sub-store
// compat handler which looks up by the path segment.
func (r *SubscriptionRepo) GetByName(ctx context.Context, userID, name string) (*SubscriptionRecord, error) {
	if userID == "" || name == "" {
		return nil, ErrSubscriptionNotFound
	}
	row := r.db.Read.QueryRowContext(ctx,
		selectSubscriptionSQL+" WHERE user_id = ? AND name = ?", userID, name)
	return scanSubscriptionRow(row)
}

// GetByShareToken resolves a subscription by its share_token. Used by the
// sub-store compat path which does not have an authenticated session.
func (r *SubscriptionRepo) GetByShareToken(ctx context.Context, token string) (*SubscriptionRecord, error) {
	if token == "" {
		return nil, ErrSubscriptionNotFound
	}
	row := r.db.Read.QueryRowContext(ctx,
		selectSubscriptionSQL+" WHERE share_token = ?", token)
	return scanSubscriptionRow(row)
}

// List paginates subscriptions for userID with optional filters.
func (r *SubscriptionRepo) List(ctx context.Context, userID string, opts SubscriptionListOptions) ([]SubscriptionRecord, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("subscription list: empty user id")
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
	if opts.Type != "" {
		if !isValidSubType(opts.Type) {
			return nil, 0, fmt.Errorf("subscription list: invalid type %q", opts.Type)
		}
		where = append(where, "type = ?")
		args = append(args, opts.Type)
	}
	clause := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.Read.QueryRowContext(ctx, "SELECT COUNT(*) FROM subscriptions"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count subscriptions: %w", err)
	}
	offset := (opts.Page - 1) * opts.PageSize
	rows, err := r.db.Read.QueryContext(ctx,
		selectSubscriptionSQL+clause+" ORDER BY created_at DESC LIMIT ? OFFSET ?",
		append(args, opts.PageSize, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()
	out := make([]SubscriptionRecord, 0, opts.PageSize)
	for rows.Next() {
		rec, err := scanSubscriptionRowMulti(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate subscriptions: %w", err)
	}
	return out, total, nil
}

// SubscriptionUpdate captures the fields a PATCH-style update may touch. Nil
// pointers leave the column untouched; non-nil values overwrite. Tags uses a
// pointer to allow distinguishing "clear" (empty slice) from "leave alone"
// (nil pointer).
type SubscriptionUpdate struct {
	Name          *string
	SourceURL     *string
	UA            *string
	SyncInterval  *int32
	Tags          *[]string
	Remark        *string
	AllowInsecure *bool
}

// Update applies a partial change to the subscription identified by (id, userID).
// Returns ErrSubscriptionNotFound when nothing matched.
func (r *SubscriptionRepo) Update(ctx context.Context, id, userID string, upd SubscriptionUpdate) error {
	if id == "" {
		return fmt.Errorf("subscription update: empty id")
	}
	sets := []string{}
	args := []any{}
	if upd.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *upd.Name)
	}
	if upd.SourceURL != nil {
		sets = append(sets, "source_url = ?")
		args = append(args, nullableString(*upd.SourceURL))
	}
	if upd.UA != nil {
		sets = append(sets, "ua = ?")
		args = append(args, nullableString(*upd.UA))
	}
	if upd.SyncInterval != nil {
		if *upd.SyncInterval <= 0 {
			return fmt.Errorf("subscription update: sync_interval must be positive")
		}
		sets = append(sets, "sync_interval = ?")
		args = append(args, *upd.SyncInterval)
	}
	if upd.Tags != nil {
		tagsJSON, err := encodeTags(*upd.Tags)
		if err != nil {
			return fmt.Errorf("subscription update: %w", err)
		}
		sets = append(sets, "tags = ?")
		args = append(args, tagsJSON)
	}
	if upd.Remark != nil {
		sets = append(sets, "remark = ?")
		args = append(args, nullableString(*upd.Remark))
	}
	if upd.AllowInsecure != nil {
		sets = append(sets, "allow_insecure = ?")
		args = append(args, *upd.AllowInsecure)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, r.now().UnixMilli(), id)
	stmt := "UPDATE subscriptions SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	if userID != "" {
		stmt += " AND user_id = ?"
		args = append(args, userID)
	}
	res, err := r.db.Write.ExecContext(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSubscriptionNotFound
	}
	return nil
}

// Delete removes the row (cascades to nodes via FK ON DELETE CASCADE).
func (r *SubscriptionRepo) Delete(ctx context.Context, id, userID string) error {
	if id == "" {
		return fmt.Errorf("subscription delete: empty id")
	}
	stmt := "DELETE FROM subscriptions WHERE id = ?"
	args := []any{id}
	if userID != "" {
		stmt += " AND user_id = ?"
		args = append(args, userID)
	}
	res, err := r.db.Write.ExecContext(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSubscriptionNotFound
	}
	return nil
}

// UpdateSyncState mirrors a completed (or failed) sync attempt.
//
// status is the canonical SyncStatus value ("ok"/"error"/"pending"); lastErr
// may be empty for success. The function also bumps updated_at so listings
// stay coherent.
func (r *SubscriptionRepo) UpdateSyncState(ctx context.Context, id, status string, lastSyncedAt time.Time, lastErr string) error {
	if id == "" {
		return fmt.Errorf("update sync state: empty id")
	}
	if status != "" && !isValidSyncStatus(status) {
		return fmt.Errorf("update sync state: invalid status %q", status)
	}
	now := r.now().UnixMilli()
	syncedMs := int64(0)
	if !lastSyncedAt.IsZero() {
		syncedMs = lastSyncedAt.UnixMilli()
	}
	res, err := r.db.Write.ExecContext(ctx, `
		UPDATE subscriptions
		   SET last_synced_at = ?, last_sync_status = ?, last_sync_error = ?, updated_at = ?
		 WHERE id = ?`,
		nullableInt64(syncedMs), nullableString(status), nullableString(lastErr), now, id,
	)
	if err != nil {
		return fmt.Errorf("update sync state: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSubscriptionNotFound
	}
	return nil
}

// UpdateRawContent persists the most recently fetched raw subscription body.
// Used by the sync service so /api/subscriptions/:id/raw can replay it later.
func (r *SubscriptionRepo) UpdateRawContent(ctx context.Context, id string, raw []byte) error {
	if id == "" {
		return fmt.Errorf("update raw: empty id")
	}
	res, err := r.db.Write.ExecContext(ctx,
		"UPDATE subscriptions SET raw_content = ?, updated_at = ? WHERE id = ?",
		nullableBytes(raw), r.now().UnixMilli(), id)
	if err != nil {
		return fmt.Errorf("update raw_content: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSubscriptionNotFound
	}
	return nil
}

// RotateShareToken regenerates the per-subscription share_token. The new value
// is returned so the caller can hand it back to the UI in the same response.
func (r *SubscriptionRepo) RotateShareToken(ctx context.Context, id, userID string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("rotate share token: empty id")
	}
	token, err := generateShareToken()
	if err != nil {
		return "", fmt.Errorf("rotate share token: %w", err)
	}
	stmt := "UPDATE subscriptions SET share_token = ?, updated_at = ? WHERE id = ?"
	args := []any{token, r.now().UnixMilli(), id}
	if userID != "" {
		stmt += " AND user_id = ?"
		args = append(args, userID)
	}
	res, err := r.db.Write.ExecContext(ctx, stmt, args...)
	if err != nil {
		return "", fmt.Errorf("rotate share token: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", ErrSubscriptionNotFound
	}
	return token, nil
}

// GetRawContent reads the raw_content blob. Returns nil when the column is
// NULL (e.g. type=manual subscriptions).
func (r *SubscriptionRepo) GetRawContent(ctx context.Context, id, userID string) ([]byte, error) {
	if id == "" {
		return nil, ErrSubscriptionNotFound
	}
	query := "SELECT raw_content FROM subscriptions WHERE id = ?"
	args := []any{id}
	if userID != "" {
		query += " AND user_id = ?"
		args = append(args, userID)
	}
	var raw []byte
	err := r.db.Read.QueryRowContext(ctx, query, args...).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("read raw_content: %w", err)
	}
	return raw, nil
}

// selectSubscriptionSQL is the shared SELECT prefix used by every read path.
// Column order must match scanSubscriptionRow.
//
// node_count is computed in a single grouped sub-query joined once per result
// row rather than a correlated sub-query, so the cost stays O(N+M) instead of
// O(N*M) on the (subscriptions, nodes) cartesian. Storing the count in the
// subscriptions table was considered and rejected — incremental sync would
// have to maintain the cache on every UpsertBatch / cascade-delete, and any
// missed bookkeeping shows up as a permanently wrong UI number (see
// commit history of this fix).
const selectSubscriptionSQL = `SELECT s.id, s.user_id, s.name, s.type,
		COALESCE(s.source_url,''), s.raw_content, COALESCE(s.ua,''),
		s.sync_interval, COALESCE(s.last_synced_at,0),
		COALESCE(s.last_sync_status,''), COALESCE(s.last_sync_error,''),
		COALESCE(s.expire_at,0), COALESCE(s.traffic_total,0), COALESCE(s.traffic_used,0),
		s.tags, COALESCE(s.remark,''), COALESCE(s.share_token,''),
		s.created_at, s.updated_at, COALESCE(s.allow_insecure,0),
		COALESCE(n.cnt, 0) AS node_count
	FROM subscriptions s
	LEFT JOIN (
		SELECT subscription_id, COUNT(*) AS cnt
		FROM nodes
		GROUP BY subscription_id
	) n ON n.subscription_id = s.id`

// scanSubscriptionRow consumes a single QueryRow result.
func scanSubscriptionRow(row *sql.Row) (*SubscriptionRecord, error) {
	var rec SubscriptionRecord
	var raw []byte
	var tagsJSON string
	var allowInsecure int64
	if err := row.Scan(
		&rec.ID, &rec.UserID, &rec.Name, &rec.Type,
		&rec.SourceURL, &raw, &rec.UA,
		&rec.SyncInterval, &rec.LastSyncedAt,
		&rec.LastSyncStatus, &rec.LastSyncError,
		&rec.ExpireAt, &rec.TrafficTotal, &rec.TrafficUsed,
		&tagsJSON, &rec.Remark, &rec.ShareToken,
		&rec.CreatedAt, &rec.UpdatedAt, &allowInsecure,
		&rec.NodeCount,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, fmt.Errorf("scan subscription: %w", err)
	}
	if len(raw) > 0 {
		rec.RawContent = raw
	}
	rec.AllowInsecure = allowInsecure != 0
	tags, err := decodeTags(tagsJSON)
	if err != nil {
		return nil, fmt.Errorf("decode tags: %w", err)
	}
	rec.Tags = tags
	return &rec, nil
}

// scanSubscriptionRowMulti is the rows.Next variant used by List.
func scanSubscriptionRowMulti(rows *sql.Rows) (*SubscriptionRecord, error) {
	var rec SubscriptionRecord
	var raw []byte
	var tagsJSON string
	var allowInsecure int64
	if err := rows.Scan(
		&rec.ID, &rec.UserID, &rec.Name, &rec.Type,
		&rec.SourceURL, &raw, &rec.UA,
		&rec.SyncInterval, &rec.LastSyncedAt,
		&rec.LastSyncStatus, &rec.LastSyncError,
		&rec.ExpireAt, &rec.TrafficTotal, &rec.TrafficUsed,
		&tagsJSON, &rec.Remark, &rec.ShareToken,
		&rec.CreatedAt, &rec.UpdatedAt, &allowInsecure,
		&rec.NodeCount,
	); err != nil {
		return nil, fmt.Errorf("scan subscription: %w", err)
	}
	if len(raw) > 0 {
		rec.RawContent = raw
	}
	rec.AllowInsecure = allowInsecure != 0
	tags, err := decodeTags(tagsJSON)
	if err != nil {
		return nil, fmt.Errorf("decode tags: %w", err)
	}
	rec.Tags = tags
	return &rec, nil
}

// encodeTags marshals tags to JSON, returning "[]" for nil/empty inputs.
func encodeTags(tags []string) (string, error) {
	if len(tags) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "", fmt.Errorf("marshal tags: %w", err)
	}
	return string(b), nil
}

// decodeTags parses the persisted JSON-array string into a slice. An empty
// string or "[]" yields an empty slice (never nil) so JSON responses are
// stable.
func decodeTags(raw string) ([]string, error) {
	if raw == "" || raw == "[]" {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}
	if out == nil {
		return []string{}, nil
	}
	return out, nil
}

// generateShareToken returns a 32-byte base64url-encoded random string
// suitable for sub-store v2 compat URLs. Length is 43 chars (no padding).
func generateShareToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

// isValidSubType matches the CHECK constraint in 0001_initial.sql.
func isValidSubType(t string) bool {
	switch t {
	case string(types.SubTypeURL), string(types.SubTypeUpload), string(types.SubTypeManual):
		return true
	}
	return false
}

// isValidSyncStatus mirrors the SyncStatus enum allowed in DB column.
func isValidSyncStatus(s string) bool {
	switch s {
	case string(types.SyncStatusOK), string(types.SyncStatusError), string(types.SyncStatusPending):
		return true
	}
	return false
}

// nullableInt64 converts 0 into sql NULL.
func nullableInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// nullableBytes converts an empty slice into sql NULL.
func nullableBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
