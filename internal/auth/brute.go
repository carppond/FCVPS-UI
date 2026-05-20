package auth

import (
	"log/slog"
	"sync"
	"time"
)

// Brute-force protection thresholds (PRD M-USER §6.3). The values intentionally
// echo the defaults documented in the tech-lead plan; tests override them via
// BruteConfig.
const (
	// DefaultBruteThreshold is the count of failed attempts within
	// DefaultBruteWindow that triggers a ban.
	DefaultBruteThreshold = 20

	// DefaultBruteWindow is the rolling window over which RecordFailure
	// accumulates strikes.
	DefaultBruteWindow = 10 * time.Minute

	// DefaultBruteBanDuration is how long IsBlocked reports true after the
	// threshold is exceeded.
	DefaultBruteBanDuration = 1 * time.Hour

	// bruteSweepInterval is the cadence at which expired entries are evicted
	// from the in-memory maps so they don't accumulate indefinitely.
	bruteSweepInterval = 5 * time.Minute
)

// BruteConfig configures the BruteProtector. Zero-value fields fall back to
// the project defaults declared above.
type BruteConfig struct {
	// Threshold is the failure count that triggers a ban (default: 20).
	Threshold int
	// Window is the rolling lookback used by Threshold (default: 10 min).
	Window time.Duration
	// BanDuration is the time IsBlocked reports true after a ban (default 1h).
	BanDuration time.Duration
	// Logger receives Warn-level entries when a ban is triggered. Nil disables.
	Logger *slog.Logger
	// Now lets tests inject a deterministic clock. Defaults to time.Now.
	Now func() time.Time
	// OnBan is an optional callback fired whenever a key transitions to the
	// banned state. v1 callers leave it nil; T-22 will wire notify.Manager.
	OnBan func(ip, username string, until time.Time)
}

// BruteProtector tracks failed login attempts keyed independently by IP and
// by username. A ban on EITHER axis blocks subsequent attempts; legitimate
// retries by the real user from a new IP still succeed once the username
// counter expires.
//
// Implementation is intentionally simple (map + mutex) — login throughput is
// orders of magnitude below the rest of the server, so an in-memory structure
// is more than sufficient. Persistence is unnecessary because a hub restart
// is a natural cool-off.
type BruteProtector struct {
	cfg BruteConfig

	mu    sync.Mutex
	ips   map[string]*bruteEntry
	users map[string]*bruteEntry

	stopCh    chan struct{}
	sweepDone chan struct{}
}

// bruteEntry tracks the failure timestamps for a single key. Once banned the
// `bannedUntil` field is non-zero; otherwise it remains zero and the
// timestamps slice is consulted for further increments.
type bruteEntry struct {
	failures    []time.Time
	bannedUntil time.Time
}

// NewBruteProtector returns a ready-to-use protector. Call Start to launch
// the background sweeper; Stop on shutdown to reclaim it.
func NewBruteProtector(cfg BruteConfig) *BruteProtector {
	if cfg.Threshold <= 0 {
		cfg.Threshold = DefaultBruteThreshold
	}
	if cfg.Window <= 0 {
		cfg.Window = DefaultBruteWindow
	}
	if cfg.BanDuration <= 0 {
		cfg.BanDuration = DefaultBruteBanDuration
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &BruteProtector{
		cfg:       cfg,
		ips:       make(map[string]*bruteEntry),
		users:     make(map[string]*bruteEntry),
		stopCh:    make(chan struct{}),
		sweepDone: make(chan struct{}),
	}
}

// Start launches the background sweeper. Safe to call once; subsequent calls
// are no-ops.
func (b *BruteProtector) Start() {
	if b == nil {
		return
	}
	go b.sweepLoop()
}

// Stop halts the background sweeper. Idempotent.
func (b *BruteProtector) Stop() {
	if b == nil {
		return
	}
	select {
	case <-b.stopCh:
		return
	default:
	}
	close(b.stopCh)
	<-b.sweepDone
}

// RecordFailure registers a single failed login attempt. Both keys may be
// non-empty (the common case); empty values are ignored on that axis.
//
// When either counter crosses the configured threshold the key transitions
// to "banned" for BanDuration and OnBan / Logger fire.
func (b *BruteProtector) RecordFailure(ip, username string) {
	if b == nil {
		return
	}
	now := b.cfg.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	ipBanned := b.recordLocked(b.ips, ip, now)
	userBanned := b.recordLocked(b.users, username, now)
	if (ipBanned || userBanned) && b.cfg.Logger != nil {
		b.cfg.Logger.Warn("auth: brute-force ban triggered",
			slog.String("ip", ip),
			slog.String("username", username),
			slog.Time("until", now.Add(b.cfg.BanDuration)))
	}
	if ipBanned && b.cfg.OnBan != nil {
		b.cfg.OnBan(ip, username, now.Add(b.cfg.BanDuration))
	}
}

// RecordSuccess clears the strike count for both axes — a successful login
// is the strongest signal we can use to know the previous failures were
// honest typos.
func (b *BruteProtector) RecordSuccess(ip, username string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if ip != "" {
		delete(b.ips, ip)
	}
	if username != "" {
		delete(b.users, username)
	}
}

// IsBlocked reports whether either axis (ip OR username) is currently in a
// banned state. The returned `until` is the wallclock time at which the ban
// expires; zero when not banned.
func (b *BruteProtector) IsBlocked(ip, username string) (bool, time.Time) {
	if b == nil {
		return false, time.Time{}
	}
	now := b.cfg.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	if blocked, until := b.checkLocked(b.ips, ip, now); blocked {
		return true, until
	}
	if blocked, until := b.checkLocked(b.users, username, now); blocked {
		return true, until
	}
	return false, time.Time{}
}

// recordLocked appends now to the supplied entry and returns true when the
// ban threshold is newly crossed. Caller must hold b.mu.
func (b *BruteProtector) recordLocked(m map[string]*bruteEntry, key string, now time.Time) bool {
	if key == "" {
		return false
	}
	entry, ok := m[key]
	if !ok {
		entry = &bruteEntry{failures: make([]time.Time, 0, 4)}
		m[key] = entry
	}
	if !entry.bannedUntil.IsZero() && now.Before(entry.bannedUntil) {
		// Still banned — keep counter at threshold so subsequent failures
		// don't spam OnBan; ban window is what matters here.
		return false
	}
	if !entry.bannedUntil.IsZero() {
		// Previous ban expired; clear state to start fresh.
		entry.failures = entry.failures[:0]
		entry.bannedUntil = time.Time{}
	}
	cutoff := now.Add(-b.cfg.Window)
	pruned := entry.failures[:0]
	for _, ts := range entry.failures {
		if ts.After(cutoff) {
			pruned = append(pruned, ts)
		}
	}
	pruned = append(pruned, now)
	entry.failures = pruned
	if len(pruned) >= b.cfg.Threshold {
		entry.bannedUntil = now.Add(b.cfg.BanDuration)
		return true
	}
	return false
}

// checkLocked returns the current ban state for key. Caller must hold b.mu.
func (b *BruteProtector) checkLocked(m map[string]*bruteEntry, key string, now time.Time) (bool, time.Time) {
	if key == "" {
		return false, time.Time{}
	}
	entry, ok := m[key]
	if !ok {
		return false, time.Time{}
	}
	if entry.bannedUntil.IsZero() {
		return false, time.Time{}
	}
	if now.Before(entry.bannedUntil) {
		return true, entry.bannedUntil
	}
	return false, time.Time{}
}

// sweepLoop periodically prunes expired counters / bans. Stops when stopCh
// is closed.
func (b *BruteProtector) sweepLoop() {
	defer close(b.sweepDone)
	ticker := time.NewTicker(bruteSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.sweep()
		}
	}
}

// sweep evicts entries whose ban has expired AND whose failure window is empty.
func (b *BruteProtector) sweep() {
	now := b.cfg.Now()
	cutoff := now.Add(-b.cfg.Window)
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, m := range []map[string]*bruteEntry{b.ips, b.users} {
		for k, entry := range m {
			if !entry.bannedUntil.IsZero() && now.Before(entry.bannedUntil) {
				continue
			}
			alive := entry.failures[:0]
			for _, ts := range entry.failures {
				if ts.After(cutoff) {
					alive = append(alive, ts)
				}
			}
			entry.failures = alive
			if len(alive) == 0 && (entry.bannedUntil.IsZero() || !now.Before(entry.bannedUntil)) {
				delete(m, k)
			}
		}
	}
}
