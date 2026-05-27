package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/shortlink"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// ShortLinkHandler exposes the M-OPS short link HTTP surface:
//
//   - GET /s/{code}                       — public 302 redirect
//   - GET /api/shortlinks                 — list current user's links
//   - POST /api/shortlinks                — create a new link
//   - DELETE /api/shortlinks/{code}       — delete by combined code (caller-only)
//   - DELETE /api/shortlinks/{fileCode}/{userCode} — alias matching contract §1
//
// The handler is intentionally thin — all business logic lives in the
// shortlink.Service so the route layer stays declarative.
type ShortLinkHandler struct {
	service *shortlink.Service
	logger  *slog.Logger
	baseURL string // optional public-facing base, used to compose ShortURL
}

// ShortLinkHandlerConfig wires the handler.
type ShortLinkHandlerConfig struct {
	Service *shortlink.Service
	Logger  *slog.Logger
	BaseURL string
}

// NewShortLinkHandler constructs the handler.
func NewShortLinkHandler(cfg ShortLinkHandlerConfig) *ShortLinkHandler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &ShortLinkHandler{
		service: cfg.Service,
		logger:  cfg.Logger,
		baseURL: cfg.BaseURL,
	}
}

// Redirect implements GET /s/{code}. Public endpoint — no auth required.
// Resolves the combined code to a target URL and emits a 302; failure
// returns 404 (the nginx-mimicking 404 body is reserved for silent-mode
// rejection, not "code not found").
func (h *ShortLinkHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.PathValue("code"))
	if code == "" {
		http.NotFound(w, r)
		return
	}
	target, err := h.service.Resolve(r.Context(), code)
	if err != nil {
		if errors.Is(err, storage.ErrShortLinkNotFound) ||
			errors.Is(err, shortlink.ErrInvalidCode) {
			http.NotFound(w, r)
			return
		}
		h.logger.Warn("shortlink: resolve failed",
			slog.String("code", code),
			slog.String("err", err.Error()))
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// List implements GET /api/shortlinks — current user's links, newest first.
func (h *ShortLinkHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		util.RespondError(w, types.ErrAuthTokenInvalid, "no user", nil, traceID)
		return
	}
	records, err := h.service.ListByUser(r.Context(), user.ID)
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, err.Error(), nil, traceID)
		return
	}
	out := make([]types.ShortLink, 0, len(records))
	for _, rec := range records {
		out = append(out, h.toDTO(r, rec))
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.ShortLink]{
		Data: out, RequestID: traceID,
	})
}

// Create implements POST /api/shortlinks. Body is types.CreateShortLinkRequest.
// The expires_at field is unix-ms; 0 = permanent.
func (h *ShortLinkHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		util.RespondError(w, types.ErrAuthTokenInvalid, "no user", nil, traceID)
		return
	}
	var req types.CreateShortLinkRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, err.Error(), nil, traceID)
		return
	}
	req.TargetURL = strings.TrimSpace(req.TargetURL)
	if req.TargetURL == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "target_url required", nil, traceID)
		return
	}
	if !isValidURL(req.TargetURL) {
		util.RespondError(w, types.ErrValidationInvalidFormat, "target_url must be http(s)", nil, traceID)
		return
	}
	var expiresPtr *time.Time
	if req.ExpiresAt > 0 {
		t := time.UnixMilli(req.ExpiresAt)
		expiresPtr = &t
	}
	rec, err := h.service.Generate(r.Context(), user.ID, req.TargetURL, expiresPtr)
	if err != nil {
		if errors.Is(err, shortlink.ErrTargetEmpty) {
			util.RespondError(w, types.ErrValidationRequiredField, err.Error(), nil, traceID)
			return
		}
		util.RespondError(w, types.ErrInternalDatabase, err.Error(), nil, traceID)
		return
	}
	dto := h.toDTO(r, *rec)
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.ShortLink]{
		Data: dto, RequestID: traceID,
	})
}

// Delete implements DELETE /api/shortlinks/{fileCode}/{userCode}.
func (h *ShortLinkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		util.RespondError(w, types.ErrAuthTokenInvalid, "no user", nil, traceID)
		return
	}
	fileCode := strings.TrimSpace(r.PathValue("fileCode"))
	userCode := strings.TrimSpace(r.PathValue("userCode"))
	if fileCode == "" || userCode == "" {
		// Fall back to the combined-code variant for the alias route.
		combined := strings.TrimSpace(r.PathValue("code"))
		split := splitCombinedAtBestMatch(r, h.service, combined)
		fileCode, userCode = split.file, split.user
	}
	if fileCode == "" || userCode == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "code required", nil, traceID)
		return
	}
	if err := h.service.Delete(r.Context(), fileCode, userCode, user.ID); err != nil {
		if errors.Is(err, storage.ErrShortLinkNotFound) {
			util.RespondError(w, types.ErrNotFoundShortLink, "short link not found", nil, traceID)
			return
		}
		util.RespondError(w, types.ErrInternalDatabase, err.Error(), nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[map[string]bool]{
		Data: map[string]bool{"deleted": true}, RequestID: traceID,
	})
}

// codeSplit is the result of best-effort splitting a combined code.
type codeSplit struct {
	file string
	user string
}

// splitCombinedAtBestMatch enumerates every split point of combined and
// returns the first that resolves to an actual row. Used only by the alias
// DELETE route — the canonical route receives both halves as path params.
func splitCombinedAtBestMatch(r *http.Request, svc *shortlink.Service, combined string) codeSplit {
	for i := 1; i < len(combined); i++ {
		if rec, err := svc.ResolveSplit(r.Context(), combined[:i], combined[i:]); err == nil && rec != nil {
			return codeSplit{file: rec.FileCode, user: rec.UserCode}
		}
	}
	return codeSplit{}
}

// toDTO copies the record into a wire-format types.ShortLink. The
// ShortURL field is composed against either the configured base URL or the
// request host so admins / users see a click-ready link.
func (h *ShortLinkHandler) toDTO(r *http.Request, rec storage.ShortLinkRecord) types.ShortLink {
	return types.ShortLink{
		FileCode:  rec.FileCode,
		UserCode:  rec.UserCode,
		UserID:    rec.UserID,
		TargetURL: rec.TargetURL,
		ShortURL:  h.composeShortURL(r, rec.FileCode+rec.UserCode),
		ExpiresAt: rec.ExpiresAt,
		CreatedAt: rec.CreatedAt,
	}
}

// composeShortURL builds <scheme>://<host>/s/<combined>. Honors the
// X-Forwarded-Proto / X-Forwarded-Host headers when behind a proxy.
func (h *ShortLinkHandler) composeShortURL(r *http.Request, combined string) string {
	if h.baseURL != "" {
		return strings.TrimRight(h.baseURL, "/") + "/s/" + combined
	}
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host + "/s/" + combined
}

// isValidURL is a tiny check that the request's target_url starts with
// http:// or https://. We do NOT call url.Parse here because Go's parser is
// extremely lenient (it accepts almost anything), which would let a
// malicious value like "javascript:..." slip in.
func isValidURL(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
