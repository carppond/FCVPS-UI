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

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/handler"
	"shiguang-vps/internal/logger"
	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/storage"
)

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

	deps := &handler.Deps{
		DB: db, Logger: log, Now: time.Now,
		Version:        "v0.0.0-dev",
		AuthManager:    authMgr,
		TokenStore:     tokens,
		BruteProtector: brute,
		TOTPManager:    totpMgr,
		UserRepo:       users,
		SessionRepo:    sessions,
		LoginRateLimit: ratelimit.New(loginRatePerSecond, loginRateBurst, 0),
		GlobalRateLimit: ratelimit.New(100, 200, 0),
	}
	mux := handler.NewRouter(deps)
	_ = mux
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps.Start(rootCtx)
	defer deps.Shutdown()

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
