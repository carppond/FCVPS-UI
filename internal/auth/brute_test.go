package auth

import (
	"testing"
	"time"
)

// fakeClock is a controllable monotonic clock used by the brute tests.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time   { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

// newTestProtector returns a BruteProtector configured for fast, deterministic
// tests: threshold=5, window=1 min, ban=10 min.
func newTestProtector(clk *fakeClock) *BruteProtector {
	return NewBruteProtector(BruteConfig{
		Threshold:   5,
		Window:      time.Minute,
		BanDuration: 10 * time.Minute,
		Now:         clk.Now,
	})
}

func TestBruteProtectorBansAfterThreshold(t *testing.T) {
	clk := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	b := newTestProtector(clk)
	const ip, user = "1.2.3.4", "alice"

	for i := 0; i < 4; i++ {
		b.RecordFailure(ip, user)
		if blocked, _ := b.IsBlocked(ip, user); blocked {
			t.Fatalf("blocked too early after %d failures", i+1)
		}
	}
	b.RecordFailure(ip, user)
	if blocked, until := b.IsBlocked(ip, user); !blocked {
		t.Fatalf("expected ban after %d failures", b.cfg.Threshold)
	} else if until.IsZero() {
		t.Fatalf("expected non-zero until for blocked key")
	}
}

func TestBruteProtectorBanExpires(t *testing.T) {
	clk := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	b := newTestProtector(clk)
	const ip, user = "1.2.3.4", "alice"
	for i := 0; i < 5; i++ {
		b.RecordFailure(ip, user)
	}
	if blocked, _ := b.IsBlocked(ip, user); !blocked {
		t.Fatalf("precondition: should be banned")
	}
	clk.Advance(b.cfg.BanDuration + time.Second)
	if blocked, _ := b.IsBlocked(ip, user); blocked {
		t.Fatalf("ban should have expired")
	}
}

func TestBruteProtectorIndependentIPAndUser(t *testing.T) {
	clk := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	b := newTestProtector(clk)
	for i := 0; i < 5; i++ {
		// Ban the IP only (no username).
		b.RecordFailure("9.9.9.9", "")
	}
	if blocked, _ := b.IsBlocked("9.9.9.9", "irrelevant"); !blocked {
		t.Fatalf("IP should be banned even without username axis")
	}
	if blocked, _ := b.IsBlocked("8.8.8.8", "bob"); blocked {
		t.Fatalf("different IP+user must not inherit the ban")
	}
}

func TestBruteProtectorAccountBanCrossIP(t *testing.T) {
	clk := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	b := newTestProtector(clk)
	// Distribute attacks across many IPs but always the same target user.
	for i := 0; i < 5; i++ {
		b.RecordFailure("10.0.0."+string(rune('0'+i)), "alice")
	}
	if blocked, _ := b.IsBlocked("10.0.0.99", "alice"); !blocked {
		t.Fatalf("account axis ban should fire regardless of IP")
	}
}

func TestBruteProtectorWindowExpires(t *testing.T) {
	clk := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	b := newTestProtector(clk)
	for i := 0; i < 4; i++ {
		b.RecordFailure("1.2.3.4", "alice")
	}
	// Advance past the rolling window — counters should reset.
	clk.Advance(b.cfg.Window + time.Second)
	b.RecordFailure("1.2.3.4", "alice")
	if blocked, _ := b.IsBlocked("1.2.3.4", "alice"); blocked {
		t.Fatalf("window expired counters should not trigger ban")
	}
}

func TestBruteProtectorRecordSuccessClears(t *testing.T) {
	clk := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	b := newTestProtector(clk)
	for i := 0; i < 4; i++ {
		b.RecordFailure("1.2.3.4", "alice")
	}
	b.RecordSuccess("1.2.3.4", "alice")
	b.RecordFailure("1.2.3.4", "alice")
	if blocked, _ := b.IsBlocked("1.2.3.4", "alice"); blocked {
		t.Fatalf("RecordSuccess should have cleared the counter")
	}
}

func TestBruteProtectorOnBanCallback(t *testing.T) {
	clk := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	fired := 0
	b := NewBruteProtector(BruteConfig{
		Threshold:   3,
		Window:      time.Minute,
		BanDuration: 10 * time.Minute,
		Now:         clk.Now,
		OnBan: func(ip, username string, until time.Time) {
			fired++
		},
	})
	for i := 0; i < 3; i++ {
		b.RecordFailure("1.2.3.4", "alice")
	}
	if fired != 1 {
		t.Fatalf("OnBan should fire exactly once; got %d", fired)
	}
}

func TestBruteProtectorNilSafe(t *testing.T) {
	var b *BruteProtector
	// All methods must tolerate a nil receiver.
	b.RecordFailure("1.2.3.4", "alice")
	b.RecordSuccess("1.2.3.4", "alice")
	if blocked, _ := b.IsBlocked("1.2.3.4", "alice"); blocked {
		t.Fatalf("nil protector should report not-blocked")
	}
	b.Start()
	b.Stop()
}
