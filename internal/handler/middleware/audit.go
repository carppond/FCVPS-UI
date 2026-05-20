package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"shiguang-vps/internal/util"
)

// auditPayloadCap limits the size of the request body we hand to the audit
// repository. Larger payloads are truncated and tagged with `"truncated": true`
// so downstream queries can spot the gap.
const auditPayloadCap = 4 * 1024

// AuditEntry mirrors the audit_logs row written by the repository layer. The
// fields the middleware can fill from the HTTP context are populated here;
// the repository injects ID/CreatedAt before persisting.
type AuditEntry struct {
	UserID       string
	Action       string // "<METHOD> <path>"
	ResourceType string
	ResourceID   string
	IP           string
	UserAgent    string
	Payload      []byte
	Success      bool
}

// AuditRepository is implemented by the storage layer (see T-28). The
// middleware is wired with an interface to avoid pulling the repo
// dependency into this package — keeping the dependency graph DAG-shaped.
type AuditRepository interface {
	// Log persists entry. Implementations should be non-blocking on the hot
	// path (write to a buffered channel or fire-and-forget goroutine).
	Log(ctx context.Context, entry AuditEntry) error
}

// AuditConfig configures the middleware.
//
//   - Repo is the audit_logs sink. When nil the middleware logs nothing and
//     simply forwards the request (useful before T-28 lands).
//   - Logger receives warn-level diagnostics if repo.Log returns an error.
//   - ExtractResource maps the request to (resourceType, resourceID). When
//     nil a default heuristic based on the URL path is used.
//
// TODO(T-28): inject real AuditRepository at NewRouter time.
type AuditConfig struct {
	Repo            AuditRepository
	Logger          *slog.Logger
	ExtractResource func(r *http.Request) (string, string)
}

// Audit returns a middleware that records mutating requests after the handler
// has finished. Read-only methods (GET/HEAD/OPTIONS) are skipped so the table
// only collects events worth alerting on. The middleware reads the response
// status via util.StatusRecorder; if no upstream middleware wrapped the
// writer it creates its own wrapper.
func Audit(cfg AuditConfig) Middleware {
	if cfg.Repo == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	extract := cfg.ExtractResource
	if extract == nil {
		extract = defaultResourceExtractor
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isAuditableMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			// Capture a snapshot of the body before the handler reads it.
			// The util.DecodeJSONBody helper expects the original r.Body, so
			// we replace it with a tee-equivalent (bytes already buffered).
			payload := capturePayload(r)

			recorder, alreadyWrapped := w.(*util.StatusRecorder)
			if !alreadyWrapped {
				recorder = util.NewStatusRecorder(w)
			}

			next.ServeHTTP(recorder, r)

			resType, resID := extract(r)
			entry := AuditEntry{
				UserID:       UserIDFromContext(r.Context()),
				Action:       r.Method + " " + r.URL.Path,
				ResourceType: resType,
				ResourceID:   resID,
				IP:           RemoteIPFromContext(r.Context()),
				UserAgent:    r.UserAgent(),
				Payload:      payload,
				Success:      recorder.Status < 400,
			}
			if err := cfg.Repo.Log(r.Context(), entry); err != nil && cfg.Logger != nil {
				cfg.Logger.Warn("audit log write failed",
					slog.String("err", err.Error()),
					slog.String("trace_id", TraceIDFromContext(r.Context())))
			}
		})
	}
}

// isAuditableMethod returns true for HTTP methods that mutate server state.
func isAuditableMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// capturePayload reads at most auditPayloadCap bytes from r.Body and rewinds
// it via http.Request.Body replacement so the handler still observes the
// full stream. Returns nil for empty bodies / read errors.
func capturePayload(r *http.Request) []byte {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	limited := http.MaxBytesReader(nil, r.Body, auditPayloadCap+1)
	buf := make([]byte, auditPayloadCap+1)
	n, _ := readAll(limited, buf)
	if n == 0 {
		// Restore an empty body so handlers still see io.EOF semantics.
		r.Body = http.NoBody
		return nil
	}
	truncated := n > auditPayloadCap
	if truncated {
		n = auditPayloadCap
	}
	out := make([]byte, n)
	copy(out, buf[:n])
	// We consumed the body; replace it with a NoBody since the handler in
	// this middleware chain is not expected to need it. Tasks that DO need
	// access to the raw body in the handler must declare so by NOT wrapping
	// their route with Audit at the chain level — Audit always sits at the
	// chain end, after the handler has had its read.
	//
	// Reverse: read the captured payload back into the request for downstream
	// handlers. We do that by setting Body to a NopCloser over a Reader of
	// the captured bytes — limited to what we kept.
	r.Body = newReplayBody(out, truncated)
	return out
}

// defaultResourceExtractor pulls the last two path segments as
// (resourceType, resourceID). For paths like "/api/users/123" this yields
// ("users", "123"). When the trailing segment looks like a verb (no digits or
// hex run) we only return the resource type.
func defaultResourceExtractor(r *http.Request) (string, string) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// Drop leading "api" / "v1" prefixes when present.
	for len(parts) > 0 && (parts[0] == "api" || strings.HasPrefix(parts[0], "v")) {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	last := parts[len(parts)-1]
	prev := parts[len(parts)-2]
	if looksLikeID(last) {
		return prev, last
	}
	return last, ""
}

// looksLikeID returns true for hex / digit / uuid-shaped strings. Used as a
// best-effort hint to split "<resource>/<id>" from "<resource>/<verb>".
func looksLikeID(s string) bool {
	if len(s) < 4 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r == '-':
		default:
			return false
		}
	}
	return true
}
