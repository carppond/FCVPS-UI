// Package handler hosts the HTTP layer for the shiguang-vps hub: the
// project-wide router, the supporting middleware suite and the per-module
// HTTP handlers.
//
// T-3 only ships the router skeleton and /healthz. Business endpoints are
// mounted by T-4..T-29 via their own handler files in this package.
package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// healthzPingTimeout caps the SQLite ping during a /healthz check so a hung
// database does not stall the probe forever.
const healthzPingTimeout = 2 * time.Second

// HealthStatus is the JSON payload returned by /healthz.
type HealthStatus struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// Healthz builds the /healthz handler. It performs a lightweight DB ping and
// returns 503 when the underlying SQLite pool is unreachable.
//
// The endpoint is intentionally public (no auth, no silent-mode prefix). It
// remains rate-limited by the global RateLimit middleware so a probe cannot
// be abused as an amplifier.
func Healthz(deps *Deps) http.Handler {
	if deps == nil {
		deps = &Deps{}
	}
	startedAt := deps.now()
	logger := deps.logger()
	version := deps.Version
	if version == "" {
		version = "dev"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uptime := deps.now().Sub(startedAt)
		traceID := middleware.TraceIDFromContext(r.Context())

		if deps.DB != nil && deps.DB.Read != nil {
			ctx, cancel := context.WithTimeout(r.Context(), healthzPingTimeout)
			defer cancel()
			if err := pingDB(ctx, deps.DB.Read); err != nil {
				if logger != nil {
					logger.Warn("healthz db ping failed",
						slog.String("err", err.Error()),
						slog.String("trace_id", traceID))
				}
				util.RespondError(w, types.ErrInternalDatabase,
					"database unreachable", nil, traceID)
				return
			}
		}

		util.RespondJSON(w, http.StatusOK, HealthStatus{
			Status:        "ok",
			Version:       version,
			UptimeSeconds: int64(uptime.Seconds()),
		})
	})
}

// pingDB issues a single round-trip to the read pool. Helper kept separate so
// future probes (file system, agent pool, etc.) can be added without
// expanding the handler body past the 80-line standards budget.
func pingDB(ctx context.Context, db *sql.DB) error {
	return db.PingContext(ctx)
}
