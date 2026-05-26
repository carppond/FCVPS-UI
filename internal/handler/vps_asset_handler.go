package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// VpsAssetHandler hosts the REST surface for M-ASSET (/api/vps-assets/*).
type VpsAssetHandler struct {
	repo   *storage.VpsAssetRepo
	logger *slog.Logger
}

// NewVpsAssetHandler wires the handler.
func NewVpsAssetHandler(repo *storage.VpsAssetRepo, logger *slog.Logger) *VpsAssetHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &VpsAssetHandler{repo: repo, logger: logger}
}

// List implements GET /api/vps-assets.
func (h *VpsAssetHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	q := r.URL.Query()
	opts := storage.VpsAssetListOptions{
		Page:     page.Page,
		PageSize: page.PageSize,
		Provider: q.Get("provider"),
		Status:   q.Get("status"),
		Location: q.Get("location"),
		Keyword:  q.Get("keyword"),
	}
	recs, total, err := h.repo.List(r.Context(), user.ID, opts)
	if err != nil {
		h.logger.Error("list vps assets", slog.String("err", err.Error()))
		util.RespondError(w, types.ErrInternalDatabase, "list vps assets", nil, traceID)
		return
	}
	items := make([]types.VpsAsset, len(recs))
	for i := range recs {
		items[i] = toVpsAssetDTO(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.VpsAsset]]{
		Data: types.PagedResponse[types.VpsAsset]{
			Items:    items,
			Total:    total,
			Page:     page.Page,
			PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// Create implements POST /api/vps-assets.
func (h *VpsAssetHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.CreateVpsAssetRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.Name == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "name required", nil, traceID)
		return
	}
	if req.Provider == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "provider required", nil, traceID)
		return
	}
	if req.ExpireAt == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "expire_at required", nil, traceID)
		return
	}
	if req.BillingCycle == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "billing_cycle required", nil, traceID)
		return
	}
	if !isValidBillingCycle(string(req.BillingCycle)) {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid billing_cycle", nil, traceID)
		return
	}

	tagsJSON := "[]"
	if len(req.Tags) > 0 {
		b, _ := json.Marshal(req.Tags)
		tagsJSON = string(b)
	}
	sshPort := req.SSHPort
	if sshPort == 0 {
		sshPort = 22
	}
	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}

	rec := storage.VpsAssetRecord{
		ID:             util.UUIDv7(),
		UserID:         user.ID,
		Name:           req.Name,
		IP:             req.IP,
		SSHPort:        sshPort,
		SSHUser:        req.SSHUser,
		OS:             req.OS,
		Location:       req.Location,
		Provider:       req.Provider,
		Price:          req.Price,
		Currency:       currency,
		BillingCycle:   string(req.BillingCycle),
		Bandwidth:      req.Bandwidth,
		MonthlyTraffic: req.MonthlyTraffic,
		CPU:            req.CPU,
		Memory:         req.Memory,
		Disk:           req.Disk,
		ExpireAt:       req.ExpireAt,
		Notes:          req.Notes,
		AgentID:        req.AgentID,
		Tags:           tagsJSON,
	}
	created, err := h.repo.Create(r.Context(), rec)
	if err != nil {
		h.logger.Error("create vps asset", slog.String("err", err.Error()))
		util.RespondError(w, types.ErrInternalDatabase, "create vps asset", nil, traceID)
		return
	}
	// Re-fetch to get computed fields.
	fetched, err := h.repo.GetByID(r.Context(), created.ID, user.ID)
	if err != nil {
		util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.VpsAsset]{
			Data:      toVpsAssetDTO(created),
			RequestID: traceID,
		})
		return
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.VpsAsset]{
		Data:      toVpsAssetDTO(fetched),
		RequestID: traceID,
	})
}

// Get implements GET /api/vps-assets/{id}.
func (h *VpsAssetHandler) Get(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrVpsAssetNotFound) {
			util.RespondError(w, types.ErrNotFoundVpsAsset, "vps asset not found", nil, traceID)
			return
		}
		h.logger.Error("get vps asset", slog.String("err", err.Error()))
		util.RespondError(w, types.ErrInternalDatabase, "get vps asset", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.VpsAsset]{
		Data:      toVpsAssetDTO(rec),
		RequestID: traceID,
	})
}

// Update implements PUT /api/vps-assets/{id}.
func (h *VpsAssetHandler) Update(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")

	var req types.UpdateVpsAssetRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}

	sets := make(map[string]any)
	if req.Name != "" {
		sets["name"] = req.Name
	}
	if req.IP != nil {
		sets["ip"] = *req.IP
	}
	if req.SSHPort != nil {
		sets["ssh_port"] = *req.SSHPort
	}
	if req.SSHUser != nil {
		sets["ssh_user"] = *req.SSHUser
	}
	if req.OS != nil {
		sets["os"] = *req.OS
	}
	if req.Location != nil {
		sets["location"] = *req.Location
	}
	if req.Provider != "" {
		sets["provider"] = req.Provider
	}
	if req.Price != nil {
		sets["price"] = *req.Price
	}
	if req.Currency != "" {
		sets["currency"] = req.Currency
	}
	if req.BillingCycle != "" {
		if !isValidBillingCycle(string(req.BillingCycle)) {
			util.RespondError(w, types.ErrValidationInvalidFormat, "invalid billing_cycle", nil, traceID)
			return
		}
		sets["billing_cycle"] = string(req.BillingCycle)
	}
	if req.Bandwidth != nil {
		sets["bandwidth"] = *req.Bandwidth
	}
	if req.MonthlyTraffic != nil {
		sets["monthly_traffic"] = *req.MonthlyTraffic
	}
	if req.CPU != nil {
		sets["cpu"] = *req.CPU
	}
	if req.Memory != nil {
		sets["memory"] = *req.Memory
	}
	if req.Disk != nil {
		sets["disk"] = *req.Disk
	}
	if req.ExpireAt != "" {
		sets["expire_at"] = req.ExpireAt
	}
	if req.Notes != nil {
		sets["notes"] = *req.Notes
	}
	if req.AgentID != nil {
		sets["agent_id"] = *req.AgentID
	}
	if req.Tags != nil {
		b, _ := json.Marshal(*req.Tags)
		sets["tags"] = string(b)
	}

	updated, err := h.repo.Update(r.Context(), id, user.ID, sets)
	if err != nil {
		if errors.Is(err, storage.ErrVpsAssetNotFound) {
			util.RespondError(w, types.ErrNotFoundVpsAsset, "vps asset not found", nil, traceID)
			return
		}
		h.logger.Error("update vps asset", slog.String("err", err.Error()))
		util.RespondError(w, types.ErrInternalDatabase, "update vps asset", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.VpsAsset]{
		Data:      toVpsAssetDTO(updated),
		RequestID: traceID,
	})
}

// Delete implements DELETE /api/vps-assets/{id}.
func (h *VpsAssetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.repo.Delete(r.Context(), id, user.ID); err != nil {
		if errors.Is(err, storage.ErrVpsAssetNotFound) {
			util.RespondError(w, types.ErrNotFoundVpsAsset, "vps asset not found", nil, traceID)
			return
		}
		h.logger.Error("delete vps asset", slog.String("err", err.Error()))
		util.RespondError(w, types.ErrInternalDatabase, "delete vps asset", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{
		RequestID: traceID,
	})
}

// Summary implements GET /api/vps-assets/summary.
func (h *VpsAssetHandler) Summary(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())

	total, expiring, expired, err := h.repo.Summary(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("vps asset summary", slog.String("err", err.Error()))
		util.RespondError(w, types.ErrInternalDatabase, "vps asset summary", nil, traceID)
		return
	}

	costMap, err := h.repo.MonthlyCostByUser(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("vps asset monthly cost", slog.String("err", err.Error()))
		util.RespondError(w, types.ErrInternalDatabase, "vps asset monthly cost", nil, traceID)
		return
	}

	costs := make([]types.VpsAssetMonthlyCost, 0, len(costMap))
	for cur, amount := range costMap {
		costs = append(costs, types.VpsAssetMonthlyCost{
			Currency:    cur,
			MonthlyCost: amount,
		})
	}

	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.VpsAssetSummary]{
		Data: types.VpsAssetSummary{
			Total:       total,
			Expiring:    expiring,
			Expired:     expired,
			MonthlyCost: costs,
		},
		RequestID: traceID,
	})
}

// toVpsAssetDTO converts a storage record to the API DTO.
func toVpsAssetDTO(rec *storage.VpsAssetRecord) types.VpsAsset {
	return types.VpsAsset{
		ID:              rec.ID,
		UserID:          rec.UserID,
		Name:            rec.Name,
		IP:              rec.IP,
		SSHPort:         rec.SSHPort,
		SSHUser:         rec.SSHUser,
		OS:              rec.OS,
		Location:        rec.Location,
		Provider:        rec.Provider,
		Price:           rec.Price,
		Currency:        rec.Currency,
		BillingCycle:    types.BillingCycle(rec.BillingCycle),
		Bandwidth:       rec.Bandwidth,
		MonthlyTraffic:  rec.MonthlyTraffic,
		CPU:             rec.CPU,
		Memory:          rec.Memory,
		Disk:            rec.Disk,
		ExpireAt:        rec.ExpireAt,
		Notes:           rec.Notes,
		AgentID:         rec.AgentID,
		Tags:            rec.TagsSlice(),
		CreatedAt:       rec.CreatedAt,
		UpdatedAt:       rec.UpdatedAt,
		DaysUntilExpiry: rec.DaysUntilExpiry,
		Status:          types.VpsAssetStatus(rec.Status),
	}
}

func isValidBillingCycle(cycle string) bool {
	switch cycle {
	case "monthly", "quarterly", "semi_annual", "annual", "biennial", "triennial":
		return true
	default:
		return false
	}
}
