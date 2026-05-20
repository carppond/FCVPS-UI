// Package util collects pure-function helpers reused across the hub. No
// sub-package may import handler/storage; util is leaf-only.
package util

import (
	"sync"
	"time"
)

// Clock abstracts the system clock for testability. Production code should
// receive a Clock via constructor injection rather than calling time.Now.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// Since returns the elapsed duration since t.
	Since(t time.Time) time.Duration
	// NowUnixMs returns the current time as Unix epoch milliseconds.
	NowUnixMs() int64
}

// RealClock is the production Clock backed by package time.
type RealClock struct{}

// Now returns time.Now().
func (RealClock) Now() time.Time { return time.Now() }

// Since returns time.Since(t).
func (RealClock) Since(t time.Time) time.Duration { return time.Since(t) }

// NowUnixMs returns the current Unix epoch in milliseconds.
func (RealClock) NowUnixMs() int64 { return time.Now().UnixMilli() }

// FixedClock is a deterministic Clock implementation for unit tests.
// Advance moves the internal cursor forward.
type FixedClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewFixedClock returns a FixedClock anchored at t.
func NewFixedClock(t time.Time) *FixedClock {
	return &FixedClock{now: t}
}

// Now returns the current fixed time.
func (c *FixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Since returns the duration between the fixed time and t.
func (c *FixedClock) Since(t time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now.Sub(t)
}

// NowUnixMs returns the fixed clock as Unix epoch milliseconds.
func (c *FixedClock) NowUnixMs() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now.UnixMilli()
}

// Advance moves the internal cursor forward by d.
func (c *FixedClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// defaultClock is the process-wide Clock used by Now / NowUnixMs.
var defaultClock Clock = RealClock{}

// Now returns the current time via the process-wide Clock.
func Now() time.Time { return defaultClock.Now() }

// NowUnixMs returns the current Unix epoch in milliseconds via the
// process-wide Clock.
func NowUnixMs() int64 { return defaultClock.NowUnixMs() }

// SetClock replaces the process-wide Clock. Intended for tests only. Call
// `defer util.SetClock(util.RealClock{})` to restore the default.
func SetClock(c Clock) {
	if c == nil {
		defaultClock = RealClock{}
		return
	}
	defaultClock = c
}
