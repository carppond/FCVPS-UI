package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"shiguang-vps/internal/agent"
	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/logger"
	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/storage"
)

// silentPrefixSettingKey is the system_settings row that stores the active
// 32-hex prefix used by the silent mode middleware.
const silentPrefixSettingKey = "silent_mode_prefix"

// Deps bundles cross-cutting collaborators the router (and the handlers it
// mounts) need. Subsequent tasks extend this struct with their own repos
// and services; T-3 only wires the fields used by /healthz and the
// middleware chain.
//
// All fields are optional: NewRouter tolerates nil collaborators so that
// tests can exercise individual handlers without bringing up a full DB.
type Deps struct {
	// DB is the SQLite pool pair. nil disables the DB ping in /healthz and
	// the silent-mode prefix watcher.
	DB *storage.DB

	// Logger is the shared slog logger. nil falls back to logger.Default().
	Logger *slog.Logger

	// Now returns the current wall-clock time. nil falls back to time.Now.
	// Tests inject a fake clock here.
	Now func() time.Time

	// SilentPrefix is the initial 32-hex prefix loaded at startup. When
	// non-empty the silent-mode middleware enforces /_app/<prefix>/ on all
	// non-whitelisted paths. Subsequent rotations are picked up by the
	// background watcher (every middleware.SilentModeReloadInterval).
	SilentPrefix string

	// GlobalRateLimit is the per-IP throughput cap used by RateLimit. nil
	// disables the middleware (useful for tests).
	GlobalRateLimit *ratelimit.Limiter

	// DevMode toggles whether panic stack traces are echoed back to the
	// client. False in production.
	DevMode bool

	// Version is the semver string surfaced by /healthz.
	Version string

	// AuditRepo is wired by T-28; nil means "do not record audit logs"
	// (current state during T-3 development).
	// TODO(T-28): inject real AuditRepository.
	AuditRepo middleware.AuditRepository

	// AuthManager / TokenStore / BruteProtector are populated by T-4 and
	// consumed by the auth + admin user routes. When nil, those routes are
	// not mounted (useful for /healthz-only tests).
	AuthManager    *auth.Manager
	TokenStore     *auth.TokenStore
	BruteProtector *auth.BruteProtector
	TOTPManager    auth.TOTPManager

	// AuthHandler / UserHandler can be supplied explicitly; otherwise NewRouter
	// constructs them from the manager / repos above.
	AuthHandler *AuthHandler
	UserHandler *UserHandler

	// UserRepo is needed by UserHandler. When nil, /api/me/* and
	// /api/admin/users/* are skipped.
	UserRepo *storage.UserRepo

	// SessionRepo backs the /api/me/sessions list / revoke endpoints. May be
	// nil for tests; those routes 501 in that case.
	SessionRepo *storage.SessionRepo

	// SubscriptionHandler is wired by T-8; nil disables /api/subscriptions/*.
	SubscriptionHandler *SubscriptionHandler

	// SubstoreCompatHandler is wired by T-8; nil disables GET /download/:name.
	SubstoreCompatHandler *SubstoreCompatHandler

	// PipelineHandler hosts /api/pipelines/* (T-19). nil disables the routes.
	PipelineHandler *PipelineHandler

	// AgentHandler hosts /api/agents/* (T-14). nil disables the routes.
	AgentHandler *AgentHandler

	// AgentWSHandler hosts GET /api/agent/ws (T-14). nil disables.
	AgentWSHandler *AgentWSHandler

	// AgentHub is exposed so cmd/server can manage its lifecycle. Not used by
	// NewRouter directly; the handlers above already capture the hub.
	AgentHub *agent.Hub

	// NodeHandler hosts /api/nodes/* + /api/subscriptions/{id}/nodes (T-11).
	// nil disables the routes.
	NodeHandler *NodeHandler

	// TCPingHandler hosts /api/tcping/* and /api/nodes/{id}/tcping (T-11).
	// nil disables.
	TCPingHandler *TCPingHandler

	// NotifyHandler hosts /api/notify/channels/* and /api/notify/events (T-22).
	// nil disables the routes.
	NotifyHandler *NotifyHandler

	// StreamHandler hosts GET /api/notify/stream (T-22 SSE). nil disables.
	StreamHandler *StreamHandler

	// LoginRateLimit is the per-(IP|username) login bucket (5/hour by default).
	// nil disables.
	LoginRateLimit *ratelimit.Limiter

	// Silent owns the live silent-mode prefix. Internal — populated by
	// NewRouter when DB is supplied.
	silent *middleware.SilentMode
	mux    *http.ServeMux
	chain  []middleware.Middleware
}

// NewRouter constructs the project's HTTP handler. It returns the
// *http.ServeMux so callers (including tests) can both directly invoke it
// (the middleware chain is applied as a top-level wrapper exposed via the
// Handler method) and mount their own handlers into it before serving.
//
// For production use, callers should serve Deps.Handler — that returns the
// mux wrapped in the global middleware chain (recover → log → ratelimit →
// silent_mode → audit). Calling ServeHTTP on the *http.ServeMux directly
// also exercises the chain because the chain wraps the mux as a whole; we
// install it via SetMuxHandler so all paths (including 404s from unknown
// routes) are subject to silent-mode enforcement.
//
// Only /healthz is mounted at this time; business endpoints will be
// registered by T-4..T-29.
func NewRouter(deps *Deps) *http.ServeMux {
	if deps == nil {
		deps = &Deps{}
	}
	mux := http.NewServeMux()

	silent := middleware.NewSilentMode(middleware.SilentModeConfig{
		InitialPrefix: deps.SilentPrefix,
		Loader:        silentPrefixLoader(deps.DB),
		Logger:        deps.logger(),
		Now:           deps.now,
	})
	deps.silent = silent
	deps.chain = []middleware.Middleware{
		middleware.Recover(deps.logger(), deps.DevMode),
		middleware.RequestLog(deps.logger(), deps.Now),
		middleware.RateLimit(deps.GlobalRateLimit),
		silent.Middleware(),
		middleware.Audit(middleware.AuditConfig{
			Repo:   deps.AuditRepo,
			Logger: deps.logger(),
		}),
	}
	deps.mux = mux

	mux.Handle("GET /healthz", Healthz(deps))
	mountUserRoutes(mux, deps)
	mountSubscriptionRoutes(mux, deps)
	mountSubstoreCompatRoutes(mux, deps)
	mountPipelineRoutes(mux, deps)
	mountAgentRoutes(mux, deps)
	mountNodeRoutes(mux, deps)
	mountTCPingRoutes(mux, deps)
	mountNotifyRoutes(mux, deps)

	// TODO(T-15): mount rule handlers (/api/rules/*).
	// TODO(T-17): mount script handlers (/api/scripts/*).
	// TODO(T-18): mount nezha compat (/api/v1/nezha/*).
	// TODO(T-21): mount traffic handlers (/api/traffic/*).
	// TODO(T-25): mount shortlink handlers (/api/shortlinks, GET /s/:code).
	// TODO(T-26): mount settings + silent-mode rotate (/api/admin/settings/*).
	// TODO(T-28): mount audit log query (/api/admin/audit).
	// TODO(T-29): mount OTA handlers (/api/admin/ota/*).

	return mux
}

// Handler returns the mux wrapped in the global middleware chain. Use this
// value in http.Server.Handler so that silent-mode enforcement, rate
// limiting and recovery cover the entire surface (including the mux's
// implicit 404s for unknown paths).
func (d *Deps) Handler() http.Handler {
	if d == nil || d.mux == nil {
		return http.NotFoundHandler()
	}
	return middleware.Chain(d.mux, d.chain...)
}

// Start launches background watchers spun up by NewRouter (currently just
// the silent-mode prefix poller). Call from main() after NewRouter and
// before http.ListenAndServe. Stop via Shutdown.
func (d *Deps) Start(ctx context.Context) {
	if d == nil || d.silent == nil {
		return
	}
	d.silent.Start(ctx)
}

// Shutdown halts background watchers. Safe to call multiple times.
func (d *Deps) Shutdown() {
	if d == nil || d.silent == nil {
		return
	}
	d.silent.Stop()
}

// SilentMode exposes the live silent-mode controller so the settings handler
// (T-26) can trigger immediate rotation without waiting for the next poll.
func (d *Deps) SilentMode() *middleware.SilentMode {
	if d == nil {
		return nil
	}
	return d.silent
}

func (d *Deps) logger() *slog.Logger {
	if d == nil || d.Logger == nil {
		return logger.Default()
	}
	return d.Logger
}

func (d *Deps) now() time.Time {
	if d == nil || d.Now == nil {
		return time.Now()
	}
	return d.Now()
}

// mountUserRoutes installs the auth + user + admin endpoints when the
// required collaborators are present in deps. Missing dependencies cause the
// routes to be quietly skipped — useful for unit tests that exercise only the
// /healthz path. Each route is wrapped with the appropriate middleware
// (Required / RequireAdmin / RequirePending2FA) plus the canonical handler.
func mountUserRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.TokenStore == nil || deps.AuthManager == nil {
		return
	}
	if deps.AuthHandler == nil {
		deps.AuthHandler = NewAuthHandler(deps.AuthManager, deps.TokenStore,
			deps.BruteProtector, deps.LoginRateLimit, deps.logger())
	}
	if deps.UserHandler == nil && deps.UserRepo != nil && deps.TOTPManager != nil {
		deps.UserHandler = NewUserHandler(deps.AuthManager, deps.UserRepo,
			deps.SessionRepo, deps.TOTPManager, deps.logger())
	}

	// Public auth endpoints.
	mux.Handle("POST /api/auth/login", http.HandlerFunc(deps.AuthHandler.Login))
	mux.Handle("POST /api/auth/verify-totp", http.HandlerFunc(deps.AuthHandler.VerifyTOTP))
	mux.Handle("POST /api/auth/verify-recovery", http.HandlerFunc(deps.AuthHandler.VerifyRecovery))

	// Authenticated endpoints share the Required middleware.
	required := auth.Required(deps.TokenStore)
	requireAdmin := auth.RequireAdmin(deps.TokenStore)

	mux.Handle("POST /api/auth/logout", required(http.HandlerFunc(deps.AuthHandler.Logout)))

	if deps.UserHandler == nil {
		return
	}
	uh := deps.UserHandler
	mux.Handle("GET /api/me", required(http.HandlerFunc(uh.Me)))
	mux.Handle("PATCH /api/me", required(http.HandlerFunc(uh.UpdateMe)))
	mux.Handle("POST /api/me/password", required(http.HandlerFunc(uh.ChangePassword)))
	mux.Handle("DELETE /api/me", required(http.HandlerFunc(uh.DeleteMe)))
	// Contract §5.1.2 documents GET for /api/me/totp/setup (no body needed —
	// the server mints + persists a new secret and returns provisioning data).
	mux.Handle("GET /api/me/totp/setup", required(http.HandlerFunc(uh.TOTPSetup)))
	mux.Handle("POST /api/me/totp/enable", required(http.HandlerFunc(uh.TOTPEnable)))
	mux.Handle("POST /api/me/totp/disable", required(http.HandlerFunc(uh.TOTPDisable)))
	mux.Handle("POST /api/me/totp/recovery-codes", required(http.HandlerFunc(uh.RegenerateRecoveryCodes)))
	mux.Handle("GET /api/me/sessions", required(http.HandlerFunc(uh.ListMySessions)))
	mux.Handle("DELETE /api/me/sessions/{id}", required(http.HandlerFunc(uh.RevokeMySession)))
	mux.Handle("POST /api/auth/refresh", required(http.HandlerFunc(deps.AuthHandler.Refresh)))

	mux.Handle("GET /api/admin/users", requireAdmin(http.HandlerFunc(uh.AdminListUsers)))
	mux.Handle("POST /api/admin/users", requireAdmin(http.HandlerFunc(uh.AdminCreateUser)))
	mux.Handle("GET /api/admin/users/{id}", requireAdmin(http.HandlerFunc(uh.AdminGetUser)))
	mux.Handle("PATCH /api/admin/users/{id}", requireAdmin(http.HandlerFunc(uh.AdminUpdateUser)))
	mux.Handle("DELETE /api/admin/users/{id}", requireAdmin(http.HandlerFunc(uh.AdminDeleteUser)))
	mux.Handle("POST /api/admin/users/{id}/reset-password", requireAdmin(http.HandlerFunc(uh.AdminResetPassword)))
	mux.Handle("POST /api/admin/users/{id}/disable-2fa", requireAdmin(http.HandlerFunc(uh.AdminDisableTOTP)))
	mux.Handle("POST /api/admin/users/{id}/revoke-sessions", requireAdmin(http.HandlerFunc(uh.AdminRevokeSessions)))
}

// mountSubscriptionRoutes installs /api/subscriptions/* when deps carries a
// SubscriptionHandler. Every route is wrapped in auth.Required so anonymous
// callers see 401 (the sub-store compat path is registered separately and is
// the only public surface of M-SUB).
func mountSubscriptionRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.SubscriptionHandler == nil || deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	sh := deps.SubscriptionHandler

	mux.Handle("GET /api/subscriptions", required(http.HandlerFunc(sh.List)))
	mux.Handle("POST /api/subscriptions", required(http.HandlerFunc(sh.Create)))
	mux.Handle("POST /api/subscriptions/upload", required(http.HandlerFunc(sh.Upload)))
	mux.Handle("GET /api/subscriptions/{id}", required(http.HandlerFunc(sh.Get)))
	// Architecture §5.1 lists PATCH for the update verb. We additionally
	// accept PUT (Tech Lead task spec) so the contract and the task spec are
	// both satisfied.
	mux.Handle("PATCH /api/subscriptions/{id}", required(http.HandlerFunc(sh.Update)))
	mux.Handle("PUT /api/subscriptions/{id}", required(http.HandlerFunc(sh.Update)))
	mux.Handle("DELETE /api/subscriptions/{id}", required(http.HandlerFunc(sh.Delete)))
	mux.Handle("POST /api/subscriptions/{id}/sync", required(http.HandlerFunc(sh.Sync)))
	mux.Handle("POST /api/subscriptions/{id}/rotate-share-token",
		required(http.HandlerFunc(sh.RotateShareToken)))
}

// mountSubstoreCompatRoutes installs the sub-store v2 compat path. Public:
// no auth middleware (token validation lives inside the handler).
func mountSubstoreCompatRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.SubstoreCompatHandler == nil {
		return
	}
	mux.Handle("GET /download/{name}", http.HandlerFunc(deps.SubstoreCompatHandler.Download))
}

// mountPipelineRoutes installs /api/pipelines/* when deps carries a
// PipelineHandler. Every endpoint requires authentication; cross-user
// resource access returns 404 (information hiding, see PipelineHandler.Get).
func mountPipelineRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.PipelineHandler == nil || deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	ph := deps.PipelineHandler

	mux.Handle("GET /api/pipelines", required(http.HandlerFunc(ph.List)))
	mux.Handle("POST /api/pipelines", required(http.HandlerFunc(ph.Create)))
	// /api/pipelines/operators must precede /api/pipelines/{id} when using a
	// router with path-prefix dispatch; net/http 1.22+ ServeMux already gives
	// literal segments priority over wildcards, so the registration order
	// here is purely cosmetic. We list it first regardless to mirror the
	// architecture §5.1.6 order.
	mux.Handle("GET /api/pipelines/operators", required(http.HandlerFunc(ph.Operators)))
	mux.Handle("POST /api/pipelines/yaml-to-ast", required(http.HandlerFunc(ph.YAMLToAST)))
	mux.Handle("POST /api/pipelines/ast-to-yaml", required(http.HandlerFunc(ph.ASTToYAML)))

	mux.Handle("GET /api/pipelines/{id}", required(http.HandlerFunc(ph.Get)))
	mux.Handle("PUT /api/pipelines/{id}", required(http.HandlerFunc(ph.Update)))
	mux.Handle("PATCH /api/pipelines/{id}", required(http.HandlerFunc(ph.Update)))
	mux.Handle("DELETE /api/pipelines/{id}", required(http.HandlerFunc(ph.Delete)))
	mux.Handle("POST /api/pipelines/{id}/run", required(http.HandlerFunc(ph.Run)))
}

// mountAgentRoutes installs /api/agents/* + /api/agent/ws. The WS endpoint
// bypasses auth.Required because it authenticates via ?token=<sha256> query
// param (and is in the silent-mode whitelist). REST endpoints are user-scoped
// except /api/admin/agents which is admin-only.
func mountAgentRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.TokenStore == nil {
		return
	}
	if deps.AgentHandler != nil {
		required := auth.Required(deps.TokenStore)
		requireAdmin := auth.RequireAdmin(deps.TokenStore)
		ah := deps.AgentHandler

		mux.Handle("GET /api/agents", required(http.HandlerFunc(ah.List)))
		mux.Handle("POST /api/agents", required(http.HandlerFunc(ah.Create)))
		mux.Handle("GET /api/agents/{id}", required(http.HandlerFunc(ah.Get)))
		mux.Handle("GET /api/agents/{id}/records", required(http.HandlerFunc(ah.Records)))
		mux.Handle("PATCH /api/agents/{id}", required(http.HandlerFunc(ah.Update)))
		mux.Handle("PUT /api/agents/{id}", required(http.HandlerFunc(ah.Update)))
		mux.Handle("DELETE /api/agents/{id}", required(http.HandlerFunc(ah.Delete)))
		mux.Handle("POST /api/agents/{id}/rotate-token", required(http.HandlerFunc(ah.RotateToken)))
		mux.Handle("POST /api/agents/{id}/regen-token", required(http.HandlerFunc(ah.RotateToken)))
		mux.Handle("POST /api/agents/{id}/command", required(http.HandlerFunc(ah.Command)))
		// Contract §1 line 261 lists POST /api/agents/:id/restart as an admin
		// helper that proxies to the WS command channel; same shape as
		// /command but with a fixed payload.
		mux.Handle("POST /api/agents/{id}/restart", required(http.HandlerFunc(ah.Command)))

		mux.Handle("GET /api/admin/agents", requireAdmin(http.HandlerFunc(ah.AdminList)))
	}
	if deps.AgentWSHandler != nil {
		mux.Handle("GET /api/agent/ws", deps.AgentWSHandler.Handler())
	}
}

// mountNodeRoutes installs the M-NODE surface (T-11):
//
//   - /api/nodes (list/get/update/delete/copy-uri)
//   - /api/subscriptions/{id}/nodes (list / manual create)
//
// All endpoints require an authenticated session; cross-user access returns
// 404 via the repo's user_id filter (information hiding, mirrors §5.1.5).
func mountNodeRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.NodeHandler == nil || deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	nh := deps.NodeHandler

	mux.Handle("GET /api/nodes", required(http.HandlerFunc(nh.List)))
	mux.Handle("GET /api/nodes/{id}", required(http.HandlerFunc(nh.Get)))
	mux.Handle("PATCH /api/nodes/{id}", required(http.HandlerFunc(nh.Update)))
	mux.Handle("PUT /api/nodes/{id}", required(http.HandlerFunc(nh.Update)))
	mux.Handle("DELETE /api/nodes/{id}", required(http.HandlerFunc(nh.Delete)))
	mux.Handle("POST /api/nodes/{id}/copy-uri", required(http.HandlerFunc(nh.CopyURI)))

	mux.Handle("GET /api/subscriptions/{id}/nodes", required(http.HandlerFunc(nh.ListBySubscription)))
	mux.Handle("POST /api/subscriptions/{id}/nodes", required(http.HandlerFunc(nh.Create)))
}

// mountTCPingRoutes installs the TCPing surface (T-11). All endpoints share
// the same authenticated middleware; per-node persistence runs inside the
// handler so the route layer stays declarative.
func mountTCPingRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.TCPingHandler == nil || deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	th := deps.TCPingHandler

	mux.Handle("POST /api/tcping/single", required(http.HandlerFunc(th.Single)))
	mux.Handle("POST /api/tcping/batch", required(http.HandlerFunc(th.Batch)))
	mux.Handle("POST /api/nodes/{id}/tcping", required(http.HandlerFunc(th.Node)))
}

// mountNotifyRoutes installs the M-NOTIFY surface (T-22):
//
//   - /api/notify/channels (CRUD + test)
//   - /api/notify/events   (paginated delivery log)
//   - /api/notify/stream   (SSE; bearer token via query param)
//
// All channel endpoints scope to the authenticated user; cross-user access
// returns 404 via the repo's user_id filter. The stream endpoint
// authenticates inside the handler so EventSource clients (which cannot set
// custom headers) work without bespoke middleware.
func mountNotifyRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.TokenStore == nil {
		return
	}
	if deps.NotifyHandler != nil {
		required := auth.Required(deps.TokenStore)
		nh := deps.NotifyHandler
		mux.Handle("GET /api/notify/channels", required(http.HandlerFunc(nh.ListChannels)))
		mux.Handle("POST /api/notify/channels", required(http.HandlerFunc(nh.CreateChannel)))
		mux.Handle("GET /api/notify/channels/{id}", required(http.HandlerFunc(nh.GetChannel)))
		mux.Handle("PUT /api/notify/channels/{id}", required(http.HandlerFunc(nh.UpdateChannel)))
		mux.Handle("PATCH /api/notify/channels/{id}", required(http.HandlerFunc(nh.UpdateChannel)))
		mux.Handle("DELETE /api/notify/channels/{id}", required(http.HandlerFunc(nh.DeleteChannel)))
		mux.Handle("POST /api/notify/channels/{id}/test", required(http.HandlerFunc(nh.TestChannel)))
		mux.Handle("GET /api/notify/events", required(http.HandlerFunc(nh.ListEvents)))
	}
	if deps.StreamHandler != nil {
		// SSE auth runs inside the handler (Authorization header OR query
		// ?token=) so EventSource clients can connect from the browser.
		mux.Handle("GET /api/notify/stream", http.HandlerFunc(deps.StreamHandler.Stream))
	}
}

// silentPrefixLoader returns a loader closure for SilentModeConfig that reads
// the prefix from system_settings. nil DB yields a nil loader which disables
// the background watcher (the initial prefix stays in effect).
func silentPrefixLoader(db *storage.DB) func(ctx context.Context) (string, error) {
	if db == nil || db.Read == nil {
		return nil
	}
	return func(ctx context.Context) (string, error) {
		var value string
		err := db.Read.QueryRowContext(ctx,
			"SELECT value FROM system_settings WHERE key = ?",
			silentPrefixSettingKey,
		).Scan(&value)
		if err != nil {
			if err == sql.ErrNoRows {
				return "", nil
			}
			return "", err
		}
		return value, nil
	}
}
