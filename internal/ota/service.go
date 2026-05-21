package ota

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/storage"
)

// ServiceConfig wires the high-level orchestrator. Everything except DB is
// optional; collaborators default to package-level constructors so a minimal
// caller (a test or the dev hub) can use NewService(ServiceConfig{DB: db}).
type ServiceConfig struct {
	DB             *storage.DB
	BinaryPath     string
	CurrentVersion string
	GitHubRepo     string
	HTTPClient     *http.Client
	APIBase        string
	Logger         *slog.Logger
	Now            func() time.Time
	// NotifyManager fires the ota_available email/telegram/etc. event when
	// the daily checker spots a newer release. nil disables notifications
	// (the SSE bus broadcast still works).
	NotifyManager *notify.Manager
	// EventBus broadcasts SSE `ota_progress` / `system` events. nil disables
	// SSE without breaking the rest of the pipeline.
	EventBus *notify.EventBus
	// AdminUserID is the recipient for ota_available emails. Empty disables
	// email — the broadcast still hits every connected admin's SSE bus.
	AdminUserID string
	// Shutdown is invoked by the applier once the swap succeeds. Defaults to
	// sending SIGTERM to the current process, matching docs §2.6.
	Shutdown func()
	// BinaryName is the asset prefix used when matching `<name>-<os>-<arch>`
	// release assets. Defaults to filepath.Base(BinaryPath).
	BinaryName string
}

// Service is the public façade the handler / main wire up. It owns the
// checker + downloader + applier so the rest of the codebase deals with one
// surface area. Concurrent Apply calls are serialised via the inflight flag.
type Service struct {
	cfg         ServiceConfig
	checker     *Checker
	downloader  *Downloader
	applier     *Applier
	logger      *slog.Logger
	now         func() time.Time
	notifyMgr   *notify.Manager
	eventBus    *notify.EventBus
	adminUserID string

	mu        sync.Mutex
	lastInfo  *ReleaseInfo
	lastError string
	lastCheck time.Time

	// inflight ensures only one Apply runs at a time. Atomic so the HTTP
	// handler can fail fast without taking mu.
	inflight atomic.Bool

	history []HistoryEntry
}

// HistoryEntry captures the result of a single Apply (or attempt). Kept in
// memory only — v1 does not persist OTA history (deferred to T-32).
type HistoryEntry struct {
	Version   string    `json:"version"`
	Status    string    `json:"status"` // "success" | "failed"
	AppliedAt time.Time `json:"applied_at"`
	Error     string    `json:"error,omitempty"`
}

// NewService builds the orchestrator. Returns an error only when the checker /
// applier sub-builders reject their config (bad repo, unresolvable binary).
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("ota: service: db required")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	checker, err := NewChecker(CheckerConfig{
		HTTPClient:     cfg.HTTPClient,
		GitHubRepo:     cfg.GitHubRepo,
		APIBase:        cfg.APIBase,
		CurrentVersion: cfg.CurrentVersion,
		Now:            now,
	})
	if err != nil {
		return nil, err
	}
	downloader := NewDownloader(DownloaderConfig{HTTPClient: cfg.HTTPClient})
	shutdown := cfg.Shutdown
	if shutdown == nil {
		shutdown = defaultShutdown
	}
	applier, err := NewApplier(ApplierConfig{
		DB:         cfg.DB,
		BinaryPath: cfg.BinaryPath,
		Shutdown:   shutdown,
		Logger:     cfg.Logger,
	})
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:         cfg,
		checker:     checker,
		downloader:  downloader,
		applier:     applier,
		logger:      cfg.Logger,
		now:         now,
		notifyMgr:   cfg.NotifyManager,
		eventBus:    cfg.EventBus,
		adminUserID: cfg.AdminUserID,
	}, nil
}

// defaultShutdown sends SIGTERM to the current process so the SIGTERM handler
// in cmd/server/main.go's graceful-shutdown path runs (docs §2.6). Wrapped
// behind a func so tests can substitute a no-op.
func defaultShutdown() {
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		return
	}
	// os.Interrupt unblocks the signal handler regardless of platform.
	_ = proc.Signal(os.Interrupt)
}

// CurrentVersion returns the version string the running binary advertises.
func (s *Service) CurrentVersion() string {
	if s == nil {
		return ""
	}
	return s.cfg.CurrentVersion
}

// CheckNow forces an immediate poll, refreshing the cached release info. The
// admin "check for updates" button calls this; the daily watcher also calls
// it via StartChecker.
func (s *Service) CheckNow(ctx context.Context) (*ReleaseInfo, error) {
	if s == nil {
		return nil, fmt.Errorf("ota: nil service")
	}
	info, err := s.checker.CheckLatest(ctx)
	s.mu.Lock()
	s.lastCheck = s.now()
	if err != nil {
		s.lastError = err.Error()
		s.mu.Unlock()
		return nil, err
	}
	s.lastInfo = info
	s.lastError = ""
	s.mu.Unlock()
	return info, nil
}

// LastInfo returns the cached release info (or nil) plus the wall-clock time
// of the last completed check.
func (s *Service) LastInfo() (*ReleaseInfo, time.Time, string) {
	if s == nil {
		return nil, time.Time{}, ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastInfo, s.lastCheck, s.lastError
}

// History returns the in-memory OTA attempt history (newest first).
func (s *Service) History() []HistoryEntry {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]HistoryEntry, len(s.history))
	for i, h := range s.history {
		out[len(s.history)-1-i] = h
	}
	return out
}

// StartChecker launches the background daily sweep. Returns a stop func; safe
// to call multiple times. The sweep skips when CurrentVersion is empty (a dev
// build with no version metadata should not pester users to "upgrade").
func (s *Service) StartChecker(ctx context.Context) func() {
	if s == nil {
		return func() {}
	}
	subCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(CheckInterval)
		defer ticker.Stop()
		// First sweep deferred by 5 minutes so cold-start logs are not
		// drowned in OTA noise.
		initial := time.NewTimer(5 * time.Minute)
		defer initial.Stop()
		select {
		case <-subCtx.Done():
			return
		case <-initial.C:
			s.sweepOnce(subCtx)
		}
		for {
			select {
			case <-subCtx.Done():
				return
			case <-ticker.C:
				s.sweepOnce(subCtx)
			}
		}
	}()
	return cancel
}

// sweepOnce performs a single check + notification cycle. Errors are logged
// but never bubble up — the goroutine keeps running.
func (s *Service) sweepOnce(ctx context.Context) {
	info, err := s.CheckNow(ctx)
	if err != nil {
		if errors.Is(err, ErrNoRelease) {
			return
		}
		if s.logger != nil {
			s.logger.Warn("ota: daily check failed", slog.String("err", err.Error()))
		}
		return
	}
	if !info.HasUpdate {
		return
	}
	// Broadcast an SSE system event so every connected admin sees an in-app
	// banner without waiting for the next page refresh.
	if s.eventBus != nil {
		s.eventBus.Broadcast(notify.SSEEvent{
			Kind: "system",
			Payload: map[string]any{
				"kind":           "ota_available",
				"latest_version": info.TagName,
				"release_url":    info.HTMLURL,
				"ts":             s.now().UnixMilli(),
			},
		})
	}
	// Email / telegram / etc. via the standard notify pipeline.
	if s.notifyMgr != nil && s.adminUserID != "" {
		emitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if _, err := s.notifyMgr.Emit(emitCtx, notify.Event{
			Type:       notify.EventOTAAvailable,
			UserID:     s.adminUserID,
			ResourceID: info.TagName,
			Subject:    fmt.Sprintf("[shiguang-vps] new release %s", info.TagName),
			Payload: notify.OTAAvailablePayload{
				LatestVersion:  info.TagName,
				CurrentVersion: s.cfg.CurrentVersion,
				ReleaseURL:     info.HTMLURL,
				PublishedAt:    info.PublishedAt.UnixMilli(),
			},
		}); err != nil && s.logger != nil {
			s.logger.Warn("ota: notify emit failed", slog.String("err", err.Error()))
		}
	}
}

// Apply runs the download → verify → swap pipeline for the currently-cached
// release (or the one supplied by the caller). Progress is broadcast as
// `ota_progress` SSE events; the final outcome is also recorded in History.
//
// Returns ErrApplyInflight if a previous Apply is still running.
func (s *Service) Apply(ctx context.Context, info *ReleaseInfo) error {
	if s == nil {
		return fmt.Errorf("ota: nil service")
	}
	if info == nil {
		info, _, _ = s.LastInfo()
	}
	if info == nil {
		return ErrNoRelease
	}
	if !s.inflight.CompareAndSwap(false, true) {
		return ErrApplyInflight
	}
	defer s.inflight.Store(false)

	binAsset, sha256Asset, ok := s.pickAssets(info)
	if !ok {
		s.recordFailure(info.TagName, "no matching asset for this os/arch")
		s.publishProgress("error", 0, 0, "no matching asset for this os/arch", info.TagName)
		return fmt.Errorf("ota: apply: no asset for %s-%s", runtime.GOOS, runtime.GOARCH)
	}

	// Stage the download next to the running binary so the rename is atomic.
	stagePath, fh, err := s.applier.StageTo()
	if err != nil {
		s.recordFailure(info.TagName, err.Error())
		s.publishProgress("error", 0, binAsset.Size, err.Error(), info.TagName)
		return err
	}

	s.publishProgress("downloading", 0, binAsset.Size, "", info.TagName)
	_, err = s.downloader.Download(ctx, binAsset.BrowserDownloadURL, fh, func(d, total int64) {
		s.publishProgress("downloading", d, total, "", info.TagName)
	})
	closeErr := fh.Close()
	if err != nil {
		_ = os.Remove(stagePath)
		s.recordFailure(info.TagName, err.Error())
		s.publishProgress("error", 0, binAsset.Size, err.Error(), info.TagName)
		return err
	}
	if closeErr != nil {
		_ = os.Remove(stagePath)
		s.recordFailure(info.TagName, closeErr.Error())
		s.publishProgress("error", 0, binAsset.Size, closeErr.Error(), info.TagName)
		return closeErr
	}

	s.publishProgress("verifying", binAsset.Size, binAsset.Size, "", info.TagName)
	hash, err := s.downloader.FetchSHA256(ctx, sha256Asset.BrowserDownloadURL)
	if err != nil {
		_ = os.Remove(stagePath)
		s.recordFailure(info.TagName, err.Error())
		s.publishProgress("error", binAsset.Size, binAsset.Size, err.Error(), info.TagName)
		return err
	}
	if err := VerifySHA256(stagePath, hash); err != nil {
		_ = os.Remove(stagePath)
		s.recordFailure(info.TagName, err.Error())
		s.publishProgress("error", binAsset.Size, binAsset.Size, err.Error(), info.TagName)
		return err
	}

	s.publishProgress("restarting", binAsset.Size, binAsset.Size, "", info.TagName)
	if err := s.applier.Apply(ctx, stagePath); err != nil {
		s.recordFailure(info.TagName, err.Error())
		s.publishProgress("error", binAsset.Size, binAsset.Size, err.Error(), info.TagName)
		return err
	}

	s.recordSuccess(info.TagName)
	s.publishProgress("done", binAsset.Size, binAsset.Size, "", info.TagName)
	return nil
}

// pickAssets returns the (binary, sha256) pair that match the current
// runtime.GOOS / runtime.GOARCH. The binary name is `<BinaryName>-<os>-<arch>`
// and the sidecar is the same name suffixed `.sha256`.
func (s *Service) pickAssets(info *ReleaseInfo) (ReleaseAsset, ReleaseAsset, bool) {
	if info == nil {
		return ReleaseAsset{}, ReleaseAsset{}, false
	}
	name := s.cfg.BinaryName
	if name == "" {
		name = "shiguang-vps"
	}
	binAsset, ok := info.PickAsset(name, runtime.GOOS, runtime.GOARCH, "")
	if !ok {
		return ReleaseAsset{}, ReleaseAsset{}, false
	}
	sha256Asset, ok := info.PickAsset(name, runtime.GOOS, runtime.GOARCH, ".sha256")
	if !ok {
		return ReleaseAsset{}, ReleaseAsset{}, false
	}
	return binAsset, sha256Asset, true
}

// publishProgress emits an `ota_progress` SSE event to every subscriber. The
// shape mirrors the contract in docs/04-api-contract.md §1.5.
func (s *Service) publishProgress(stage string, downloaded, total int64, msg, version string) {
	if s == nil || s.eventBus == nil {
		return
	}
	s.eventBus.Broadcast(notify.SSEEvent{
		Kind: "ota_progress",
		Payload: map[string]any{
			"stage":      stage,
			"downloaded": downloaded,
			"total":      total,
			"version":    version,
			"error":      msg,
			"ts":         s.now().UnixMilli(),
		},
	})
}

// recordSuccess prepends a success entry to the in-memory history (capped at
// 50 entries to keep the slice bounded).
func (s *Service) recordSuccess(version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = appendHistoryEntry(s.history, HistoryEntry{
		Version:   version,
		Status:    "success",
		AppliedAt: s.now(),
	})
}

func (s *Service) recordFailure(version, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = appendHistoryEntry(s.history, HistoryEntry{
		Version:   version,
		Status:    "failed",
		AppliedAt: s.now(),
		Error:     msg,
	})
}

// historyCap bounds the in-memory history slice. History is best-effort; v1
// does not persist it across restarts.
const historyCap = 50

func appendHistoryEntry(history []HistoryEntry, e HistoryEntry) []HistoryEntry {
	history = append(history, e)
	if len(history) > historyCap {
		history = history[len(history)-historyCap:]
	}
	return history
}

// ErrApplyInflight is returned when a second Apply call arrives while one is
// still running. Handlers map this onto HTTP 409.
var ErrApplyInflight = errors.New("ota: apply already in progress")
