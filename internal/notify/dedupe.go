package notify

import (
	"crypto/sha1"
	"encoding/hex"
	"sync"
	"time"
)

// DefaultDedupeWindow matches architecture §6.6: events sharing the same
// (event_type, resource_id) within 5 minutes collapse into a single notice.
const DefaultDedupeWindow = 5 * time.Minute

// DedupeKey returns the canonical dedupe key for an (event_type, resource_id)
// pair: sha1(eventType + "|" + resourceID), hex-encoded. Used by both the
// in-process Dedupe cache and the on-disk notification_events.dedupe_key
// column so a restart of the hub does not lose the window.
func DedupeKey(eventType, resourceID string) string {
	sum := sha1.Sum([]byte(eventType + "|" + resourceID))
	return hex.EncodeToString(sum[:])
}

// Dedupe is a simple time-window deduper. The implementation is intentionally
// minimal — a map keyed by sha1 with last-emit timestamps. We do NOT use the
// LRU cache from hashicorp/golang-lru because (a) the cardinality is bounded
// (one entry per active alert, expired entries are pruned on access) and
// (b) the LRU eviction policy would discard recent entries under load, which
// would defeat the deduper's purpose.
type Dedupe struct {
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	lastAt  map[string]time.Time
}

// NewDedupe returns a deduper with the given window. When window is zero,
// DefaultDedupeWindow is used. now defaults to time.Now.
func NewDedupe(window time.Duration, now func() time.Time) *Dedupe {
	if window <= 0 {
		window = DefaultDedupeWindow
	}
	if now == nil {
		now = time.Now
	}
	return &Dedupe{
		window: window,
		now:    now,
		lastAt: make(map[string]time.Time),
	}
}

// Window returns the configured dedupe window. Exposed for diagnostic logs
// and tests.
func (d *Dedupe) Window() time.Duration {
	if d == nil {
		return 0
	}
	return d.window
}

// ShouldEmit returns true when no event with the same key has been emitted
// within the window. The call atomically marks the key as just-emitted when
// it returns true; callers should treat the boolean as authoritative.
//
// Calling ShouldEmit with an empty key always returns true (and does not
// mutate state) — callers that opt out of dedupe pass "" as the resource ID.
func (d *Dedupe) ShouldEmit(key string) bool {
	if d == nil || key == "" {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	if last, ok := d.lastAt[key]; ok {
		if now.Sub(last) < d.window {
			return false
		}
	}
	d.lastAt[key] = now
	// Cheap GC: prune entries older than 2× the window so the map does not
	// grow unbounded under churn. Bounded cost: O(n) over a fast-growing map
	// is amortised away because pruning runs at most once per ShouldEmit
	// call, and the map only contains entries that ShouldEmit has accepted.
	if len(d.lastAt) > 256 {
		threshold := now.Add(-2 * d.window)
		for k, v := range d.lastAt {
			if v.Before(threshold) {
				delete(d.lastAt, k)
			}
		}
	}
	return true
}

// Forget removes a key from the deduper, allowing immediate re-emission.
// Used by SendTest to bypass dedupe for the "test" button.
func (d *Dedupe) Forget(key string) {
	if d == nil || key == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.lastAt, key)
}
