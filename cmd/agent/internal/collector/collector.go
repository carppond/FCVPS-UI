package collector

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"shiguang-vps/pkg/agentlib"
)

// DefaultNetSampleInterval is the inter-sample gap used to compute network
// throughput when the caller does not override it. 1 s matches the CPU
// sampler so the two slow-by-design collectors can run in parallel without
// stretching the total Collect() walltime.
const DefaultNetSampleInterval = 1 * time.Second

// Config tunes the aggregator. Zero values fall back to sensible defaults.
type Config struct {
	// AgentID is stamped onto every emitted payload (agentlib expects a non
	// empty agent_id field on metrics). The aggregator does not validate it
	// — the caller's responsibility.
	AgentID string

	// NetSampleInterval controls the gap between two snapshots used to
	// compute net_in_speed / net_out_speed. Defaults to 1 s.
	NetSampleInterval time.Duration

	// PerCollectorTimeout caps individual collector runs (CPU/Net share
	// the whole window because they sleep internally; the others are
	// fast). 0 → 3 s default.
	PerCollectorTimeout time.Duration
}

// Aggregator fans out to every collector and assembles a single MetricsPayload
// suitable for the agent → hub WebSocket frame.
//
// The struct holds no long-lived state (yet) — kept as a struct so future
// caches (e.g. memoised disk lookups) can be added without changing call
// sites.
type Aggregator struct {
	cfg Config
}

// NewAggregator builds an Aggregator with the supplied config applied.
func NewAggregator(cfg Config) *Aggregator {
	if cfg.NetSampleInterval <= 0 {
		cfg.NetSampleInterval = DefaultNetSampleInterval
	}
	if cfg.PerCollectorTimeout <= 0 {
		cfg.PerCollectorTimeout = 3 * time.Second
	}
	return &Aggregator{cfg: cfg}
}

// Collect runs every collector in parallel using errgroup. The first error
// cancels the remaining goroutines and is returned to the caller. The
// transport layer is expected to log + skip the metrics frame on error
// rather than terminate the agent.
//
// Note: parallel sampling means CPU + Net run their sleep windows
// concurrently; the upper bound on Collect's walltime is therefore roughly
// max(cpuSampleInterval, NetSampleInterval) plus a small overhead, NOT the
// sum.
func (a *Aggregator) Collect(ctx context.Context) (*agentlib.MetricsPayload, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var (
		mu sync.Mutex
		// CPUCores is static (logical core count) — set directly, no collector.
		payload = &agentlib.MetricsPayload{
			AgentID:  a.cfg.AgentID,
			CPUCores: int32(runtime.NumCPU()),
		}
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		cctx, cancel := context.WithTimeout(gctx, a.cfg.PerCollectorTimeout)
		defer cancel()
		v, err := CPU(cctx)
		if err != nil {
			return err
		}
		mu.Lock()
		payload.CPUPercent = v
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		used, total, sUsed, sTotal, err := Memory()
		if err != nil {
			return err
		}
		mu.Lock()
		payload.MemUsed = int64(used)
		payload.MemTotal = int64(total)
		payload.SwapUsed = int64(sUsed)
		payload.SwapTotal = int64(sTotal)
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		used, total, err := Disk()
		if err != nil {
			return err
		}
		mu.Lock()
		payload.DiskUsed = int64(used)
		payload.DiskTotal = int64(total)
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		cctx, cancel := context.WithTimeout(gctx, a.cfg.PerCollectorTimeout)
		defer cancel()
		// Cumulative counters
		snap, err := NetSnapshot()
		if err != nil {
			return err
		}
		// Rate (sleeps a.cfg.NetSampleInterval)
		in, out, err := NetIO(cctx, a.cfg.NetSampleInterval)
		if err != nil {
			return err
		}
		mu.Lock()
		payload.NetIn = int64(snap.BytesRecv)
		payload.NetOut = int64(snap.BytesSent)
		payload.NetInSpeed = int64(in)
		payload.NetOutSpeed = int64(out)
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		l1, l5, l15, err := Load()
		if err != nil {
			return err
		}
		mu.Lock()
		payload.Load1 = l1
		payload.Load5 = l5
		payload.Load15 = l15
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		tcp, udp, err := Connections()
		if err != nil {
			return err
		}
		mu.Lock()
		payload.ConnTCP = int32(tcp)
		payload.ConnUDP = int32(udp)
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		up, err := Uptime()
		if err != nil {
			return err
		}
		mu.Lock()
		payload.Uptime = int64(up)
		mu.Unlock()
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("collector aggregate: %w", err)
	}
	return payload, nil
}
