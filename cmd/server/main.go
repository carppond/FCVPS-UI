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
	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/handler"
	"shiguang-vps/internal/logger"
	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/ota"
	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/substore"
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

	// T-8 + T-11: subscription module + real node persistence. The
	// NodeRepoAdapter bridges substore.NodeRepo / NodeFetcher to the concrete
	// storage.NodeRepo without creating a storage → substore import cycle.
	nodeAdapter := substore.NewNodeRepoAdapter(nodeRepo)
	syncService, err := substore.NewSyncService(substore.SyncServiceConfig{
		Repo:     subscriptions,
		NodeRepo: nodeAdapter,
		Logger:   log,
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

	// T-22: notification subsystem. Builds the channel registry (5 batch-1
	// kinds), the SSE event bus and the manager. The cleanup worker is
	// stopped at shutdown via the returned cancel func.
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

	deps := &handler.Deps{
		DB: db, Logger: log, Now: time.Now,
		Version:               "v0.0.0-dev",
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
		LoginRateLimit:        ratelimit.New(loginRatePerSecond, loginRateBurst, 0),
		GlobalRateLimit:       ratelimit.New(100, 200, 0),
	}
	mux := handler.NewRouter(deps)
	_ = mux
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps.Start(rootCtx)
	defer deps.Shutdown()
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
	// Ensure the hub broadcasts bye{server_shutdown} during graceful exit.
	defer agentHub.Close()

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           deps.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
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
