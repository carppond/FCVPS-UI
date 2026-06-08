package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"shiguang-vps/internal/util"
)

// ErrAlertRuleNotFound is returned when a rule does not exist or belongs to a
// different user.
var ErrAlertRuleNotFound = errors.New("storage: alert rule not found")

// AlertRuleRecord is the DB projection of an alert_rules row. AgentID is empty
// when the rule applies to all of the user's agents.
type AlertRuleRecord struct {
	ID          string
	UserID      string
	Name        string
	Enabled     bool
	AgentID     string
	Metric      string
	Threshold   float64
	DurationSec int
	CooldownSec int
	CreatedAt   int64
	UpdatedAt   int64
}

// AlertRuleRepo is the alert_rules aggregate store.
type AlertRuleRepo struct {
	db  *DB
	now func() time.Time
}

// NewAlertRuleRepo constructs the repo. now defaults to time.Now when nil.
func NewAlertRuleRepo(db *DB, now func() time.Time) *AlertRuleRepo {
	if now == nil {
		now = time.Now
	}
	return &AlertRuleRepo{db: db, now: now}
}

const selectAlertRuleSQL = `SELECT id, user_id, name, enabled, COALESCE(agent_id,''),
		metric, threshold, duration_sec, cooldown_sec, created_at, updated_at
	FROM alert_rules`

func scanAlertRuleRow(row *sql.Row) (*AlertRuleRecord, error) {
	var r AlertRuleRecord
	var enabled int64
	err := row.Scan(&r.ID, &r.UserID, &r.Name, &enabled, &r.AgentID,
		&r.Metric, &r.Threshold, &r.DurationSec, &r.CooldownSec, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAlertRuleNotFound
		}
		return nil, fmt.Errorf("scan alert rule: %w", err)
	}
	r.Enabled = enabled != 0
	return &r, nil
}

func scanAlertRuleRows(rows *sql.Rows) ([]AlertRuleRecord, error) {
	var out []AlertRuleRecord
	for rows.Next() {
		var r AlertRuleRecord
		var enabled int64
		if err := rows.Scan(&r.ID, &r.UserID, &r.Name, &enabled, &r.AgentID,
			&r.Metric, &r.Threshold, &r.DurationSec, &r.CooldownSec, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan alert rule row: %w", err)
		}
		r.Enabled = enabled != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

// Create inserts a new rule. ID/timestamps are generated when zero.
func (r *AlertRuleRepo) Create(ctx context.Context, rec AlertRuleRecord) (*AlertRuleRecord, error) {
	if rec.UserID == "" || rec.Name == "" || rec.Metric == "" {
		return nil, fmt.Errorf("alert rule create: user_id/name/metric required")
	}
	if rec.ID == "" {
		rec.ID = util.UUIDv7()
	}
	now := r.now().UnixMilli()
	rec.CreatedAt, rec.UpdatedAt = now, now
	if rec.CooldownSec <= 0 {
		rec.CooldownSec = 3600
	}
	_, err := r.db.Write.ExecContext(ctx,
		`INSERT INTO alert_rules (id, user_id, name, enabled, agent_id, metric,
			threshold, duration_sec, cooldown_sec, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.UserID, rec.Name, boolToInt(rec.Enabled), nullableString(rec.AgentID),
		rec.Metric, rec.Threshold, rec.DurationSec, rec.CooldownSec, rec.CreatedAt, rec.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert alert rule: %w", err)
	}
	return &rec, nil
}

// GetByID returns a rule owned by userID.
func (r *AlertRuleRepo) GetByID(ctx context.Context, id, userID string) (*AlertRuleRecord, error) {
	if id == "" || userID == "" {
		return nil, ErrAlertRuleNotFound
	}
	row := r.db.Read.QueryRowContext(ctx, selectAlertRuleSQL+" WHERE id = ? AND user_id = ?", id, userID)
	return scanAlertRuleRow(row)
}

// ListByUser returns all rules owned by userID, newest first.
func (r *AlertRuleRepo) ListByUser(ctx context.Context, userID string) ([]AlertRuleRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("alert rule list: empty user_id")
	}
	rows, err := r.db.Read.QueryContext(ctx, selectAlertRuleSQL+" WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()
	return scanAlertRuleRows(rows)
}

// ListEnabled returns every enabled rule across all users (for the evaluation
// engine). Not user-scoped by design.
func (r *AlertRuleRepo) ListEnabled(ctx context.Context) ([]AlertRuleRecord, error) {
	rows, err := r.db.Read.QueryContext(ctx, selectAlertRuleSQL+" WHERE enabled = 1")
	if err != nil {
		return nil, fmt.Errorf("list enabled alert rules: %w", err)
	}
	defer rows.Close()
	return scanAlertRuleRows(rows)
}

// Update applies the non-nil fields in sets to the rule and returns the fresh
// record. sets keys are column names.
func (r *AlertRuleRepo) Update(ctx context.Context, id, userID string, sets map[string]any) (*AlertRuleRecord, error) {
	if len(sets) == 0 {
		return r.GetByID(ctx, id, userID)
	}
	cols := make([]string, 0, len(sets)+1)
	args := make([]any, 0, len(sets)+3)
	for col, v := range sets {
		cols = append(cols, col+" = ?")
		args = append(args, v)
	}
	cols = append(cols, "updated_at = ?")
	args = append(args, r.now().UnixMilli(), id, userID)
	stmt := "UPDATE alert_rules SET " + strings.Join(cols, ", ") + " WHERE id = ? AND user_id = ?"
	res, err := r.db.Write.ExecContext(ctx, stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("update alert rule: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrAlertRuleNotFound
	}
	return r.GetByID(ctx, id, userID)
}

// Delete removes a rule owned by userID.
func (r *AlertRuleRepo) Delete(ctx context.Context, id, userID string) error {
	res, err := r.db.Write.ExecContext(ctx, "DELETE FROM alert_rules WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAlertRuleNotFound
	}
	return nil
}
