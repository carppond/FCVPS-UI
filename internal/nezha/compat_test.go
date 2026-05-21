package nezha_test

import (
	"encoding/json"
	"testing"
	"time"

	"shiguang-vps/internal/nezha"
)

// TestNezhaToAgentRecordFullMapping checks every field map listed in
// docs/05-tech-lead-plan §1.7 + ADR 0003 附录 A lands at the expected
// AgentMetricRecord column. The constants are deliberately distinct so a
// crossed wire (e.g. NetIn ↔ NetOut) shows up as a mismatched expect/actual
// rather than a coincidental pass.
func TestNezhaToAgentRecordFullMapping(t *testing.T) {
	hb := nezha.NezhaHeartbeat{
		State: &nezha.NezhaState{
			CPU:             12.5,
			MemUsed:         1024,
			SwapUsed:        2048,
			DiskUsed:        4096,
			NetInSpeed:      11,
			NetOutSpeed:     22,
			NetInTransfer:   1_000_000_000_000,
			NetOutTransfer:  2_000_000_000_000,
			Load1:           0.5,
			Load5:           0.3,
			Load15:          0.2,
			TCPConnCount:    10,
			UDPConnCount:    20,
			ProcessCount:    100,
			Uptime:          86400,
		},
		Host: &nezha.NezhaHost{
			MemTotal:  8 * 1024 * 1024 * 1024,
			SwapTotal: 1 * 1024 * 1024 * 1024,
			DiskTotal: 100 * 1024 * 1024 * 1024,
		},
	}
	now := time.Unix(1700000000, 0).UTC()
	res := nezha.NezhaToAgentRecord(hb, "agent-1", now)
	rec := res.Record

	if rec.AgentID != "agent-1" {
		t.Fatalf("agent id: %q", rec.AgentID)
	}
	if rec.RecordedAt != now.UnixMilli() {
		t.Fatalf("recorded_at: %d want %d", rec.RecordedAt, now.UnixMilli())
	}
	if rec.CPUPercent != 12.5 {
		t.Fatalf("cpu: %v", rec.CPUPercent)
	}
	if rec.MemUsed != 1024 || rec.MemTotal != hb.Host.MemTotal {
		t.Fatalf("mem mismatch: used=%d total=%d", rec.MemUsed, rec.MemTotal)
	}
	if rec.SwapUsed != 2048 || rec.SwapTotal != hb.Host.SwapTotal {
		t.Fatalf("swap mismatch: used=%d total=%d", rec.SwapUsed, rec.SwapTotal)
	}
	if rec.DiskUsed != 4096 || rec.DiskTotal != hb.Host.DiskTotal {
		t.Fatalf("disk mismatch: used=%d total=%d", rec.DiskUsed, rec.DiskTotal)
	}
	if rec.NetInSpeed != 11 || rec.NetOutSpeed != 22 {
		t.Fatalf("net speed mismatch: in=%d out=%d", rec.NetInSpeed, rec.NetOutSpeed)
	}
	if rec.NetIn != 1_000_000_000_000 || rec.NetOut != 2_000_000_000_000 {
		t.Fatalf("net transfer mismatch: in=%d out=%d", rec.NetIn, rec.NetOut)
	}
	if rec.Load1 != 0.5 || rec.Load5 != 0.3 || rec.Load15 != 0.2 {
		t.Fatalf("load mismatch: %v/%v/%v", rec.Load1, rec.Load5, rec.Load15)
	}
	if rec.ConnTCP != 10 || rec.ConnUDP != 20 {
		t.Fatalf("conn mismatch: tcp=%d udp=%d", rec.ConnTCP, rec.ConnUDP)
	}
	if rec.ProcessCount != 100 {
		t.Fatalf("process count: %d", rec.ProcessCount)
	}
	if rec.Uptime != 86400 {
		t.Fatalf("uptime: %d", rec.Uptime)
	}
	if res.HasWarnings() {
		t.Fatalf("expected no warnings, got %v", res.Warnings)
	}
}

// TestNezhaToAgentRecordMissingState ensures a heartbeat without a state block
// is admitted (zeros) and surfaces a warning so operators can spot it.
func TestNezhaToAgentRecordMissingState(t *testing.T) {
	hb := nezha.NezhaHeartbeat{Host: &nezha.NezhaHost{MemTotal: 1024}}
	res := nezha.NezhaToAgentRecord(hb, "a", time.Unix(0, 0))
	if res.Record.CPUPercent != 0 || res.Record.MemUsed != 0 {
		t.Fatalf("expected zero state values, got %+v", res.Record)
	}
	if res.Record.MemTotal != 1024 {
		t.Fatalf("expected host mem_total carried, got %d", res.Record.MemTotal)
	}
	if !res.HasWarnings() {
		t.Fatalf("expected warning for missing state block")
	}
}

// TestNezhaToAgentRecordPartialStateWarn checks that an "all-zero" state block
// is flagged — this catches Nezha agents that submit a state struct with every
// numeric field empty (which would otherwise silently masquerade as healthy).
func TestNezhaToAgentRecordPartialStateWarn(t *testing.T) {
	hb := nezha.NezhaHeartbeat{State: &nezha.NezhaState{Uptime: 30}}
	res := nezha.NezhaToAgentRecord(hb, "a", time.Unix(0, 0))
	if !res.HasWarnings() {
		t.Fatalf("expected warning for all-zero state, got none")
	}
}

// TestExtractHostInfoPlatformPassthrough verifies the Nezha "platform" string
// reaches AgentRecord.OS verbatim (Nezha already uses "linux"/"darwin"/etc).
func TestExtractHostInfoPlatformPassthrough(t *testing.T) {
	cases := []struct {
		name string
		hb   nezha.NezhaHeartbeat
		want nezha.HostInfo
	}{
		{
			name: "linux",
			hb:   nezha.NezhaHeartbeat{Host: &nezha.NezhaHost{Platform: "linux", Arch: "amd64", Version: "0.18.0"}},
			want: nezha.HostInfo{OS: "linux", Arch: "amd64", Version: "0.18.0"},
		},
		{
			name: "darwin",
			hb:   nezha.NezhaHeartbeat{Host: &nezha.NezhaHost{Platform: "darwin", Arch: "arm64"}},
			want: nezha.HostInfo{OS: "darwin", Arch: "arm64"},
		},
		{
			name: "platform empty fallback to version",
			hb:   nezha.NezhaHeartbeat{Host: &nezha.NezhaHost{PlatformVersion: "Ubuntu 22.04", Arch: "amd64"}},
			want: nezha.HostInfo{OS: "Ubuntu 22.04", Arch: "amd64"},
		},
		{
			name: "nil host",
			hb:   nezha.NezhaHeartbeat{},
			want: nezha.HostInfo{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nezha.ExtractHostInfo(tc.hb)
			if got != tc.want {
				t.Fatalf("ExtractHostInfo: got %+v want %+v", got, tc.want)
			}
		})
	}
}

// TestUnmarshalNezhaHeartbeatPreservesUnknown captures upstream protocol drift
// — an agent that sends a "gpu_temp" field today should not 500 the hub, but
// the field should land in Extra so the warning log can surface it.
func TestUnmarshalNezhaHeartbeatPreservesUnknown(t *testing.T) {
	raw := []byte(`{
		"secret": "s",
		"state": {"cpu": 1.5},
		"host":  {"platform": "linux"},
		"gpu_temp": {"nv0": 60}
	}`)
	hb, err := nezha.UnmarshalNezhaHeartbeat(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if hb.Secret != "s" || hb.State == nil || hb.State.CPU != 1.5 {
		t.Fatalf("typed fields not parsed: %+v", hb)
	}
	if _, ok := hb.Extra["gpu_temp"]; !ok {
		t.Fatalf("expected unknown field in Extra, got %+v", hb.Extra)
	}
	// Mapping should surface "unknown top-level fields ignored".
	res := nezha.NezhaToAgentRecord(*hb, "a", time.Unix(0, 0))
	if !res.HasWarnings() {
		t.Fatalf("expected warning for unknown top-level field")
	}
}

// TestUnmarshalNezhaHeartbeatInvalidJSON guarantees malformed payloads return
// an error rather than panicking. The handler converts the error to a silent
// 404.
func TestUnmarshalNezhaHeartbeatInvalidJSON(t *testing.T) {
	_, err := nezha.UnmarshalNezhaHeartbeat([]byte(`not json`))
	if err == nil {
		t.Fatalf("expected error from invalid JSON")
	}
	// And confirm the legitimate path still roundtrips a minimal object so the
	// negative test above is meaningful.
	min := []byte(`{}`)
	hb, err := nezha.UnmarshalNezhaHeartbeat(min)
	if err != nil || hb == nil {
		t.Fatalf("empty object should be accepted: err=%v hb=%v", err, hb)
	}
}

// TestNezhaToAgentRecordRoundTripJSON makes sure the proto types serialise
// the way docs/04-api-contract §7 documents them — important for the agent
// contract because the field names are wire-visible.
func TestNezhaToAgentRecordRoundTripJSON(t *testing.T) {
	hb := nezha.NezhaHeartbeat{
		State: &nezha.NezhaState{CPU: 12.5, MemUsed: 1, NetInSpeed: 2, NetInTransfer: 3, Load1: 0.1},
		Host:  &nezha.NezhaHost{Platform: "linux", Arch: "amd64", MemTotal: 100},
	}
	buf, err := json.Marshal(hb)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	round, err := nezha.UnmarshalNezhaHeartbeat(buf)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if round.State.CPU != hb.State.CPU || round.Host.Platform != hb.Host.Platform {
		t.Fatalf("roundtrip mismatch: %+v", round)
	}
}
