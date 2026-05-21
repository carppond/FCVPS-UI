package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// NotificationEventRecord is the storage projection of a notification_events
// row. PayloadJSON is opaque — callers serialise / deserialise via the
// notify.Manager's event payload structs.
type NotificationEventRecord struct {
	ID          int64
	UserID      string
	ChannelID   string
	EventType   string
	DedupeKey   string
	PayloadJSON string
	Status      string
	SentAt      int64
	Error       string
	CreatedAt   int64
}

// NotificationEventListOptions narrows / paginates a list query.
type NotificationEventListOptions struct {
	Page      int
	PageSize  int
	EventType string // optional exact filter
	Status    string // optional exact filter
}

// ErrNotificationEventNotFound is returned by lookups against a missing row.
var ErrNotificationEventNotFound = errors.New("storage: notification event not found")

// NotificationEventRepo encapsulates SQL access to notification_events.
type NotificationEventRepo struct {
	db  *DB
	now func() time.Time
}

// NewNotificationEventRepo wires a repo to db. When now is nil, time.Now is
// used.
func NewNotificationEventRepo(db *DB, now func() time.Time) *NotificationEventRepo {
	if now == nil {
		now = time.Now
	}
	return &NotificationEventRepo{db: db, now: now}
}

// Insert appends a delivery log row. The returned record has the assigned
// AUTOINCREMENT id populated.
func (r *NotificationEventRepo) Insert(ctx context.Context, rec NotificationEventRecord) (*NotificationEventRecord, error) {
	if rec.UserID == "" || rec.EventType == "" || rec.Status == "" {
		return nil, fmt.Errorf("notify event insert: required field missing")
	}
	if rec.PayloadJSON == "" {
		rec.PayloadJSON = "{}"
	}
	if rec.CreatedAt == 0 {
		rec.CreatedAt = r.now().UnixMilli()
	}
	var channelArg any
	if rec.ChannelID != "" {
		channelArg = rec.ChannelID
	}
	var sentAtArg any
	if rec.SentAt != 0 {
		sentAtArg = rec.SentAt
	}
	var errorArg any
	if rec.Error != "" {
		errorArg = rec.Error
	}
	var dedupeArg any
	if rec.DedupeKey != "" {
		dedupeArg = rec.DedupeKey
	}
	res, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO notification_events(user_id, channel_id, event_type,
			dedupe_key, payload, status, sent_at, error, created_at)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		rec.UserID, channelArg, rec.EventType, dedupeArg, rec.PayloadJSON,
		rec.Status, sentAtArg, errorArg, rec.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert notify event: %w", err)
	}
	id, _ := res.LastInsertId()
	rec.ID = id
	return &rec, nil
}

// ListByUser paginates a user's events newest-first.
func (r *NotificationEventRepo) ListByUser(ctx context.Context, userID string, opts NotificationEventListOptions) ([]NotificationEventRecord, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("notify event list-by-user: empty user_id")
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
	if opts.EventType != "" {
		where = append(where, "event_type = ?")
		args = append(args, opts.EventType)
	}
	if opts.Status != "" {
		where = append(where, "status = ?")
		args = append(args, opts.Status)
	}
	clause := " WHERE " + strings.Join(where, " AND ")
	var total int64
	if err := r.db.Read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM notification_events"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}
	offset := (opts.Page - 1) * opts.PageSize
	rows, err := r.db.Read.QueryContext(ctx,
		selectNotifyEventSQL+clause+" ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?",
		append(args, opts.PageSize, offset)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	out := make([]NotificationEventRecord, 0, opts.PageSize)
	for rows.Next() {
		rec, err := scanNotifyEventMulti(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate events: %w", err)
	}
	return out, total, nil
}

// CountRecentByDedupeKey counts events whose dedupe_key matches `key` and
// whose created_at is within the window ending at `now`. Used by the in-DB
// fallback path of notify.Dedupe so a restart of the hub does not lose the
// 5-minute window for already-fired events.
func (r *NotificationEventRepo) CountRecentByDedupeKey(ctx context.Context, key string, sinceMs int64) (int64, error) {
	if key == "" {
		return 0, nil
	}
	var n int64
	err := r.db.Read.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_events
		 WHERE dedupe_key = ? AND created_at >= ?`,
		key, sinceMs,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count dedupe events: %w", err)
	}
	return n, nil
}

// DeleteOlderThan removes events whose created_at is < cutoffMs. Returns the
// number of rows removed. Used by the 30-day cleanup task.
func (r *NotificationEventRepo) DeleteOlderThan(ctx context.Context, cutoffMs int64) (int64, error) {
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM notification_events WHERE created_at < ?", cutoffMs)
	if err != nil {
		return 0, fmt.Errorf("delete old events: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

const selectNotifyEventSQL = `SELECT id, user_id, COALESCE(channel_id,''),
		event_type, COALESCE(dedupe_key,''), payload, status,
		COALESCE(sent_at,0), COALESCE(error,''), created_at
		FROM notification_events`

func scanNotifyEventMulti(rows *sql.Rows) (*NotificationEventRecord, error) {
	var rec NotificationEventRecord
	if err := rows.Scan(&rec.ID, &rec.UserID, &rec.ChannelID, &rec.EventType,
		&rec.DedupeKey, &rec.PayloadJSON, &rec.Status, &rec.SentAt,
		&rec.Error, &rec.CreatedAt); err != nil {
		return nil, fmt.Errorf("scan event: %w", err)
	}
	return &rec, nil
}
