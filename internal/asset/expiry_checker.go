// Package asset implements VPS asset management background workers.
package asset

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/storage"
)

// ExpiryCheckerConfig wires the checker to its collaborators.
type ExpiryCheckerConfig struct {
	VpsRepo  *storage.VpsAssetRepo
	Notify   *notify.Manager
	Logger   *slog.Logger
	Now      func() time.Time
}

// ExpiryChecker is a background worker that checks VPS assets nearing expiry
// and emits vps_expiry notifications. It runs at midnight UTC daily and checks
// three thresholds: 7 days, 3 days, and 0 days (the day of expiry).
type ExpiryChecker struct {
	cfg    ExpiryCheckerConfig
	logger *slog.Logger
	now    func() time.Time

	// mu protects lastNotified for debounce.
	mu           sync.Mutex
	lastNotified map[string]time.Time // key: "vpsID:threshold" → last notify time
}

// NewExpiryChecker returns a new checker.
func NewExpiryChecker(cfg ExpiryCheckerConfig) (*ExpiryChecker, error) {
	if cfg.VpsRepo == nil || cfg.Notify == nil {
		return nil, fmt.Errorf("expiry checker: repos + notify required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &ExpiryChecker{
		cfg:          cfg,
		logger:       cfg.Logger,
		now:          cfg.Now,
		lastNotified: make(map[string]time.Time),
	}, nil
}

// thresholds defines the notification tiers in days.
var thresholds = []int{7, 3, 0}

// Run performs a single expiry check cycle.
func (c *ExpiryChecker) Run(ctx context.Context) {
	// Fetch all VPS expiring within 7 days (covers all thresholds).
	assets, err := c.cfg.VpsRepo.ListAllExpiring(ctx, 7)
	if err != nil {
		c.logger.Error("expiry checker: list expiring", slog.String("err", err.Error()))
		return
	}
	if len(assets) == 0 {
		return
	}

	for i := range assets {
		a := &assets[i]
		days := a.DaysUntilExpiry
		for _, t := range thresholds {
			if days <= t && !c.recentlyNotified(a.ID, t) {
				subject := fmt.Sprintf(`VPS "%s" (%s) 将在 %d 天后到期（%s）`, a.Name, a.Provider, days, a.ExpireAt)
				if days <= 0 {
					subject = fmt.Sprintf(`VPS "%s" (%s) 已到期（%s）`, a.Name, a.Provider, a.ExpireAt)
				}
				_, err := c.cfg.Notify.Emit(ctx, notify.Event{
					Type:       notify.EventVpsExpiry,
					UserID:     a.UserID,
					ResourceID: a.ID,
					Subject:    subject,
					Payload: notify.VpsExpiryPayload{
						VpsID:    a.ID,
						VpsName:  a.Name,
						Provider: a.Provider,
						Days:     days,
						ExpireAt: a.ExpireAt,
					},
				})
				if err != nil {
					c.logger.Warn("expiry checker: emit failed",
						slog.String("vps_id", a.ID),
						slog.Int("threshold", t),
						slog.String("err", err.Error()))
				} else {
					c.markNotified(a.ID, t)
				}
				// Only send for the most aggressive threshold that applies.
				break
			}
		}
	}
	c.cleanupOldEntries()
}

// StartDaily runs the checker at midnight UTC daily. Returns a stop function.
func (c *ExpiryChecker) StartDaily(ctx context.Context) func() {
	innerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Run once at startup after a short delay.
		select {
		case <-innerCtx.Done():
			return
		case <-time.After(30 * time.Second):
		}
		c.Run(innerCtx)

		for {
			next := nextMidnightUTC(c.now())
			timer := time.NewTimer(time.Until(next))
			select {
			case <-innerCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
				c.Run(innerCtx)
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

// recentlyNotified checks debounce: same VPS + same threshold within 24h.
func (c *ExpiryChecker) recentlyNotified(vpsID string, threshold int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := fmt.Sprintf("%s:%d", vpsID, threshold)
	t, ok := c.lastNotified[key]
	if !ok {
		return false
	}
	return c.now().Sub(t) < 24*time.Hour
}

// markNotified records the notify timestamp.
func (c *ExpiryChecker) markNotified(vpsID string, threshold int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := fmt.Sprintf("%s:%d", vpsID, threshold)
	c.lastNotified[key] = c.now()
}

// cleanupOldEntries purges entries older than 48 hours from the debounce map
// to prevent unbounded memory growth.
func (c *ExpiryChecker) cleanupOldEntries() {
	c.mu.Lock()
	defer c.mu.Unlock()
	cutoff := c.now().Add(-48 * time.Hour)
	for k, t := range c.lastNotified {
		if t.Before(cutoff) {
			delete(c.lastNotified, k)
		}
	}
}

// nextMidnightUTC returns the next 00:00 UTC after t.
func nextMidnightUTC(t time.Time) time.Time {
	u := t.UTC()
	next := time.Date(u.Year(), u.Month(), u.Day()+1, 0, 0, 0, 0, time.UTC)
	return next
}
