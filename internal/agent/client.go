package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"shiguang-vps/internal/storage"
	"shiguang-vps/pkg/agentlib"
)

// Connection is the abstraction the Client uses to talk to a remote agent.
// It mirrors a tiny subset of *websocket.Conn so tests can plug in a fake
// without spinning up an HTTP server.
type Connection interface {
	ReadMessage() (messageType int, data []byte, err error)
	WriteMessage(messageType int, data []byte) error
	WriteControl(messageType int, data []byte, deadline time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	SetReadLimit(limit int64)
	SetPongHandler(h func(appData string) error)
	Close() error
}

// Compile-time check: *websocket.Conn satisfies Connection.
var _ Connection = (*websocket.Conn)(nil)

// ClientConfig wires a Client to the surrounding hub + storage.
type ClientConfig struct {
	Agent             storage.AgentRecord // resolved at handshake; persistent
	Conn              Connection
	HeartbeatInterval time.Duration // advertised in hello_ack (default 30s)
	IdleTimeout       time.Duration // 3× HeartbeatInterval by default
	WriteTimeout      time.Duration // per outbound message; default 10s
	MaxMessageBytes   int64         // safety cap; default 256 KiB
	Hub               *Hub
	RecordRepo        *storage.AgentRecordRepo
	AgentRepo         *storage.AgentRepo
	EventBus          EventBus
	Logger            *slog.Logger
	Now               func() time.Time
}

// Client is a single connected agent. Lifecycle:
//
//  1. AcceptConnection (handler) constructs the Client + calls Run.
//  2. Run starts read + write pumps; both terminate when Close is called or
//     the underlying connection errors out.
//  3. The hub removes the client from its registry on disconnect and writes
//     `agents.status = "offline"` + emits an SSE event.
//
// All public methods are safe for concurrent callers; the write pump is the
// only goroutine that touches the underlying connection's Write* path.
type Client struct {
	cfg ClientConfig

	send       chan []byte
	closeOnce  sync.Once
	closed     atomic.Bool
	cancelOnce sync.Once
	ctx        context.Context
	cancel     context.CancelFunc

	// metrics caches the most recent metrics payload so /api/agents can serve
	// the freshest snapshot without a DB hit.
	metrics atomic.Pointer[MetricsPayload]
}

// NewClient builds a Client with sensible defaults. The caller still needs to
// call Run().
func NewClient(cfg ClientConfig) *Client {
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 30 * time.Second
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 3 * cfg.HeartbeatInterval
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = 10 * time.Second
	}
	if cfg.MaxMessageBytes <= 0 {
		cfg.MaxMessageBytes = 256 * 1024
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.EventBus == nil {
		cfg.EventBus = NoopEventBus{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		cfg:    cfg,
		send:   make(chan []byte, 32),
		ctx:    ctx,
		cancel: cancel,
	}
}

// ID returns the underlying agent ID (UUID).
func (c *Client) ID() string { return c.cfg.Agent.ID }

// UserID returns the owning user's ID.
func (c *Client) UserID() string { return c.cfg.Agent.UserID }

// Name returns the human-friendly name.
func (c *Client) Name() string { return c.cfg.Agent.Name }

// LatestMetrics returns the most recent metrics snapshot or nil when none has
// been received yet.
func (c *Client) LatestMetrics() *MetricsPayload {
	return c.metrics.Load()
}

// Send enqueues a pre-encoded envelope for the write pump. Returns false if
// the connection has been closed.
func (c *Client) Send(env *Envelope) error {
	if c.closed.Load() {
		return errors.New("agent client: closed")
	}
	data, err := agentlib.MarshalEnvelope(env)
	if err != nil {
		return err
	}
	select {
	case c.send <- data:
		return nil
	case <-c.ctx.Done():
		return errors.New("agent client: closed")
	default:
		// The write pump is back-pressured — drop the message rather than
		// blocking the caller. This is acceptable for v1 because the only
		// hub→agent traffic is cmd dispatch, which is admin-triggered.
		return errors.New("agent client: send buffer full")
	}
}

// SendCommand wraps Send for the typed cmd payload.
func (c *Client) SendCommand(cmdID string, cmd CmdPayload) error {
	raw, err := agentlib.MarshalPayload(cmd)
	if err != nil {
		return err
	}
	return c.Send(&Envelope{
		Type:    MsgCmd,
		ID:      cmdID,
		Payload: raw,
		TS:      c.cfg.Now().UnixMilli(),
	})
}

// SendBye attempts to deliver a `bye` envelope and then closes the connection.
// The write goes directly to the underlying conn (rather than via the send
// channel) so callers can rely on the frame landing even when no writePump is
// running yet (e.g. during Hub.Unregister before Client.Run starts).
// Errors are swallowed — the close is best-effort.
func (c *Client) SendBye(reason string) {
	if c.closed.Load() {
		return
	}
	raw, err := agentlib.MarshalPayload(ByePayload{Reason: reason})
	if err == nil {
		data, mErr := agentlib.MarshalEnvelope(&Envelope{
			Type:    MsgBye,
			ID:      "bye-" + reason,
			Payload: raw,
			TS:      c.cfg.Now().UnixMilli(),
		})
		if mErr == nil {
			_ = c.cfg.Conn.SetWriteDeadline(c.cfg.Now().Add(c.cfg.WriteTimeout))
			_ = c.cfg.Conn.WriteMessage(websocket.TextMessage, data)
		}
	}
	c.Close()
}

// Close terminates the client. Safe to call multiple times.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		c.cancelOnce.Do(c.cancel)
		close(c.send)
		_ = c.cfg.Conn.Close()
	})
}

// Run drives the read + write pumps. Blocks until the connection terminates.
// Callers should invoke from a goroutine spawned by the handler.
func (c *Client) Run() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c.writePump()
	}()
	go func() {
		defer wg.Done()
		c.readPump()
	}()
	wg.Wait()
	// Unregister from the hub and mark offline. Both are idempotent.
	if c.cfg.Hub != nil {
		c.cfg.Hub.unregister(c)
	}
	if c.cfg.AgentRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = c.cfg.AgentRepo.UpdateStatus(ctx, c.ID(), "offline")
		cancel()
	}
	c.cfg.EventBus.PublishAgentStatus(c.ID(), "offline")
}

// readPump is the inbound goroutine. It enforces the idle timeout via
// SetReadDeadline + the gorilla pong handler so any traffic (including
// control frames) keeps the connection alive.
func (c *Client) readPump() {
	conn := c.cfg.Conn
	conn.SetReadLimit(c.cfg.MaxMessageBytes)
	resetDeadline := func() {
		_ = conn.SetReadDeadline(c.cfg.Now().Add(c.cfg.IdleTimeout))
	}
	resetDeadline()
	conn.SetPongHandler(func(string) error { resetDeadline(); return nil })

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if !c.closed.Load() {
				c.cfg.Logger.Info("agent ws: read terminated",
					slog.String("agent_id", c.ID()),
					slog.String("err", err.Error()))
			}
			return
		}
		resetDeadline()
		env, err := agentlib.UnmarshalEnvelope(data)
		if err != nil {
			c.cfg.Logger.Warn("agent ws: invalid envelope",
				slog.String("agent_id", c.ID()),
				slog.String("err", err.Error()))
			continue
		}
		c.dispatch(env)
	}
}

// dispatch routes one inbound envelope to the appropriate handler. Unknown
// types are logged at debug and dropped (forward compatibility: a future agent
// might emit messages the current hub does not yet understand).
func (c *Client) dispatch(env *Envelope) {
	switch env.Type {
	case MsgHeartbeat:
		hb, err := agentlib.UnmarshalHeartbeat(env.Payload)
		if err != nil {
			c.cfg.Logger.Warn("agent ws: heartbeat parse",
				slog.String("agent_id", c.ID()),
				slog.String("err", err.Error()))
			return
		}
		c.handleHeartbeat(hb)
	case MsgMetrics:
		m, err := agentlib.UnmarshalMetrics(env.Payload)
		if err != nil {
			c.cfg.Logger.Warn("agent ws: metrics parse",
				slog.String("agent_id", c.ID()),
				slog.String("err", err.Error()))
			return
		}
		c.handleMetrics(m)
	case MsgCmdAck:
		ack, err := agentlib.UnmarshalCmdAck(env.Payload)
		if err == nil {
			c.cfg.Logger.Info("agent ws: cmd ack",
				slog.String("agent_id", c.ID()),
				slog.String("cmd_id", ack.CmdID),
				slog.Bool("ok", ack.OK),
				slog.String("error", ack.Error),
			)
		}
	case MsgBye:
		bye, _ := agentlib.UnmarshalBye(env.Payload)
		reason := ""
		if bye != nil {
			reason = bye.Reason
		}
		c.cfg.Logger.Info("agent ws: bye from agent",
			slog.String("agent_id", c.ID()),
			slog.String("reason", reason))
		c.Close()
	default:
		c.cfg.Logger.Debug("agent ws: unknown message",
			slog.String("agent_id", c.ID()),
			slog.String("type", string(env.Type)))
	}
}

// handleHeartbeat refreshes last_seen_at + status=online on every beat.
func (c *Client) handleHeartbeat(hb *HeartbeatPayload) {
	if c.cfg.AgentRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.cfg.AgentRepo.UpdateLastSeen(ctx, c.ID(), "online", "", "", ""); err != nil {
		c.cfg.Logger.Warn("agent ws: heartbeat persist",
			slog.String("agent_id", c.ID()),
			slog.String("err", err.Error()))
	}
}

// handleMetrics persists the metric sample + caches it in memory + notifies
// SSE listeners.
func (c *Client) handleMetrics(m *MetricsPayload) {
	// Cache the latest snapshot for /api/agents.
	c.metrics.Store(m)

	now := c.cfg.Now().UnixMilli()
	if c.cfg.RecordRepo != nil {
		rec := storage.AgentMetricRecord{
			AgentID:      c.ID(),
			RecordedAt:   now,
			CPUPercent:   m.CPUPercent,
			MemUsed:      m.MemUsed,
			MemTotal:     m.MemTotal,
			SwapUsed:     m.SwapUsed,
			SwapTotal:    m.SwapTotal,
			DiskUsed:     m.DiskUsed,
			DiskTotal:    m.DiskTotal,
			NetIn:        m.NetIn,
			NetOut:       m.NetOut,
			NetInSpeed:   m.NetInSpeed,
			NetOutSpeed:  m.NetOutSpeed,
			Load1:        m.Load1,
			Load5:        m.Load5,
			Load15:       m.Load15,
			ConnTCP:      m.ConnTCP,
			ConnUDP:      m.ConnUDP,
			Uptime:       m.Uptime,
			ProcessCount: m.ProcessCount,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := c.cfg.RecordRepo.Insert(ctx, rec); err != nil {
			c.cfg.Logger.Warn("agent ws: metrics persist",
				slog.String("agent_id", c.ID()),
				slog.String("err", err.Error()))
		}
		cancel()
	}
	c.cfg.EventBus.PublishAgentMetrics(c.ID(), m)
}

// writePump is the outbound goroutine. It drains the send channel + emits
// websocket ping frames at half the heartbeat interval so the underlying
// TCP socket does not get torn down by idle middleboxes.
func (c *Client) writePump() {
	ticker := time.NewTicker(c.cfg.HeartbeatInterval / 2)
	defer ticker.Stop()
	conn := c.cfg.Conn
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(c.cfg.Now().Add(c.cfg.WriteTimeout))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				c.cfg.Logger.Info("agent ws: write terminated",
					slog.String("agent_id", c.ID()),
					slog.String("err", err.Error()))
				return
			}
		case <-ticker.C:
			deadline := c.cfg.Now().Add(c.cfg.WriteTimeout)
			if err := conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				c.cfg.Logger.Debug("agent ws: ping failed",
					slog.String("agent_id", c.ID()),
					slog.String("err", err.Error()))
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// SendHelloAck pushes the handshake response to the agent. Returns the raw
// envelope so callers can verify it in tests.
func (c *Client) SendHelloAck(hubVersion string) error {
	ack := HelloAckPayload{
		OK:                true,
		HeartbeatInterval: int32(c.cfg.HeartbeatInterval / time.Second),
		HubVersion:        hubVersion,
	}
	raw, err := agentlib.MarshalPayload(ack)
	if err != nil {
		return err
	}
	return c.Send(&Envelope{
		Type:    MsgHelloAck,
		ID:      "hello-ack",
		Payload: raw,
		TS:      c.cfg.Now().UnixMilli(),
	})
}

// encodeEnvelope is a small helper used by tests + the handler to ship a JSON
// envelope without paying the (small) reflection cost of go json.Marshal at
// every call site.
func encodeEnvelope(t MessageType, id string, payload any, ts int64) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return agentlib.MarshalEnvelope(&Envelope{
		Type:    t,
		ID:      id,
		Payload: raw,
		TS:      ts,
	})
}
