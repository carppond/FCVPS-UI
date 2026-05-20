package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"shiguang-vps/internal/substore"
)

// SubstoreCompatHandler hosts GET /download/:name?token=xxx.
//
// The endpoint is public (no auth middleware) but every failure mode collapses
// to HTTP 404 so a probing client cannot distinguish "wrong token" from
// "subscription does not exist" — silent mode posture per
// docs/05-tech-lead-plan.md §1.3.
type SubstoreCompatHandler struct {
	svc    *substore.SubstoreCompatService
	logger *slog.Logger
}

// NewSubstoreCompatHandler wires the handler. svc must be non-nil.
func NewSubstoreCompatHandler(svc *substore.SubstoreCompatService, logger *slog.Logger) *SubstoreCompatHandler {
	return &SubstoreCompatHandler{svc: svc, logger: logger}
}

// Download implements GET /download/{name}.
//
// Query string carries the share_token. The response is the Clash YAML body
// with the canonical Content-Type plus an X-Total-Nodes header so the
// receiving client (or operator inspecting via curl) can validate the count
// before parsing.
func (h *SubstoreCompatHandler) Download(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if name == "" || token == "" {
		h.notFound(w)
		return
	}
	result, err := h.svc.ServeDownload(r.Context(), name, token)
	if err != nil {
		if errors.Is(err, substore.ErrCompatNotFound) {
			h.notFound(w)
			return
		}
		if h.logger != nil {
			h.logger.Error("substore compat download failed",
				slog.String("name", name),
				slog.String("err", err.Error()))
		}
		h.notFound(w) // silent mode: never reveal 500s to anon clients
		return
	}
	w.Header().Set("Content-Type", result.YAMLType)
	w.Header().Set("X-Total-Nodes", strconv.Itoa(result.TotalNodes))
	// Disable downstream caches so subsequent token rotations take effect.
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(result.Body)
}

// notFound writes a minimal 404 response with no leak surface.
func (h *SubstoreCompatHandler) notFound(w http.ResponseWriter) {
	http.NotFound(w, &http.Request{}) // body: "404 page not found\n"
}
