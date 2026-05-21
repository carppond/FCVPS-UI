package agent

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"shiguang-vps/internal/storage"
	"shiguang-vps/pkg/agentlib"
)

// HubConfig wires the Hub to the surrounding system.
type HubConfig struct {
	AgentRepo  *storage.AgentRepo
	RecordRepo *storage.AgentRecordRepo
	EventBus   EventBus
	Logger     *slog.Logger
	Now        func() time.Time
	HubVersion string // surfaced via hello_ack

	// HeartbeatInterval is the advertised heartbeat cadence. Defaults to 30 s.
	HeartbeatInterval time.Duration
	// IdleTimeout is the read deadline before the connection is killed.
	// Defaults to 3× HeartbeatInterval (90 s when defaults apply).
	IdleTimeout time.Duration
}

// Hub owns the live set of connected agents and exposes the dispatch primitives
// the rest of the system needs (snapshot, command send, presence checks).
//
// Goroutine safety: agents is guarded by mu (RWMutex); reads are O(1) lookup
// + bounded scans. Connection lifecycle is fully owned by the Hub — once
// Register is called, the Client's Run() goroutine drives termination + the
// Hub unregisters via the unregister callback from Client.Run.
type Hub struct {
	cfg HubConfig

	mu     sync.RWMutex
	agents map[string]*Client // by agent ID
}

// NewHub builds a Hub with sensible defaults. Pass NoopEventBus when the SSE
// layer is not yet wired (T-14 default).
func NewHub(cfg HubConfig) *Hub {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.EventBus == nil {
		cfg.EventBus = NoopEventBus{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 30 * time.Second
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 3 * cfg.HeartbeatInterval
	}
	if cfg.HubVersion == "" {
		cfg.HubVersion = ProtocolVersion
	}
	return &Hub{cfg: cfg, agents: make(map[string]*Client)}
}

// HeartbeatInterval exposes the advertised heartbeat cadence.
func (h *Hub) HeartbeatInterval() time.Duration { return h.cfg.HeartbeatInterval }

// IdleTimeout exposes the configured read-idle timeout.
func (h *Hub) IdleTimeout() time.Duration { return h.cfg.IdleTimeout }

// HubVersion returns the hub version string for hello_ack.
func (h *Hub) HubVersion() string { return h.cfg.HubVersion }

// EventBus returns the configured event bus.
func (h *Hub) EventBus() EventBus { return h.cfg.EventBus }

// Now returns the hub's wall clock (test-injectable).
func (h *Hub) Now() time.Time { return h.cfg.Now() }

// Logger returns the configured logger.
func (h *Hub) Logger() *slog.Logger { return h.cfg.Logger }

// Register inserts client into the hub. If an existing client is registered
// under the same agent ID it is replaced — the old connection receives bye
// {reason: "token_rotated"} and is closed (typical when the agent reconnects
// after a brief network blip).
func (h *Hub) Register(client *Client) {
	if client == nil {
		return
	}
	h.mu.Lock()
	old, exists := h.agents[client.ID()]
	h.agents[client.ID()] = client
	h.mu.Unlock()
	if exists && old != client {
		h.cfg.Logger.Info("agent hub: replacing previous connection",
			slog.String("agent_id", client.ID()))
		old.SendBye(ByeReasonTokenRotated)
	}
	if h.cfg.AgentRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = h.cfg.AgentRepo.UpdateLastSeen(ctx, client.ID(), "online",
			client.cfg.Agent.Version, client.cfg.Agent.OS, client.cfg.Agent.Arch)
		cancel()
	}
	h.cfg.EventBus.PublishAgentStatus(client.ID(), "online")
}

// unregister removes client from the hub iff the registered entry still points
// to it (so a stale unregister from a replaced connection cannot evict the new
// one). Callers should prefer Client.Close, which triggers this via Run().
func (h *Hub) unregister(client *Client) {
	if client == nil {
		return
	}
	h.mu.Lock()
	if cur, ok := h.agents[client.ID()]; ok && cur == client {
		delete(h.agents, client.ID())
	}
	h.mu.Unlock()
}

// Unregister force-removes the agent ID from the hub registry, closing the
// client if present. Intended for admin actions (delete agent, rotate token).
func (h *Hub) Unregister(agentID string, reason string) {
	h.mu.Lock()
	cli, ok := h.agents[agentID]
	if ok {
		delete(h.agents, agentID)
	}
	h.mu.Unlock()
	if ok && cli != nil {
		cli.SendBye(reason)
	}
}

// IsOnline reports whether the agent currently has an active connection.
func (h *Hub) IsOnline(agentID string) bool {
	h.mu.RLock()
	_, ok := h.agents[agentID]
	h.mu.RUnlock()
	return ok
}

// Client returns the current client for agentID, or nil if offline.
func (h *Hub) Client(agentID string) *Client {
	h.mu.RLock()
	cli := h.agents[agentID]
	h.mu.RUnlock()
	return cli
}

// AgentStatus is the per-agent snapshot returned by Snapshot. It bundles the
// online flag with the most-recent metrics so /api/agents can serve a single
// response without per-agent fan-out.
type AgentStatus struct {
	AgentID       string
	Online        bool
	LatestMetrics *MetricsPayload
}

// Snapshot returns a copy of the current presence + metrics for every
// connected agent. Stable iteration order is not guaranteed.
func (h *Hub) Snapshot() []AgentStatus {
	h.mu.RLock()
	out := make([]AgentStatus, 0, len(h.agents))
	for id, cli := range h.agents {
		out = append(out, AgentStatus{
			AgentID:       id,
			Online:        true,
			LatestMetrics: cli.LatestMetrics(),
		})
	}
	h.mu.RUnlock()
	return out
}

// SnapshotByID returns the presence info for a single agent. The returned
// AgentStatus is always populated (Online=false + nil metrics when offline).
func (h *Hub) SnapshotByID(agentID string) AgentStatus {
	h.mu.RLock()
	cli, ok := h.agents[agentID]
	h.mu.RUnlock()
	if !ok || cli == nil {
		return AgentStatus{AgentID: agentID, Online: false}
	}
	return AgentStatus{
		AgentID:       agentID,
		Online:        true,
		LatestMetrics: cli.LatestMetrics(),
	}
}

// ErrAgentOffline is the canonical "agent not connected" sentinel returned by
// SendCommand. Handlers translate to types.ErrAgentOffline (409 Conflict).
var ErrAgentOffline = errors.New("agent hub: agent offline")

// SendCommand pushes a typed cmd payload to the agent. Returns
// ErrAgentOffline when the agent is not connected. cmdID is the envelope ID
// the agent will echo back in cmd_ack.
func (h *Hub) SendCommand(ctx context.Context, agentID, cmdID string, payload CmdPayload) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	h.mu.RLock()
	cli, ok := h.agents[agentID]
	h.mu.RUnlock()
	if !ok || cli == nil {
		return ErrAgentOffline
	}
	return cli.SendCommand(cmdID, payload)
}

// Close shuts the hub down: sends bye{server_shutdown} to every agent and
// closes each connection. Safe to call multiple times.
func (h *Hub) Close() {
	h.mu.Lock()
	clients := make([]*Client, 0, len(h.agents))
	for _, c := range h.agents {
		clients = append(clients, c)
	}
	h.agents = make(map[string]*Client)
	h.mu.Unlock()
	for _, c := range clients {
		c.SendBye(ByeReasonServerShutdown)
	}
}

// Stats is a tiny diagnostic projection — used by /healthz extensions and
// debug dumps. Cheap to call.
type Stats struct {
	Online int
}

// Stats returns counters about the live hub.
func (h *Hub) Stats() Stats {
	h.mu.RLock()
	n := len(h.agents)
	h.mu.RUnlock()
	return Stats{Online: n}
}

// MarshalEnvelope is a small convenience the handler uses to compose the
// initial hello_ack without importing pkg/agentlib directly.
func MarshalEnvelope(env *Envelope) ([]byte, error) {
	return agentlib.MarshalEnvelope(env)
}
