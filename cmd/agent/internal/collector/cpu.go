// Package collector groups the host-metric samplers that feed the agent's
// metrics envelope. Each collector lives in its own file so a single failing
// data source (e.g. unreadable /proc/stat in a container) can be diagnosed
// without scanning unrelated code.
//
// Collectors return zero/identifiable defaults on best-effort failure paths
// but propagate hard errors to the aggregator, which decides whether to skip
// the metrics frame entirely (see collector.Collect).
package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

// cpuSampleInterval is the sampling window used to derive the CPU busy
// percentage. 1 s strikes a balance between responsiveness and noise (the
// gopsutil docs explicitly warn against intervals < 100 ms).
const cpuSampleInterval = 1 * time.Second

// CPU returns the host-wide CPU busy percentage averaged over a 1 s window.
// The value lies in [0, 100]. Per-CPU sampling is intentionally disabled —
// the metrics protocol only carries an aggregate field.
//
// The caller should pass a context with a deadline > cpuSampleInterval so a
// context cancellation can short-circuit the sampler.
func CPU(ctx context.Context) (float64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	pcts, err := cpu.PercentWithContext(ctx, cpuSampleInterval, false)
	if err != nil {
		return 0, fmt.Errorf("collector cpu: sample: %w", err)
	}
	if len(pcts) == 0 {
		return 0, nil
	}
	v := pcts[0]
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v, nil
}
