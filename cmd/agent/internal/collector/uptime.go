package collector

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/host"
)

// Uptime returns the host's uptime in seconds. Falls through to gopsutil/host
// which is cross-platform (Linux: /proc/uptime, macOS: sysctl, Windows:
// GetTickCount64).
func Uptime() (uint64, error) {
	s, err := host.Uptime()
	if err != nil {
		return 0, fmt.Errorf("collector uptime: %w", err)
	}
	return s, nil
}
