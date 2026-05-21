package collector

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/mem"
)

// Memory returns the host's virtual + swap memory usage in bytes.
//
// On platforms where swap is not present (e.g. Docker containers with no
// swapaccount) the swap values fall back to zero rather than surfacing an
// error — losing the metric frame for a missing swap counter would be a
// poor tradeoff.
func Memory() (used, total, swapUsed, swapTotal uint64, err error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("collector memory: virtual: %w", err)
	}
	sm, swapErr := mem.SwapMemory()
	if swapErr != nil {
		// Treat missing swap as zero — see godoc above.
		return vm.Used, vm.Total, 0, 0, nil
	}
	return vm.Used, vm.Total, sm.Used, sm.Total, nil
}
