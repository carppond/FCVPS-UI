package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
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

// TrafficHandler hosts the M-TRAFFIC REST surface mounted at /api/traffic/*.
//
// Endpoints:
//
//	GET  /api/traffic/summary               — current-month summary for the caller
//	GET  /api/traffic/history?range=7d|30d|90d&view=day|month — trend points
//	GET  /api/traffic/by-agent              — current-month per-agent breakdown
//	PUT  /api/traffic/threshold             — admin: configure percentage levels
//	PUT  /api/traffic/limit                 — admin: configure monthly limit (bytes)
//
// All read endpoints scope to the authenticated user; admins use the same
// reads (their own view) but additionally can flip system-wide settings via
// the PUT endpoints.
type TrafficHandler struct {
	trafficRepo  *storage.TrafficRepo
	agentRepo    *storage.AgentRepo
	settingsRepo *storage.SettingsRepo
	logger       *slog.Logger
}

// NewTrafficHandler wires the handler. agentRepo is required so the
// /summary + /by-agent responses can resolve agent names.
func NewTrafficHandler(trafficRepo *storage.TrafficRepo, agentRepo *storage.AgentRepo, settingsRepo *storage.SettingsRepo, logger *slog.Logger) *TrafficHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &TrafficHandler{
		trafficRepo:  trafficRepo,
		agentRepo:    agentRepo,
		settingsRepo: settingsRepo,
		logger:       logger,
	}
}

// Summary implements GET /api/traffic/summary. The response is the current
// calendar month rolled up across agents, with per-agent breakdowns.
func (h *TrafficHandler) Summary(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	now := time.Now().UTC()
	limit := h.loadMonthlyLimit(r.Context())
	summary, err := h.trafficRepo.GetMonthSummary(r.Context(), user.ID,
		now.Year(), now.Month(), limit)
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "traffic summary", nil, traceID)
		return
	}
	dto := h.summaryToDTO(r, summary)
	util.RespondJSON(w, http.StatusOK, types.APIResponse[trafficSummaryDTO]{
		Data:      dto,
		RequestID: traceID,
	})
}

// History implements GET /api/traffic/history. Query params:
//
//	range=7d|30d|90d   — window relative to today (default 30d)
//	view=day|month     — bucket granularity (default day)
//
// Response is an array of TrafficChartPoint sorted ascending by date.
func (h *TrafficHandler) History(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	window := parseHistoryRange(r.URL.Query().Get("range"))
	view := strings.ToLower(r.URL.Query().Get("view"))
	now := time.Now().UTC()
	to := now
	from := now.AddDate(0, 0, -window)
	var (
		rows []storage.TrafficRecord
		err  error
	)
	if view == "month" {
		rows, err = h.trafficRepo.ListMonthlyTotals(r.Context(), user.ID, from, to)
	} else {
		rows, err = h.trafficRepo.ListDailyTotals(r.Context(), user.ID, from, to)
	}
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "traffic history", nil, traceID)
		return
	}
	points := make([]types.TrafficChartPoint, len(rows))
	for i := range rows {
		points[i] = types.TrafficChartPoint{
			Date:     rows[i].Date,
			TotalIn:  rows[i].TotalIn,
			TotalOut: rows[i].TotalOut,
		}
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.TrafficChartPoint]{
		Data:      points,
		RequestID: traceID,
	})
}

// ByAgent implements GET /api/traffic/by-agent. Mirrors Summary but returns
// only the per-agent slice so the frontend can render a pie / stacked bar.
func (h *TrafficHandler) ByAgent(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	now := time.Now().UTC()
	limit := h.loadMonthlyLimit(r.Context())
	summary, err := h.trafficRepo.GetMonthSummary(r.Context(), user.ID,
		now.Year(), now.Month(), limit)
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "traffic by-agent", nil, traceID)
		return
	}
	dto := h.summaryToDTO(r, summary)
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.AgentTrafficSummary]{
		Data:      dto.Agents,
		RequestID: traceID,
	})
}

// SetThreshold implements PUT /api/traffic/threshold. Admin-only. Accepts a
// TrafficThresholdRequest; threshold_percent in [0,100] adds (or replaces)
// the level; total_limit, when non-zero, also updates the monthly limit.
//
// For v1 we treat the payload as the canonical list of levels (sorted,
// deduped) so a single PUT can both add and remove levels. The handler
// always echoes back the new full list.
func (h *TrafficHandler) SetThreshold(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	if user.Role != string(types.RoleAdmin) {
		util.RespondError(w, types.ErrAuthForbidden, "admin required", nil, traceID)
		return
	}
	var req thresholdRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	levels := req.Percents
	if len(levels) == 0 && req.ThresholdPercent > 0 {
		levels = []int32{req.ThresholdPercent}
	}
	if len(levels) == 0 {
		levels = []int32{80, 90, 100}
	}
	for _, l := range levels {
		if l < 1 || l > 100 {
			util.RespondError(w, types.ErrValidationOutOfRange,
				"threshold_percent must be in [1,100]", nil, traceID)
			return
		}
	}
	parts := make([]string, 0, len(levels))
	seen := make(map[int32]bool, len(levels))
	for _, l := range levels {
		if seen[l] {
			continue
		}
		seen[l] = true
		parts = append(parts, strconv.Itoa(int(l)))
	}
	if err := h.settingsRepo.Set(r.Context(), traffic.SettingTrafficThresholdPercents, strings.Join(parts, ",")); err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "save threshold", nil, traceID)
		return
	}
	if req.TotalLimit > 0 {
		if err := h.settingsRepo.Set(r.Context(), traffic.SettingMonthlyTrafficLimit,
			strconv.FormatInt(req.TotalLimit, 10)); err != nil {
			util.RespondError(w, types.ErrInternalDatabase, "save limit", nil, traceID)
			return
		}
	}
	resp := thresholdResponse{
		Percents:   levelsFromInt32(levels),
		TotalLimit: req.TotalLimit,
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[thresholdResponse]{
		Data:      resp,
		RequestID: traceID,
	})
}

// SetLimit implements PUT /api/traffic/limit. Admin-only. Body: { "total_limit": <bytes> }.
// A value of 0 clears the limit (threshold checker becomes a no-op).
func (h *TrafficHandler) SetLimit(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	if user.Role != string(types.RoleAdmin) {
		util.RespondError(w, types.ErrAuthForbidden, "admin required", nil, traceID)
		return
	}
	var req limitRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.TotalLimit < 0 {
		util.RespondError(w, types.ErrValidationOutOfRange,
			"total_limit must be >= 0", nil, traceID)
		return
	}
	if err := h.settingsRepo.Set(r.Context(), traffic.SettingMonthlyTrafficLimit,
		strconv.FormatInt(req.TotalLimit, 10)); err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "save limit", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[limitRequest]{
		Data:      req,
		RequestID: traceID,
	})
}

// summaryToDTO maps the storage summary onto the wire DTO used by handlers.
// Agent IDs are resolved to names via a single ListByUser call so a busy
// page does not require N+1 lookups.
func (h *TrafficHandler) summaryToDTO(r *http.Request, summary *storage.TrafficSummary) trafficSummaryDTO {
	user := auth.MustUserFromContext(r.Context())
	dto := trafficSummaryDTO{
		UserID:      summary.UserID,
		PeriodStart: summary.PeriodStart,
		PeriodEnd:   summary.PeriodEnd,
		TotalLimit:  summary.TotalLimit,
		TotalUsed:   summary.TotalUsed,
		TotalIn:     summary.TotalIn,
		TotalOut:    summary.TotalOut,
		Agents:      make([]types.AgentTrafficSummary, 0, len(summary.Agents)),
	}
	if summary.TotalLimit > 0 {
		dto.UsagePercent = float64(summary.TotalUsed) / float64(summary.TotalLimit) * 100
		if dto.UsagePercent < 0 {
			dto.UsagePercent = 0
		}
	}
	// Map agent_id → name. A missing agent (deleted) keeps the ID as label.
	nameByID := make(map[string]string, 8)
	if h.agentRepo != nil {
		agents, _, err := h.agentRepo.ListByUser(r.Context(), user.ID, storage.AgentListOptions{
			Page: 1, PageSize: 500,
		})
		if err == nil {
			for i := range agents {
				nameByID[agents[i].ID] = agents[i].Name
			}
		}
	}
	for _, a := range summary.Agents {
		name := nameByID[a.AgentID]
		if name == "" {
			name = a.AgentID
		}
		dto.Agents = append(dto.Agents, types.AgentTrafficSummary{
			AgentID:   a.AgentID,
			AgentName: name,
			TotalIn:   a.TotalIn,
			TotalOut:  a.TotalOut,
			TotalUsed: a.TotalUsed,
			Limit:     a.Limit,
			Source:    a.Source,
		})
	}
	return dto
}

// loadMonthlyLimit reads system_settings.monthly_traffic_limit. Missing /
// invalid rows yield 0 (interpreted by callers as "no limit").
func (h *TrafficHandler) loadMonthlyLimit(ctx context.Context) int64 {
	if h.settingsRepo == nil {
		return 0
	}
	raw, err := h.settingsRepo.Get(ctx, traffic.SettingMonthlyTrafficLimit)
	if err != nil {
		if !errors.Is(err, storage.ErrSettingNotFound) {
			h.logger.Warn("traffic: load monthly limit", slog.String("err", err.Error()))
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

// parseHistoryRange maps "7d"/"30d"/"90d" to the corresponding window. Unknown
// inputs default to 30 days. We deliberately keep the parser tiny — the
// frontend exposes a fixed dropdown.
func parseHistoryRange(s string) int {
	switch strings.ToLower(s) {
	case "7d":
		return 7
	case "30d":
		return 30
	case "90d":
		return 90
	case "365d", "1y":
		return 365
	default:
		return 30
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Wire DTOs — kept package-private so the canonical types.TrafficSummary stays
// the user-facing one. The split lets us add UsagePercent (computed) without
// mutating internal/types/api.go.
// ──────────────────────────────────────────────────────────────────────────

type trafficSummaryDTO struct {
	UserID       string                       `json:"user_id"`
	PeriodStart  string                       `json:"period_start"`
	PeriodEnd    string                       `json:"period_end"`
	TotalLimit   int64                        `json:"total_limit,omitempty"`
	TotalUsed    int64                        `json:"total_used"`
	TotalIn      int64                        `json:"total_in"`
	TotalOut     int64                        `json:"total_out"`
	UsagePercent float64                      `json:"usage_percent"`
	Agents       []types.AgentTrafficSummary  `json:"agents"`
}

// thresholdRequest accepts either a single percentage or a full slice. The
// handler normalises to the slice form so callers stay simple.
type thresholdRequest struct {
	ThresholdPercent int32   `json:"threshold_percent,omitempty"`
	Percents         []int32 `json:"percents,omitempty"`
	TotalLimit       int64   `json:"total_limit,omitempty"`
}

type thresholdResponse struct {
	Percents   []int `json:"percents"`
	TotalLimit int64 `json:"total_limit,omitempty"`
}

type limitRequest struct {
	TotalLimit int64 `json:"total_limit"`
}

func levelsFromInt32(in []int32) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

