package nezha

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"shiguang-vps/internal/agent"
	"shiguang-vps/internal/storage"
)

// Adapter consumes parsed Nezha heartbeats and persists them via the same code
// paths the native WebSocket hub uses (agent_records insert + agents.last_seen
// stamp + EventBus publish). Swappable so tests can stub everything out.
type Adapter interface {
	OnHeartbeat(ctx context.Context, agentID string, hb NezhaHeartbeat) error
}

// AdapterDeps wires the default Adapter implementation. AgentRepo +
// RecordRepo are required; EventBus / Logger / Now have sensible defaults.
type AdapterDeps struct {
	AgentRepo  *storage.AgentRepo
	RecordRepo *storage.AgentRecordRepo
	EventBus   agent.EventBus
	Logger     *slog.Logger
	Now        func() time.Time
}

// NewAdapter builds the default Adapter. Panics when AgentRepo or RecordRepo
// is nil — there is no useful behaviour without them.
func NewAdapter(deps AdapterDeps) Adapter {
	if deps.AgentRepo == nil || deps.RecordRepo == nil {
		panic("nezha.NewAdapter: AgentRepo + RecordRepo are required")
	}
	if deps.EventBus == nil {
		deps.EventBus = agent.NoopEventBus{}
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &defaultAdapter{deps: deps}
}

// defaultAdapter writes one agent_records row per heartbeat, stamps
// agents.last_seen and publishes a metrics event so any SSE listeners (T-22)
// pick it up the same way the native WS path does.
type defaultAdapter struct {
	deps AdapterDeps
}

// OnHeartbeat implements Adapter.
func (a *defaultAdapter) OnHeartbeat(ctx context.Context, agentID string, hb NezhaHeartbeat) error {
	if agentID == "" {
		return fmt.Errorf("nezha adapter: empty agent id")
	}
	now := a.deps.Now()
	res := NezhaToAgentRecord(hb, agentID, now)
	LogWarnings(a.deps.Logger, agentID, res.Warnings)

	if err := a.deps.RecordRepo.Insert(ctx, res.Record); err != nil {
		return fmt.Errorf("nezha adapter: insert record: %w", err)
	}

	hostInfo := ExtractHostInfo(hb)
	if err := a.deps.AgentRepo.UpdateLastSeen(ctx, agentID, "online",
		hostInfo.Version, hostInfo.OS, hostInfo.Arch); err != nil {
		// Non-fatal: the record was persisted; surfacing a 500 to the agent
		// would just trigger a noisy retry storm. Log and move on.
		a.deps.Logger.Warn("nezha adapter: update last seen failed",
			slog.String("agent_id", agentID),
			slog.String("err", err.Error()))
	}

	// Publish metrics + status events so any SSE subscribers receive the same
	// stream they would for a native agent. The MetricsPayload is built from
	// the same converted record (one source of truth).
	metrics := metricsPayloadFromRecord(res.Record)
	a.deps.EventBus.PublishAgentMetrics(agentID, metrics)
	a.deps.EventBus.PublishAgentStatus(agentID, "online")

	return nil
}

// metricsPayloadFromRecord converts the stored AgentMetricRecord into the
// MetricsPayload type the EventBus expects. Mirrors the field set the native
// WS hub publishes so SSE consumers do not branch on agent.kind.
func metricsPayloadFromRecord(rec storage.AgentMetricRecord) *agent.MetricsPayload {
	return &agent.MetricsPayload{
		AgentID:      rec.AgentID,
		CPUPercent:   rec.CPUPercent,
		MemUsed:      rec.MemUsed,
		MemTotal:     rec.MemTotal,
		SwapUsed:     rec.SwapUsed,
		SwapTotal:    rec.SwapTotal,
		DiskUsed:     rec.DiskUsed,
		DiskTotal:    rec.DiskTotal,
		NetIn:        rec.NetIn,
		NetOut:       rec.NetOut,
		NetInSpeed:   rec.NetInSpeed,
		NetOutSpeed:  rec.NetOutSpeed,
		Load1:        rec.Load1,
		Load5:        rec.Load5,
		Load15:       rec.Load15,
		ConnTCP:      rec.ConnTCP,
		ConnUDP:      rec.ConnUDP,
		Uptime:       rec.Uptime,
		ProcessCount: rec.ProcessCount,
	}
}
