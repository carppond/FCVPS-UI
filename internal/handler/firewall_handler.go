package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/firewall"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// securityGroupNote is appended to every firewall response. The hub can only
// touch the local OS firewall (ufw); a cloud provider's security group is a
// second, independent gate it cannot see or change — so we always remind the
// operator to check it when a port still seems unreachable.
const securityGroupNote = "本机防火墙规则已生效。若端口仍无法从外部访问，请到云服务商控制台检查安全组（security group）是否也放行了该端口 —— 这一层面板无法管理。"

// FirewallHandler hosts /api/admin/firewall/* (admin-only). It manages the
// local host's ufw rules via firewall.Service. The handler stays declarative;
// privilege/validation/locking all live in the service.
type FirewallHandler struct {
	svc    *firewall.Service
	logger *slog.Logger
}

// NewFirewallHandler wires the handler. svc may be nil — endpoints then 500
// with ErrInternalUnknown so the misconfiguration is obvious.
func NewFirewallHandler(svc *firewall.Service, logger *slog.Logger) *FirewallHandler {
	return &FirewallHandler{svc: svc, logger: logger}
}

type firewallStatusResponse struct {
	Status firewall.Status `json:"status"`
	Rules  []firewall.Rule `json:"rules"`
	Note   string          `json:"note"`
}

type firewallPortRequest struct {
	Port  int    `json:"port"`
	Proto string `json:"proto"` // "tcp" | "udp"; empty defaults to tcp on allow
}

// Status implements GET /api/admin/firewall/status. Always 200: the
// environment probe is embedded in the body (available / active / can_manage /
// reason) so the UI renders the feature as usable, read-only, or disabled.
func (h *FirewallHandler) Status(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.svc == nil {
		util.RespondError(w, types.ErrInternalUnknown, "firewall service unavailable", nil, traceID)
		return
	}
	resp := h.buildStatus(r)
	util.RespondJSON(w, http.StatusOK, types.APIResponse[firewallStatusResponse]{
		Data: resp, RequestID: traceID,
	})
}

// Allow implements POST /api/admin/firewall/allow. Body: {port, proto}. Adds
// an allow-in rule and returns the refreshed status so the UI updates in one
// round-trip.
func (h *FirewallHandler) Allow(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.svc == nil {
		util.RespondError(w, types.ErrInternalUnknown, "firewall service unavailable", nil, traceID)
		return
	}
	var req firewallPortRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid request body", nil, traceID)
		return
	}
	proto := req.Proto
	if proto == "" {
		proto = "tcp"
	}
	if err := h.svc.AllowPort(r.Context(), req.Port, proto); err != nil {
		h.respondServiceErr(w, err, traceID)
		return
	}
	h.audit(r, "firewall.allow", req.Port, proto)
	util.RespondJSON(w, http.StatusOK, types.APIResponse[firewallStatusResponse]{
		Data: h.buildStatus(r), RequestID: traceID,
	})
}

// Delete implements POST /api/admin/firewall/delete. Body: {port, proto}. The
// spec is reconstructed server-side (port or port/proto) so the client never
// supplies a raw ufw argument. Protected ports (SSH / panel access) are
// refused by the service with 403.
func (h *FirewallHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.svc == nil {
		util.RespondError(w, types.ErrInternalUnknown, "firewall service unavailable", nil, traceID)
		return
	}
	var req firewallPortRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid request body", nil, traceID)
		return
	}
	if req.Port < 1 || req.Port > 65535 {
		util.RespondError(w, types.ErrValidationOutOfRange, "port out of range", nil, traceID)
		return
	}
	spec := strconv.Itoa(req.Port)
	if req.Proto == "tcp" || req.Proto == "udp" {
		spec += "/" + req.Proto
	}
	if err := h.svc.DeletePort(r.Context(), spec); err != nil {
		h.respondServiceErr(w, err, traceID)
		return
	}
	h.audit(r, "firewall.delete", req.Port, req.Proto)
	util.RespondJSON(w, http.StatusOK, types.APIResponse[firewallStatusResponse]{
		Data: h.buildStatus(r), RequestID: traceID,
	})
}

// buildStatus probes status and (when manageable) lists rules.
func (h *FirewallHandler) buildStatus(r *http.Request) firewallStatusResponse {
	st := h.svc.DetectStatus(r.Context())
	resp := firewallStatusResponse{Status: st, Note: securityGroupNote}
	if st.CanManage {
		if rules, err := h.svc.ListRules(r.Context()); err == nil {
			resp.Rules = rules
		} else if h.logger != nil {
			h.logger.Warn("firewall: list rules failed", slog.String("err", err.Error()))
		}
	}
	return resp
}

// respondServiceErr maps firewall.Service errors onto API error codes.
func (h *FirewallHandler) respondServiceErr(w http.ResponseWriter, err error, traceID string) {
	switch {
	case errors.Is(err, firewall.ErrProtectedPort):
		util.RespondError(w, types.ErrAuthForbidden,
			"该端口受保护（SSH / 面板访问端口），不可删除", nil, traceID)
	case errors.Is(err, firewall.ErrInvalidPort):
		util.RespondError(w, types.ErrValidationOutOfRange, "端口需为 1-65535", nil, traceID)
	case errors.Is(err, firewall.ErrInvalidProto):
		util.RespondError(w, types.ErrValidationInvalidFormat, "协议仅支持 tcp / udp", nil, traceID)
	case errors.Is(err, firewall.ErrNotDeletable):
		util.RespondError(w, types.ErrValidationOutOfRange,
			"该规则不是简单端口规则，无法通过面板删除", nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Warn("firewall: operation failed", slog.String("err", err.Error()))
		}
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
	}
}

// audit emits a structured log line for the mutation. The middleware audit
// trail records the request envelope; this adds the actor + port detail.
func (h *FirewallHandler) audit(r *http.Request, action string, port int, proto string) {
	if h.logger == nil {
		return
	}
	actor := ""
	if u, ok := auth.UserFromContext(r.Context()); ok && u != nil {
		actor = u.Username
	}
	h.logger.Info("firewall mutation",
		slog.String("action", action),
		slog.String("actor", actor),
		slog.Int("port", port),
		slog.String("proto", proto),
	)
}
