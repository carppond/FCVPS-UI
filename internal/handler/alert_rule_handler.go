package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// AlertRuleHandler hosts /api/alert-rules/* (M-ALERT — probe alert rules).
type AlertRuleHandler struct {
	repo   *storage.AlertRuleRepo
	logger *slog.Logger
}

// NewAlertRuleHandler wires the handler.
func NewAlertRuleHandler(repo *storage.AlertRuleRepo, logger *slog.Logger) *AlertRuleHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AlertRuleHandler{repo: repo, logger: logger}
}

// validMetric reports whether m is an accepted alert metric.
func validMetric(m types.AlertMetric) bool {
	switch m {
	case types.AlertMetricCPU, types.AlertMetricMem, types.AlertMetricDisk, types.AlertMetricOffline:
		return true
	default:
		return false
	}
}

func alertRuleToDTO(rec *storage.AlertRuleRecord) types.AlertRule {
	return types.AlertRule{
		ID:          rec.ID,
		UserID:      rec.UserID,
		Name:        rec.Name,
		Enabled:     rec.Enabled,
		AgentID:     rec.AgentID,
		Metric:      types.AlertMetric(rec.Metric),
		Threshold:   rec.Threshold,
		DurationSec: rec.DurationSec,
		CooldownSec: rec.CooldownSec,
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}

// List implements GET /api/alert-rules.
func (h *AlertRuleHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	recs, err := h.repo.ListByUser(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("list alert rules", slog.String("err", err.Error()), slog.String("trace_id", traceID))
		util.RespondError(w, types.ErrInternalDatabase, "list alert rules", nil, traceID)
		return
	}
	items := make([]types.AlertRule, len(recs))
	for i := range recs {
		items[i] = alertRuleToDTO(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.AlertRule]]{
		Data: types.PagedResponse[types.AlertRule]{
			Items: items, Total: int64(len(items)), Page: 1, PageSize: len(items),
		},
		RequestID: traceID,
	})
}

// Create implements POST /api/alert-rules.
func (h *AlertRuleHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.CreateAlertRuleRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "name required", nil, traceID)
		return
	}
	if !validMetric(req.Metric) {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid metric", nil, traceID)
		return
	}
	if req.Metric != types.AlertMetricOffline && (req.Threshold <= 0 || req.Threshold > 100) {
		util.RespondError(w, types.ErrValidationOutOfRange, "threshold must be 1-100", nil, traceID)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rec, err := h.repo.Create(r.Context(), storage.AlertRuleRecord{
		UserID:      user.ID,
		Name:        req.Name,
		Enabled:     enabled,
		AgentID:     strings.TrimSpace(req.AgentID),
		Metric:      string(req.Metric),
		Threshold:   req.Threshold,
		DurationSec: req.DurationSec,
		CooldownSec: req.CooldownSec,
	})
	if err != nil {
		h.logger.Error("create alert rule", slog.String("err", err.Error()), slog.String("trace_id", traceID))
		util.RespondError(w, types.ErrInternalDatabase, "create alert rule", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.AlertRule]{
		Data: alertRuleToDTO(rec), RequestID: traceID,
	})
}

// Get implements GET /api/alert-rules/{id}.
func (h *AlertRuleHandler) Get(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	rec, err := h.repo.GetByID(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		h.respondErr(w, traceID, err, "get alert rule")
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.AlertRule]{
		Data: alertRuleToDTO(rec), RequestID: traceID,
	})
}

// Update implements PATCH/PUT /api/alert-rules/{id}.
func (h *AlertRuleHandler) Update(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.UpdateAlertRuleRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	sets := map[string]any{}
	if n := strings.TrimSpace(req.Name); n != "" {
		sets["name"] = n
	}
	if req.Metric != "" {
		if !validMetric(req.Metric) {
			util.RespondError(w, types.ErrValidationInvalidFormat, "invalid metric", nil, traceID)
			return
		}
		sets["metric"] = string(req.Metric)
	}
	if req.AgentID != nil {
		sets["agent_id"] = nullableAgentID(strings.TrimSpace(*req.AgentID))
	}
	if req.Threshold != nil {
		if *req.Threshold < 0 || *req.Threshold > 100 {
			util.RespondError(w, types.ErrValidationOutOfRange, "threshold must be 0-100", nil, traceID)
			return
		}
		sets["threshold"] = *req.Threshold
	}
	if req.DurationSec != nil {
		sets["duration_sec"] = *req.DurationSec
	}
	if req.CooldownSec != nil {
		sets["cooldown_sec"] = *req.CooldownSec
	}
	if req.Enabled != nil {
		sets["enabled"] = boolToInt(*req.Enabled)
	}
	rec, err := h.repo.Update(r.Context(), r.PathValue("id"), user.ID, sets)
	if err != nil {
		h.respondErr(w, traceID, err, "update alert rule")
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.AlertRule]{
		Data: alertRuleToDTO(rec), RequestID: traceID,
	})
}

// Delete implements DELETE /api/alert-rules/{id}.
func (h *AlertRuleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	if err := h.repo.Delete(r.Context(), r.PathValue("id"), user.ID); err != nil {
		h.respondErr(w, traceID, err, "delete alert rule")
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

func (h *AlertRuleHandler) respondErr(w http.ResponseWriter, traceID string, err error, action string) {
	if errors.Is(err, storage.ErrAlertRuleNotFound) {
		util.RespondError(w, types.ErrNotFoundAlertRule, "alert rule not found", nil, traceID)
		return
	}
	h.logger.Error(action, slog.String("err", err.Error()), slog.String("trace_id", traceID))
	util.RespondError(w, types.ErrInternalDatabase, action, nil, traceID)
}

// boolToInt mirrors the storage helper for the SET map (1/0).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullableAgentID returns nil for an empty agent id so the column is NULLed
// (rule applies to all agents).
func nullableAgentID(s string) any {
	if s == "" {
		return nil
	}
	return s
}
