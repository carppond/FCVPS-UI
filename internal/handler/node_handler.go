// Package handler — node module endpoints.
//
// Implements the M-NODE HTTP surface (T-11):
//
//   - GET    /api/nodes                       — list across user
//   - GET    /api/nodes/{id}                  — detail
//   - POST   /api/subscriptions/{subID}/nodes — manual create (URI string)
//   - PATCH  /api/nodes/{id}                  — partial update (tag/tags/chain)
//   - DELETE /api/nodes/{id}                  — delete
//   - POST   /api/nodes/{id}/copy-uri         — return raw_uri (clipboard)
//
// TCPing endpoints (POST /api/tcping/*, POST /api/nodes/{id}/tcping) live in
// tcping_handler.go but share the same NodeHandler instance for repo access.
package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/substore"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// NodeHandler hosts the M-NODE endpoints. It keeps a reference to the
// subscription repo so manual-create can validate that the owning
// subscription belongs to the calling user and is of type=manual.
type NodeHandler struct {
	nodes  *storage.NodeRepo
	subs   *storage.SubscriptionRepo
	logger *slog.Logger
}

// NewNodeHandler wires the handler. Either repo may be nil — the routes
// degrade to 500 in that case; the router only mounts the handler when both
// are present.
func NewNodeHandler(nodes *storage.NodeRepo, subs *storage.SubscriptionRepo, logger *slog.Logger) *NodeHandler {
	return &NodeHandler{nodes: nodes, subs: subs, logger: logger}
}

// List implements GET /api/nodes.
//
// Query params:
//
//   - page / page_size — standard pagination.
//   - protocol         — exact filter (vmess / vless / ...).
//   - tag              — JSON-membership filter against nodes.tags.
//   - search           — case-insensitive substring match on tag/server/raw_uri.
//   - subscription_id  — narrow to a single subscription (also satisfied by
//     /api/subscriptions/:id/nodes which calls into this same handler).
//   - sort             — "latency_asc" | "latency_desc" | "created_asc".
func (h *NodeHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	q := r.URL.Query()
	records, total, err := h.nodes.ListByUser(r.Context(), user.ID, storage.NodeListOptions{
		Page:           page.Page,
		PageSize:       page.PageSize,
		Search:         strings.TrimSpace(q.Get("search")),
		Protocol:       strings.TrimSpace(q.Get("protocol")),
		Tag:            strings.TrimSpace(q.Get("tag")),
		SubscriptionID: strings.TrimSpace(q.Get("subscription_id")),
		Sort:           strings.TrimSpace(q.Get("sort")),
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	items := make([]types.NodeWithLatency, len(records))
	for i := range records {
		items[i] = storage.NodeRecordToDTOWithLatency(&records[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.NodeWithLatency]]{
		Data: types.PagedResponse[types.NodeWithLatency]{
			Items:    items,
			Total:    total,
			Page:     page.Page,
			PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// ListBySubscription implements GET /api/subscriptions/{id}/nodes. Same
// payload as List but scoped to a single subscription.
func (h *NodeHandler) ListBySubscription(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	subID := r.PathValue("id")
	// Validate the subscription belongs to the user (avoids leaking IDs).
	if _, err := h.subs.GetByID(r.Context(), subID, user.ID); err != nil {
		h.respondSubErr(w, traceID, err)
		return
	}
	records, err := h.nodes.ListBySubscription(r.Context(), subID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	items := make([]types.NodeWithLatency, len(records))
	for i := range records {
		items[i] = storage.NodeRecordToDTOWithLatency(&records[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.NodeWithLatency]]{
		Data: types.PagedResponse[types.NodeWithLatency]{
			Items:    items,
			Total:    int64(len(items)),
			Page:     1,
			PageSize: len(items),
		},
		RequestID: traceID,
	})
}

// Get implements GET /api/nodes/{id}.
func (h *NodeHandler) Get(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	rec, err := h.nodes.GetByID(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.NodeWithLatency]{
		Data:      storage.NodeRecordToDTOWithLatency(rec),
		RequestID: traceID,
	})
}

// Create implements POST /api/subscriptions/{id}/nodes for manual creation.
//
// The caller must own a subscription whose type == "manual"; url / upload
// subscriptions are managed by SyncService and rejecting manual writes here
// avoids accidental drift between subscriptions.raw_content and the nodes
// table.
func (h *NodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	subID := r.PathValue("id")
	sub, err := h.subs.GetByID(r.Context(), subID, user.ID)
	if err != nil {
		h.respondSubErr(w, traceID, err)
		return
	}
	if sub.Type != string(types.SubTypeManual) {
		util.RespondError(w, types.ErrValidationInvalidFormat,
			"only manual subscriptions accept hand-added nodes", nil, traceID)
		return
	}

	var req types.AddNodeRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	rawURI := strings.TrimSpace(req.RawURI)
	if rawURI == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "raw_uri required", nil, traceID)
		return
	}
	parsed, err := substore.ParseURI(rawURI)
	if err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat,
			"parse uri: "+err.Error(), nil, traceID)
		return
	}
	cfg := parsedNodeToMap(parsed)
	rec := storage.NodeRecord{
		SubscriptionID: subID,
		RawURI:         rawURI,
		Protocol:       parsed.Protocol,
		Server:         parsed.Server,
		Port:           int32(parsed.Port),
		Tag:            firstNonEmpty(parsed.Name, parsed.Tag),
		Tags:           []string{},
		ParsedConfig:   cfg,
	}
	created, err := h.nodes.Create(r.Context(), rec)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	// Re-read so joined fields (user_id) populate.
	stored, err := h.nodes.GetByID(r.Context(), created.ID, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.NodeWithLatency]{
		Data:      storage.NodeRecordToDTOWithLatency(stored),
		RequestID: traceID,
	})
}

// Update implements PATCH/PUT /api/nodes/{id}.
//
// Body shape mirrors types.UpdateNodeRequest; non-present fields leave the
// column untouched.
func (h *NodeHandler) Update(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	var req types.UpdateNodeRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	upd := storage.NodeUpdate{}
	if req.Tags != nil {
		upd.Tags = &req.Tags
	}
	if req.ChainParentID != "" {
		// We accept the special value "-" to clear the link so PATCH
		// semantics still allow "unset". This keeps the omitempty JSON tag
		// while preserving explicit-clear capability.
		val := req.ChainParentID
		if val == "-" {
			val = ""
		}
		upd.ChainParentID = &val
	}
	if err := h.nodes.Update(r.Context(), id, user.ID, upd); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	rec, err := h.nodes.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.NodeWithLatency]{
		Data:      storage.NodeRecordToDTOWithLatency(rec),
		RequestID: traceID,
	})
}

// Delete implements DELETE /api/nodes/{id}.
func (h *NodeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	if err := h.nodes.Delete(r.Context(), r.PathValue("id"), user.ID); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// CopyURI implements POST /api/nodes/{id}/copy-uri. Returns the raw_uri so
// the front end can place it on the clipboard. The endpoint is a POST rather
// than a GET so it can be audited (the audit middleware only records
// non-idempotent verbs).
func (h *NodeHandler) CopyURI(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	rec, err := h.nodes.GetByID(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[map[string]string]{
		Data:      map[string]string{"raw_uri": rec.RawURI},
		RequestID: traceID,
	})
}

// respondStorageErr translates repo-layer errors into the canonical envelope.
func (h *NodeHandler) respondStorageErr(w http.ResponseWriter, traceID string, err error) {
	switch {
	case errors.Is(err, storage.ErrNodeNotFound):
		util.RespondError(w, types.ErrNotFoundNode, "node not found", nil, traceID)
	case errors.Is(err, storage.ErrSubscriptionNotFound):
		util.RespondError(w, types.ErrNotFoundSubscription, "subscription not found", nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Error("node handler failed",
				slog.String("err", err.Error()), slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalUnknown, "internal error", nil, traceID)
	}
}

// respondSubErr is a thin wrapper used when the failing call was against the
// subscription repo (so the not-found code is ErrNotFoundSubscription).
func (h *NodeHandler) respondSubErr(w http.ResponseWriter, traceID string, err error) {
	if errors.Is(err, storage.ErrSubscriptionNotFound) {
		util.RespondError(w, types.ErrNotFoundSubscription, "subscription not found", nil, traceID)
		return
	}
	if h.logger != nil {
		h.logger.Error("node handler subscription lookup",
			slog.String("err", err.Error()), slog.String("trace_id", traceID))
	}
	util.RespondError(w, types.ErrInternalUnknown, "internal error", nil, traceID)
}

// parsedNodeToMap mirrors the substore adapter helper but is duplicated here
// because the handler package cannot import substore's unexported helpers.
// Kept narrow: only the fields the API DTO surfaces are written.
func parsedNodeToMap(p *substore.ParsedNode) map[string]any {
	if p == nil {
		return map[string]any{}
	}
	m := map[string]any{
		"name":     p.Name,
		"protocol": p.Protocol,
		"server":   p.Server,
		"port":     p.Port,
		"network":  p.Network,
		"tls":      p.TLS,
	}
	if p.UUID != "" {
		m["uuid"] = p.UUID
	}
	if p.Password != "" {
		m["password"] = p.Password
	}
	if p.Method != "" {
		m["method"] = p.Method
	}
	if p.SNI != "" {
		m["sni"] = p.SNI
	}
	if len(p.ALPN) > 0 {
		m["alpn"] = p.ALPN
	}
	if p.Path != "" {
		m["path"] = p.Path
	}
	if p.Host != "" {
		m["host"] = p.Host
	}
	if p.Reality {
		m["reality"] = true
	}
	if p.Tag != "" {
		m["tag"] = p.Tag
	}
	if len(p.Raw) > 0 {
		m["_raw"] = p.Raw
	}
	return m
}

// firstNonEmpty returns the first non-empty argument or "" if all are empty.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
