package collector

import (
	"context"
	"testing"
	"time"
)

// TestAggregatorCollect verifies the aggregator returns a payload with
// non-nil fields stamped with the configured AgentID. We don't assert exact
// values — they are environment-dependent — but every numeric field should
// be ≥ 0 and totals should be ≥ used.
func TestAggregatorCollect(t *testing.T) {
	a := NewAggregator(Config{
		AgentID:           "test-agent",
		NetSampleInterval: 200 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, err := a.Collect(ctx)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if p.AgentID != "test-agent" {
		t.Errorf("AgentID = %q, want test-agent", p.AgentID)
	}
	if p.CPUPercent < 0 || p.CPUPercent > 100 {
		t.Errorf("CPUPercent = %.2f, want [0, 100]", p.CPUPercent)
	}
	if p.MemTotal <= 0 {
		t.Errorf("MemTotal should be > 0, got %d", p.MemTotal)
	}
	if p.MemUsed < 0 || p.MemUsed > p.MemTotal {
		t.Errorf("MemUsed=%d out of [0, %d]", p.MemUsed, p.MemTotal)
	}
	if p.DiskTotal <= 0 {
		t.Errorf("DiskTotal should be > 0, got %d", p.DiskTotal)
	}
	if p.Uptime < 0 {
		t.Errorf("Uptime should be ≥ 0, got %d", p.Uptime)
	}
}

// TestAggregatorDefaults verifies the zero-config aggregator picks up
// sensible defaults.
func TestAggregatorDefaults(t *testing.T) {
	a := NewAggregator(Config{AgentID: "x"})
	if a.cfg.NetSampleInterval != DefaultNetSampleInterval {
		t.Errorf("NetSampleInterval = %v, want %v", a.cfg.NetSampleInterval, DefaultNetSampleInterval)
	}
	if a.cfg.PerCollectorTimeout <= 0 {
		t.Errorf("PerCollectorTimeout must be positive, got %v", a.cfg.PerCollectorTimeout)
	}
}

// TestDeltaPerSec checks the network-rate helper handles counter resets +
// regular increments.
func TestDeltaPerSec(t *testing.T) {
	cases := []struct {
		name    string
		first   uint64
		second  uint64
		seconds uint64
		want    uint64
	}{
		{"normal", 0, 1000, 1, 1000},
		{"reset", 1000, 500, 1, 0}, // counter rolled over
		{"steady", 5000, 5000, 1, 0},
		{"interval2s", 0, 4000, 2, 2000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deltaPerSec(tc.first, tc.second, tc.seconds)
			if got != tc.want {
				t.Errorf("deltaPerSec(%d, %d, %d) = %d, want %d",
					tc.first, tc.second, tc.seconds, got, tc.want)
			}
		})
	}
}
