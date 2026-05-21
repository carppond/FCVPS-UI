package collector

import (
	"context"
	"fmt"
	"runtime"

	"github.com/shirou/gopsutil/v4/load"
)

// Load returns the 1/5/15-minute Unix load averages. On Windows (no /proc/
// loadavg analogue) the function degrades to a single point sample of CPU
// percent so the metric is at least non-zero — operators are expected to
// look at cpu_percent on Windows hosts anyway.
func Load() (load1, load5, load15 float64, err error) {
	if runtime.GOOS == "windows" {
		// On Windows we lack a stable loadavg source — fall back to the
		// current CPU percent so the panels still show *something*.
		ctx, cancel := context.WithTimeout(context.Background(), cpuSampleInterval+500e6)
		defer cancel()
		v, cpuErr := CPU(ctx)
		if cpuErr != nil {
			return 0, 0, 0, fmt.Errorf("collector load: windows fallback: %w", cpuErr)
		}
		return v / 100, v / 100, v / 100, nil
	}
	avg, err := load.Avg()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("collector load: avg: %w", err)
	}
	return avg.Load1, avg.Load5, avg.Load15, nil
}
