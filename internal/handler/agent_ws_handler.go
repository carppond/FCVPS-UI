package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"shiguang-vps/internal/agent"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/util"
	"shiguang-vps/pkg/agentlib"
)

// AgentWSHandler implements GET /api/agent/ws. It:
//
//  1. Reads the ?token=<bearer> query parameter (404 if missing/unknown).
//  2. Looks up the agent by sha256(token).
//  3. Performs WebSocket upgrade.
//  4. Waits for the agent's hello envelope; validates protocol version.
//  5. Sends hello_ack and hands the connection to a Client + the Hub.
//
// 404 (not 401) is used for every failure pre-upgrade so an unauthenticated
// scanner cannot distinguish "no agent ws here" from "bad token". See
// docs/05-tech-lead-plan.md §1.8 for the version-negotiation contract.
type AgentWSHandler struct {
	hub         *agent.Hub
	repo        *storage.AgentRepo
	recordRepo  *storage.AgentRecordRepo
	upgrader    websocket.Upgrader
	logger      *slog.Logger
	now         func() time.Time
	helloDeadln time.Duration // wait this long for the first hello
}

// NewAgentWSHandler wires the handler. logger may be nil.
func NewAgentWSHandler(hub *agent.Hub, repo *storage.AgentRepo, recordRepo *storage.AgentRecordRepo, logger *slog.Logger) *AgentWSHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AgentWSHandler{
		hub:        hub,
		repo:       repo,
		recordRepo: recordRepo,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 10 * time.Second,
			ReadBufferSize:   4096,
			WriteBufferSize:  4096,
			// CheckOrigin returning true is the documented pattern for
			// cross-origin agent connections — agents do not provide an Origin
			// header, and the bearer token authenticates the call.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		logger:      logger,
		now:         time.Now,
		helloDeadln: 10 * time.Second,
	}
}

// Handler returns the http.Handler — registered by router.go.
func (h *AgentWSHandler) Handler() http.Handler {
	return http.HandlerFunc(h.ServeHTTP)
}

// ServeHTTP implements GET /api/agent/ws.
func (h *AgentWSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	token := r.URL.Query().Get("token")
	if token == "" {
		// Mimic 404 — never disclose that the endpoint exists.
		middleware.Mimic404(w)
		return
	}
	tokenHash := util.SHA256Hex(token)
	rec, err := h.repo.GetByTokenHash(r.Context(), tokenHash)
	if err != nil {
		middleware.Mimic404(w)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade already wrote the response.
		h.logger.Info("agent ws: upgrade failed",
			slog.String("agent_id", rec.ID),
			slog.String("err", err.Error()),
			slog.String("trace_id", traceID),
		)
		return
	}

	// Read hello envelope with a generous deadline.
	_ = conn.SetReadDeadline(h.now().Add(h.helloDeadln))
	_, data, err := conn.ReadMessage()
	if err != nil {
		h.logger.Info("agent ws: hello read failed",
			slog.String("agent_id", rec.ID),
			slog.String("err", err.Error()))
		_ = conn.Close()
		return
	}
	env, err := agentlib.UnmarshalEnvelope(data)
	if err != nil || env.Type != agent.MsgHello {
		h.sendByeAndClose(conn, agent.ByeReasonVersionUnsupported)
		return
	}
	hello, err := agentlib.UnmarshalHello(env.Payload)
	if err != nil {
		h.sendByeAndClose(conn, agent.ByeReasonVersionUnsupported)
		return
	}
	// agent_id in payload must match the row resolved via token. Mismatch is a
	// silent close — possible token sharing across agents.
	if hello.AgentID != rec.ID {
		h.logger.Warn("agent ws: hello agent_id mismatch",
			slog.String("expected", rec.ID),
			slog.String("got", hello.AgentID))
		_ = conn.Close()
		return
	}
	if !agent.IsVersionCompatible(hello.Version) {
		h.logger.Warn("agent ws: incompatible protocol version",
			slog.String("agent_id", rec.ID),
			slog.String("agent_version", hello.Version),
			slog.String("hub_version", agent.ProtocolVersion))
		h.sendByeAndClose(conn, agent.ByeReasonVersionUnsupported)
		return
	}

	// Persist the handshake-supplied OS/arch/version so admins can see what's
	// connected without extra round-trips.
	rec.Version = hello.Version
	rec.OS = hello.OS
	rec.Arch = hello.Arch
	_ = h.repo.UpdateLastSeen(r.Context(), rec.ID, "online", hello.Version, hello.OS, hello.Arch)

	// Build the Client + Hub registration.
	cli := agent.NewClient(agent.ClientConfig{
		Agent:             *rec,
		Conn:              conn,
		HeartbeatInterval: h.hub.HeartbeatInterval(),
		IdleTimeout:       h.hub.IdleTimeout(),
		Hub:               h.hub,
		RecordRepo:        h.recordRepo,
		AgentRepo:         h.repo,
		EventBus:          h.hub.EventBus(),
		Logger:            h.logger,
		Now:               h.now,
	})
	h.hub.Register(cli)

	// Send hello_ack and hand off to Run.
	if err := cli.SendHelloAck(h.hub.HubVersion()); err != nil {
		h.logger.Warn("agent ws: hello_ack send failed",
			slog.String("agent_id", rec.ID),
			slog.String("err", err.Error()))
		cli.Close()
		return
	}
	// Reset read deadline now that handshake is over — Client.readPump
	// installs its own deadline + pong handler.
	_ = conn.SetReadDeadline(time.Time{})
	cli.Run()
}

// sendByeAndClose pushes a single bye frame at low cost and then drops the
// connection. Used during handshake failure paths where the Client is not
// yet wired.
func (h *AgentWSHandler) sendByeAndClose(conn *websocket.Conn, reason string) {
	raw, _ := agentlib.MarshalPayload(agent.ByePayload{Reason: reason})
	env := &agent.Envelope{
		Type:    agent.MsgBye,
		ID:      "bye-" + reason,
		Payload: raw,
		TS:      h.now().UnixMilli(),
	}
	data, err := agentlib.MarshalEnvelope(env)
	if err == nil {
		_ = conn.SetWriteDeadline(h.now().Add(5 * time.Second))
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
	_ = conn.Close()
}

