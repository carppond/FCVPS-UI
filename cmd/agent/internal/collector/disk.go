package collector

import (
	"fmt"
	"runtime"

	"github.com/shirou/gopsutil/v4/disk"
)

// rootMountpoint is the partition we inspect for the disk metric. Windows
// drops the leading slash and uses C: as the conventional system drive.
func rootMountpoint() string {
	if runtime.GOOS == "windows" {
		return "C:\\"
	}
	return "/"
}

// Disk returns the used + total bytes for the root partition (or C:\ on
// Windows). Multi-disk hosts are out of scope for v1 — the protocol carries
// a single pair of disk counters.
func Disk() (used, total uint64, err error) {
	u, err := disk.Usage(rootMountpoint())
	if err != nil {
		return 0, 0, fmt.Errorf("collector disk: usage %q: %w", rootMountpoint(), err)
	}
	return u.Used, u.Total, nil
}
