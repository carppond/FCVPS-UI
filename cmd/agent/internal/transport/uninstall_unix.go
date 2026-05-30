//go:build !windows

package transport

import (
	"fmt"
	"os/exec"
	"syscall"
)

// uninstallScript stops the systemd service (if any), removes the unit +
// binary, and kills the agent PID directly so hosts without systemd are also
// covered. The leading sleep gives the cmd_ack a moment to flush before this
// process kills the agent. %d is the agent PID.
const uninstallScript = `sleep 1
if command -v systemctl >/dev/null 2>&1; then
  systemctl disable --now shiguang-agent 2>/dev/null || true
  rm -f /etc/systemd/system/shiguang-agent.service
  systemctl daemon-reload 2>/dev/null || true
fi
kill %d 2>/dev/null || true
rm -f /usr/local/bin/shiguang-agent`

// spawnDetachedUninstall starts the uninstaller in a NEW session (Setsid) so it
// is not in the agent's process group / systemd cgroup — `systemctl stop`
// therefore cannot kill it mid-run. It returns once the child has started.
//
// A package var (not a plain func) so tests can stub it — the real
// implementation actually rm's the binary + kills the PID, which must never
// run inside a test process.
var spawnDetachedUninstall = func(pid int) error {
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(uninstallScript, pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn uninstaller: %w", err)
	}
	// Release so we don't leave a zombie; the child keeps running detached.
	return cmd.Process.Release()
}
