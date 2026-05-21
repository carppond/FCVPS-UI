package nezha

import (
	"log/slog"
	"time"

	"shiguang-vps/internal/storage"
)

// MapResult bundles the converted AgentMetricRecord with structured warnings
// surfaced during the mapping. Callers (handler / adapter) decide whether to
// log warnings as a single line or fan them out.
type MapResult struct {
	Record   storage.AgentMetricRecord
	Warnings []string
}

// HasWarnings reports whether the mapping surfaced any field-level concerns.
func (r MapResult) HasWarnings() bool { return len(r.Warnings) > 0 }

// HostInfo distils the agents-table updates the Nezha host snapshot triggers.
// The handler applies these via AgentRepo.UpdateLastSeen so the agents row
// keeps os / arch / version in sync with the agent's self-report.
type HostInfo struct {
	OS      string
	Arch    string
	Version string
}

// NezhaToAgentRecord projects a Nezha v2 heartbeat into the canonical
// AgentMetricRecord schema. agentID is the resolved agents.id (NOT the
// secret) and recordedAt is the wall-clock the handler observed the request.
//
// Field mapping (§1.7 + ADR 0003 附录 A):
//
//	cpu               → CPUPercent
//	mem_used          → MemUsed
//	disk_used         → DiskUsed
//	swap_used         → SwapUsed
//	net_in_speed      → NetInSpeed
//	net_out_speed     → NetOutSpeed
//	net_in_transfer   → NetIn   (cumulative bytes)
//	net_out_transfer  → NetOut  (cumulative bytes)
//	load_1 / 5 / 15   → Load1 / Load5 / Load15
//	tcp_conn_count    → ConnTCP
//	udp_conn_count    → ConnUDP
//	process_count     → ProcessCount
//	uptime            → Uptime
//
// MemTotal / SwapTotal / DiskTotal come from the host snapshot (sent on first
// connect only) — the mapper carries them through when present but leaves them
// zero otherwise so a subsequent heartbeat without host overrides cannot
// clobber a previously-stored total with 0.
//
// Missing fields are filled with zero values and surface as warnings in
// MapResult.Warnings. The handler logs them at warn level so operators see
// upstream protocol drift (§risk 5) without 500'ing the request.
func NezhaToAgentRecord(hb NezhaHeartbeat, agentID string, recordedAt time.Time) MapResult {
	rec := storage.AgentMetricRecord{
		AgentID:    agentID,
		RecordedAt: recordedAt.UnixMilli(),
	}
	warnings := make([]string, 0, 4)

	if hb.State == nil {
		warnings = append(warnings, "missing state block")
	} else {
		s := hb.State
		rec.CPUPercent = s.CPU
		rec.MemUsed = s.MemUsed
		rec.SwapUsed = s.SwapUsed
		rec.DiskUsed = s.DiskUsed
		rec.NetInSpeed = s.NetInSpeed
		rec.NetOutSpeed = s.NetOutSpeed
		rec.NetIn = s.NetInTransfer
		rec.NetOut = s.NetOutTransfer
		rec.Load1 = s.Load1
		rec.Load5 = s.Load5
		rec.Load15 = s.Load15
		rec.ConnTCP = s.TCPConnCount
		rec.ConnUDP = s.UDPConnCount
		rec.ProcessCount = s.ProcessCount
		rec.Uptime = s.Uptime

		// Per §1.7 these five are the load-bearing metrics; warn when none
		// were populated so noisy agents do not silently degrade the chart.
		if s.CPU == 0 && s.MemUsed == 0 && s.DiskUsed == 0 &&
			s.NetInTransfer == 0 && s.NetOutTransfer == 0 {
			warnings = append(warnings, "state has zero values for cpu/mem/disk/net (likely fill_zero on missing fields)")
		}
	}

	if hb.Host != nil {
		rec.MemTotal = hb.Host.MemTotal
		rec.SwapTotal = hb.Host.SwapTotal
		rec.DiskTotal = hb.Host.DiskTotal
	}

	if len(hb.Extra) > 0 {
		warnings = append(warnings, "unknown top-level fields ignored")
	}

	return MapResult{Record: rec, Warnings: warnings}
}

// ExtractHostInfo distils the agents-table updates the host snapshot triggers.
// Returns an empty (zero-value) HostInfo when hb.Host is nil; the caller can
// short-circuit the UpdateLastSeen call in that case to avoid clobbering
// existing values with empty strings.
//
// Platform values are passed through verbatim — Nezha's "darwin" / "linux" /
// "windows" already match the canonical OS strings the native agent reports.
// Only when Platform is empty AND PlatformVersion is non-empty do we treat
// the latter as the OS hint (some Nezha builds put "Ubuntu 22.04" there
// without setting Platform).
func ExtractHostInfo(hb NezhaHeartbeat) HostInfo {
	if hb.Host == nil {
		return HostInfo{}
	}
	out := HostInfo{
		OS:      hb.Host.Platform,
		Arch:    hb.Host.Arch,
		Version: hb.Host.Version,
	}
	if out.OS == "" && hb.Host.PlatformVersion != "" {
		out.OS = hb.Host.PlatformVersion
	}
	return out
}

// LogWarnings emits structured warnings via logger. agentID is included so the
// operator can correlate the warning with a specific Nezha agent. Safe to call
// with a nil logger (becomes a no-op).
func LogWarnings(logger *slog.Logger, agentID string, warnings []string) {
	if logger == nil || len(warnings) == 0 {
		return
	}
	for _, w := range warnings {
		logger.Warn("nezha compat: field mapping",
			slog.String("agent_id", agentID),
			slog.String("warning", w),
		)
	}
}
