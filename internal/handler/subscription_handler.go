package handler

import (
	"errors"
	"io"
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

// maxUploadBytes caps the multipart upload size for POST /api/subscriptions/upload.
// 4 MiB comfortably covers a hand-crafted Clash YAML; larger files indicate
// abuse or a misuse of the upload path (URL subscriptions should be re-fetched).
const maxUploadBytes = 4 * 1024 * 1024

// SubscriptionHandler hosts /api/subscriptions/* endpoints.
type SubscriptionHandler struct {
	repo         *storage.SubscriptionRepo
	nodeRepo     *storage.NodeRepo
	pipelineRepo *storage.PipelineRepo
	sync         *substore.SyncService
	logger       *slog.Logger
}

// NewSubscriptionHandler wires the handler. sync may be nil; manual-create
// flows still work but POST /sync degrades to 501.
func NewSubscriptionHandler(repo *storage.SubscriptionRepo, sync *substore.SyncService, logger *slog.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{repo: repo, sync: sync, logger: logger}
}

// SetPipelineRepo wires the pipeline binding store. The optional setter lets
// callers (cmd/server) attach the repo after construction so existing
// callers / tests that do not exercise the binding endpoints stay
// unchanged. nil disables the GET / PUT /api/subscriptions/{id}/pipelines
// endpoints (handlers respond 501).
func (h *SubscriptionHandler) SetNodeRepo(repo *storage.NodeRepo) {
	if h == nil {
		return
	}
	h.nodeRepo = repo
}

func (h *SubscriptionHandler) SetPipelineRepo(repo *storage.PipelineRepo) {
	if h == nil {
		return
	}
	h.pipelineRepo = repo
}

// List implements GET /api/subscriptions.
//
// Returns the page slice + total count; share_token is intentionally omitted
// from the Subscription DTO so it does not leak in bulk listings (see
// docs/05-tech-lead-plan.md §1.3).
func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	subs, total, err := h.repo.List(r.Context(), user.ID, storage.SubscriptionListOptions{
		Page:     page.Page,
		PageSize: page.PageSize,
		Keyword:  r.URL.Query().Get("keyword"),
		Type:     r.URL.Query().Get("type"),
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	items := make([]types.Subscription, len(subs))
	for i := range subs {
		items[i] = subscriptionRecordToDTO(&subs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.Subscription]]{
		Data: types.PagedResponse[types.Subscription]{
			Items:    items,
			Total:    total,
			Page:     page.Page,
			PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// Create implements POST /api/subscriptions for type=url|manual.
//
// Upload-style creates flow through POST /api/subscriptions/upload (multipart).
func (h *SubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.CreateSubscriptionRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.Name == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "name required", nil, traceID)
		return
	}
	switch req.Type {
	case types.SubTypeURL:
		if req.SourceURL == "" {
			util.RespondError(w, types.ErrValidationRequiredField, "source_url required", nil, traceID)
			return
		}
	case types.SubTypeManual:
		// Manual subscriptions accept no upstream source; nodes added later.
	case types.SubTypeUpload:
		util.RespondError(w, types.ErrValidationInvalidFormat,
			"use POST /api/subscriptions/upload for uploads", nil, traceID)
		return
	default:
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid type", nil, traceID)
		return
	}
	rec := storage.SubscriptionRecord{
		ID:            util.UUIDv7(),
		UserID:        user.ID,
		Name:          req.Name,
		Type:          string(req.Type),
		SourceURL:     req.SourceURL,
		UA:            req.UA,
		SyncInterval:  req.SyncInterval,
		Tags:          req.Tags,
		Remark:        req.Remark,
		AllowInsecure: req.AllowInsecure,
	}
	created, err := h.repo.Create(r.Context(), rec)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.SubscriptionDetail]{
		Data:      subscriptionRecordToDetail(created),
		RequestID: traceID,
	})
}

// Get implements GET /api/subscriptions/{id}.
//
// Returns SubscriptionDetail including share_token. Node list is filled by
// T-11; in this task it stays an empty slice so the contract is honoured.
func (h *SubscriptionHandler) Get(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	detail := subscriptionRecordToDetail(rec)
	if h.nodeRepo != nil {
		nodeRecs, err := h.nodeRepo.ListBySubscription(r.Context(), id)
		if err == nil {
			nodes := make([]types.Node, len(nodeRecs))
			for i := range nodeRecs {
				nodes[i] = storage.NodeRecordToDTO(&nodeRecs[i])
			}
			detail.Nodes = nodes
			detail.NodesTotal = int32(len(nodes))
		}
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.SubscriptionDetail]{
		Data:      detail,
		RequestID: traceID,
	})
}

// Update implements PUT /api/subscriptions/{id}.
//
// Per architecture §5.1 the canonical verb is PATCH; we accept the more
// commonly used PUT too via the same handler (the route registration is
// PATCH; PUT is registered alongside in mountSubscriptionRoutes).
func (h *SubscriptionHandler) Update(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	var req types.UpdateSubscriptionRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	upd := storage.SubscriptionUpdate{}
	if req.Name != "" {
		upd.Name = stringPtr(req.Name)
	}
	if req.SourceURL != "" {
		upd.SourceURL = stringPtr(req.SourceURL)
	}
	if req.UA != "" {
		upd.UA = stringPtr(req.UA)
	}
	if req.SyncInterval > 0 {
		upd.SyncInterval = int32Ptr(req.SyncInterval)
	}
	if req.Tags != nil {
		upd.Tags = &req.Tags
	}
	if req.Remark != "" {
		upd.Remark = stringPtr(req.Remark)
	}
	if req.AllowInsecure != nil {
		upd.AllowInsecure = req.AllowInsecure
	}
	if err := h.repo.Update(r.Context(), id, user.ID, upd); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.SubscriptionDetail]{
		Data:      subscriptionRecordToDetail(rec),
		RequestID: traceID,
	})
}

// Delete implements DELETE /api/subscriptions/{id}.
func (h *SubscriptionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.repo.Delete(r.Context(), id, user.ID); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// Sync implements POST /api/subscriptions/{id}/sync.
func (h *SubscriptionHandler) Sync(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if h.sync == nil {
		util.RespondError(w, types.ErrInternalUnknown, "sync service unavailable", nil, traceID)
		return
	}
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	result, err := h.sync.SyncOne(r.Context(), rec)
	if err != nil {
		// The sync layer already updated last_sync_status=error; map common
		// validation errors to 400 and treat the rest as 500.
		if h.logger != nil {
			h.logger.Warn("subscription sync failed",
				slog.String("subscription_id", id),
				slog.String("err", err.Error()))
		}
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.SyncResult]{
		Data: *result, RequestID: traceID,
	})
}

// RotateShareToken implements POST /api/subscriptions/{id}/rotate-share-token.
func (h *SubscriptionHandler) RotateShareToken(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	token, err := h.repo.RotateShareToken(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.RotateShareTokenResponse]{
		Data:      types.RotateShareTokenResponse{ShareToken: token},
		RequestID: traceID,
	})
}

// Upload implements POST /api/subscriptions/upload (multipart/form-data).
//
// Form fields:
//   - file: required, the YAML / URI-list body.
//   - name: required.
//   - tags / remark / ua / sync_interval: optional.
func (h *SubscriptionHandler) Upload(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	// Hard-cap the body BEFORE parsing — ParseMultipartForm's argument is only
	// the in-memory threshold; oversize uploads otherwise spill to temp disk.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid multipart body", nil, traceID)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "name required", nil, traceID)
		return
	}
	file, fheader, err := r.FormFile("file")
	if err != nil {
		util.RespondError(w, types.ErrValidationRequiredField, "file required", nil, traceID)
		return
	}
	defer file.Close()
	if fheader.Size > maxUploadBytes {
		util.RespondError(w, types.ErrValidationOutOfRange, "file too large", nil, traceID)
		return
	}
	body, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "read file: "+err.Error(), nil, traceID)
		return
	}
	if int64(len(body)) > maxUploadBytes {
		util.RespondError(w, types.ErrValidationOutOfRange, "file too large", nil, traceID)
		return
	}
	rec := storage.SubscriptionRecord{
		ID:         util.UUIDv7(),
		UserID:     user.ID,
		Name:       name,
		Type:       string(types.SubTypeUpload),
		RawContent: body,
		UA:         r.FormValue("ua"),
		Remark:     r.FormValue("remark"),
	}
	if tagsField := r.FormValue("tags"); tagsField != "" {
		// Form-encoded tags arrive as comma separated values.
		parts := strings.Split(tagsField, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		rec.Tags = parts
	}
	created, err := h.repo.Create(r.Context(), rec)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	// Try to sync immediately so nodes appear without a second request.
	if h.sync != nil {
		if _, err := h.sync.SyncOne(r.Context(), created); err != nil && h.logger != nil {
			h.logger.Warn("upload sync failed",
				slog.String("subscription_id", created.ID),
				slog.String("err", err.Error()))
		}
	}
	// Refresh to surface the post-sync sync_status fields.
	if reloaded, err := h.repo.GetByID(r.Context(), created.ID, user.ID); err == nil {
		created = reloaded
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.SubscriptionDetail]{
		Data:      subscriptionRecordToDetail(created),
		RequestID: traceID,
	})
}

// GetPipelines implements GET /api/subscriptions/{id}/pipelines.
//
// Cross-user access returns 404 (information hiding) — the subscription
// lookup is scoped to the caller's user_id before the bindings are
// listed. When the pipeline repo is unwired the endpoint replies 501.
func (h *SubscriptionHandler) GetPipelines(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	if h.pipelineRepo == nil {
		util.RespondError(w, types.ErrInternalUnknown,
			"pipeline bindings unavailable", nil, traceID)
		return
	}
	id := r.PathValue("id")
	if _, err := h.repo.GetByID(r.Context(), id, user.ID); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	rows, err := h.pipelineRepo.ListBindings(r.Context(), id)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	out := make([]types.PipelineBinding, 0, len(rows))
	for _, b := range rows {
		out = append(out, types.PipelineBinding{
			SubscriptionID: b.SubscriptionID,
			PipelineID:     b.PipelineID,
			Position:       b.Position,
			Enabled:        b.Enabled,
		})
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.PipelineBinding]{
		Data: out, RequestID: traceID,
	})
}

// PutPipelines implements PUT /api/subscriptions/{id}/pipelines.
//
// The request body is an UpdatePipelineBindingsRequest. Every pipeline_id in
// the list must belong to the calling user — otherwise the entire request
// is rejected with 404 (information hiding). The replacement is atomic
// inside PipelineRepo.ReplaceBindings.
func (h *SubscriptionHandler) PutPipelines(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	if h.pipelineRepo == nil {
		util.RespondError(w, types.ErrInternalUnknown,
			"pipeline bindings unavailable", nil, traceID)
		return
	}
	id := r.PathValue("id")
	if _, err := h.repo.GetByID(r.Context(), id, user.ID); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	var req types.UpdatePipelineBindingsRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	// Validate every referenced pipeline belongs to the caller before we
	// touch the table. The repo's user_id-scoped GetByID returns
	// ErrPipelineNotFound for both "does not exist" and "belongs to another
	// user" — both surface to the caller as 404, which is the intended
	// behaviour (cross-user 404 == information hiding).
	bindings := make([]storage.PipelineBindingRecord, 0, len(req.Bindings))
	for i, b := range req.Bindings {
		if b.PipelineID == "" {
			util.RespondError(w, types.ErrValidationRequiredField,
				"pipeline_id required", nil, traceID)
			return
		}
		if _, err := h.pipelineRepo.GetByID(r.Context(), b.PipelineID, user.ID); err != nil {
			if errors.Is(err, storage.ErrPipelineNotFound) {
				util.RespondError(w, types.ErrNotFoundPipeline,
					"pipeline not found", nil, traceID)
				return
			}
			h.respondStorageErr(w, traceID, err)
			return
		}
		pos := b.Position
		if pos == 0 {
			pos = int32(i + 1)
		}
		bindings = append(bindings, storage.PipelineBindingRecord{
			SubscriptionID: id,
			PipelineID:     b.PipelineID,
			Position:       pos,
			Enabled:        b.Enabled,
		})
	}
	if err := h.pipelineRepo.ReplaceBindings(r.Context(), id, bindings); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	out := make([]types.PipelineBinding, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, types.PipelineBinding{
			SubscriptionID: b.SubscriptionID,
			PipelineID:     b.PipelineID,
			Position:       b.Position,
			Enabled:        b.Enabled,
		})
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.PipelineBinding]{
		Data: out, RequestID: traceID,
	})
}

// respondStorageErr translates repo errors into the canonical envelope.
func (h *SubscriptionHandler) respondStorageErr(w http.ResponseWriter, traceID string, err error) {
	switch {
	case errors.Is(err, storage.ErrSubscriptionNotFound):
		util.RespondError(w, types.ErrNotFoundSubscription, "subscription not found", nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Error("subscription handler failed",
				slog.String("err", err.Error()), slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalUnknown, "internal error", nil, traceID)
	}
}

// subscriptionRecordToDTO projects the storage record to the API DTO. Note
// share_token is intentionally NOT included; use subscriptionRecordToDetail
// when the full detail is required.
func subscriptionRecordToDTO(rec *storage.SubscriptionRecord) types.Subscription {
	if rec == nil {
		return types.Subscription{}
	}
	dto := types.Subscription{
		ID:             rec.ID,
		UserID:         rec.UserID,
		Name:           rec.Name,
		Type:           types.SubType(rec.Type),
		SourceURL:      rec.SourceURL,
		UA:             rec.UA,
		SyncInterval:   rec.SyncInterval,
		LastSyncedAt:   rec.LastSyncedAt,
		LastSyncStatus: types.SyncStatus(rec.LastSyncStatus),
		LastSyncError:  rec.LastSyncError,
		ExpireAt:       rec.ExpireAt,
		TrafficTotal:   rec.TrafficTotal,
		TrafficUsed:    rec.TrafficUsed,
		Tags:           rec.Tags,
		Remark:         rec.Remark,
		AllowInsecure:  rec.AllowInsecure,
		NodeCount:      rec.NodeCount,
		CreatedAt:      rec.CreatedAt,
		UpdatedAt:      rec.UpdatedAt,
	}
	if dto.Tags == nil {
		dto.Tags = []string{}
	}
	return dto
}

// subscriptionRecordToDetail wraps the storage record into the SubscriptionDetail
// DTO. Nodes / PipelineBindings are left to their zero values; T-11 fills them
// from the node repo (and the explicit GET /api/subscriptions/:id/nodes
// endpoint).
func subscriptionRecordToDetail(rec *storage.SubscriptionRecord) types.SubscriptionDetail {
	return types.SubscriptionDetail{
		Subscription:     subscriptionRecordToDTO(rec),
		ShareToken:       rec.ShareToken,
		Nodes:            []types.Node{},
		NodesTotal:       0,
		PipelineBindings: []types.PipelineBinding{},
	}
}

func stringPtr(s string) *string { return &s }
func int32Ptr(v int32) *int32    { return &v }
