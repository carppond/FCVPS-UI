package storage

import (
	"context"
	"errors"
	"fmt"
)

// PipelineBindingRecord is the storage projection of a pipeline_bindings row.
type PipelineBindingRecord struct {
	SubscriptionID string
	PipelineID     string
	Position       int32
	Enabled        bool
}

// ErrPipelineBindingNotFound is returned when an Unbind targets a row that
// does not exist.
var ErrPipelineBindingNotFound = errors.New("storage: pipeline binding not found")

// Bind inserts or upserts a (subscription_id, pipeline_id) binding with the
// given position. Existing bindings have their position + enabled flag
// refreshed (so the call is idempotent for re-orders).
func (r *PipelineRepo) Bind(ctx context.Context, subscriptionID, pipelineID string, position int32, enabled bool) error {
	if subscriptionID == "" || pipelineID == "" {
		return fmt.Errorf("pipeline bind: empty subscription_id / pipeline_id")
	}
	// SQLite upsert syntax requires named conflict targets; the composite PK
	// (subscription_id, pipeline_id) is exactly what we conflict on.
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO pipeline_bindings(subscription_id, pipeline_id, position, enabled)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(subscription_id, pipeline_id)
		DO UPDATE SET position = excluded.position, enabled = excluded.enabled`,
		subscriptionID, pipelineID, position, boolToInt(enabled),
	)
	if err != nil {
		return fmt.Errorf("bind pipeline: %w", err)
	}
	return nil
}

// Unbind deletes a (subscription_id, pipeline_id) binding. Returns
// ErrPipelineBindingNotFound when no row matched.
func (r *PipelineRepo) Unbind(ctx context.Context, subscriptionID, pipelineID string) error {
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM pipeline_bindings WHERE subscription_id = ? AND pipeline_id = ?",
		subscriptionID, pipelineID)
	if err != nil {
		return fmt.Errorf("unbind pipeline: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrPipelineBindingNotFound
	}
	return nil
}

// ListBindings returns every binding for subscriptionID ordered by position.
func (r *PipelineRepo) ListBindings(ctx context.Context, subscriptionID string) ([]PipelineBindingRecord, error) {
	rows, err := r.db.Read.QueryContext(ctx, `
		SELECT subscription_id, pipeline_id, position, enabled
		  FROM pipeline_bindings
		 WHERE subscription_id = ?
		 ORDER BY position ASC, pipeline_id ASC`, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("list bindings: %w", err)
	}
	defer rows.Close()
	out := make([]PipelineBindingRecord, 0, 4)
	for rows.Next() {
		var rec PipelineBindingRecord
		var enabled int
		if err := rows.Scan(&rec.SubscriptionID, &rec.PipelineID, &rec.Position, &enabled); err != nil {
			return nil, fmt.Errorf("scan binding: %w", err)
		}
		rec.Enabled = enabled == 1
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bindings: %w", err)
	}
	return out, nil
}

// ReplaceBindings atomically replaces every binding for subscriptionID with
// the supplied list. Used by PUT /api/subscriptions/:id/pipelines.
func (r *PipelineRepo) ReplaceBindings(ctx context.Context, subscriptionID string, bindings []PipelineBindingRecord) error {
	if subscriptionID == "" {
		return fmt.Errorf("replace bindings: empty subscription_id")
	}
	tx, err := r.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM pipeline_bindings WHERE subscription_id = ?", subscriptionID); err != nil {
		return fmt.Errorf("clear bindings: %w", err)
	}
	for _, b := range bindings {
		if b.PipelineID == "" {
			return fmt.Errorf("replace bindings: empty pipeline_id")
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO pipeline_bindings(subscription_id, pipeline_id, position, enabled)
			VALUES(?, ?, ?, ?)`,
			subscriptionID, b.PipelineID, b.Position, boolToInt(b.Enabled),
		); err != nil {
			return fmt.Errorf("insert binding: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bindings: %w", err)
	}
	return nil
}
