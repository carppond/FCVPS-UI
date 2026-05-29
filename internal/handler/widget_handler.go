package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/traffic"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// widgetTopAgents caps how many per-agent rows the widget payload carries. The
// medium home-screen widget shows ~3 rows; keeping the payload tiny stays well
// inside WidgetKit's refresh budget.
const widgetTopAgents = 3

// WidgetHandler hosts /api/widget/* — the mobile home-screen traffic widget.
//
//   - POST   /api/widget/token    — mint/rotate the read-only widget token (session auth)
//   - DELETE /api/widget/token    — revoke it (disable widget) (session auth)
//   - GET    /api/widget/token    — whether the caller has a token (session auth)
//   - GET    /api/widget/traffic  — tiny traffic payload (WIDGET token auth)
//
// The /traffic endpoint authenticates with the scoped widget token (not a
// session) so the native widget extension never holds a full session token.
type WidgetHandler struct {
	tokens       *storage.WidgetTokenRepo
	trafficRepo  *storage.TrafficRepo
	agentRepo    *storage.AgentRepo
	settingsRepo *storage.SettingsRepo
	logger       *slog.Logger
	now          func() time.Time
}

// NewWidgetHandler wires the handler. now defaults to time.Now.
func NewWidgetHandler(tokens *storage.WidgetTokenRepo, trafficRepo *storage.TrafficRepo, agentRepo *storage.AgentRepo, settingsRepo *storage.SettingsRepo, logger *slog.Logger) *WidgetHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &WidgetHandler{
		tokens:       tokens,
		trafficRepo:  trafficRepo,
		agentRepo:    agentRepo,
		settingsRepo: settingsRepo,
		logger:       logger,
		now:          time.Now,
	}
}

type widgetTokenResponse struct {
	Token string `json:"token"`
}

type widgetTokenStatusResponse struct {
	Enabled bool `json:"enabled"`
}

type widgetAgentItem struct {
	Name string `json:"name"`
	Used int64  `json:"used"`
}

type widgetTrafficPayload struct {
	Used      int64             `json:"used"`
	Limit     int64             `json:"limit"`
	Percent   float64           `json:"percent"`
	Top       []widgetAgentItem `json:"top"`
	UpdatedAt int64             `json:"updated_at"`
}

// MintToken implements POST /api/widget/token. Generates a fresh read-only
// token, persists only its hash (replacing any previous token for the user),
// and returns the plaintext exactly once for the app to store in its shared
// container.
func (h *WidgetHandler) MintToken(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.tokens == nil {
		util.RespondError(w, types.ErrInternalUnknown, "widget service unavailable", nil, traceID)
		return
	}
	user := auth.MustUserFromContext(r.Context())
	token := util.Base64URL(util.RandomBytes(auth.AccessTokenBytes))
	hash := util.SHA256Hex(token)
	if err := h.tokens.Replace(r.Context(), user.ID, hash); err != nil {
		h.logger.Warn("widget: mint token failed", slog.String("err", err.Error()))
		util.RespondError(w, types.ErrInternalDatabase, "mint widget token", nil, traceID)
		return
	}
	h.logger.Info("widget token minted", slog.String("actor", user.Username))
	util.RespondJSON(w, http.StatusOK, types.APIResponse[widgetTokenResponse]{
		Data: widgetTokenResponse{Token: token}, RequestID: traceID,
	})
}

// RevokeToken implements DELETE /api/widget/token (disable widget).
func (h *WidgetHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.tokens == nil {
		util.RespondError(w, types.ErrInternalUnknown, "widget service unavailable", nil, traceID)
		return
	}
	user := auth.MustUserFromContext(r.Context())
	if err := h.tokens.DeleteByUser(r.Context(), user.ID); err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "revoke widget token", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[widgetTokenStatusResponse]{
		Data: widgetTokenStatusResponse{Enabled: false}, RequestID: traceID,
	})
}

// TokenStatus implements GET /api/widget/token — reports whether the caller
// currently has a widget token (so the settings toggle renders correctly)
// without ever returning the token value.
func (h *WidgetHandler) TokenStatus(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.tokens == nil {
		util.RespondError(w, types.ErrInternalUnknown, "widget service unavailable", nil, traceID)
		return
	}
	user := auth.MustUserFromContext(r.Context())
	enabled, err := h.tokens.ExistsForUser(r.Context(), user.ID)
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "widget token status", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[widgetTokenStatusResponse]{
		Data: widgetTokenStatusResponse{Enabled: enabled}, RequestID: traceID,
	})
}

// Traffic implements GET /api/widget/traffic. Authenticated by the scoped
// widget token (Authorization: Bearer <t> or ?token=<t>), NOT a session — this
// route is mounted without auth.Required and is silent-mode whitelisted so the
// native widget can reach it directly. Returns a tiny month-to-date payload.
func (h *WidgetHandler) Traffic(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.tokens == nil || h.trafficRepo == nil {
		util.RespondError(w, types.ErrInternalUnknown, "widget service unavailable", nil, traceID)
		return
	}
	token := extractWidgetToken(r)
	if token == "" {
		util.RespondError(w, types.ErrAuthTokenInvalid, "missing widget token", nil, traceID)
		return
	}
	userID, err := h.tokens.Lookup(r.Context(), util.SHA256Hex(token))
	if err != nil {
		// Both "not found" and DB error map to an opaque invalid-token response
		// so a caller cannot probe which tokens exist.
		util.RespondError(w, types.ErrAuthTokenInvalid, "invalid widget token", nil, traceID)
		return
	}
	_ = h.tokens.TouchLastUsed(r.Context(), util.SHA256Hex(token)) // best-effort

	now := h.now().UTC()
	limit := h.loadMonthlyLimit(r.Context())
	summary, err := h.trafficRepo.GetMonthSummary(r.Context(), userID, now.Year(), now.Month(), limit)
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "widget traffic", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[widgetTrafficPayload]{
		Data: h.buildPayload(r.Context(), userID, summary, now), RequestID: traceID,
	})
}

// buildPayload rolls the storage summary into the tiny widget payload: total
// used/limit/percent plus the top-N agents by usage, with names resolved.
func (h *WidgetHandler) buildPayload(ctx context.Context, userID string, summary *storage.TrafficSummary, now time.Time) widgetTrafficPayload {
	out := widgetTrafficPayload{
		Used:      summary.TotalUsed,
		Limit:     summary.TotalLimit,
		Top:       make([]widgetAgentItem, 0, widgetTopAgents),
		UpdatedAt: now.UnixMilli(),
	}
	if summary.TotalLimit > 0 {
		out.Percent = float64(summary.TotalUsed) / float64(summary.TotalLimit) * 100
		if out.Percent < 0 {
			out.Percent = 0
		}
	}
	// Resolve agent_id → name in one query (avoid N+1).
	nameByID := make(map[string]string, 8)
	if h.agentRepo != nil {
		agents, _, err := h.agentRepo.ListByUser(ctx, userID, storage.AgentListOptions{Page: 1, PageSize: 500})
		if err == nil {
			for i := range agents {
				nameByID[agents[i].ID] = agents[i].Name
			}
		}
	}
	rows := make([]widgetAgentItem, 0, len(summary.Agents))
	for _, a := range summary.Agents {
		name := nameByID[a.AgentID]
		if name == "" {
			name = a.AgentID
		}
		rows = append(rows, widgetAgentItem{Name: name, Used: a.TotalUsed})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Used > rows[j].Used })
	if len(rows) > widgetTopAgents {
		rows = rows[:widgetTopAgents]
	}
	out.Top = rows
	return out
}

// loadMonthlyLimit mirrors TrafficHandler.loadMonthlyLimit — reads
// system_settings.monthly_traffic_limit; missing/invalid → 0 (no limit).
func (h *WidgetHandler) loadMonthlyLimit(ctx context.Context) int64 {
	if h.settingsRepo == nil {
		return 0
	}
	raw, err := h.settingsRepo.Get(ctx, traffic.SettingMonthlyTrafficLimit)
	if err != nil {
		if !errors.Is(err, storage.ErrSettingNotFound) {
			h.logger.Warn("widget: load monthly limit", slog.String("err", err.Error()))
		}
		return 0
	}
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// extractWidgetToken pulls the token from the Authorization: Bearer header or
// the ?token= query param (the latter lets the SwiftUI URLSession set it
// without custom headers if needed).
func extractWidgetToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if after, ok := strings.CutPrefix(h, "Bearer "); ok {
			return strings.TrimSpace(after)
		}
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}
