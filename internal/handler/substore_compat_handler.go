package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if name == "" || token == "" {
		h.notFound(w)
		return
	}
	result, err := h.svc.ServeDownload(r.Context(), name, token, target)
	if err != nil {
		if errors.Is(err, substore.ErrCompatNotFound) {
			h.notFound(w)
			return
		}
		if h.logger != nil {
			h.logger.Error("substore compat download failed",
				slog.String("name", name),
				slog.String("target", target),
				slog.String("err", err.Error()))
		}
		h.notFound(w) // silent mode: never reveal 500s to anon clients
		return
	}
	ct := result.ContentType
	if ct == "" {
		ct = result.YAMLType
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Total-Nodes", strconv.Itoa(result.TotalNodes))
	// Name the profile after the subscription so clients (Clash Verge etc.)
	// don't fall back to the URL's last path segment — which, when imported via
	// a short link like /s/11, would otherwise show the profile as "11".
	w.Header().Set("Content-Disposition", contentDisposition(name))
	// Disable downstream caches so subsequent token rotations take effect.
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(result.Body)
}

// contentDisposition builds an attachment header whose filename is the
// subscription name. It pairs an ASCII-only fallback with an RFC 5987
// filename* so non-ASCII (CJK) names survive in clients that read it.
func contentDisposition(name string) string {
	ascii := asciiFilenameFallback(name)
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s",
		ascii, url.PathEscape(name))
}

// asciiFilenameFallback strips name down to safe printable ASCII (no quotes /
// control chars), defaulting to "subscription" when nothing is left.
func asciiFilenameFallback(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 0x20 && r < 0x7f && r != '"' && r != '\\' {
			b.WriteRune(r)
		}
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		return s
	}
	return "subscription"
}

// notFound writes a minimal 404 response with no leak surface.
func (h *SubstoreCompatHandler) notFound(w http.ResponseWriter) {
	http.NotFound(w, &http.Request{}) // body: "404 page not found\n"
}
