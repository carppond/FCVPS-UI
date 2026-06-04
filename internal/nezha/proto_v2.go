// Package nezha implements the Nezha agent v2 protocol compatibility layer.
//
// Scope (per docs/05-tech-lead-plan §1.7 + ADR 0003 附录 A):
//
//   - Accept Nezha v2 heartbeat payloads (State + Host minimal subset).
//   - Map to the canonical AgentMetricRecord schema so the rest of the system
//     (UI, traffic aggregator, notifications) is protocol-agnostic.
//   - Do NOT support v0 / v1 — Nezha v1 is EOL and the field shapes diverge.
//
// The package is wired to two HTTP routes which share a single handler:
//
//   - POST /api/v1/nezha/heartbeat (Nezha agent default path)
//   - POST /api/v1/nezha/report    (alias retained for forward-compat)
//
// Authentication:
//   - Authorization: Bearer <secret>   (preferred)
//   - ?secret=<token>                  (fallback)
//   - body."secret" field              (Nezha agent default behaviour)
//
// Failure mode: any auth error (missing / unknown / wrong-kind agent) returns
// a silent 404 (Nginx-shaped body) per PRD §6.3 / ADR 0006.
package nezha

import "encoding/json"

// NezhaHeartbeat is the Nezha v2 heartbeat envelope as accepted by the hub.
//
// The struct is intentionally permissive — every nested field is optional so a
// partially-populated agent (e.g. one that disables swap reporting) is still
// admitted. Unknown top-level fields are preserved in Extra so the warning log
// in compat.go can surface upstream protocol drift (§risk 5).
type NezhaHeartbeat struct {
	// Secret is the bearer secret embedded in the body. Nezha clients populate
	// this when neither Authorization header nor ?secret= query are configured.
	Secret string `json:"secret,omitempty"`

	State *NezhaState `json:"state,omitempty"`
	Host  *NezhaHost  `json:"host,omitempty"`

	// Extra captures any unknown top-level fields. Populated by
	// UnmarshalNezhaHeartbeat; not consulted during the field mapping.
	Extra map[string]json.RawMessage `json:"-"`
}

// NezhaState is the runtime metric snapshot. All numeric fields are optional
// (zero value = "not reported"); the mapper writes zero into the equivalent
// AgentMetricRecord field when absent.
type NezhaState struct {
	CPU             float64 `json:"cpu,omitempty"`
	MemUsed         int64   `json:"mem_used,omitempty"`
	SwapUsed        int64   `json:"swap_used,omitempty"`
	DiskUsed        int64   `json:"disk_used,omitempty"`
	NetInSpeed      int64   `json:"net_in_speed,omitempty"`
	NetOutSpeed     int64   `json:"net_out_speed,omitempty"`
	NetInTransfer   int64   `json:"net_in_transfer,omitempty"`
	NetOutTransfer  int64   `json:"net_out_transfer,omitempty"`
	Load1           float64 `json:"load_1,omitempty"`
	Load5           float64 `json:"load_5,omitempty"`
	Load15          float64 `json:"load_15,omitempty"`
	TCPConnCount    int32   `json:"tcp_conn_count,omitempty"`
	UDPConnCount    int32   `json:"udp_conn_count,omitempty"`
	ProcessCount    int32   `json:"process_count,omitempty"`
	Uptime          int64   `json:"uptime,omitempty"`
	Temperatures    json.RawMessage `json:"temperatures,omitempty"` // accepted, not mapped
	GPU             json.RawMessage `json:"gpu,omitempty"`          // accepted, not mapped
}

// NezhaHost is the static host descriptor (only sent on first connect or when
// the agent restarts). Used to update the agents table's os / arch columns.
type NezhaHost struct {
	Platform        string   `json:"platform,omitempty"`
	PlatformVersion string   `json:"platform_version,omitempty"`
	CPU             []string `json:"cpu,omitempty"`
	MemTotal        int64    `json:"mem_total,omitempty"`
	SwapTotal       int64    `json:"swap_total,omitempty"`
	DiskTotal       int64    `json:"disk_total,omitempty"`
	Arch            string   `json:"arch,omitempty"`
	Virtualization  string   `json:"virtualization,omitempty"`
	BootTime        int64    `json:"boot_time,omitempty"`
	Version         string   `json:"version,omitempty"`
}

// UnmarshalNezhaHeartbeat parses raw JSON into NezhaHeartbeat while preserving
// unknown top-level fields under Extra. Unknown nested fields are silently
// dropped — only the top-level surface needs introspection for the warning
// log path (see §risk 5).
//
// Returns an error only when the input is not syntactically valid JSON or the
// root is not an object.
func UnmarshalNezhaHeartbeat(raw []byte) (*NezhaHeartbeat, error) {
	hb := &NezhaHeartbeat{}
	if err := json.Unmarshal(raw, hb); err != nil {
		return nil, err
	}
	// Second pass: capture every top-level key into Extra, then prune the keys
	// we already recognise so the leftover map only contains the unknowns.
	var raw2 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &raw2); err != nil {
		// Not a JSON object — leave Extra nil; the typed parse above already
		// succeeded so the caller still gets the populated struct.
		return hb, nil //nolint:nilerr // 二次解析只为收集 Extra,失败可降级
	}
	delete(raw2, "secret")
	delete(raw2, "state")
	delete(raw2, "host")
	if len(raw2) > 0 {
		hb.Extra = raw2
	}
	return hb, nil
}
