package nezha

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/util"
)

// Handler hosts POST /api/v1/nezha/heartbeat and POST /api/v1/nezha/report.
// Both paths route to ServeHTTP — the alias exists for forward-compatibility
// with the Nezha agent's two default URLs (§1.7).
type Handler struct {
	agentRepo *storage.AgentRepo
	adapter   Adapter
	logger    *slog.Logger
}

// HandlerConfig wires the handler. agentRepo + adapter are required.
type HandlerConfig struct {
	AgentRepo *storage.AgentRepo
	Adapter   Adapter
	Logger    *slog.Logger
}

// NewHandler builds the handler. Panics when required dependencies are nil
// so misconfigured callers fail fast at startup (consistent with the rest of
// internal/handler).
func NewHandler(cfg HandlerConfig) *Handler {
	if cfg.AgentRepo == nil || cfg.Adapter == nil {
		panic("nezha.NewHandler: AgentRepo + Adapter are required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Handler{
		agentRepo: cfg.AgentRepo,
		adapter:   cfg.Adapter,
		logger:    cfg.Logger,
	}
}

// ServeHTTP implements http.Handler. The function is the single shared entry
// point for both /heartbeat and /report — see (h *Handler).Routes for the
// path → method binding.
//
// Failure modes (per §1.7 + ADR 0006):
//
//   - Wrong method                          → 404 (silent)
//   - Missing / unknown / wrong-kind secret → 404 (silent)
//   - Body unparseable                      → 200 with code=1
//     (a noisy 400 would let scanners fingerprint the route; the Nezha agent
//      treats any 2xx as success and stops retrying, which is what we want
//      even when the payload was junk.)
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if r.Method != http.MethodPost {
		// Silent 404 — never advertise "Method not allowed".
		middleware.Mimic404(w)
		return
	}

	// Read the body up-front so secret extraction can fall through to the
	// JSON-embedded secret (Nezha's default).
	const maxBody = 1 << 20 // 1 MiB — Nezha heartbeats are typically < 4 KB.
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		h.logger.Warn("nezha handler: read body failed",
			slog.String("err", err.Error()),
			slog.String("trace_id", traceID))
		middleware.Mimic404(w)
		return
	}
	_ = r.Body.Close()

	hb, parseErr := UnmarshalNezhaHeartbeat(body)
	if parseErr != nil {
		// Body wasn't valid JSON. We still need the secret to authenticate,
		// so without it we can't safely return any signal. Silent 404 is the
		// conservative choice.
		h.logger.Warn("nezha handler: body parse failed",
			slog.String("err", parseErr.Error()),
			slog.String("trace_id", traceID))
		middleware.Mimic404(w)
		return
	}

	secret := extractSecret(r, hb)
	if secret == "" {
		middleware.Mimic404(w)
		return
	}
	tokenHash := util.SHA256Hex(secret)
	rec, err := h.agentRepo.GetByTokenHash(r.Context(), tokenHash)
	if err != nil {
		if !errors.Is(err, storage.ErrAgentNotFound) {
			h.logger.Warn("nezha handler: repo lookup failed",
				slog.String("err", err.Error()),
				slog.String("trace_id", traceID))
		}
		middleware.Mimic404(w)
		return
	}
	if rec.Kind != "nezha_compat" {
		// Token is real but belongs to a native agent — treat as unknown.
		h.logger.Warn("nezha handler: token kind mismatch",
			slog.String("agent_id", rec.ID),
			slog.String("kind", rec.Kind),
			slog.String("trace_id", traceID))
		middleware.Mimic404(w)
		return
	}

	if err := h.adapter.OnHeartbeat(r.Context(), rec.ID, *hb); err != nil {
		h.logger.Error("nezha handler: adapter failed",
			slog.String("agent_id", rec.ID),
			slog.String("err", err.Error()),
			slog.String("trace_id", traceID))
		// Nezha-style error envelope; the agent treats this as "retry later".
		writeNezhaResponse(w, http.StatusInternalServerError, 1, "internal error")
		return
	}
	writeNezhaResponse(w, http.StatusOK, 0, "ok")
}

// extractSecret implements the three-way secret resolution Nezha agents use:
//
//  1. Authorization: Bearer <secret>  (preferred — explicit)
//  2. ?secret=<token>                  (some Nezha builds support this)
//  3. body.secret                      (Nezha agent default)
//
// Returns the first non-empty value found.
func extractSecret(r *http.Request, hb *NezhaHeartbeat) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(h, "Bearer ") {
			if tok := strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")); tok != "" {
				return tok
			}
		}
	}
	if q := r.URL.Query().Get("secret"); q != "" {
		return q
	}
	if hb != nil && hb.Secret != "" {
		return hb.Secret
	}
	return ""
}

// writeNezhaResponse serialises the Nezha-style `{code, message}` envelope.
// The agent inspects the HTTP status first and `code == 0` second — both are
// required for the heartbeat to be marked successful upstream.
func writeNezhaResponse(w http.ResponseWriter, status, code int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    code,
		"message": message,
	})
}
