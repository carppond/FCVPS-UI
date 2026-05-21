package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// AuditHandler hosts GET /api/admin/audit (and the legacy alias
// /api/audit/logs from the task spec). Admin role required at the router
// level. Non-admin users see only their own rows.
type AuditHandler struct {
	repo   *storage.AuditRepo
	logger *slog.Logger
}

// AuditHandlerConfig wires the handler.
type AuditHandlerConfig struct {
	Repo   *storage.AuditRepo
	Logger *slog.Logger
}

// NewAuditHandler constructs the handler.
func NewAuditHandler(cfg AuditHandlerConfig) *AuditHandler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &AuditHandler{repo: cfg.Repo, logger: cfg.Logger}
}

// listResponse is the wire shape returned by List. The contract describes a
// paginated envelope; we surface (data, total, page, page_size) inside the
// canonical APIResponse.Data field.
type auditListResponse struct {
	Items    []types.AuditLog `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// List implements GET /api/admin/audit. Admins see every row; non-admin
// callers are scoped to their own user_id (the router wires this method
// under both /api/admin/audit and /api/audit/logs — the role check happens
// in middleware for the admin path; the user-scoped variant lives inside
// this method).
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		util.RespondError(w, types.ErrAuthTokenInvalid, "no user", nil, traceID)
		return
	}
	isAdmin := user.Role == string(types.RoleAdmin)
	q := r.URL.Query()
	filter := storage.AuditLogFilter{
		Action:   q.Get("action"),
		Page:     parseIntQuery(q.Get("page"), 1),
		PageSize: parseIntQuery(q.Get("page_size"), 50),
	}
	if v := q.Get("from"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.From = n
		}
	}
	if v := q.Get("to"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.To = n
		}
	}
	if isAdmin {
		filter.UserID = q.Get("user_id")
	} else {
		filter.UserID = user.ID
	}
	rows, total, err := h.repo.List(r.Context(), filter)
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, err.Error(), nil, traceID)
		return
	}
	out := auditListResponse{
		Items:    make([]types.AuditLog, 0, len(rows)),
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}
	for _, rec := range rows {
		var payload any
		if rec.Payload != "" {
			// Try to deserialize; fall back to raw string so the UI can
			// still render the bytes for non-JSON payloads.
			if err := json.Unmarshal([]byte(rec.Payload), &payload); err != nil {
				payload = rec.Payload
			}
		}
		out.Items = append(out.Items, types.AuditLog{
			ID:           rec.ID,
			UserID:       rec.UserID,
			Action:       rec.Action,
			ResourceType: rec.ResourceType,
			ResourceID:   rec.ResourceID,
			IP:           rec.IP,
			UserAgent:    rec.UserAgent,
			Payload:      payload,
			Success:      rec.Success,
			CreatedAt:    rec.CreatedAt,
		})
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[auditListResponse]{
		Data: out, RequestID: traceID,
	})
}

// parseIntQuery returns the integer value of s, or defaultValue when s is
// empty / unparseable.
func parseIntQuery(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return defaultValue
	}
	return n
}
