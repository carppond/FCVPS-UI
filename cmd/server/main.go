// Package main is the entry point for the shiguang-vps hub server.
//
// Startup sequence (docs/05-tech-lead-plan §2.5):
//
//  1. Parse config (CLI flags + env + defaults).
//  2. Build the slog logger.
//  3. Open SQLite + run migrations.
//  4. Wire auth manager, token store, brute protector, totp manager.
//  5. EnsureAdmin — first-boot bootstrap of the initial admin account.
//  6. Build the router + start background watchers.
//  7. Serve HTTP with graceful shutdown on SIGINT / SIGTERM.
//
// Shutdown sequence (docs/05-tech-lead-plan §2.6) handles the reverse, with a
// 30 s deadline on http.Server.Shutdown before the DB / logger are closed.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shiguang-vps/internal/agent"
	"shiguang-vps/internal/audit"
	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/handler"
	"shiguang-vps/internal/logger"
	"shiguang-vps/internal/nezha"
	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/ops"
	"shiguang-vps/internal/ota"
	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/scriptengine"
	"shiguang-vps/internal/shortlink"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/substore"
	"shiguang-vps/internal/traffic"
	"shiguang-vps/internal/util/safehttp"
)

// hubBinaryVersion is the semver advertised to the OTA checker. Replaced at
// build-time via -ldflags "-X main.hubBinaryVersion=v1.2.3". Empty / dev
// values mean the OTA daily sweep skips notifications (a dev binary should
// not pester operators to upgrade).
var hubBinaryVersion = ""

// hubGitHubRepo overrides the upstream release source. Falls back to
// ota.DefaultGitHubRepo at runtime when the env var OTA_GITHUB_REPO is unset.
var hubGitHubRepo = ""

// shutdownTimeout caps how long graceful shutdown waits for in-flight requests
// to drain before forcing the server closed.
const shutdownTimeout = 30 * time.Second

// loginRatePerSecond is the per-(IP|username) login bucket refill rate
// (5 attempts per hour ≈ 0.00139/s). burst=5 lets honest users tolerate a
// quick mistyped-password retry without being rate-limited.
const loginRatePerSecond = 5.0 / 3600.0
const loginRateBurst = 5

func main() {
	if err := run(); err != nil {
		// Use stderr because the logger may not yet be initialised.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(nil)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := logger.Init(logger.Options{
		Level: cfg.Log.Level, Format: cfg.Log.Format, File: cfg.Log.File,
		MaxSizeMB: cfg.Log.MaxSizeMB, MaxAgeDay: cfg.Log.MaxAgeDays,
		Backups:   cfg.Log.MaxBackups,
	}); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	log := logger.Default()
	log.Info("shiguang-vps server starting",
		slog.String("http_addr", cfg.HTTP.Addr),
		slog.String("data_dir", cfg.Database.DataDir),
	)

	db, err := storage.Open(cfg.Database)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("db close", slog.String("err", err.Error()))
		}
	}()
	if err := db.RunMigrations(context.Background()); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	users := storage.NewUserRepo(db, time.Now)
	sessions := storage.NewSessionRepo(db, time.Now)
	subscriptions := storage.NewSubscriptionRepo(db, time.Now)
	nodeRepo := storage.NewNodeRepo(db, time.Now)
	tokens, err := auth.NewTokenStore(auth.TokenStoreConfig{
		Sessions: sessions, Users: users, TTL: cfg.Session.TTL,
	})
	if err != nil {
		return fmt.Errorf("token store: %w", err)
	}
	totpMgr := auth.NewTOTPManager(auth.NewStorageUserAdapter(users), time.Now)
	brute := auth.NewBruteProtector(auth.BruteConfig{Logger: log})
	brute.Start()
	defer brute.Stop()

	authMgr, err := auth.NewManager(auth.ManagerConfig{
		Users: users, Sessions: sessions, Tokens: tokens, TOTP: totpMgr,
		Brute:  brute,
		Logger: log,
	})
	if err != nil {
		return fmt.Errorf("auth manager: %w", err)
	}

	username, plain, err := authMgr.EnsureAdmin(context.Background())
	if err != nil {
		return fmt.Errorf("ensure admin: %w", err)
	}
	if username != "" {
		// First-boot bootstrap: print the generated password exactly once.
		// Operators must capture this from the log; there is no second chance.
		log.Warn("ADMIN BOOTSTRAPPED",
			slog.String("username", username),
			slog.String("password", plain),
			slog.String("note", "save this password; it will not be shown again"),
		)
	}

	// Silent mode initialization (T-26). EnsureInitial generates the prefix
	// once on first boot and reuses the persisted value on subsequent boots,
	// so it must run BEFORE NewRouter — the middleware reads InitialPrefix
	// synchronously and the background watcher only takes over after the
	// first poll interval. settingsRepo is constructed here so the silent-
	// mode manager and downstream consumers (T-18 traffic loops, T-26
	// settings handler) share a single instance.
	settingsRepo := storage.NewSettingsRepo(db, time.Now)
	silentMgr, err := ops.NewSilentMode(ops.SilentModeConfig{
		Repo:   settingsRepo,
		Logger: log,
	})
	if err != nil {
		return fmt.Errorf("silent mode init: %w", err)
	}
	silentPrefix, err := silentMgr.EnsureInitial(context.Background())
	if err != nil {
		return fmt.Errorf("silent mode ensure initial: %w", err)
	}
	if silentPrefix != "" {
		// First-boot bootstrap: print the entry URL exactly once so the
		// operator knows where to log in. The full prefix is logged here
		// intentionally — operators must capture it from this single line,
		// mirroring the ADMIN BOOTSTRAPPED line above. Subsequent boots also
		// re-emit this so a fresh `docker logs hub` can recover the URL
		// without touching the DB.
		log.Warn("SILENT MODE READY",
			slog.String("entry_url", fmt.Sprintf("http://%s/_app/%s/", cfg.HTTP.Addr, silentPrefix)),
			slog.String("prefix", silentPrefix),
			slog.String("note", "this URL is required to reach the login page"),
		)
	}

	// T-8 + T-11: subscription module + real node persistence. The
	// NodeRepoAdapter bridges substore.NodeRepo / NodeFetcher to the concrete
	// storage.NodeRepo without creating a storage → substore import cycle.
	nodeAdapter := substore.NewNodeRepoAdapter(nodeRepo)
	// Bug-5 (review-round1): force every outbound subscription fetch through
	// the SSRF-safe dialer. AllowPrivate is governed by the
	// `allow_private_networks` system setting (read once at startup; admins
	// who flip it need to restart so the protection lifetime is auditable).
	allowPrivate := readAllowPrivateNetworks(db, log)
	subscriptionHTTPClient := safehttp.NewClient(safehttp.Config{
		AllowPrivate: allowPrivate,
	}, substore.DefaultHTTPTimeout)
	syncService, err := substore.NewSyncService(substore.SyncServiceConfig{
		Repo:       subscriptions,
		NodeRepo:   nodeAdapter,
		HTTPClient: subscriptionHTTPClient,
		Logger:     log,
	})
	if err != nil {
		return fmt.Errorf("sync service: %w", err)
	}
	compatService, err := substore.NewSubstoreCompatService(substore.SubstoreCompatConfig{
		Repo:     subscriptions,
		NodeRepo: nodeAdapter,
		Sync:     syncService,
	})
	if err != nil {
		return fmt.Errorf("substore compat service: %w", err)
	}
	subHandler := handler.NewSubscriptionHandler(subscriptions, syncService, log)
	compatHandler := handler.NewSubstoreCompatHandler(compatService, log)
	nodeHandler := handler.NewNodeHandler(nodeRepo, subscriptions, log)
	tcpingHandler := handler.NewTCPingHandler(nodeRepo, log)

	// T-14: agent module wiring. The hub owns the live WebSocket sessions; on
	// startup we mark every pre-existing row offline so stale "online" entries
	// from a previous crash do not falsely report presence.
	agentRepo := storage.NewAgentRepo(db, time.Now)
	agentRecordRepo := storage.NewAgentRecordRepo(db)
	if err := agentRepo.MarkAllOffline(context.Background()); err != nil {
		log.Warn("agent repo: mark-all-offline", slog.String("err", err.Error()))
	}
	agentHub := agent.NewHub(agent.HubConfig{
		AgentRepo:  agentRepo,
		RecordRepo: agentRecordRepo,
		EventBus:   agent.NoopEventBus{}, // T-22 swaps in the SSE publisher.
		Logger:     log,
		Now:        time.Now,
		HubVersion: agent.ProtocolVersion,
	})
	agentHandler := handler.NewAgentHandler(agentRepo, agentRecordRepo, agentHub, log)
	agentWSHandler := handler.NewAgentWSHandler(agentHub, agentRepo, agentRecordRepo, log)

	// T-17: Nezha agent v2 compatibility layer. The adapter shares the same
	// AgentRepo + AgentRecordRepo + EventBus the native WS hub uses so a
	// nezha_compat agent looks identical to the rest of the system. Routes
	// (/api/v1/nezha/heartbeat + /report) sit inside the silent-mode
	// whitelist; auth happens inside the handler (secret → sha256 → repo
	// lookup).
	nezhaAdapter := nezha.NewAdapter(nezha.AdapterDeps{
		AgentRepo:  agentRepo,
		RecordRepo: agentRecordRepo,
		EventBus:   agent.NoopEventBus{}, // T-22 will inject the real SSE bus.
		Logger:     log,
		Now:        time.Now,
	})
	nezhaHandler := nezha.NewHandler(nezha.HandlerConfig{
		AgentRepo: agentRepo,
		Adapter:   nezhaAdapter,
		Logger:    log,
	})

	// T-22: notification subsystem. Builds the channel registry (5 batch-1
	// kinds), the SSE event bus and the manager. The cleanup worker is
	// stopped at shutdown via the returned cancel func.
	//
	// Bug-5 (review-round1): every webhook-style channel inherits the
	// SSRF-safe HTTP client below before the registry binds factories.
	notify.SetDefaultHTTPClient(safehttp.NewClient(safehttp.Config{
		AllowPrivate: allowPrivate,
	}, 15*time.Second))
	notify.RegisterBuiltins(notify.DefaultRegistry)
	notifyChannels := storage.NewNotificationChannelRepo(db, time.Now)
	notifyEvents := storage.NewNotificationEventRepo(db, time.Now)
	notifyBus := notify.NewEventBus()
	notifyMgr, err := notify.NewManager(notify.ManagerConfig{
		ChannelRepo: notifyChannels,
		EventRepo:   notifyEvents,
		UserRepo:    users,
		Registry:    notify.DefaultRegistry,
		Bus:         notifyBus,
		Logger:      log,
		Now:         time.Now,
	})
	if err != nil {
		return fmt.Errorf("notify manager: %w", err)
	}
	notifyHandler := handler.NewNotifyHandler(notifyChannels, notifyEvents, notifyMgr, notify.DefaultRegistry, log)
	streamHandler := handler.NewStreamHandler(tokens, notifyBus, log)

	// T-24 / bug-3 (review-round1): wire the Telegram bot so the webhook
	// routes can be mounted. The bot is best-effort — if the BotConfig
	// cannot be assembled (e.g. nil whitelist resolver because the user repo
	// is offline) the webhook handler stays nil and the routes degrade to
	// 404 instead of crashing the server.
	tgRouter := notify.NewCommandRouter()
	notify.RegisterCommands(tgRouter, notify.CommandsConfig{
		Nodes:         nodeRepo,
		Subscriptions: subscriptions,
		Agents:        agentRepo,
		Channels:      notifyChannels,
		Users:         users,
		AdminCheck:    handler.AdminCheckFromUserRepo(users),
		Now:           time.Now,
	})
	tgBot, tgBotErr := notify.NewBot(notify.BotConfig{
		BotToken:  os.Getenv("TG_BOT_TOKEN"),
		Router:    tgRouter,
		Whitelist: handler.BuildTGWhitelistResolver(notifyChannels, users),
	})
	var tgWebhookHandler *handler.TGWebhookHandler
	var tgBotSettingsHandler *handler.TGBotSettingsHandler
	if tgBotErr != nil {
		log.Warn("tg bot disabled", slog.String("err", tgBotErr.Error()))
	} else {
		// Reuse the settingsRepo constructed during the silent-mode
		// bootstrap above; SettingsRepo is a thin wrapper around the shared
		// *storage.DB pool so a single instance can fan out to every caller.
		tgWebhookHandler = handler.NewTGWebhookHandler(tgBot, settingsRepo, log)
		tgBotSettingsHandler = handler.NewTGBotSettingsHandler(settingsRepo, notifyChannels, tgBot, log)
	}

	// T-27: OTA service. The hub version + repo can be overridden via env
	// vars (OTA_GITHUB_REPO) so private forks point at their own release
	// stream without rebuilding. The admin user-id is best-effort: we look up
	// the bootstrapped admin via users.LookupAdmin so the ota_available
	// email reaches the operator without manual config.
	otaRepo := hubGitHubRepo
	if v := os.Getenv("OTA_GITHUB_REPO"); v != "" {
		otaRepo = v
	}
	otaSvc, err := ota.NewService(ota.ServiceConfig{
		DB:             db,
		CurrentVersion: hubBinaryVersion,
		GitHubRepo:     otaRepo,
		Logger:         log,
		Now:            time.Now,
		NotifyManager:  notifyMgr,
		EventBus:       notifyBus,
		AdminUserID:    lookupAdminUserID(users, log),
	})
	if err != nil {
		return fmt.Errorf("ota service: %w", err)
	}
	otaHandler := handler.NewOTAHandler(otaSvc, log)

	// T-18: traffic aggregation, monthly reset, threshold checker and the
	// 7-day agent_records cleanup. settingsRepo (constructed alongside the
	// silent-mode bootstrap above) doubles as the persistence layer for
	// threshold state + monthly limit + reset-day; reused here to avoid two
	// instances racing on the same write pool.
	trafficRepo := storage.NewTrafficRepo(db, time.Now)
	trafficAggregator, err := traffic.NewAggregator(traffic.AggregatorConfig{
		AgentRepo:       agentRepo,
		AgentRecordRepo: agentRecordRepo,
		TrafficRepo:     trafficRepo,
		SettingsRepo:    settingsRepo,
		Logger:          log,
	})
	if err != nil {
		return fmt.Errorf("traffic aggregator: %w", err)
	}
	trafficThreshold, err := traffic.NewThreshold(traffic.ThresholdConfig{
		TrafficRepo:  trafficRepo,
		SettingsRepo: settingsRepo,
		Notify:       notifyMgr,
		Logger:       log,
	})
	if err != nil {
		return fmt.Errorf("traffic threshold: %w", err)
	}
	trafficMonthlyReset, err := traffic.NewMonthlyReset(traffic.MonthlyResetConfig{
		TrafficRepo:  trafficRepo,
		SettingsRepo: settingsRepo,
		UserRepo:     users,
		Threshold:    trafficThreshold,
		Notify:       notifyMgr,
		Logger:       log,
	})
	if err != nil {
		return fmt.Errorf("traffic monthly reset: %w", err)
	}
	trafficCleanup, err := traffic.NewCleanup(traffic.CleanupConfig{
		AgentRecordRepo: agentRecordRepo,
		Logger:          log,
	})
	if err != nil {
		return fmt.Errorf("traffic cleanup: %w", err)
	}
	trafficHandler := handler.NewTrafficHandler(trafficRepo, agentRepo, settingsRepo, log)

	// T-13: M-SCRIPT goja sandbox + per-user CRUD. The engine is process-
	// wide (sync.Pool inside) so a single instance handles every concurrent
	// /test invocation without per-request VM construction cost.
	scriptRepo := storage.NewScriptRepo(db, time.Now)
	scriptEngine := scriptengine.NewEngine(log)
	scriptHandler := handler.NewScriptHandler(scriptRepo, scriptEngine, log)

	// T-19 / bug-4 (review-round1): wire the pipeline repo so the
	// subscription handler can serve the binding endpoints + the existing
	// PipelineHandler can manage CRUD.
	pipelineRepo := storage.NewPipelineRepo(db, time.Now)
	pipelineHandler := handler.NewPipelineHandler(pipelineRepo, subscriptions, log)
	subHandler.SetPipelineRepo(pipelineRepo)

	// T-12: M-RULE handler. Reuses the substore NodeRepoAdapter from above so
	// the preview endpoint can re-render Clash YAML against real nodes.
	customRuleRepo := storage.NewCustomRuleRepo(db, time.Now)
	ruleHandler := handler.NewRuleHandler(customRuleRepo, subscriptions, nodeAdapter, log)

	// T-12 follow-up: rule-set (rule-provider) handler + proxy-group handler.
	// rule-set sync uses the SSRF-safe HTTP client (HEAD-only; allows public
	// internet egress to reach gh-proxy / jsdelivr / raw.githubusercontent
	// while keeping internal addresses blocked unless allow_private_networks
	// is set).
	ruleSetRepo := storage.NewRuleSetProviderRepo(db, time.Now)
	ruleSetHTTPClient := safehttp.NewClient(safehttp.Config{
		AllowPrivate: allowPrivate,
	}, 10*time.Second)
	ruleSetHandler := handler.NewRuleSetHandler(ruleSetRepo, ruleSetHTTPClient, log, time.Now)
	proxyGroupRepo := storage.NewProxyGroupRepo(db, time.Now)
	proxyGroupHandler := handler.NewProxyGroupHandler(proxyGroupRepo, log)

	// T-26: settings + backup handlers. SettingsHandler exposes
	// /api/admin/settings and the silent-mode rotate endpoint; it borrows the
	// same ops.SilentMode instance constructed during the bootstrap above so
	// rotations take effect immediately without waiting for the watcher poll.
	settingsHandler := handler.NewSettingsHandler(handler.SettingsHandlerConfig{
		Repo:   settingsRepo,
		Silent: silentMgr,
		Logger: log,
	})
	backupSvc, err := ops.NewBackup(ops.BackupConfig{
		DB:     db,
		Repo:   settingsRepo,
		Logger: log,
	})
	if err != nil {
		return fmt.Errorf("backup service: %w", err)
	}
	backupHandler := handler.NewBackupHandler(backupSvc, log)

	// T-28: short-link / audit / install-script handlers.
	//
	// audit.Logger drains middleware AuditEntry values onto a worker goroutine
	// so the request hot path never blocks on the DB write. It is also wired
	// into deps.AuditRepo below so the middleware chain has somewhere to send
	// entries. Stop() in the shutdown defer flushes any in-flight buffer.
	auditRepo := storage.NewAuditRepo(db, time.Now)
	auditLogger := audit.New(audit.Config{
		Repo:   auditRepo,
		Logger: log,
		Now:    time.Now,
	})
	auditHandler := handler.NewAuditHandler(handler.AuditHandlerConfig{
		Repo:   auditRepo,
		Logger: log,
	})
	shortlinkRepo := storage.NewShortLinkRepo(db, time.Now)
	shortlinkSvc := shortlink.New(shortlinkRepo, log, time.Now)
	shortlinkHandler := handler.NewShortLinkHandler(handler.ShortLinkHandlerConfig{
		Service: shortlinkSvc,
		Logger:  log,
	})
	installScriptHandler := handler.NewInstallScriptHandler(handler.InstallScriptHandlerConfig{
		Logger: log,
	})

	deps := &handler.Deps{
		DB: db, Logger: log, Now: time.Now,
		Version:               "v0.0.0-dev",
		SilentPrefix:          silentPrefix,
		AuthManager:           authMgr,
		TokenStore:            tokens,
		BruteProtector:        brute,
		TOTPManager:           totpMgr,
		UserRepo:              users,
		SessionRepo:           sessions,
		SubscriptionHandler:   subHandler,
		SubstoreCompatHandler: compatHandler,
		AgentHandler:          agentHandler,
		AgentWSHandler:        agentWSHandler,
		AgentHub:              agentHub,
		NodeHandler:           nodeHandler,
		TCPingHandler:         tcpingHandler,
		NotifyHandler:         notifyHandler,
		StreamHandler:         streamHandler,
		OTAHandler:            otaHandler,
		NezhaHandler:          nezhaHandler,
		TrafficHandler:        trafficHandler,
		ScriptHandler:         scriptHandler,
		PipelineHandler:       pipelineHandler,
		RuleHandler:           ruleHandler,
		RuleSetHandler:        ruleSetHandler,
		ProxyGroupHandler:     proxyGroupHandler,
		SettingsHandler:       settingsHandler,
		BackupHandler:         backupHandler,
		ShortLinkHandler:      shortlinkHandler,
		AuditHandler:          auditHandler,
		AuditRepo:             auditLogger,
		InstallScriptHandler:  installScriptHandler,
		TGWebhookHandler:      tgWebhookHandler,
		TGBotSettingsHandler:  tgBotSettingsHandler,
		LoginRateLimit:        ratelimit.New(loginRatePerSecond, loginRateBurst, 0),
		GlobalRateLimit:       ratelimit.New(100, 200, 0),
	}
	mux := handler.NewRouter(deps)
	_ = mux
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps.Start(rootCtx)
	defer deps.Shutdown()
	// T-28: drain audit log entries on a worker goroutine. Stop() during
	// shutdown flushes whatever is still on the in-memory queue so a SIGTERM
	// does not silently drop the last few audit rows.
	auditLogger.Start(rootCtx)
	defer auditLogger.Stop()
	// T-14: 7-day retention sweep for agent_records. Runs at 24h cadence; a
	// short kick-off delay defers the first sweep so server startup stays
	// snappy under load.
	go runAgentRecordsRetention(rootCtx, agentRecordRepo, log)
	// T-22: kick off the notification_events retention sweep (30 days).
	stopNotifyCleanup := notifyMgr.StartCleanup(rootCtx)
	defer stopNotifyCleanup()
	// T-27: daily OTA poll. Stops on shutdown so the goroutine does not
	// outlive the DB pool (the checker logs would otherwise spam errors).
	stopOTAChecker := otaSvc.StartChecker(rootCtx)
	defer stopOTAChecker()
	// T-18: traffic background workers. Three loops:
	//   - aggregator at 00:30 UTC rolls yesterday's agent_records.
	//   - monthly reset at 00:00 UTC fires recap + clears threshold state.
	//   - cleanup at 03:00 UTC purges agent_records > 7 days.
	// Each StartDaily returns its own stop func so shutdown deterministically
	// cancels every goroutine before the DB pool is closed.
	stopTrafficAggregator := trafficAggregator.StartDaily(rootCtx)
	defer stopTrafficAggregator()
	stopMonthlyReset := trafficMonthlyReset.StartDaily(rootCtx)
	defer stopMonthlyReset()
	stopTrafficCleanup := trafficCleanup.StartDaily(rootCtx)
	defer stopTrafficCleanup()
	// Ensure the hub broadcasts bye{server_shutdown} during graceful exit.
	defer agentHub.Close()

	// Bug-7 (review-round1): bound every wall-clock budget so a slowloris
	// client cannot pin a goroutine indefinitely. The long-running paths
	// (OTA download / SSE / subscription sync) manage their own ctx
	// timeouts internally and do not hit these limits because they stream
	// the body in one shot before holding the connection open.
	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           deps.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	// Surface listener errors back to the main goroutine.
	srvErrCh := make(chan error, 1)
	go func() {
		log.Info("listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErrCh <- err
			return
		}
		srvErrCh <- nil
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-srvErrCh:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
	case sig := <-sigCh:
		log.Info("shutdown signal received", slog.String("signal", sig.String()))
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown", slog.String("err", err.Error()))
	}
	return nil
}

// readAllowPrivateNetworks reads the `allow_private_networks` system
// setting (bug-5 of docs/06-review-backend-round1.md). The setting opts
// the deployment OUT of the SSRF dialer guard so admins who legitimately
// fetch http://10.x.y.z/some/internal can do so. Default is false.
//
// Errors / missing rows are treated as "false" — fail closed.
func readAllowPrivateNetworks(db *storage.DB, log *slog.Logger) bool {
	if db == nil {
		return false
	}
	repo := storage.NewSettingsRepo(db, time.Now)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	value, err := repo.Get(ctx, "allow_private_networks")
	if err != nil {
		if !errors.Is(err, storage.ErrSettingNotFound) && log != nil {
			log.Warn("safehttp: read allow_private_networks failed",
				slog.String("err", err.Error()))
		}
		return false
	}
	switch value {
	case "true", "1", "yes", "on":
		if log != nil {
			log.Warn("safehttp: allow_private_networks=true; outbound SSRF guard DISABLED")
		}
		return true
	}
	return false
}

// lookupAdminUserID returns the first admin user's ID, or "" when no admin
// exists yet (the OTA service tolerates an empty user-id — the SSE broadcast
// still works, only the email/telegram emission is skipped).
func lookupAdminUserID(users *storage.UserRepo, log *slog.Logger) string {
	if users == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	list, _, err := users.List(ctx, storage.UserListOptions{
		Page: 1, PageSize: 1, Role: "admin",
	})
	if err != nil {
		if log != nil {
			log.Warn("ota: lookup admin user failed", slog.String("err", err.Error()))
		}
		return ""
	}
	if len(list) == 0 {
		return ""
	}
	return list[0].ID
}

// agentRecordsRetention is the 7-day retention window for high-frequency
// metric samples (PRD M-AGENT.5 + Tech Lead §2.5/2.6).
const agentRecordsRetention = 7 * 24 * time.Hour

// runAgentRecordsRetention drives the daily DeleteOlderThan sweep. Errors are
// logged but never fatal — the retention loop should outlive transient DB
// hiccups.
func runAgentRecordsRetention(ctx context.Context, repo *storage.AgentRecordRepo, log *slog.Logger) {
	// Kick off after 5 minutes so cold-start logs are not buried under the
	// retention cron output.
	initial := time.NewTimer(5 * time.Minute)
	defer initial.Stop()
	select {
	case <-ctx.Done():
		return
	case <-initial.C:
	}
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	sweep := func() {
		sweepCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer cancel()
		n, err := repo.DeleteOlderThan(sweepCtx, time.Now().Add(-agentRecordsRetention))
		if err != nil {
			log.Warn("agent records retention: sweep failed",
				slog.String("err", err.Error()))
			return
		}
		if n > 0 {
			log.Info("agent records retention: sweep complete",
				slog.Int64("deleted_rows", n))
		}
	}
	sweep()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweep()
		}
	}
}
