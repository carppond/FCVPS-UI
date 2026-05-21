package agent

// EventBus is the publish-side interface the hub uses to notify the SSE layer
// (T-22) of agent activity. It is intentionally minimal — events are
// best-effort and the bus may drop them under back-pressure.
//
// Method semantics:
//
//   - PublishAgentStatus: status transitioned to one of "online" | "offline" |
//     "degraded". Receivers fan this out as SSE `agent_status` events.
//   - PublishAgentMetrics: a fresh metrics sample landed. Receivers may want
//     to throttle (the agent emits one per heartbeat ≈ 30 s).
//
// T-14 ships a no-op implementation (NoopEventBus). T-22 swaps in the real
// SSE publisher via the hub's EventBus field.
type EventBus interface {
	PublishAgentStatus(agentID, status string)
	PublishAgentMetrics(agentID string, m *MetricsPayload)
}

// NoopEventBus drops every event. Used by default until T-22 wires the SSE
// publisher into cmd/server/main.go.
type NoopEventBus struct{}

// PublishAgentStatus implements EventBus.
func (NoopEventBus) PublishAgentStatus(string, string) {}

// PublishAgentMetrics implements EventBus.
func (NoopEventBus) PublishAgentMetrics(string, *MetricsPayload) {}
