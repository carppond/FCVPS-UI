package handler

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// maxBatchSubscriptionIDs caps how many subscriptions a single batch request may
// touch. The web UI only selects within the current page, so this is a generous
// abuse backstop rather than a functional limit.
const maxBatchSubscriptionIDs = 100

// batchSyncConcurrency bounds how many subscriptions sync in parallel so a batch
// of a full page doesn't open a flood of upstream connections at once.
const batchSyncConcurrency = 5

// validateBatchIDs returns a deduplicated, non-empty id list or ("", false-ish)
// via the error response when the input is empty or over the cap.
func (h *SubscriptionHandler) validateBatchIDs(w http.ResponseWriter, traceID string, ids []string) ([]string, bool) {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		util.RespondError(w, types.ErrValidationRequiredField, "ids required", nil, traceID)
		return nil, false
	}
	if len(out) > maxBatchSubscriptionIDs {
		util.RespondError(w, types.ErrValidationOutOfRange, "too many ids", nil, traceID)
		return nil, false
	}
	return out, true
}

// runSubscriptionBatch applies fn to each id sequentially, collecting a per-id
// ok/error result. fn must scope its repo calls to the owner so other users'
// subscriptions are never touched. A failure on one id never aborts the rest.
func runSubscriptionBatch(ids []string, fn func(id string) error) types.SubscriptionBatchResult {
	res := types.SubscriptionBatchResult{Results: make([]types.SubscriptionBatchItemResult, 0, len(ids))}
	for _, id := range ids {
		item := types.SubscriptionBatchItemResult{ID: id, OK: true}
		if err := fn(id); err != nil {
			item.OK, item.Error = false, err.Error()
			res.FailedCount++
		} else {
			res.SucceededCount++
		}
		res.Results = append(res.Results, item)
	}
	return res
}

// BatchSync implements POST /api/subscriptions/batch-sync. Syncs the selected
// subscriptions with a bounded concurrency; each result reports ok/error.
func (h *SubscriptionHandler) BatchSync(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	if h.sync == nil {
		util.RespondError(w, types.ErrInternalUnknown, "sync service unavailable", nil, traceID)
		return
	}
	var req types.SubscriptionBatchSyncRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	ids, ok := h.validateBatchIDs(w, traceID, req.IDs)
	if !ok {
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.SubscriptionBatchResult]{
		Data:      h.batchSyncRun(r.Context(), user.ID, ids),
		RequestID: traceID,
	})
}

// batchSyncRun fans the per-id sync out across batchSyncConcurrency workers,
// preserving input order in the result slice.
func (h *SubscriptionHandler) batchSyncRun(ctx context.Context, userID string, ids []string) types.SubscriptionBatchResult {
	results := make([]types.SubscriptionBatchItemResult, len(ids))
	sem := make(chan struct{}, batchSyncConcurrency)
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, id string) {
			defer wg.Done()
			defer func() { <-sem }()
			item := types.SubscriptionBatchItemResult{ID: id, OK: true}
			rec, err := h.repo.GetByID(ctx, id, userID)
			if err != nil {
				item.OK, item.Error = false, err.Error()
			} else if _, serr := h.sync.SyncOne(ctx, rec); serr != nil {
				item.OK, item.Error = false, serr.Error()
			}
			results[i] = item
		}(i, id)
	}
	wg.Wait()
	res := types.SubscriptionBatchResult{Results: results}
	for _, it := range results {
		if it.OK {
			res.SucceededCount++
		} else {
			res.FailedCount++
		}
	}
	return res
}

// BatchDelete implements POST /api/subscriptions/batch-delete.
func (h *SubscriptionHandler) BatchDelete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.SubscriptionBatchDeleteRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	ids, ok := h.validateBatchIDs(w, traceID, req.IDs)
	if !ok {
		return
	}
	res := runSubscriptionBatch(ids, func(id string) error {
		return h.repo.Delete(r.Context(), id, user.ID)
	})
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.SubscriptionBatchResult]{
		Data: res, RequestID: traceID,
	})
}

// BatchTags implements POST /api/subscriptions/batch-tags (add/remove tags).
func (h *SubscriptionHandler) BatchTags(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.SubscriptionBatchTagsRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	ids, ok := h.validateBatchIDs(w, traceID, req.IDs)
	if !ok {
		return
	}
	if len(req.Add) == 0 && len(req.Remove) == 0 {
		util.RespondError(w, types.ErrValidationRequiredField, "add or remove required", nil, traceID)
		return
	}
	res := runSubscriptionBatch(ids, func(id string) error {
		rec, err := h.repo.GetByID(r.Context(), id, user.ID)
		if err != nil {
			return err
		}
		newTags := applyTagDelta(rec.Tags, req.Add, req.Remove)
		return h.repo.Update(r.Context(), id, user.ID, storage.SubscriptionUpdate{Tags: &newTags})
	})
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.SubscriptionBatchResult]{
		Data: res, RequestID: traceID,
	})
}

// BatchUpdate implements POST /api/subscriptions/batch-update for the shared
// fields (sync_interval, allow_insecure). Per-item fields are excluded.
func (h *SubscriptionHandler) BatchUpdate(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.SubscriptionBatchUpdateRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	ids, ok := h.validateBatchIDs(w, traceID, req.IDs)
	if !ok {
		return
	}
	if req.SyncInterval == nil && req.AllowInsecure == nil {
		util.RespondError(w, types.ErrValidationRequiredField, "nothing to update", nil, traceID)
		return
	}
	res := runSubscriptionBatch(ids, func(id string) error {
		upd := storage.SubscriptionUpdate{}
		if req.SyncInterval != nil {
			upd.SyncInterval = req.SyncInterval
		}
		if req.AllowInsecure != nil {
			upd.AllowInsecure = req.AllowInsecure
		}
		return h.repo.Update(r.Context(), id, user.ID, upd)
	})
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.SubscriptionBatchResult]{
		Data: res, RequestID: traceID,
	})
}

// applyTagDelta merges add into existing, drops remove, dedupes and preserves
// order (existing first, then new adds). Blank tags are ignored.
func applyTagDelta(existing, add, remove []string) []string {
	removeSet := make(map[string]bool, len(remove))
	for _, t := range remove {
		removeSet[strings.TrimSpace(t)] = true
	}
	seen := make(map[string]bool, len(existing)+len(add))
	out := make([]string, 0, len(existing)+len(add))
	appendTag := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" || removeSet[t] || seen[t] {
			return
		}
		seen[t] = true
		out = append(out, t)
	}
	for _, t := range existing {
		appendTag(t)
	}
	for _, t := range add {
		appendTag(t)
	}
	return out
}
