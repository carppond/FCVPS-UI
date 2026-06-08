package storage

import (
	"context"
	"fmt"
	"time"

	"shiguang-vps/internal/util"
)

// syncLogKeepPerSub caps how many history rows are retained per subscription;
// older rows are pruned on each insert so the table stays bounded.
const syncLogKeepPerSub = 50

// SubscriptionSyncLogRecord is one sync attempt's outcome.
type SubscriptionSyncLogRecord struct {
	ID             string
	SubscriptionID string
	UserID         string
	Status         string // ok | error
	NodeCount      int
	Error          string
	CreatedAt      int64
}

// SubscriptionSyncLogRepo stores per-subscription sync history.
type SubscriptionSyncLogRepo struct {
	db  *DB
	now func() time.Time
}

// NewSubscriptionSyncLogRepo constructs the repo. now defaults to time.Now.
func NewSubscriptionSyncLogRepo(db *DB, now func() time.Time) *SubscriptionSyncLogRepo {
	if now == nil {
		now = time.Now
	}
	return &SubscriptionSyncLogRepo{db: db, now: now}
}

// Record appends a sync-log row and prunes rows beyond syncLogKeepPerSub for
// that subscription. Best-effort: pruning failure is not fatal.
func (r *SubscriptionSyncLogRepo) Record(ctx context.Context, rec SubscriptionSyncLogRecord) error {
	if rec.SubscriptionID == "" || rec.UserID == "" || rec.Status == "" {
		return fmt.Errorf("sync log record: subscription_id/user_id/status required")
	}
	if rec.ID == "" {
		rec.ID = util.UUIDv7()
	}
	if rec.CreatedAt == 0 {
		rec.CreatedAt = r.now().UnixMilli()
	}
	if _, err := r.db.Write.ExecContext(ctx,
		`INSERT INTO subscription_sync_logs (id, subscription_id, user_id, status, node_count, error, created_at)
		 VALUES (?,?,?,?,?,?,?)`,
		rec.ID, rec.SubscriptionID, rec.UserID, rec.Status, rec.NodeCount,
		nullableString(rec.Error), rec.CreatedAt); err != nil {
		return fmt.Errorf("insert sync log: %w", err)
	}
	// Prune: keep only the newest syncLogKeepPerSub rows for this subscription.
	_, _ = r.db.Write.ExecContext(ctx,
		`DELETE FROM subscription_sync_logs
		  WHERE subscription_id = ?
		    AND id NOT IN (
		      SELECT id FROM subscription_sync_logs
		       WHERE subscription_id = ?
		       ORDER BY created_at DESC LIMIT ?
		    )`,
		rec.SubscriptionID, rec.SubscriptionID, syncLogKeepPerSub)
	return nil
}

// ListBySubscription returns recent logs for a subscription owned by userID,
// newest first. limit is clamped to [1, syncLogKeepPerSub].
func (r *SubscriptionSyncLogRepo) ListBySubscription(ctx context.Context, subID, userID string, limit int) ([]SubscriptionSyncLogRecord, error) {
	if subID == "" || userID == "" {
		return nil, fmt.Errorf("sync log list: subscription_id/user_id required")
	}
	if limit <= 0 || limit > syncLogKeepPerSub {
		limit = syncLogKeepPerSub
	}
	rows, err := r.db.Read.QueryContext(ctx,
		`SELECT id, subscription_id, user_id, status, node_count, COALESCE(error,''), created_at
		   FROM subscription_sync_logs
		  WHERE subscription_id = ? AND user_id = ?
		  ORDER BY created_at DESC LIMIT ?`,
		subID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list sync logs: %w", err)
	}
	defer rows.Close()
	var out []SubscriptionSyncLogRecord
	for rows.Next() {
		var rec SubscriptionSyncLogRecord
		if err := rows.Scan(&rec.ID, &rec.SubscriptionID, &rec.UserID, &rec.Status,
			&rec.NodeCount, &rec.Error, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan sync log: %w", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}
