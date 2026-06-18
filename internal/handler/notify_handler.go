package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// NotifyHandler hosts /api/notify/channels/* and /api/notify/events.
type NotifyHandler struct {
	channels *storage.NotificationChannelRepo
	events   *storage.NotificationEventRepo
	manager  *notify.Manager
	registry *notify.Registry
	logger   *slog.Logger
}

// NewNotifyHandler wires the handler. Any nil collaborator disables the
// associated route family (router.go gates on the constructor's presence).
func NewNotifyHandler(channels *storage.NotificationChannelRepo, events *storage.NotificationEventRepo, mgr *notify.Manager, registry *notify.Registry, logger *slog.Logger) *NotifyHandler {
	if registry == nil {
		registry = notify.DefaultRegistry
	}
	return &NotifyHandler{
		channels: channels,
		events:   events,
		manager:  mgr,
		registry: registry,
		logger:   logger,
	}
}

// ListChannels implements GET /api/notify/channels.
func (h *NotifyHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	recs, total, err := h.channels.List(r.Context(), user.ID, storage.NotificationChannelListOptions{
		Page:     page.Page,
		PageSize: page.PageSize,
		Kind:     r.URL.Query().Get("kind"),
		Keyword:  r.URL.Query().Get("keyword"),
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	items := make([]types.NotificationChannel, len(recs))
	for i := range recs {
		items[i] = notifyChannelRecordToDTO(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.NotificationChannel]]{
		Data: types.PagedResponse[types.NotificationChannel]{
			Items:    items,
			Total:    total,
			Page:     page.Page,
			PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// CreateChannel implements POST /api/notify/channels.
func (h *NotifyHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.CreateChannelRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.Kind == "" || req.Name == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "kind and name required", nil, traceID)
		return
	}
	cfgMap, ok := configAsMap(req.Config)
	if !ok {
		util.RespondError(w, types.ErrValidationInvalidFormat, "config must be an object", nil, traceID)
		return
	}
	if _, err := h.registry.Build(string(req.Kind), cfgMap); err != nil {
		util.RespondError(w, types.ErrValidationSchemaMismatch, err.Error(), nil, traceID)
		return
	}
	cfgJSON, _ := json.Marshal(cfgMap)
	events := make([]string, len(req.EventTypes))
	for i, e := range req.EventTypes {
		events[i] = string(e)
	}
	created, err := h.channels.Create(r.Context(), storage.NotificationChannelRecord{
		ID:         util.UUIDv7(),
		UserID:     user.ID,
		Kind:       string(req.Kind),
		Name:       req.Name,
		ConfigJSON: string(cfgJSON),
		Template:   req.Template,
		EventTypes: events,
		Enabled:    req.Enabled,
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.NotificationChannel]{
		Data:      notifyChannelRecordToDTO(created),
		RequestID: traceID,
	})
}

// GetChannel implements GET /api/notify/channels/{id}.
func (h *NotifyHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.channels.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.NotificationChannel]{
		Data:      notifyChannelRecordToDTO(rec),
		RequestID: traceID,
	})
}

// UpdateChannel implements PUT /api/notify/channels/{id}.
func (h *NotifyHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	var req types.UpdateChannelRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	existing, err := h.channels.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	upd := storage.NotificationChannelRecord{
		ID:       id,
		UserID:   user.ID,
		Name:     req.Name,
		Template: req.Template,
	}
	if req.Config != nil {
		cfgMap, ok := configAsMap(req.Config)
		if !ok {
			util.RespondError(w, types.ErrValidationInvalidFormat, "config must be an object", nil, traceID)
			return
		}
		// Restore any secret values the client sent back as the redaction
		// sentinel (i.e. left unchanged) before validating + persisting.
		mergeChannelSecrets(existing.Kind, cfgMap, existing.ConfigJSON)
		if _, err := h.registry.Build(existing.Kind, cfgMap); err != nil {
			util.RespondError(w, types.ErrValidationSchemaMismatch, err.Error(), nil, traceID)
			return
		}
		cfgJSON, _ := json.Marshal(cfgMap)
		upd.ConfigJSON = string(cfgJSON)
	}
	if req.EventTypes != nil {
		events := make([]string, len(req.EventTypes))
		for i, e := range req.EventTypes {
			events[i] = string(e)
		}
		upd.EventTypes = events
	}
	if req.Enabled != nil {
		upd.Enabled = *req.Enabled
	} else {
		upd.Enabled = existing.Enabled
	}
	if err := h.channels.Update(r.Context(), upd); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	rec, err := h.channels.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.NotificationChannel]{
		Data:      notifyChannelRecordToDTO(rec),
		RequestID: traceID,
	})
}

// DeleteChannel implements DELETE /api/notify/channels/{id}.
func (h *NotifyHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.channels.Delete(r.Context(), id, user.ID); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// TestChannel implements POST /api/notify/channels/{id}/test.
func (h *NotifyHandler) TestChannel(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if h.manager == nil {
		util.RespondError(w, types.ErrInternalUnknown, "notify manager unavailable", nil, traceID)
		return
	}
	if err := h.manager.SendTest(r.Context(), id, user.ID); err != nil {
		if errors.Is(err, storage.ErrNotificationChannelNotFound) {
			util.RespondError(w, types.ErrNotFoundChannel, "channel not found", nil, traceID)
			return
		}
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// ListEvents implements GET /api/notify/events. Per-user delivery log.
func (h *NotifyHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	recs, total, err := h.events.ListByUser(r.Context(), user.ID, storage.NotificationEventListOptions{
		Page:      page.Page,
		PageSize:  page.PageSize,
		EventType: r.URL.Query().Get("event_type"),
		Status:    r.URL.Query().Get("status"),
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	items := make([]types.NotificationEvent, len(recs))
	for i := range recs {
		items[i] = notifyEventRecordToDTO(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.NotificationEvent]]{
		Data: types.PagedResponse[types.NotificationEvent]{
			Items:    items,
			Total:    total,
			Page:     page.Page,
			PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// respondStorageErr translates notify repo errors into the canonical envelope.
func (h *NotifyHandler) respondStorageErr(w http.ResponseWriter, traceID string, err error) {
	switch {
	case errors.Is(err, storage.ErrNotificationChannelNotFound):
		util.RespondError(w, types.ErrNotFoundChannel, "channel not found", nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Error("notify handler db failed",
				slog.String("err", err.Error()), slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalDatabase, "internal error", nil, traceID)
	}
}

// configAsMap normalises req.Config (any) into map[string]any. Returns ok=false
// when the JSON shape is not an object.
func configAsMap(cfg any) (map[string]any, bool) {
	if cfg == nil {
		return map[string]any{}, true
	}
	if m, ok := cfg.(map[string]any); ok {
		return m, true
	}
	return nil, false
}

// notifyChannelRecordToDTO projects a storage record into the contract DTO.
func notifyChannelRecordToDTO(rec *storage.NotificationChannelRecord) types.NotificationChannel {
	var cfg any
	if rec.ConfigJSON != "" {
		_ = json.Unmarshal([]byte(rec.ConfigJSON), &cfg)
	}
	events := make([]types.EventType, len(rec.EventTypes))
	for i, e := range rec.EventTypes {
		events[i] = types.EventType(e)
	}
	return types.NotificationChannel{
		ID:         rec.ID,
		UserID:     rec.UserID,
		Kind:       types.ChannelKind(rec.Kind),
		Name:       rec.Name,
		Config:     redactChannelConfig(rec.Kind, cfg),
		Template:   rec.Template,
		EventTypes: events,
		Enabled:    rec.Enabled,
		CreatedAt:  rec.CreatedAt,
		UpdatedAt:  rec.UpdatedAt,
	}
}

// notifyEventRecordToDTO projects a storage delivery record into the DTO.
func notifyEventRecordToDTO(rec *storage.NotificationEventRecord) types.NotificationEvent {
	var payload any
	if rec.PayloadJSON != "" {
		_ = json.Unmarshal([]byte(rec.PayloadJSON), &payload)
	}
	return types.NotificationEvent{
		ID:        rec.ID,
		UserID:    rec.UserID,
		ChannelID: rec.ChannelID,
		EventType: types.EventType(rec.EventType),
		Payload:   payload,
		Status:    types.EventStatus(rec.Status),
		SentAt:    rec.SentAt,
		Error:     rec.Error,
		CreatedAt: rec.CreatedAt,
	}
}
