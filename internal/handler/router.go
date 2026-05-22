package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"shiguang-vps/internal/agent"
	"shiguang-vps/internal/auth"
	auditpkg "shiguang-vps/internal/audit"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/logger"
	"shiguang-vps/internal/nezha"
	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/storage"
)

// silentPrefixSettingKey is the system_settings row that stores the active
// 32-hex prefix used by the silent mode middleware.
const silentPrefixSettingKey = "silent_mode_prefix"

// silentEnabledSettingKey is the system_settings row that stores whether
// silent mode is currently enforced. The prefix row may exist while this
// flag is false (operators can disable + re-enable without losing the URL).
const silentEnabledSettingKey = "silent_mode_enabled"

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

	// SilentPrefix is the initial 32-hex prefix loaded at startup. The
	// middleware retains the value across enable/disable cycles so that
	// re-enabling reuses the same entry URL. Subsequent rotations are
	// picked up by the background watcher (every
	// middleware.SilentModeReloadInterval).
	SilentPrefix string

	// SilentEnabled is the initial "silent mode active" flag loaded at
	// startup. When false the middleware is a no-op regardless of
	// SilentPrefix; flipping the flag via the admin enable / disable
	// endpoints reloads it within one poll cycle (or immediately when the
	// handler calls SetEnabled).
	SilentEnabled bool

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

	// OTAHandler hosts /api/admin/ota/* (T-27). nil disables the routes.
	OTAHandler *OTAHandler

	// SettingsHandler hosts /api/admin/settings* and silent-mode rotation
	// (T-26). nil disables the routes.
	SettingsHandler *SettingsHandler

	// BackupHandler hosts /api/admin/backup and /api/admin/backup/restore
	// (T-26). nil disables the routes.
	BackupHandler *BackupHandler

	// NezhaHandler hosts POST /api/v1/nezha/heartbeat + /report (T-17).
	// nil disables the routes (typical for unit tests that don't exercise
	// the Nezha compat layer).
	NezhaHandler *nezha.Handler

	// ScriptHandler hosts /api/scripts/* (T-13 / M-SCRIPT). nil disables
	// the routes (typical for unit tests that don't exercise the script
	// engine).
	ScriptHandler *ScriptHandler

	// RuleHandler hosts /api/rules/* (T-12). nil disables the routes.
	RuleHandler *RuleHandler

	// RuleSetHandler hosts /api/rule-sets/* (T-12 follow-up). nil disables.
	// Manages mihomo / Clash-Meta rule-providers (CRUD + sync + presets).
	RuleSetHandler *RuleSetHandler

	// ProxyGroupHandler hosts /api/proxy-groups/* (T-12 follow-up). nil
	// disables the routes.
	ProxyGroupHandler *ProxyGroupHandler

	// TrafficHandler hosts /api/traffic/* (T-18). nil disables the routes.
	TrafficHandler *TrafficHandler

	// LoginRateLimit is the per-(IP|username) login bucket (5/hour by default).
	// nil disables.
	LoginRateLimit *ratelimit.Limiter

	// ShortLinkHandler hosts /api/shortlinks/* + GET /s/{code} (T-28).
	// nil disables the routes.
	ShortLinkHandler *ShortLinkHandler

	// AuditHandler hosts GET /api/admin/audit (T-28). nil disables.
	AuditHandler *AuditHandler

	// InstallScriptHandler hosts GET /install-agent.sh + /dl/agent-*
	// (T-28). nil disables.
	InstallScriptHandler *InstallScriptHandler

	// TGWebhookHandler receives Telegram bot updates at
	// POST /api/notify/telegram/webhook/{token}. The webhook path itself is
	// unauthenticated (it validates a per-deployment token in the URL); nil
	// disables the route entirely.
	TGWebhookHandler *TGWebhookHandler

	// TGBotSettingsHandler hosts the authenticated admin endpoints for
	// inspecting / rotating the webhook token. nil disables those routes.
	TGBotSettingsHandler *TGBotSettingsHandler

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
		InitialPrefix:  deps.SilentPrefix,
		InitialEnabled: deps.SilentEnabled,
		Loader:         silentPrefixLoader(deps.DB),
		EnabledLoader:  silentEnabledLoader(deps.DB),
		Logger:         deps.logger(),
		Now:            deps.now,
	})
	deps.silent = silent
	deps.chain = []middleware.Middleware{
		middleware.Recover(deps.logger(), deps.DevMode),
		middleware.RequestLog(deps.logger(), deps.Now),
		middleware.RateLimit(deps.GlobalRateLimit),
		silent.Middleware(),
		middleware.Audit(middleware.AuditConfig{
			Repo:            deps.AuditRepo,
			Logger:          deps.logger(),
			ExtractResource: auditpkg.ExtractResource,
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
	mountOTARoutes(mux, deps)
	mountNezhaRoutes(mux, deps)
	mountSettingsRoutes(mux, deps)
	mountBackupRoutes(mux, deps)
	mountScriptRoutes(mux, deps)
	mountRuleRoutes(mux, deps)
	mountRuleSetRoutes(mux, deps)
	mountProxyGroupRoutes(mux, deps)
	mountTrafficRoutes(mux, deps)
	mountShortLinkRoutes(mux, deps)
	mountAuditRoutes(mux, deps)
	mountInstallScriptRoutes(mux, deps)
	mountTGWebhookRoutes(mux, deps)

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

	// Pipeline binding endpoints (architecture §5.1.4 line 1271-1273). The
	// handler 501s when the pipeline repo is unwired so non-pipeline test
	// stacks remain unaffected.
	mux.Handle("GET /api/subscriptions/{id}/pipelines",
		required(http.HandlerFunc(sh.GetPipelines)))
	mux.Handle("PUT /api/subscriptions/{id}/pipelines",
		required(http.HandlerFunc(sh.PutPipelines)))
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

// mountOTARoutes installs the M-OPS OTA admin surface (T-27):
//
//   - GET  /api/admin/ota/status   — cached release info (no GitHub call)
//   - GET  /api/admin/ota/check    — force an immediate GitHub poll
//   - POST /api/admin/ota/apply    — kick off download → verify → restart
//   - GET  /api/admin/ota/history  — in-memory log of past upgrade attempts
//
// All endpoints require role=admin. Progress events flow through the SSE
// channel mounted by mountNotifyRoutes (kind = "ota_progress").
func mountOTARoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.OTAHandler == nil || deps.TokenStore == nil {
		return
	}
	requireAdmin := auth.RequireAdmin(deps.TokenStore)
	oh := deps.OTAHandler
	mux.Handle("GET /api/admin/ota/status", requireAdmin(http.HandlerFunc(oh.Status)))
	mux.Handle("GET /api/admin/ota/check", requireAdmin(http.HandlerFunc(oh.Check)))
	mux.Handle("POST /api/admin/ota/apply", requireAdmin(http.HandlerFunc(oh.Apply)))
	mux.Handle("GET /api/admin/ota/history", requireAdmin(http.HandlerFunc(oh.History)))
}

// mountNezhaRoutes installs the Nezha agent v2 compatibility endpoints
// (T-17). Both paths route to the same handler — the alias mirrors the two
// default URLs Nezha agents post to so an operator does not need to know
// which our hub prefers. The routes are inside the silent-mode whitelist
// (see middleware/silent_mode.go silentWhitelist) so they remain reachable
// regardless of the /_app/<prefix> rotation state; the handler itself does
// the bearer-secret validation and returns a silent 404 on any auth failure.
func mountNezhaRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.NezhaHandler == nil {
		return
	}
	h := deps.NezhaHandler
	mux.Handle("POST /api/v1/nezha/heartbeat", h)
	mux.Handle("POST /api/v1/nezha/report", h)
}

// mountScriptRoutes installs /api/scripts/* (T-13 / M-SCRIPT). Every
// endpoint is user-scoped; cross-user access yields 404 via the repo's
// user_id filter (information hiding). The /test endpoint is rate-limited
// implicitly through the global limiter — the script engine's 5s timeout
// caps any single invocation.
func mountScriptRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.ScriptHandler == nil || deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	sh := deps.ScriptHandler
	mux.Handle("GET /api/scripts", required(http.HandlerFunc(sh.List)))
	mux.Handle("POST /api/scripts", required(http.HandlerFunc(sh.Create)))
	mux.Handle("GET /api/scripts/{id}", required(http.HandlerFunc(sh.Get)))
	mux.Handle("PUT /api/scripts/{id}", required(http.HandlerFunc(sh.Update)))
	mux.Handle("PATCH /api/scripts/{id}", required(http.HandlerFunc(sh.Update)))
	mux.Handle("DELETE /api/scripts/{id}", required(http.HandlerFunc(sh.Delete)))
	mux.Handle("POST /api/scripts/{id}/test", required(http.HandlerFunc(sh.Test)))
	mux.Handle("GET /api/scripts/{id}/logs", required(http.HandlerFunc(sh.Logs)))
}

// mountTrafficRoutes installs the M-TRAFFIC surface (T-18):
//
//   - GET /api/traffic/summary    — current-month rolled-up summary
//   - GET /api/traffic/history    — chart points (range=7d|30d|90d, view=day|month)
//   - GET /api/traffic/by-agent   — per-agent breakdown for the current month
//   - PUT /api/traffic/threshold  — admin: configure alert percentages
//   - PUT /api/traffic/limit      — admin: configure monthly limit (bytes)
//
// Read endpoints require a session and scope to the caller. Mutations
// additionally require role=admin (enforced inside the handler so the route
// registration stays uniform).
func mountTrafficRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.TrafficHandler == nil || deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	th := deps.TrafficHandler
	mux.Handle("GET /api/traffic/summary", required(http.HandlerFunc(th.Summary)))
	mux.Handle("GET /api/traffic/history", required(http.HandlerFunc(th.History)))
	mux.Handle("GET /api/traffic/by-agent", required(http.HandlerFunc(th.ByAgent)))
	mux.Handle("PUT /api/traffic/threshold", required(http.HandlerFunc(th.SetThreshold)))
	// Contract §1 also lists POST /api/traffic/threshold — alias it.
	mux.Handle("POST /api/traffic/threshold", required(http.HandlerFunc(th.SetThreshold)))
	mux.Handle("PUT /api/traffic/limit", required(http.HandlerFunc(th.SetLimit)))
}

// mountSettingsRoutes installs the M-OPS settings surface (T-26):
//
//   - GET   /api/admin/settings                — full k/v map (sensitive masked)
//   - PUT   /api/admin/settings                — batch update (alias: PATCH)
//   - POST  /api/admin/silent-mode/rotate      — generate + apply new prefix
//
// All endpoints are admin-only. The silent-mode rotation force-logs every user
// (purges sessions table) and immediately swaps the middleware's live prefix.
func mountSettingsRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.SettingsHandler == nil || deps.TokenStore == nil {
		return
	}
	requireAdmin := auth.RequireAdmin(deps.TokenStore)
	sh := deps.SettingsHandler
	mux.Handle("GET /api/admin/settings", requireAdmin(http.HandlerFunc(sh.Get)))
	mux.Handle("PUT /api/admin/settings", requireAdmin(http.HandlerFunc(sh.Update)))
	mux.Handle("PATCH /api/admin/settings", requireAdmin(http.HandlerFunc(sh.Update)))
	mux.Handle("POST /api/admin/silent-mode/rotate",
		requireAdmin(http.HandlerFunc(sh.RotateSilent)))
	mux.Handle("POST /api/admin/silent-mode/enable",
		requireAdmin(http.HandlerFunc(sh.EnableSilent)))
	mux.Handle("POST /api/admin/silent-mode/disable",
		requireAdmin(http.HandlerFunc(sh.DisableSilent)))
	mux.Handle("GET /api/admin/silent-mode/status",
		requireAdmin(http.HandlerFunc(sh.StatusSilent)))
	// Contract §1 line 307 names the legacy endpoint
	// POST /api/admin/settings/silent-mode — register it as an alias so the
	// public contract holds without forcing the UI to know about both spellings.
	mux.Handle("POST /api/admin/settings/silent-mode",
		requireAdmin(http.HandlerFunc(sh.RotateSilent)))
}

// mountBackupRoutes installs the M-OPS backup surface (T-26):
//
//   - POST /api/admin/backup           — create + stream tar.gz to client
//   - POST /api/admin/backup/restore   — multipart upload + restore in-place
//
// Both endpoints are admin-only. Restore returns restart_required=true so the
// UI can surface the "service is restarting" banner; the actual exec is the
// operator's responsibility.
func mountBackupRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.BackupHandler == nil || deps.TokenStore == nil {
		return
	}
	requireAdmin := auth.RequireAdmin(deps.TokenStore)
	bh := deps.BackupHandler
	mux.Handle("POST /api/admin/backup", requireAdmin(http.HandlerFunc(bh.Create)))
	mux.Handle("POST /api/admin/backup/restore",
		requireAdmin(http.HandlerFunc(bh.Restore)))
	// Contract §1 line 310 names the restore endpoint /api/admin/restore as
	// well — keep both spellings live so docs + UI agree.
	mux.Handle("POST /api/admin/restore", requireAdmin(http.HandlerFunc(bh.Restore)))
}

// mountRuleRoutes is a forward-compat stub installed alongside T-17 so the
// parallel-agent T-12 work can wire the RuleHandler without re-touching the
// router skeleton. When RuleHandler is nil the function is a no-op.
func mountRuleRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.RuleHandler == nil || deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	rh := deps.RuleHandler
	mux.Handle("GET /api/rules", required(http.HandlerFunc(rh.List)))
	mux.Handle("POST /api/rules", required(http.HandlerFunc(rh.Create)))
	// Literal sub-paths take priority over {id} per net/http 1.22+ ServeMux,
	// so the ordering below is purely cosmetic — we list literal paths first
	// to mirror the contract §1 ordering.
	mux.Handle("GET /api/rules/templates", required(http.HandlerFunc(rh.Templates)))
	mux.Handle("POST /api/rules/reorder", required(http.HandlerFunc(rh.Reorder)))
	mux.Handle("PUT /api/rules/order", required(http.HandlerFunc(rh.Reorder)))
	mux.Handle("GET /api/rules/preview/{subID}", required(http.HandlerFunc(rh.Preview)))

	mux.Handle("GET /api/rules/{id}", required(http.HandlerFunc(rh.Get)))
	mux.Handle("PUT /api/rules/{id}", required(http.HandlerFunc(rh.Update)))
	mux.Handle("PATCH /api/rules/{id}", required(http.HandlerFunc(rh.Update)))
	mux.Handle("DELETE /api/rules/{id}", required(http.HandlerFunc(rh.Delete)))
}

// mountRuleSetRoutes installs the /api/rule-sets/* surface (T-12 follow-up).
//
// Endpoints:
//   - GET    /api/rule-sets               — list with ?keyword + pagination
//   - POST   /api/rule-sets               — create
//   - GET    /api/rule-sets/{id}          — read
//   - PUT    /api/rule-sets/{id}          — update (PATCH alias)
//   - DELETE /api/rule-sets/{id}          — delete
//   - POST   /api/rule-sets/{id}/sync     — trigger immediate URL probe
//   - GET    /api/rule-sets/presets       — built-in catalog (no DB)
//
// All endpoints are user-scoped via auth.Required. The presets endpoint
// is intentionally NOT a literal sub-path of {id} (registered first so
// net/http picks the literal route).
func mountRuleSetRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.RuleSetHandler == nil || deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	rh := deps.RuleSetHandler
	mux.Handle("GET /api/rule-sets", required(http.HandlerFunc(rh.List)))
	mux.Handle("POST /api/rule-sets", required(http.HandlerFunc(rh.Create)))
	mux.Handle("GET /api/rule-sets/presets", required(http.HandlerFunc(rh.Presets)))
	mux.Handle("GET /api/rule-sets/{id}", required(http.HandlerFunc(rh.Get)))
	mux.Handle("PUT /api/rule-sets/{id}", required(http.HandlerFunc(rh.Update)))
	mux.Handle("PATCH /api/rule-sets/{id}", required(http.HandlerFunc(rh.Update)))
	mux.Handle("DELETE /api/rule-sets/{id}", required(http.HandlerFunc(rh.Delete)))
	mux.Handle("POST /api/rule-sets/{id}/sync", required(http.HandlerFunc(rh.Sync)))
}

// mountProxyGroupRoutes installs the /api/proxy-groups/* surface (T-12
// follow-up).
//
// Endpoints:
//   - GET    /api/proxy-groups              — list, sort_order ASC
//   - POST   /api/proxy-groups              — create
//   - POST   /api/proxy-groups/reorder      — batch reorder by id list
//   - GET    /api/proxy-groups/presets      — built-in catalog (no DB)
//   - GET    /api/proxy-groups/{id}         — read
//   - PUT    /api/proxy-groups/{id}         — update (PATCH alias)
//   - DELETE /api/proxy-groups/{id}         — delete
func mountProxyGroupRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.ProxyGroupHandler == nil || deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	ph := deps.ProxyGroupHandler
	mux.Handle("GET /api/proxy-groups", required(http.HandlerFunc(ph.List)))
	mux.Handle("POST /api/proxy-groups", required(http.HandlerFunc(ph.Create)))
	mux.Handle("POST /api/proxy-groups/reorder", required(http.HandlerFunc(ph.Reorder)))
	mux.Handle("GET /api/proxy-groups/presets", required(http.HandlerFunc(ph.Presets)))
	mux.Handle("GET /api/proxy-groups/{id}", required(http.HandlerFunc(ph.Get)))
	mux.Handle("PUT /api/proxy-groups/{id}", required(http.HandlerFunc(ph.Update)))
	mux.Handle("PATCH /api/proxy-groups/{id}", required(http.HandlerFunc(ph.Update)))
	mux.Handle("DELETE /api/proxy-groups/{id}", required(http.HandlerFunc(ph.Delete)))
}

// mountShortLinkRoutes installs the T-28 short-link surface:
//
//   - GET    /s/{code}                                     (public 302 redirect)
//   - GET    /api/shortlinks                               (user-scoped list)
//   - POST   /api/shortlinks                               (user-scoped create)
//   - DELETE /api/shortlinks/{fileCode}/{userCode}         (contract §1)
//   - DELETE /api/shortlinks/{code}                        (combined-code alias)
//
// The public /s/{code} endpoint is in the silent-mode whitelist so a
// shared short link works without an auth gate.
func mountShortLinkRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.ShortLinkHandler == nil {
		return
	}
	sh := deps.ShortLinkHandler
	mux.Handle("GET /s/{code}", http.HandlerFunc(sh.Redirect))

	if deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	mux.Handle("GET /api/shortlinks", required(http.HandlerFunc(sh.List)))
	mux.Handle("POST /api/shortlinks", required(http.HandlerFunc(sh.Create)))
	mux.Handle("DELETE /api/shortlinks/{fileCode}/{userCode}",
		required(http.HandlerFunc(sh.Delete)))
	mux.Handle("DELETE /api/shortlinks/{code}",
		required(http.HandlerFunc(sh.Delete)))
}

// mountAuditRoutes installs the T-28 audit query endpoints. Admin-only
// scope is enforced inside the handler so the user-scoped variant can share
// the same code path.
func mountAuditRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.AuditHandler == nil || deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	requireAdmin := auth.RequireAdmin(deps.TokenStore)
	ah := deps.AuditHandler
	mux.Handle("GET /api/admin/audit", requireAdmin(http.HandlerFunc(ah.List)))
	// Contract §1 line 308 also exposes /api/audit/logs scoped to the
	// caller (user_id auto-filtered).
	mux.Handle("GET /api/audit/logs", required(http.HandlerFunc(ah.List)))
}

// mountTGWebhookRoutes installs the Telegram bot endpoints (bug-3 of
// docs/06-review-backend-round1.md):
//
//   - POST /api/notify/telegram/webhook/{token}     (public — token in URL)
//   - GET  /api/notify/telegram/status              (auth required)
//   - POST /api/notify/telegram/webhook/rotate      (auth + admin required)
//   - POST /api/notify/telegram/webhook/install     (auth + admin required)
//
// The webhook endpoint is intentionally NOT wrapped in auth.Required — the
// caller is Telegram's infrastructure and authenticates via the per-deploy
// token embedded in the URL path. The silent-mode whitelist already
// includes /api/notify/telegram/webhook so the route is reachable
// regardless of prefix rotation (silent_mode.go silentWhitelist).
func mountTGWebhookRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil {
		return
	}
	if deps.TGWebhookHandler != nil {
		mux.Handle("POST /api/notify/telegram/webhook/{token}",
			http.HandlerFunc(deps.TGWebhookHandler.Webhook))
	}
	if deps.TGBotSettingsHandler == nil || deps.TokenStore == nil {
		return
	}
	required := auth.Required(deps.TokenStore)
	requireAdmin := auth.RequireAdmin(deps.TokenStore)
	bh := deps.TGBotSettingsHandler
	mux.Handle("GET /api/notify/telegram/status", required(http.HandlerFunc(bh.Status)))
	mux.Handle("POST /api/notify/telegram/webhook/rotate",
		requireAdmin(http.HandlerFunc(bh.RotateWebhookToken)))
	mux.Handle("POST /api/notify/telegram/webhook/install",
		requireAdmin(http.HandlerFunc(bh.SetWebhook)))
}

// mountInstallScriptRoutes installs the T-28 agent install endpoints. The
// install-agent.sh path is intentionally NOT auth-gated — admin posts the
// pre-shared token in the URL, so anonymous curl can run it. Per §2.9 the
// silent-mode whitelist must include both /install-agent.sh and /dl/agent-*
// so deployments behind a custom prefix still serve the script.
func mountInstallScriptRoutes(mux *http.ServeMux, deps *Deps) {
	if deps == nil || deps.InstallScriptHandler == nil {
		return
	}
	ih := deps.InstallScriptHandler
	mux.Handle("GET /install-agent.sh", http.HandlerFunc(ih.InstallScript))
	mux.Handle("GET /dl/{asset}", http.HandlerFunc(ih.AgentDownload))
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

// silentEnabledLoader returns a loader closure that reads the
// silent_mode_enabled flag from system_settings. Missing row → false (fail
// closed: the middleware should never enforce the prefix when the DB has not
// explicitly opted in). nil DB yields a nil loader.
func silentEnabledLoader(db *storage.DB) func(ctx context.Context) (bool, error) {
	if db == nil || db.Read == nil {
		return nil
	}
	return func(ctx context.Context) (bool, error) {
		var value string
		err := db.Read.QueryRowContext(ctx,
			"SELECT value FROM system_settings WHERE key = ?",
			silentEnabledSettingKey,
		).Scan(&value)
		if err != nil {
			if err == sql.ErrNoRows {
				return false, nil
			}
			return false, err
		}
		return value == "true", nil
	}
}
