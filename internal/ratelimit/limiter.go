// Package ratelimit implements a thread-safe token-bucket limiter keyed by an
// arbitrary string (typically the client IP, or the IP + account composite key
// used for login throttling).
//
// Buckets are stored in a fixed-capacity LRU so that an attacker spraying
// unique source IPs cannot exhaust hub memory. When the cache is full the
// least-recently-used bucket is evicted; if it later returns it starts with a
// fresh allowance — the design accepts this minor accuracy loss in exchange
// for an O(1) worst-case footprint.
//
// The underlying bucket comes from golang.org/x/time/rate; we only own the LRU
// and the lookup synchronisation.
package ratelimit

import (
	"container/list"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// DefaultCapacity is the fallback bucket cache size when New receives a
// non-positive capacity argument.
const DefaultCapacity = 10000

// Limiter wraps a token-bucket per key. Use New to construct.
//
// Zero-value Limiter is not usable; callers MUST go through New.
type Limiter struct {
	mu sync.Mutex

	rate     rate.Limit
	burst    int
	capacity int

	// entries maps the key to its LRU element. Each element's value is a
	// *bucketEntry; the front of the list is the most recently used bucket.
	entries map[string]*list.Element
	lru     *list.List
}

// bucketEntry pairs a key with its rate.Limiter so we can recover the key
// during LRU eviction without holding a separate reverse map.
type bucketEntry struct {
	key    string
	bucket *rate.Limiter
}

// New constructs a Limiter with the supplied throughput, burst size and LRU
// capacity. A capacity ≤ 0 falls back to DefaultCapacity.
//
//	r = sustained allowed events per second (e.g. 100 for 100 req/s)
//	b = burst size  (number of tokens the bucket can hold at once)
//	c = max distinct keys cached at once
func New(r float64, b int, c int) *Limiter {
	if c <= 0 {
		c = DefaultCapacity
	}
	if b < 1 {
		b = 1
	}
	return &Limiter{
		rate:     rate.Limit(r),
		burst:    b,
		capacity: c,
		entries:  make(map[string]*list.Element, c),
		lru:      list.New(),
	}
}

// Allow checks whether one token is available for key. It returns:
//
//   - allowed: true if a token was consumed (request may proceed).
//   - retryAfter: the recommended duration to wait before retrying when
//     allowed is false. Always 0 when allowed is true.
//
// The method is safe for concurrent use.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	if l == nil {
		return true, 0
	}
	return l.AllowAt(key, time.Now())
}

// AllowAt is identical to Allow but uses the supplied wall-clock time. It
// exists so tests can drive the bucket deterministically.
func (l *Limiter) AllowAt(key string, now time.Time) (bool, time.Duration) {
	if l == nil {
		return true, 0
	}
	l.mu.Lock()
	bucket := l.getOrCreateLocked(key)
	l.mu.Unlock()

	reservation := bucket.ReserveN(now, 1)
	if !reservation.OK() {
		// Burst smaller than 1; should be unreachable given the burst≥1 floor
		// in New, but keep the guard so callers fail closed.
		return false, time.Second
	}
	delay := reservation.DelayFrom(now)
	if delay == 0 {
		return true, 0
	}
	// Cancel the reservation so the consumed token is returned to the bucket;
	// otherwise repeated denials would still drain capacity.
	reservation.CancelAt(now)
	return false, delay
}

// Len returns the current number of cached buckets. Intended for tests.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lru.Len()
}

// getOrCreateLocked retrieves the bucket for key, creating it (and evicting
// the LRU tail if necessary) when absent. Must be called with l.mu held.
func (l *Limiter) getOrCreateLocked(key string) *rate.Limiter {
	if elem, ok := l.entries[key]; ok {
		l.lru.MoveToFront(elem)
		return elem.Value.(*bucketEntry).bucket
	}
	bucket := rate.NewLimiter(l.rate, l.burst)
	elem := l.lru.PushFront(&bucketEntry{key: key, bucket: bucket})
	l.entries[key] = elem
	l.evictIfFullLocked()
	return bucket
}

// evictIfFullLocked removes the oldest entry while the cache exceeds capacity.
// Must be called with l.mu held.
func (l *Limiter) evictIfFullLocked() {
	for l.lru.Len() > l.capacity {
		tail := l.lru.Back()
		if tail == nil {
			return
		}
		entry := tail.Value.(*bucketEntry)
		l.lru.Remove(tail)
		delete(l.entries, entry.key)
	}
}
