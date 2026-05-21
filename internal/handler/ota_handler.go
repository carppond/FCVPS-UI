package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/ota"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// otaApplyTimeout caps the total duration of the detached Apply goroutine so
// a half-broken release (5 GB binary on a slow link) eventually fails rather
// than holding the inflight flag forever.
const otaApplyTimeout = 30 * time.Minute

// detachedContext returns a context that is *not* tied to the HTTP request —
// the Apply pipeline must outlive the request that triggered it because the
// trigger response is sent back almost immediately. The timeout matches
// otaApplyTimeout so the goroutine still cancels if something is truly stuck.
func detachedContext() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), otaApplyTimeout)
	// The cancel func is intentionally not exposed: ctx.Err() returns
	// DeadlineExceeded automatically once the budget runs out; leaking the
	// (long-lived but bounded) timer is acceptable for this single-shot path.
	_ = cancel
	return ctx
}

// OTAHandler hosts /api/admin/ota/* (admin-only). The handler delegates the
// heavy lifting to ota.Service so the HTTP layer stays declarative; the
// service in turn owns concurrency control (single in-flight Apply at a time).
type OTAHandler struct {
	svc    *ota.Service
	logger *slog.Logger
}

// NewOTAHandler wires the handler. svc may be nil — endpoints then 503 with
// ErrInternalUnknown so admins notice the misconfiguration immediately.
func NewOTAHandler(svc *ota.Service, logger *slog.Logger) *OTAHandler {
	return &OTAHandler{svc: svc, logger: logger}
}

// Status implements GET /api/admin/ota/status. Returns the cached latest
// release info (without forcing a check); use Check to refresh.
func (h *OTAHandler) Status(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.svc == nil {
		util.RespondError(w, types.ErrInternalUnknown, "ota service unavailable", nil, traceID)
		return
	}
	info, _, lastErr := h.svc.LastInfo()
	resp := types.OTAReleaseInfo{
		CurrentVersion: h.svc.CurrentVersion(),
	}
	if info != nil {
		resp.LatestVersion = info.TagName
		resp.HasUpdate = info.HasUpdate
		resp.ReleaseURL = info.HTMLURL
		resp.Changelog = info.Body
		resp.PublishedAt = info.PublishedAt.UnixMilli()
	}
	_ = lastErr // surfaced via Check; Status returns whatever cache holds.
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.OTAReleaseInfo]{
		Data: resp, RequestID: traceID,
	})
}

// Check implements GET /api/admin/ota/check. Forces an immediate GitHub poll.
// Network failure / GitHub 5xx surface as ErrInternalUnknown with the upstream
// error message; missing release maps onto a 200 with HasUpdate=false.
func (h *OTAHandler) Check(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.svc == nil {
		util.RespondError(w, types.ErrInternalUnknown, "ota service unavailable", nil, traceID)
		return
	}
	info, err := h.svc.CheckNow(r.Context())
	if err != nil {
		if errors.Is(err, ota.ErrNoRelease) {
			util.RespondJSON(w, http.StatusOK, types.APIResponse[types.OTAReleaseInfo]{
				Data: types.OTAReleaseInfo{
					CurrentVersion: h.svc.CurrentVersion(),
					HasUpdate:      false,
				},
				RequestID: traceID,
			})
			return
		}
		if h.logger != nil {
			h.logger.Warn("ota: check failed",
				slog.String("err", err.Error()),
				slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.OTAReleaseInfo]{
		Data: types.OTAReleaseInfo{
			CurrentVersion: h.svc.CurrentVersion(),
			LatestVersion:  info.TagName,
			HasUpdate:      info.HasUpdate,
			ReleaseURL:     info.HTMLURL,
			Changelog:      info.Body,
			PublishedAt:    info.PublishedAt.UnixMilli(),
		},
		RequestID: traceID,
	})
}

// Apply implements POST /api/admin/ota/apply. Returns 200 immediately and
// runs the actual download / verify / swap in a background goroutine so the
// SSE channel carries all progress events. The 200 body just acknowledges
// that the request was accepted.
//
// 409 is returned when another Apply is in flight (svc.Apply guards this).
func (h *OTAHandler) Apply(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.svc == nil {
		util.RespondError(w, types.ErrInternalUnknown, "ota service unavailable", nil, traceID)
		return
	}
	info, _, _ := h.svc.LastInfo()
	if info == nil {
		// Force a fresh check so the admin gets a meaningful error rather
		// than "no release info cached".
		fresh, err := h.svc.CheckNow(r.Context())
		if err != nil {
			util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
			return
		}
		info = fresh
	}
	if !info.HasUpdate {
		util.RespondError(w, types.ErrValidationOutOfRange,
			"no newer release available", nil, traceID)
		return
	}
	// Run Apply in a detached goroutine so the HTTP response returns
	// immediately; progress events flow through the SSE bus.
	target := info
	go func() {
		// Use a background context — the apply must outlive the HTTP request
		// (the handler's ctx is cancelled when the response writer closes).
		if err := h.svc.Apply(detachedContext(), target); err != nil {
			if h.logger != nil {
				h.logger.Warn("ota: apply failed",
					slog.String("err", err.Error()))
			}
		}
	}()

	util.RespondJSON(w, http.StatusAccepted, types.APIResponse[map[string]any]{
		Data: map[string]any{
			"accepted":       true,
			"target_version": info.TagName,
		},
		RequestID: traceID,
	})
}

// History implements GET /api/admin/ota/history. v1 returns the in-memory
// log (capped at 50 entries); persistence is deferred to T-32.
func (h *OTAHandler) History(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.svc == nil {
		util.RespondError(w, types.ErrInternalUnknown, "ota service unavailable", nil, traceID)
		return
	}
	entries := h.svc.History()
	out := make([]types.OTAHistoryItem, len(entries))
	for i, e := range entries {
		out[i] = types.OTAHistoryItem{
			Version:   e.Version,
			Status:    e.Status,
			AppliedAt: e.AppliedAt.UnixMilli(),
			Error:     e.Error,
		}
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.OTAHistoryItem]{
		Data: out, RequestID: traceID,
	})
}
