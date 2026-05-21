package notify

import (
	"testing"
	"time"
)

func TestDedupeKey_Deterministic(t *testing.T) {
	t.Parallel()
	a := DedupeKey("node_offline", "node-123")
	b := DedupeKey("node_offline", "node-123")
	if a != b {
		t.Fatalf("expected stable key, got %q vs %q", a, b)
	}
	if a == DedupeKey("node_offline", "node-456") {
		t.Fatalf("expected distinct keys for different resource ids")
	}
	if a == DedupeKey("traffic_threshold", "node-123") {
		t.Fatalf("expected distinct keys for different event types")
	}
}

func TestDedupe_ShouldEmit_WithinWindow_Blocks(t *testing.T) {
	t.Parallel()
	clock := newClock(time.Unix(1700000000, 0))
	d := NewDedupe(5*time.Minute, clock.Now)
	key := DedupeKey("node_offline", "n1")
	if !d.ShouldEmit(key) {
		t.Fatalf("first emit should succeed")
	}
	// Same-key emit at +30s must be suppressed.
	clock.Advance(30 * time.Second)
	if d.ShouldEmit(key) {
		t.Fatalf("second emit at +30s should be deduped")
	}
	// +4m later still in window.
	clock.Advance(4 * time.Minute)
	if d.ShouldEmit(key) {
		t.Fatalf("emit at +4m30s should still be deduped")
	}
}

func TestDedupe_ShouldEmit_AfterWindow_Allows(t *testing.T) {
	t.Parallel()
	clock := newClock(time.Unix(1700000000, 0))
	d := NewDedupe(5*time.Minute, clock.Now)
	key := DedupeKey("node_offline", "n1")
	if !d.ShouldEmit(key) {
		t.Fatalf("first emit should succeed")
	}
	clock.Advance(5*time.Minute + time.Second)
	if !d.ShouldEmit(key) {
		t.Fatalf("emit after window should be allowed")
	}
}

func TestDedupe_DifferentKeys_Independent(t *testing.T) {
	t.Parallel()
	d := NewDedupe(5*time.Minute, nil)
	k1 := DedupeKey("node_offline", "n1")
	k2 := DedupeKey("node_offline", "n2")
	if !d.ShouldEmit(k1) || !d.ShouldEmit(k2) {
		t.Fatalf("distinct keys must not interfere with each other")
	}
	if d.ShouldEmit(k1) || d.ShouldEmit(k2) {
		t.Fatalf("re-emit immediately should be deduped")
	}
}

func TestDedupe_EmptyKey_AlwaysAllows(t *testing.T) {
	t.Parallel()
	d := NewDedupe(5*time.Minute, nil)
	for i := 0; i < 5; i++ {
		if !d.ShouldEmit("") {
			t.Fatalf("empty key must always be allowed (iter %d)", i)
		}
	}
}

func TestDedupe_Forget(t *testing.T) {
	t.Parallel()
	d := NewDedupe(5*time.Minute, nil)
	key := DedupeKey("login_anomaly", "u1")
	if !d.ShouldEmit(key) {
		t.Fatalf("first emit")
	}
	if d.ShouldEmit(key) {
		t.Fatalf("immediate re-emit should be deduped")
	}
	d.Forget(key)
	if !d.ShouldEmit(key) {
		t.Fatalf("after Forget, emit should be allowed again")
	}
}

// clock is a manual clock helper used across tests in this package.
type fakeClock struct {
	now time.Time
}

func newClock(t time.Time) *fakeClock { return &fakeClock{now: t} }
func (c *fakeClock) Now() time.Time   { return c.now }
func (c *fakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}
