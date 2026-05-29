package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"shiguang-vps/internal/agent"
	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// AgentHandler hosts the REST surface for M-AGENT (/api/agents/*).
//
// All endpoints require an authenticated user. Cross-user access yields 404
// (information hiding, consistent with PipelineHandler / SubscriptionHandler).
// The token-mint helpers run inside the handler so we never store plaintext
// tokens — only sha256 hex.
type AgentHandler struct {
	repo       *storage.AgentRepo
	recordRepo *storage.AgentRecordRepo
	hub        *agent.Hub
	logger     *slog.Logger
}

// NewAgentHandler wires the handler.
func NewAgentHandler(repo *storage.AgentRepo, recordRepo *storage.AgentRecordRepo, hub *agent.Hub, logger *slog.Logger) *AgentHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AgentHandler{repo: repo, recordRepo: recordRepo, hub: hub, logger: logger}
}

// List implements GET /api/agents. The response augments each row with the
// hub's online flag + the latest metrics snapshot.
func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	recs, total, err := h.repo.ListByUser(r.Context(), user.ID, storage.AgentListOptions{
		Page:     page.Page,
		PageSize: page.PageSize,
		Keyword:  r.URL.Query().Get("keyword"),
	})
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "list agents", nil, traceID)
		return
	}
	items := make([]agentListItem, len(recs))
	for i := range recs {
		items[i] = h.buildListItem(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[agentListItem]]{
		Data: types.PagedResponse[agentListItem]{
			Items:    items,
			Total:    total,
			Page:     page.Page,
			PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// Create implements POST /api/agents. Generates a fresh token (returned in
// plaintext for the one-and-only time) and persists sha256(token).
//
// Kind selection (T-17):
//   - "native"       → 32-char hex token (16 random bytes); install_command is
//                      the curl one-liner for the native agent installer.
//   - "nezha_compat" → 22-char base64url token (16 random bytes, Nezha-style);
//                      install_command is a Server-URL hint string; the
//                      response additionally surfaces install_hint_i18n_key
//                      so the frontend can render the migration guide.
//   - "" (default)   → "native".
func (h *AgentHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.CreateAgentRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.Name == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "name required", nil, traceID)
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = types.AgentKindNative
	}
	if kind != types.AgentKindNative && kind != types.AgentKindNezhaCompat {
		util.RespondError(w, types.ErrValidationInvalidFormat,
			"kind must be \"native\" or \"nezha_compat\"", nil, traceID)
		return
	}
	var token string
	if kind == types.AgentKindNezhaCompat {
		token = util.RandomBase64URL(16) // 22-char base64url, mirrors Nezha
	} else {
		token = util.RandomHex32()
	}
	rec := storage.AgentRecord{
		ID:        util.UUIDv7(),
		UserID:    user.ID,
		Name:      req.Name,
		TokenHash: util.SHA256Hex(token),
		Kind:      string(kind),
		Status:    "offline",
	}
	created, err := h.repo.Create(r.Context(), rec)
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "create agent", nil, traceID)
		return
	}
	resp := types.AgentCreateResponse{
		Agent:          agentRecordToDTO(created, false, nil),
		Token:          token,
		InstallCommand: buildInstallCommand(kind, token, created.Name),
	}
	if kind == types.AgentKindNezhaCompat {
		// Frontend renders the migration guide under this i18n key (T-16).
		// The string itself lives in web/src/locales/<lang>/agent.json.
		resp.InstallHintI18nKey = "agent.nezha_compat.install_hint"
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.AgentCreateResponse]{
		Data:      resp,
		RequestID: traceID,
	})
}

// Get implements GET /api/agents/:id. Returns the agent + most-recent metrics
// drawn from the hub cache (falling back to the latest DB row when offline).
func (h *AgentHandler) Get(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrAgentNotFound) {
			util.RespondError(w, types.ErrNotFoundAgent, "agent not found", nil, traceID)
			return
		}
		util.RespondError(w, types.ErrInternalDatabase, "get agent", nil, traceID)
		return
	}
	item := h.buildListItem(rec)
	util.RespondJSON(w, http.StatusOK, types.APIResponse[agentListItem]{
		Data:      item,
		RequestID: traceID,
	})
}

// Records implements GET /api/agents/:id/records (T-14 §C extension). Returns
// the last hour of high-frequency samples by default; the caller may override
// via ?from=<unix-ms>&limit=<n>.
func (h *AgentHandler) Records(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if _, err := h.repo.GetByID(r.Context(), id, user.ID); err != nil {
		if errors.Is(err, storage.ErrAgentNotFound) {
			util.RespondError(w, types.ErrNotFoundAgent, "agent not found", nil, traceID)
			return
		}
		util.RespondError(w, types.ErrInternalDatabase, "get agent", nil, traceID)
		return
	}
	since := time.Now().Add(-time.Hour)
	limit := 720 // ~6 samples/min × 60 min × 2
	recs, err := h.recordRepo.ListRecent(r.Context(), id, since, limit)
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "list records", nil, traceID)
		return
	}
	out := make([]types.AgentMetric, len(recs))
	for i := range recs {
		out[i] = agentMetricRecordToDTO(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.AgentMetric]{
		Data:      out,
		RequestID: traceID,
	})
}

// Update implements PUT/PATCH /api/agents/:id (name only in v1).
func (h *AgentHandler) Update(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	var req types.UpdateAgentRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.Name == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "name required", nil, traceID)
		return
	}
	if err := h.repo.UpdateProfile(r.Context(), id, user.ID, req.Name); err != nil {
		if errors.Is(err, storage.ErrAgentNotFound) {
			util.RespondError(w, types.ErrNotFoundAgent, "agent not found", nil, traceID)
			return
		}
		util.RespondError(w, types.ErrInternalDatabase, "update agent", nil, traceID)
		return
	}
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "reload agent after update", nil, traceID)
		return
	}
	item := h.buildListItem(rec)
	util.RespondJSON(w, http.StatusOK, types.APIResponse[agentListItem]{
		Data:      item,
		RequestID: traceID,
	})
}

// Delete implements DELETE /api/agents/:id. Disconnects any live connection +
// cascades the agent_records cleanup via the FK.
func (h *AgentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.repo.Delete(r.Context(), id, user.ID); err != nil {
		if errors.Is(err, storage.ErrAgentNotFound) {
			util.RespondError(w, types.ErrNotFoundAgent, "agent not found", nil, traceID)
			return
		}
		util.RespondError(w, types.ErrInternalDatabase, "delete agent", nil, traceID)
		return
	}
	if h.hub != nil {
		h.hub.Unregister(id, agent.ByeReasonAgentDeleted)
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// RotateToken implements POST /api/agents/:id/rotate-token. The previous live
// connection (if any) is disconnected with bye{token_rotated}.
func (h *AgentHandler) RotateToken(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if _, err := h.repo.GetByID(r.Context(), id, user.ID); err != nil {
		if errors.Is(err, storage.ErrAgentNotFound) {
			util.RespondError(w, types.ErrNotFoundAgent, "agent not found", nil, traceID)
			return
		}
		util.RespondError(w, types.ErrInternalDatabase, "get agent", nil, traceID)
		return
	}
	token := util.RandomHex32()
	if err := h.repo.RotateToken(r.Context(), id, user.ID, util.SHA256Hex(token)); err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "rotate token", nil, traceID)
		return
	}
	if h.hub != nil {
		h.hub.Unregister(id, agent.ByeReasonTokenRotated)
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.RotateTokenResponse]{
		Data:      types.RotateTokenResponse{Token: token},
		RequestID: traceID,
	})
}

// commandRequest is the inbound body for POST /api/agents/:id/command.
type commandRequest struct {
	Cmd  string            `json:"cmd"`
	Args map[string]string `json:"args,omitempty"`
}

// Command implements POST /api/agents/:id/command. Returns 409 ERR_AGENT_OFFLINE
// when the target is not connected.
func (h *AgentHandler) Command(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	var req commandRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if !isAllowedCommand(req.Cmd) {
		util.RespondError(w, types.ErrValidationInvalidFormat, "unsupported command", nil, traceID)
		return
	}
	if _, err := h.repo.GetByID(r.Context(), id, user.ID); err != nil {
		if errors.Is(err, storage.ErrAgentNotFound) {
			util.RespondError(w, types.ErrNotFoundAgent, "agent not found", nil, traceID)
			return
		}
		util.RespondError(w, types.ErrInternalDatabase, "get agent", nil, traceID)
		return
	}
	if h.hub == nil {
		util.RespondError(w, types.ErrAgentOffline, "hub disabled", nil, traceID)
		return
	}
	cmdID := util.UUIDv7()
	err := h.hub.SendCommand(r.Context(), id, cmdID, agent.CmdPayload{
		Cmd:  agent.CmdType(req.Cmd),
		Args: req.Args,
	})
	if err != nil {
		if errors.Is(err, agent.ErrAgentOffline) {
			util.RespondError(w, types.ErrAgentOffline, "agent offline", nil, traceID)
			return
		}
		util.RespondError(w, types.ErrInternalUnknown, "send command", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusAccepted, types.APIResponse[map[string]string]{
		Data:      map[string]string{"cmd_id": cmdID},
		RequestID: traceID,
	})
}

// AdminList implements GET /api/admin/agents (admin sees every agent).
func (h *AgentHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	recs, err := h.repo.ListAll(r.Context())
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "list agents", nil, traceID)
		return
	}
	items := make([]agentListItem, len(recs))
	for i := range recs {
		items[i] = h.buildListItem(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]agentListItem]{
		Data:      items,
		RequestID: traceID,
	})
}

// agentListItem is the DTO for the list / detail endpoints. It bundles the
// canonical Agent shape with the live online flag + the freshest metric
// snapshot (omitted when nil so the JSON stays compact for offline rows).
type agentListItem struct {
	types.Agent
	Online        bool               `json:"online"`
	LatestMetrics *types.AgentMetric `json:"latest_metrics,omitempty"`
}

// buildListItem produces the DTO from a storage record + the hub's live state.
func (h *AgentHandler) buildListItem(rec *storage.AgentRecord) agentListItem {
	online := false
	var metrics *types.AgentMetric
	if h.hub != nil {
		online = h.hub.IsOnline(rec.ID)
		if online {
			if m := h.hub.SnapshotByID(rec.ID).LatestMetrics; m != nil {
				dto := wsMetricsToDTO(rec.ID, time.Now().UnixMilli(), m)
				metrics = &dto
			}
		}
	}
	return agentListItem{
		Agent:         agentRecordToDTO(rec, online, metrics),
		Online:        online,
		LatestMetrics: metrics,
	}
}

// agentRecordToDTO projects the storage row to the API DTO. status mirrors the
// hub's view when available, otherwise it falls back to the column value.
func agentRecordToDTO(rec *storage.AgentRecord, online bool, _ *types.AgentMetric) types.Agent {
	status := types.AgentStatus(rec.Status)
	if online && status != types.AgentStatusOnline {
		status = types.AgentStatusOnline
	}
	if !online && status == types.AgentStatusOnline {
		// Avoid lying when hub knows the agent is gone but DB hasn't been
		// updated yet (race window during graceful shutdown).
		status = types.AgentStatusOffline
	}
	return types.Agent{
		ID:         rec.ID,
		UserID:     rec.UserID,
		Name:       rec.Name,
		Kind:       types.AgentKind(rec.Kind),
		Version:    rec.Version,
		OS:         rec.OS,
		Arch:       rec.Arch,
		PublicIP:   rec.PublicIP,
		LastSeenAt: rec.LastSeenAt,
		Status:     status,
		CreatedAt:  rec.CreatedAt,
		UpdatedAt:  rec.UpdatedAt,
	}
}

// agentMetricRecordToDTO maps storage → API DTO for /agents/:id/records.
func agentMetricRecordToDTO(rec *storage.AgentMetricRecord) types.AgentMetric {
	return types.AgentMetric{
		AgentID:      rec.AgentID,
		RecordedAt:   rec.RecordedAt,
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

// wsMetricsToDTO converts the live in-memory payload to the API DTO.
func wsMetricsToDTO(agentID string, recordedAt int64, m *agent.MetricsPayload) types.AgentMetric {
	return types.AgentMetric{
		AgentID:      agentID,
		RecordedAt:   recordedAt,
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
}

// isAllowedCommand whitelists the cmd types T-14 supports. CmdRestart is part
// of the protocol but the agent CLI handler ships with T-15; we still accept
// it so the wire format is exercised end-to-end.
func isAllowedCommand(c string) bool {
	switch agent.CmdType(c) {
	case agent.CmdRefreshSubscription, agent.CmdCollectNow, agent.CmdRestart:
		return true
	}
	return false
}

// buildInstallCommand surfaces the one-liner install string the UI displays
// after a successful POST /api/agents. The actual install-script handler is
// hosted by T-29; the URL is a placeholder for now and the agent / token are
// the load-bearing portions.
//
// For kind=nezha_compat the return value is a Server-URL hint instead of an
// installer one-liner: the operator already has a Nezha agent binary; we just
// need to tell them what URL to point it at and which token to use.
func buildInstallCommand(kind types.AgentKind, token, name string) string {
	if kind == types.AgentKindNezhaCompat {
		return fmt.Sprintf(
			`# Set Server = "<hub>/api/v1/nezha" and ClientSecret = "%s" in your existing Nezha agent config (agent.yaml). Agent name on hub: %s`,
			token, name,
		)
	}
	// Token goes in the ?token= query — the install-script handler bakes it
	// (and the hub URL, OS/arch) into the rendered script, so the one-liner is
	// just `curl … | bash` with no env vars or args. The "<hub>" placeholder is
	// substituted with the panel's origin by the web UI before display.
	return fmt.Sprintf(`curl -fsSL "<hub>/install-agent.sh?token=%s" | bash`, token)
}
