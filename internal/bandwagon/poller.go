package bandwagon

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"shiguang-vps/internal/storage"
)

// Poller periodically refreshes the cached BandwagonHost figures for every
// agent that has credentials configured. The traffic summary reads the cache
// (bwg_used / bwg_limit / bwg_synced_at) and prefers it over measured usage.
type Poller struct {
	repo   *storage.AgentRepo
	client *http.Client
	logger *slog.Logger
	now    func() time.Time
}

// NewPoller wires the poller. client should be timeout-bounded (e.g. a
// safehttp client); nil falls back to http.DefaultClient.
func NewPoller(repo *storage.AgentRepo, client *http.Client, logger *slog.Logger, now func() time.Time) *Poller {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	return &Poller{repo: repo, client: client, logger: logger, now: now}
}

// RunOnce refreshes the cache for all credentialed agents. Per-agent failures
// are logged and skipped so one bad key does not stall the rest.
func (p *Poller) RunOnce(ctx context.Context) {
	if p.repo == nil {
		return
	}
	agents, err := p.repo.ListBwgAgents(ctx)
	if err != nil {
		p.logger.Warn("bandwagon: list agents", slog.String("err", err.Error()))
		return
	}
	for _, a := range agents {
		info, err := FetchServiceInfo(ctx, p.client, a.Veid, a.APIKey)
		if err != nil {
			p.logger.Warn("bandwagon: fetch",
				slog.String("agent_id", a.ID), slog.String("err", err.Error()))
			continue
		}
		if err := p.repo.UpdateBwgCache(ctx, a.ID,
			info.DataCounter, info.PlanMonthlyData, info.DataNextReset, p.now().UnixMilli()); err != nil {
			p.logger.Warn("bandwagon: update cache",
				slog.String("agent_id", a.ID), slog.String("err", err.Error()))
		}
	}
}

// StartPeriodic runs RunOnce shortly after boot and then every interval. It
// returns a cancel func that stops the loop (mirrors traffic.*.StartDaily).
func (p *Poller) StartPeriodic(ctx context.Context, interval time.Duration) func() {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	subCtx, cancel := context.WithCancel(ctx)
	go func() {
		timer := time.NewTimer(time.Minute) // first sweep ~1 min after boot
		defer timer.Stop()
		for {
			select {
			case <-subCtx.Done():
				return
			case <-timer.C:
				p.RunOnce(subCtx)
				timer.Reset(interval)
			}
		}
	}()
	return cancel
}
