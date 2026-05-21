package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/net"
)

// NetCounters captures one sample of aggregate network IO across every
// non-loopback interface (gopsutil collapses per-interface counters when the
// pernic argument is false).
type NetCounters struct {
	BytesRecv uint64
	BytesSent uint64
}

// NetSnapshot returns the cumulative byte counters for the host. Useful when
// the caller wants to maintain its own delta tracking across multiple
// metrics-collection cycles.
func NetSnapshot() (NetCounters, error) {
	stats, err := net.IOCounters(false)
	if err != nil {
		return NetCounters{}, fmt.Errorf("collector net: io counters: %w", err)
	}
	if len(stats) == 0 {
		return NetCounters{}, nil
	}
	return NetCounters{
		BytesRecv: stats[0].BytesRecv,
		BytesSent: stats[0].BytesSent,
	}, nil
}

// NetIO samples the host network counters twice (separated by `interval`) and
// returns the per-second rate of inbound + outbound traffic in bytes/sec.
//
// `interval` must be > 0. The context cancels the inter-sample sleep but does
// not abort an in-flight gopsutil call; long ctx deadlines (≥ 2× interval)
// are recommended.
func NetIO(ctx context.Context, interval time.Duration) (inBps, outBps uint64, err error) {
	if interval <= 0 {
		return 0, 0, fmt.Errorf("collector net: interval must be > 0")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	first, err := NetSnapshot()
	if err != nil {
		return 0, 0, err
	}
	t := time.NewTimer(interval)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return 0, 0, ctx.Err()
	case <-t.C:
	}
	second, err := NetSnapshot()
	if err != nil {
		return 0, 0, err
	}
	seconds := uint64(interval.Seconds())
	if seconds == 0 {
		seconds = 1
	}
	return deltaPerSec(first.BytesRecv, second.BytesRecv, seconds),
		deltaPerSec(first.BytesSent, second.BytesSent, seconds), nil
}

// deltaPerSec computes (second − first) / seconds with overflow-safe
// wraparound: counter resets (e.g. interface reload) return 0 rather than a
// huge bogus rate.
func deltaPerSec(first, second uint64, seconds uint64) uint64 {
	if second < first {
		return 0
	}
	diff := second - first
	return diff / seconds
}

// ConnConnections returns TCP + UDP connection counts (host-wide). Kept
// separate from NetIO so the aggregator can call it without paying the
// sampling-interval cost.
func ConnConnections() (tcp, udp uint64, err error) {
	tcpConns, err := net.Connections("tcp")
	if err != nil {
		return 0, 0, fmt.Errorf("collector net: tcp connections: %w", err)
	}
	udpConns, err := net.Connections("udp")
	if err != nil {
		return uint64(len(tcpConns)), 0, fmt.Errorf("collector net: udp connections: %w", err)
	}
	return uint64(len(tcpConns)), uint64(len(udpConns)), nil
}
