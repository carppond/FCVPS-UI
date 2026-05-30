//go:build windows

package transport

import "fmt"

// spawnDetachedUninstall is unsupported on Windows — the agent there is not
// installed via the bash installer / systemd, so there is no unit to remove.
// A package var to mirror the unix build (stubbable in tests).
var spawnDetachedUninstall = func(pid int) error {
	return fmt.Errorf("self-uninstall not supported on windows")
}
