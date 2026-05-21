package transport

import (
	"context"
	"log/slog"
	"time"

	"shiguang-vps/cmd/agent/internal/collector"
	"shiguang-vps/pkg/agentlib"
)

// Heartbeat fires the periodic heartbeat + metrics frames. Construction is
// cheap (no goroutine started until Run is called) — the Client owns its
// lifetime.
type Heartbeat struct {
	client   *Client
	interval time.Duration
}

// NewHeartbeat wires the heartbeat to its parent client.
func NewHeartbeat(c *Client) *Heartbeat {
	return &Heartbeat{client: c, interval: c.HeartbeatInterval()}
}

// Run blocks until ctx is cancelled. Each tick sends:
//
//   - heartbeat (lightweight: agent_id + uptime)
//   - metrics (full aggregator snapshot; skipped on collector error)
//
// The first tick is immediate so the hub gets initial telemetry without
// waiting one full interval after handshake.
func (h *Heartbeat) Run(ctx context.Context) {
	h.tick(ctx)
	t := time.NewTicker(h.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.tick(ctx)
		}
	}
}

// tick is one heartbeat + metrics cycle. Errors are logged but never abort
// the loop (per spec: "collector failure → skip + log").
func (h *Heartbeat) tick(ctx context.Context) {
	// Heartbeat: super cheap, send unconditionally.
	up, err := collector.Uptime()
	if err != nil {
		h.client.cfg.Logger.Warn("agent heartbeat: uptime collector failed",
			slog.String("err", err.Error()))
		up = 0
	}
	if err := h.client.enqueue(agentlib.MsgHeartbeat, newMsgID("hb"), agentlib.HeartbeatPayload{
		AgentID: h.client.cfg.AgentID,
		Uptime:  int64(up),
	}); err != nil {
		h.client.cfg.Logger.Warn("agent heartbeat: enqueue failed",
			slog.String("err", err.Error()))
	}

	// Metrics: bounded by per-collector timeouts within the aggregator.
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	payload, err := h.client.cfg.Aggregator.Collect(cctx)
	if err != nil {
		h.client.cfg.Logger.Warn("agent heartbeat: collector failed; skipping metrics",
			slog.String("err", err.Error()))
		return
	}
	if err := h.client.enqueue(agentlib.MsgMetrics, newMsgID("m"), payload); err != nil {
		h.client.cfg.Logger.Warn("agent heartbeat: metrics enqueue failed",
			slog.String("err", err.Error()))
	}
}
