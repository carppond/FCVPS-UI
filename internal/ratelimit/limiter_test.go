package ratelimit

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestLimiter_AllowsWithinBurst(t *testing.T) {
	l := New(1, 5, 100) // 1 req/s steady, 5 burst
	now := time.Now()
	for i := 0; i < 5; i++ {
		ok, retry := l.AllowAt("client-a", now)
		if !ok {
			t.Fatalf("burst window: request %d denied unexpectedly (retry=%v)", i, retry)
		}
	}
	ok, retry := l.AllowAt("client-a", now)
	if ok {
		t.Fatalf("6th request inside burst window should be denied")
	}
	if retry <= 0 {
		t.Fatalf("retry-after should be positive when denied; got %v", retry)
	}
}

func TestLimiter_RefillsOverTime(t *testing.T) {
	l := New(2, 1, 100) // 2 req/s, burst 1
	now := time.Now()
	if ok, _ := l.AllowAt("k", now); !ok {
		t.Fatalf("first request should pass")
	}
	if ok, _ := l.AllowAt("k", now); ok {
		t.Fatalf("immediate second request should be denied")
	}
	// Wait 600ms ⇒ at 2 req/s we have 1.2 tokens.
	later := now.Add(600 * time.Millisecond)
	if ok, _ := l.AllowAt("k", later); !ok {
		t.Fatalf("request after refill should pass")
	}
}

func TestLimiter_IsolatesKeys(t *testing.T) {
	l := New(1, 1, 100)
	now := time.Now()
	if ok, _ := l.AllowAt("alice", now); !ok {
		t.Fatalf("alice first request blocked")
	}
	if ok, _ := l.AllowAt("bob", now); !ok {
		t.Fatalf("bob first request blocked — keys not isolated")
	}
	if ok, _ := l.AllowAt("alice", now); ok {
		t.Fatalf("alice second request not throttled")
	}
}

func TestLimiter_LRUEvictsOldestKey(t *testing.T) {
	l := New(1, 1, 3) // capacity 3
	now := time.Now()
	for i := 0; i < 3; i++ {
		l.AllowAt(fmt.Sprintf("k-%d", i), now)
	}
	if l.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", l.Len())
	}
	// Touch k-0 to make it MRU.
	l.AllowAt("k-0", now)
	// Insert a 4th key — should evict k-1 (now LRU).
	l.AllowAt("k-3", now)
	if l.Len() != 3 {
		t.Fatalf("LRU should cap at 3, got %d", l.Len())
	}
	// k-1 was evicted — first hit after eviction is a fresh bucket so it passes
	// even though previously throttled.
	if ok, _ := l.AllowAt("k-1", now); !ok {
		t.Fatalf("evicted key should reset to fresh bucket")
	}
}

func TestLimiter_ConcurrentSafe(t *testing.T) {
	// `go test -race` covers the safety property; this test simply exercises
	// the limiter with parallel callers to give the race detector something to
	// chew on.
	l := New(1000, 1000, 1000)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				key := strconv.Itoa((id + j) % 32)
				l.Allow(key)
			}
		}(i)
	}
	wg.Wait()
	if l.Len() == 0 {
		t.Fatalf("limiter should have cached buckets after concurrent use")
	}
}

func TestLimiter_NilSafe(t *testing.T) {
	var l *Limiter
	ok, retry := l.Allow("anything")
	if !ok || retry != 0 {
		t.Fatalf("nil limiter must allow everything; got ok=%v retry=%v", ok, retry)
	}
}
