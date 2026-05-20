package util_test

import (
	"regexp"
	"testing"

	"shiguang-vps/internal/util"
)

var uuidV7Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUUIDv7Format(t *testing.T) {
	id := util.UUIDv7()
	if len(id) != util.UUIDv7Size {
		t.Fatalf("UUIDv7 length = %d, want %d", len(id), util.UUIDv7Size)
	}
	if !uuidV7Pattern.MatchString(id) {
		t.Fatalf("UUIDv7 %q does not match v7 pattern", id)
	}
}

func TestUUIDv7Monotonic(t *testing.T) {
	const n = 5000
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = util.UUIDv7()
	}
	// Strict less-than on string compare matches the RFC ordering, because
	// the timestamp prefix dominates and the per-ms seq is appended big-endian.
	for i := 1; i < n; i++ {
		if ids[i] <= ids[i-1] {
			t.Fatalf("monotonicity violated: ids[%d]=%q <= ids[%d]=%q", i, ids[i], i-1, ids[i-1])
		}
	}
}

func TestUUIDv7Uniqueness(t *testing.T) {
	const n = 10_000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := util.UUIDv7()
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate UUIDv7 at iteration %d: %s", i, id)
		}
		seen[id] = struct{}{}
	}
}

func TestRandomHex32(t *testing.T) {
	s := util.RandomHex32()
	if len(s) != 32 {
		t.Fatalf("RandomHex32 length = %d, want 32", len(s))
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			t.Fatalf("RandomHex32 contains invalid char %q", c)
		}
	}
}

func TestRandomHex32DistinctAcrossCalls(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 100; i++ {
		s := util.RandomHex32()
		if _, ok := seen[s]; ok {
			t.Fatalf("duplicate RandomHex32 within 100 calls: %s", s)
		}
		seen[s] = struct{}{}
	}
}

func TestRandomBytesLength(t *testing.T) {
	for _, n := range []int{0, 1, 16, 64} {
		got := util.RandomBytes(n)
		if len(got) != n {
			t.Fatalf("RandomBytes(%d) len = %d", n, len(got))
		}
	}
	if got := util.RandomBytes(-1); len(got) != 0 {
		t.Fatalf("RandomBytes(-1) should return empty slice, got len=%d", len(got))
	}
}
