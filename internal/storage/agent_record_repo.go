package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AgentMetricRecord is the storage projection of an agent_records row.
//
// All numeric fields share the protocol types in pkg/agentlib/protocol.go +
// internal/types/api.go (AgentMetric). RecordedAt is unix-ms.
type AgentMetricRecord struct {
	AgentID      string
	RecordedAt   int64
	CPUPercent   float64
	MemUsed      int64
	MemTotal     int64
	SwapUsed     int64
	SwapTotal    int64
	DiskUsed     int64
	DiskTotal    int64
	NetIn        int64
	NetOut       int64
	NetInSpeed   int64
	NetOutSpeed  int64
	ConnTCP      int32
	ConnUDP      int32
	Load1        float64
	Load5        float64
	Load15       float64
	Uptime       int64
	ProcessCount int32
}

// AgentRecordRepo encapsulates SQL access to the agent_records table. Rows are
// high-frequency metric samples written by the WS hub; the 7-day retention is
// enforced by DeleteOlderThan invoked from a daily cron (cmd/server/main.go).
type AgentRecordRepo struct {
	db *DB
}

// NewAgentRecordRepo wires a repo to db.
func NewAgentRecordRepo(db *DB) *AgentRecordRepo {
	return &AgentRecordRepo{db: db}
}

// Insert appends one metric sample. Most callers should prefer InsertBatch.
func (r *AgentRecordRepo) Insert(ctx context.Context, rec AgentMetricRecord) error {
	return r.InsertBatch(ctx, []AgentMetricRecord{rec})
}

// InsertBatch persists multiple samples in a single multi-row INSERT. Empty
// input is a no-op.
//
// All rows share the same statement to amortise driver overhead — typical
// payload from the hub is 1 row per heartbeat, but the bulk path is exposed so
// that future buffered-write strategies (T-21) do not require a new method.
func (r *AgentRecordRepo) InsertBatch(ctx context.Context, recs []AgentMetricRecord) error {
	if len(recs) == 0 {
		return nil
	}
	cols := "(agent_id, recorded_at, cpu_percent, mem_used, mem_total, swap_used, swap_total," +
		" disk_used, disk_total, net_in, net_out, net_in_speed, net_out_speed," +
		" conn_tcp, conn_udp, load1, load5, load15, uptime, process_count)"
	placeholders := strings.Repeat(",(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", len(recs))
	stmt := "INSERT INTO agent_records " + cols + " VALUES " + placeholders[1:]
	args := make([]any, 0, 20*len(recs))
	for i := range recs {
		rec := recs[i]
		if rec.AgentID == "" {
			return fmt.Errorf("insert agent record: empty agent id at index %d", i)
		}
		if rec.RecordedAt == 0 {
			rec.RecordedAt = time.Now().UnixMilli()
		}
		args = append(args,
			rec.AgentID, rec.RecordedAt, rec.CPUPercent,
			rec.MemUsed, rec.MemTotal, rec.SwapUsed, rec.SwapTotal,
			rec.DiskUsed, rec.DiskTotal,
			rec.NetIn, rec.NetOut, rec.NetInSpeed, rec.NetOutSpeed,
			rec.ConnTCP, rec.ConnUDP,
			rec.Load1, rec.Load5, rec.Load15,
			rec.Uptime, rec.ProcessCount,
		)
	}
	if _, err := r.db.Write.ExecContext(ctx, stmt, args...); err != nil {
		return fmt.Errorf("insert agent records: %w", err)
	}
	return nil
}

// ListRecent returns at most `limit` samples for the given agent ID with
// recorded_at >= since (unix-ms). Results are ordered newest-first.
func (r *AgentRecordRepo) ListRecent(ctx context.Context, agentID string, since time.Time, limit int) ([]AgentMetricRecord, error) {
	if agentID == "" {
		return nil, fmt.Errorf("list recent records: empty agent id")
	}
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	sinceMs := since.UnixMilli()
	rows, err := r.db.Read.QueryContext(ctx, `
		SELECT agent_id, recorded_at, cpu_percent, mem_used, mem_total,
		       swap_used, swap_total, disk_used, disk_total,
		       net_in, net_out, net_in_speed, net_out_speed,
		       conn_tcp, conn_udp, load1, load5, load15, uptime, process_count
		  FROM agent_records
		 WHERE agent_id = ? AND recorded_at >= ?
		 ORDER BY recorded_at DESC
		 LIMIT ?`, agentID, sinceMs, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent records: %w", err)
	}
	defer rows.Close()
	out := make([]AgentMetricRecord, 0, limit)
	for rows.Next() {
		var rec AgentMetricRecord
		if err := rows.Scan(
			&rec.AgentID, &rec.RecordedAt, &rec.CPUPercent,
			&rec.MemUsed, &rec.MemTotal, &rec.SwapUsed, &rec.SwapTotal,
			&rec.DiskUsed, &rec.DiskTotal,
			&rec.NetIn, &rec.NetOut, &rec.NetInSpeed, &rec.NetOutSpeed,
			&rec.ConnTCP, &rec.ConnUDP,
			&rec.Load1, &rec.Load5, &rec.Load15,
			&rec.Uptime, &rec.ProcessCount,
		); err != nil {
			return nil, fmt.Errorf("scan agent record: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent records: %w", err)
	}
	return out, nil
}

// DeleteOlderThan purges every row with recorded_at < cutoff. Returns the
// number of rows deleted. Intended for the 7-day retention cron.
func (r *AgentRecordRepo) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	cutoffMs := cutoff.UnixMilli()
	res, err := r.db.Write.ExecContext(ctx,
		"DELETE FROM agent_records WHERE recorded_at < ?", cutoffMs)
	if err != nil {
		return 0, fmt.Errorf("delete old agent records: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
