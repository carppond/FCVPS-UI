package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// TrafficRecord is the storage projection of a traffic_records row. One row per
// (date, user, agent) tuple. TotalLimit is nullable in SQL — a zero value here
// is interpreted as "no per-row limit" (the monthly user-wide limit lives in
// system_settings.monthly_traffic_limit instead).
type TrafficRecord struct {
	Date       string // YYYY-MM-DD
	UserID     string
	AgentID    string
	TotalLimit int64
	TotalUsed  int64
	TotalIn    int64
	TotalOut   int64
}

// TrafficAgentBreakdown carries a per-agent slice of TrafficSummary so the
// frontend can render a stacked bar / pie chart of agent shares.
type TrafficAgentBreakdown struct {
	AgentID   string
	TotalIn   int64
	TotalOut  int64
	TotalUsed int64
	// Effective monthly quota for this agent: bandwagon (provider API, when
	// synced) overrides manual; Source is "bandwagon" | "manual" | "".
	Limit  int64
	Source string
}

// TrafficSummary aggregates traffic_records across a billing period. It is the
// monthly view returned by GetMonthSummary and consumed by the threshold
// checker.
type TrafficSummary struct {
	UserID      string
	PeriodStart string // YYYY-MM-DD
	PeriodEnd   string // YYYY-MM-DD
	TotalLimit  int64  // user-wide monthly limit (bytes); 0 = no limit
	TotalUsed   int64
	TotalIn     int64
	TotalOut    int64
	Agents      []TrafficAgentBreakdown
}

// TrafficRepo encapsulates SQL access to the traffic_records table. Rows are
// upserted by the daily aggregator and queried by the user-facing /api/traffic
// surface. All filters carry user_id so cross-user reads return zero rows.
type TrafficRepo struct {
	db  *DB
	now func() time.Time
}

// NewTrafficRepo wires a repo to db. When now is nil, time.Now is used.
func NewTrafficRepo(db *DB, now func() time.Time) *TrafficRepo {
	if now == nil {
		now = time.Now
	}
	return &TrafficRepo{db: db, now: now}
}

// ErrTrafficRecordNotFound is returned when a lookup misses the table. Most
// callers use ListByUser / GetMonthSummary which return empty slices instead;
// the sentinel is reserved for explicit point lookups by primary key.
var ErrTrafficRecordNotFound = errors.New("storage: traffic record not found")

// UpsertDaily writes a single (date, user, agent) row, replacing any prior
// row with the same key. The aggregator passes one record per agent per day;
// callers that need to write totals for an entire user (no agent breakdown)
// pass an empty agentID.
func (r *TrafficRepo) UpsertDaily(ctx context.Context, rec TrafficRecord) error {
	if rec.Date == "" || rec.UserID == "" {
		return fmt.Errorf("traffic upsert: date and user_id required")
	}
	agentArg := nullableString(rec.AgentID)
	limitArg := nullableInt64(rec.TotalLimit)
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO traffic_records(date, user_id, agent_id, total_limit,
		                            total_used, total_in, total_out)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(date, user_id, agent_id) DO UPDATE SET
			total_limit = excluded.total_limit,
			total_used  = excluded.total_used,
			total_in    = excluded.total_in,
			total_out   = excluded.total_out`,
		rec.Date, rec.UserID, agentArg, limitArg,
		rec.TotalUsed, rec.TotalIn, rec.TotalOut,
	)
	if err != nil {
		return fmt.Errorf("upsert traffic record: %w", err)
	}
	return nil
}

// ListByUser returns every row for userID whose date falls within [from, to]
// (inclusive on both ends). Rows are ordered ascending by date so the
// frontend can feed them directly into a recharts LineChart.
func (r *TrafficRepo) ListByUser(ctx context.Context, userID string, from, to time.Time) ([]TrafficRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("traffic list: empty user_id")
	}
	fromS := from.UTC().Format("2006-01-02")
	toS := to.UTC().Format("2006-01-02")
	rows, err := r.db.Read.QueryContext(ctx, `
		SELECT date, user_id, COALESCE(agent_id,''), COALESCE(total_limit,0),
		       total_used, total_in, total_out
		  FROM traffic_records
		 WHERE user_id = ? AND date >= ? AND date <= ?
		 ORDER BY date ASC, agent_id ASC`,
		userID, fromS, toS,
	)
	if err != nil {
		return nil, fmt.Errorf("list traffic: %w", err)
	}
	defer rows.Close()
	out := make([]TrafficRecord, 0, 32)
	for rows.Next() {
		var rec TrafficRecord
		if err := rows.Scan(&rec.Date, &rec.UserID, &rec.AgentID,
			&rec.TotalLimit, &rec.TotalUsed, &rec.TotalIn, &rec.TotalOut); err != nil {
			return nil, fmt.Errorf("scan traffic: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate traffic: %w", err)
	}
	return out, nil
}

// GetMonthSummary aggregates every row for userID across the calendar month
// (year, month, 1) → (year, month+1, 1) - 1 day, returning per-agent
// breakdowns alongside the total. When the user has no rows for the month, an
// empty summary (zeroed counters) is returned — never ErrTrafficRecordNotFound.
//
// monthlyLimit is propagated verbatim onto TotalLimit so callers can compute
// usage_percent without a second query.
func (r *TrafficRepo) GetMonthSummary(ctx context.Context, userID string, year int, month time.Month, monthlyLimit int64) (*TrafficSummary, error) {
	if userID == "" {
		return nil, fmt.Errorf("traffic summary: empty user_id")
	}
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	periodStart := start.Format("2006-01-02")
	periodEnd := end.Format("2006-01-02")

	summary := &TrafficSummary{
		UserID:      userID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		TotalLimit:  monthlyLimit,
	}

	// 1. Measured usage per agent (NULL agent_id → "" user-wide bucket).
	type usage struct{ used, in, out int64 }
	usageByID := map[string]usage{}
	urows, err := r.db.Read.QueryContext(ctx, `
		SELECT COALESCE(agent_id,'') AS aid,
		       SUM(total_used), SUM(total_in), SUM(total_out)
		  FROM traffic_records
		 WHERE user_id = ? AND date >= ? AND date <= ?
		 GROUP BY aid`,
		userID, periodStart, periodEnd,
	)
	if err != nil {
		return nil, fmt.Errorf("summary usage: %w", err)
	}
	defer urows.Close()
	for urows.Next() {
		var aid string
		var u usage
		if err := urows.Scan(&aid, &u.used, &u.in, &u.out); err != nil {
			return nil, fmt.Errorf("scan usage row: %w", err)
		}
		usageByID[aid] = u
	}
	if err := urows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usage: %w", err)
	}

	add := func(b TrafficAgentBreakdown) {
		summary.Agents = append(summary.Agents, b)
		summary.TotalUsed += b.TotalUsed
		summary.TotalIn += b.TotalIn
		summary.TotalOut += b.TotalOut
	}

	// 2. Drive the breakdown off the user's agents so a configured quota (manual
	// limit or synced BandwagonHost figure) shows even before any traffic has
	// been aggregated for the month.
	arows, err := r.db.Read.QueryContext(ctx, `
		SELECT id, COALESCE(traffic_limit,0), COALESCE(bwg_used,0),
		       COALESCE(bwg_limit,0), COALESCE(bwg_synced_at,0)
		  FROM agents WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("summary agents: %w", err)
	}
	defer arows.Close()
	seen := map[string]bool{}
	for arows.Next() {
		var id string
		var tLimit, bwgUsed, bwgLimit, bwgSynced int64
		if err := arows.Scan(&id, &tLimit, &bwgUsed, &bwgLimit, &bwgSynced); err != nil {
			return nil, fmt.Errorf("scan agent row: %w", err)
		}
		seen[id] = true
		u := usageByID[id]
		b := TrafficAgentBreakdown{AgentID: id, TotalUsed: u.used, TotalIn: u.in, TotalOut: u.out}
		switch {
		case bwgSynced > 0 && bwgLimit > 0:
			b.TotalUsed = bwgUsed
			b.Limit = bwgLimit
			b.Source = "bandwagon"
		case tLimit > 0:
			b.Limit = tLimit
			b.Source = "manual"
		}
		add(b)
	}
	if err := arows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agents: %w", err)
	}

	// 3. Usage with no matching agent (user-wide "" bucket or deleted agents).
	for aid, u := range usageByID {
		if seen[aid] {
			continue
		}
		add(TrafficAgentBreakdown{AgentID: aid, TotalUsed: u.used, TotalIn: u.in, TotalOut: u.out})
	}

	return summary, nil
}

// SumWindow returns (totalUsed, totalIn, totalOut) for userID across an
// arbitrary date range. Used by the threshold checker (current-month window)
// without needing a per-agent breakdown.
func (r *TrafficRepo) SumWindow(ctx context.Context, userID string, from, to time.Time) (int64, int64, int64, error) {
	if userID == "" {
		return 0, 0, 0, fmt.Errorf("traffic sum: empty user_id")
	}
	fromS := from.UTC().Format("2006-01-02")
	toS := to.UTC().Format("2006-01-02")
	var used, in, out sql.NullInt64
	err := r.db.Read.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total_used),0),
		       COALESCE(SUM(total_in),0),
		       COALESCE(SUM(total_out),0)
		  FROM traffic_records
		 WHERE user_id = ? AND date >= ? AND date <= ?`,
		userID, fromS, toS,
	).Scan(&used, &in, &out)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("sum traffic window: %w", err)
	}
	return used.Int64, in.Int64, out.Int64, nil
}

// ListDailyTotals returns one row per date in [from, to], summing across
// agents. Result is ordered ascending by date — ideal for the chart data
// path which does not care about per-agent breakdown.
func (r *TrafficRepo) ListDailyTotals(ctx context.Context, userID string, from, to time.Time) ([]TrafficRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("traffic daily totals: empty user_id")
	}
	fromS := from.UTC().Format("2006-01-02")
	toS := to.UTC().Format("2006-01-02")
	rows, err := r.db.Read.QueryContext(ctx, `
		SELECT date,
		       SUM(total_used) AS used,
		       SUM(total_in)   AS in_b,
		       SUM(total_out)  AS out_b
		  FROM traffic_records
		 WHERE user_id = ? AND date >= ? AND date <= ?
		 GROUP BY date
		 ORDER BY date ASC`,
		userID, fromS, toS,
	)
	if err != nil {
		return nil, fmt.Errorf("list daily totals: %w", err)
	}
	defer rows.Close()
	out := make([]TrafficRecord, 0, 32)
	for rows.Next() {
		var rec TrafficRecord
		rec.UserID = userID
		if err := rows.Scan(&rec.Date, &rec.TotalUsed, &rec.TotalIn, &rec.TotalOut); err != nil {
			return nil, fmt.Errorf("scan daily totals: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily totals: %w", err)
	}
	return out, nil
}

// ListMonthlyTotals returns one row per YYYY-MM in [from, to], summing across
// agents. The returned TrafficRecord.Date carries YYYY-MM-01 so the frontend
// can pivot to a month-bucket axis without special casing.
func (r *TrafficRepo) ListMonthlyTotals(ctx context.Context, userID string, from, to time.Time) ([]TrafficRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("traffic monthly totals: empty user_id")
	}
	fromS := from.UTC().Format("2006-01-02")
	toS := to.UTC().Format("2006-01-02")
	rows, err := r.db.Read.QueryContext(ctx, `
		SELECT substr(date, 1, 7) AS ym,
		       SUM(total_used) AS used,
		       SUM(total_in)   AS in_b,
		       SUM(total_out)  AS out_b
		  FROM traffic_records
		 WHERE user_id = ? AND date >= ? AND date <= ?
		 GROUP BY ym
		 ORDER BY ym ASC`,
		userID, fromS, toS,
	)
	if err != nil {
		return nil, fmt.Errorf("list monthly totals: %w", err)
	}
	defer rows.Close()
	out := make([]TrafficRecord, 0, 12)
	for rows.Next() {
		var ym string
		var rec TrafficRecord
		rec.UserID = userID
		if err := rows.Scan(&ym, &rec.TotalUsed, &rec.TotalIn, &rec.TotalOut); err != nil {
			return nil, fmt.Errorf("scan monthly totals: %w", err)
		}
		rec.Date = ym + "-01"
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monthly totals: %w", err)
	}
	return out, nil
}

// ListUserIDs returns the distinct user_ids that own traffic_records rows.
// The aggregator uses this to find which users need a day rolled up.
func (r *TrafficRepo) ListUserIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.Read.QueryContext(ctx,
		`SELECT DISTINCT user_id FROM traffic_records`)
	if err != nil {
		return nil, fmt.Errorf("list user ids: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, 8)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan user id: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user ids: %w", err)
	}
	return out, nil
}

// DeleteByUser removes every row owned by userID. Mainly used by user-delete
// flows; the foreign-key cascade on user_id handles the same job, so this is
// a no-op safety net.
func (r *TrafficRepo) DeleteByUser(ctx context.Context, userID string) (int64, error) {
	if userID == "" {
		return 0, fmt.Errorf("traffic delete by user: empty user_id")
	}
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM traffic_records WHERE user_id = ?", userID)
	if err != nil {
		return 0, fmt.Errorf("delete traffic by user: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
