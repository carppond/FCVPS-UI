package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// NotificationChannelRecord is the storage projection of a notification_channels
// row. Per architecture §4.2 the table stores per-user opt-in channels with
// their channel-specific JSON config blob, a list of subscribed event types
// (TEXT JSON array) and an optional override template.
//
// Note: ConfigJSON is the raw on-disk JSON; callers (notify.Manager) deserialise
// it into the kind-specific struct.
type NotificationChannelRecord struct {
	ID         string
	UserID     string
	Kind       string
	Name       string
	ConfigJSON string
	Template   string
	EventTypes []string
	Enabled    bool
	CreatedAt  int64
	UpdatedAt  int64
}

// NotificationChannelListOptions narrows / paginates a List query.
type NotificationChannelListOptions struct {
	Page     int
	PageSize int
	Kind     string // optional exact filter
	Keyword  string // matched against name (LIKE %kw%)
}

// ErrNotificationChannelNotFound is the canonical not-found sentinel.
var ErrNotificationChannelNotFound = errors.New("storage: notification channel not found")

// NotificationChannelRepo encapsulates SQL access to notification_channels.
type NotificationChannelRepo struct {
	db  *DB
	now func() time.Time
}

// NewNotificationChannelRepo wires a repo to db. When now is nil, time.Now is
// used.
func NewNotificationChannelRepo(db *DB, now func() time.Time) *NotificationChannelRepo {
	if now == nil {
		now = time.Now
	}
	return &NotificationChannelRepo{db: db, now: now}
}

// Create inserts a new notification_channels row.
func (r *NotificationChannelRepo) Create(ctx context.Context, rec NotificationChannelRecord) (*NotificationChannelRecord, error) {
	if rec.ID == "" {
		return nil, fmt.Errorf("notify channel create: empty id")
	}
	if rec.UserID == "" || rec.Kind == "" || rec.Name == "" {
		return nil, fmt.Errorf("notify channel create: required field missing")
	}
	if rec.ConfigJSON == "" {
		rec.ConfigJSON = "{}"
	}
	now := r.now().UnixMilli()
	if rec.CreatedAt == 0 {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt == 0 {
		rec.UpdatedAt = now
	}
	eventsJSON := encodeStringArray(rec.EventTypes)
	enabled := 0
	if rec.Enabled {
		enabled = 1
	}
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO notification_channels(
			id, user_id, kind, name, config_json, template, event_types,
			enabled, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.UserID, rec.Kind, rec.Name, rec.ConfigJSON,
		rec.Template, eventsJSON, enabled, rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert notify channel: %w", err)
	}
	return &rec, nil
}

// GetByID returns a channel row scoped to userID. Cross-user GETs surface as
// ErrNotificationChannelNotFound (information hiding).
func (r *NotificationChannelRepo) GetByID(ctx context.Context, id, userID string) (*NotificationChannelRecord, error) {
	if id == "" || userID == "" {
		return nil, fmt.Errorf("notify channel get: empty id / user_id")
	}
	row := r.db.Read.QueryRowContext(ctx, selectNotifyChannelSQL+
		` WHERE id = ? AND user_id = ?`, id, userID)
	return scanNotifyChannel(row)
}

// ListByUser returns enabled channels for the given user, optionally filtered
// by event type opt-in. When eventType is empty, every enabled channel is
// returned regardless of opt-in. The filter is applied in Go (not SQL) so the
// JSON column does not need a generated-column index — channel counts per user
// are small (≤ 10 in v1).
func (r *NotificationChannelRepo) ListByUser(ctx context.Context, userID, eventType string) ([]NotificationChannelRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("notify channel list-by-user: empty user_id")
	}
	rows, err := r.db.Read.QueryContext(ctx,
		selectNotifyChannelSQL+` WHERE user_id = ? AND enabled = 1 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()
	out := make([]NotificationChannelRecord, 0, 4)
	for rows.Next() {
		rec, err := scanNotifyChannelMulti(rows)
		if err != nil {
			return nil, err
		}
		if eventType == "" || hasString(rec.EventTypes, eventType) {
			out = append(out, *rec)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channels: %w", err)
	}
	return out, nil
}

// ListAllByKind returns every channel of the given kind across every user.
// The Telegram bot's whitelist resolver (T-24) uses this to map an inbound
// chat ID to its owning user without a per-user fan-out. Channel counts are
// modest (≤ a few thousand for v1) so an unbounded scan is acceptable.
func (r *NotificationChannelRepo) ListAllByKind(ctx context.Context, kind string) ([]NotificationChannelRecord, error) {
	if kind == "" {
		return nil, fmt.Errorf("notify channel list-all: empty kind")
	}
	rows, err := r.db.Read.QueryContext(ctx,
		selectNotifyChannelSQL+` WHERE kind = ? ORDER BY created_at ASC`,
		kind,
	)
	if err != nil {
		return nil, fmt.Errorf("list channels by kind: %w", err)
	}
	defer rows.Close()
	out := make([]NotificationChannelRecord, 0, 16)
	for rows.Next() {
		rec, err := scanNotifyChannelMulti(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channels: %w", err)
	}
	return out, nil
}

// List paginates channels for a user, newest first.
func (r *NotificationChannelRepo) List(ctx context.Context, userID string, opts NotificationChannelListOptions) ([]NotificationChannelRecord, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("notify channel list: empty user_id")
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 50
	}
	if opts.PageSize > 200 {
		opts.PageSize = 200
	}
	args := []any{userID}
	where := []string{"user_id = ?"}
	if opts.Kind != "" {
		where = append(where, "kind = ?")
		args = append(args, opts.Kind)
	}
	if opts.Keyword != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+opts.Keyword+"%")
	}
	clause := " WHERE " + strings.Join(where, " AND ")
	var total int64
	if err := r.db.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM notification_channels"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count channels: %w", err)
	}
	offset := (opts.Page - 1) * opts.PageSize
	rows, err := r.db.Read.QueryContext(ctx,
		selectNotifyChannelSQL+clause+" ORDER BY created_at DESC LIMIT ? OFFSET ?",
		append(args, opts.PageSize, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()
	out := make([]NotificationChannelRecord, 0, opts.PageSize)
	for rows.Next() {
		rec, err := scanNotifyChannelMulti(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate channels: %w", err)
	}
	return out, total, nil
}

// Update overwrites mutable fields. Empty strings are treated as "leave
// unchanged"; the Enabled bool is unconditionally written (so callers must
// always supply the desired value). Returns ErrNotificationChannelNotFound
// when (id, user_id) does not exist.
func (r *NotificationChannelRepo) Update(ctx context.Context, rec NotificationChannelRecord) error {
	if rec.ID == "" || rec.UserID == "" {
		return fmt.Errorf("notify channel update: empty id / user_id")
	}
	cur, err := r.GetByID(ctx, rec.ID, rec.UserID)
	if err != nil {
		return err
	}
	if rec.Name != "" {
		cur.Name = rec.Name
	}
	if rec.ConfigJSON != "" {
		cur.ConfigJSON = rec.ConfigJSON
	}
	cur.Template = rec.Template
	if rec.EventTypes != nil {
		cur.EventTypes = rec.EventTypes
	}
	cur.Enabled = rec.Enabled
	now := r.now().UnixMilli()
	eventsJSON := encodeStringArray(cur.EventTypes)
	enabled := 0
	if cur.Enabled {
		enabled = 1
	}
	res, err := r.db.Write.ExecContext(ctx, `
		UPDATE notification_channels
		   SET name = ?, config_json = ?, template = ?, event_types = ?,
		       enabled = ?, updated_at = ?
		 WHERE id = ? AND user_id = ?`,
		cur.Name, cur.ConfigJSON, cur.Template, eventsJSON, enabled, now,
		rec.ID, rec.UserID,
	)
	if err != nil {
		return fmt.Errorf("update channel: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotificationChannelNotFound
	}
	return nil
}

// Delete removes the channel row. SET NULL on notification_events.channel_id
// (per migration 0001) preserves the audit trail.
func (r *NotificationChannelRepo) Delete(ctx context.Context, id, userID string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("notify channel delete: empty id / user_id")
	}
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM notification_channels WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotificationChannelNotFound
	}
	return nil
}

const selectNotifyChannelSQL = `SELECT id, user_id, kind, name, config_json,
		COALESCE(template, ''), event_types, enabled, created_at, updated_at
		FROM notification_channels`

func scanNotifyChannel(row *sql.Row) (*NotificationChannelRecord, error) {
	var rec NotificationChannelRecord
	var eventsJSON string
	var enabled int
	err := row.Scan(&rec.ID, &rec.UserID, &rec.Kind, &rec.Name, &rec.ConfigJSON,
		&rec.Template, &eventsJSON, &enabled, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotificationChannelNotFound
		}
		return nil, fmt.Errorf("scan channel: %w", err)
	}
	rec.EventTypes = decodeStringArray(eventsJSON)
	rec.Enabled = enabled == 1
	return &rec, nil
}

func scanNotifyChannelMulti(rows *sql.Rows) (*NotificationChannelRecord, error) {
	var rec NotificationChannelRecord
	var eventsJSON string
	var enabled int
	if err := rows.Scan(&rec.ID, &rec.UserID, &rec.Kind, &rec.Name, &rec.ConfigJSON,
		&rec.Template, &eventsJSON, &enabled, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan channel: %w", err)
	}
	rec.EventTypes = decodeStringArray(eventsJSON)
	rec.Enabled = enabled == 1
	return &rec, nil
}
