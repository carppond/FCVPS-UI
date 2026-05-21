// Package agent owns the hub-side WebSocket fan-in for FCVPS agents.
//
// The package re-exports the shared protocol types from pkg/agentlib so
// downstream code (handlers, tests) does not need to depend on the agent-side
// library directly. It also defines hub-only types — the EventBus interface
// for SSE fan-out (wired by T-22) and the AgentStatus snapshot consumed by
// /api/agents.
//
// Design notes (Tech Lead §1.8 + task T-14 scope):
//
//   - Protocol version negotiation: hub compares HelloPayload.Version's major
//     against ProtocolVersion. Mismatch → bye{reason: "version_unsupported"}.
//   - Tokens: agent connects via ?token=<base64url(32B)>; hub looks up by
//     sha256 hex. Plaintext tokens never leave the agent → handler trip.
//   - Heartbeat: hub advertises 30 s in hello_ack; idle timeout = 90 s
//     (no message of any kind for 3× the advertised interval).
//   - Commands: refresh_subscription + collect_now are wired in v1; the
//     restart command exists in the protocol but the agent CLI handler is
//     deferred to T-15.
package agent

import (
	"shiguang-vps/pkg/agentlib"
)

// Aliases — keep the hub code readable without leaking the agentlib package
// name through every signature.
type (
	Envelope         = agentlib.Envelope
	HelloPayload     = agentlib.HelloPayload
	HelloAckPayload  = agentlib.HelloAckPayload
	HeartbeatPayload = agentlib.HeartbeatPayload
	MetricsPayload   = agentlib.MetricsPayload
	CmdPayload       = agentlib.CmdPayload
	CmdAckPayload    = agentlib.CmdAckPayload
	ByePayload       = agentlib.ByePayload

	MessageType = agentlib.MessageType
	CmdType     = agentlib.CmdType
)

// Re-export message-type constants so the hub code can reference them without
// an alias-chain through agentlib.
const (
	MsgHello     = agentlib.MsgHello
	MsgHelloAck  = agentlib.MsgHelloAck
	MsgHeartbeat = agentlib.MsgHeartbeat
	MsgMetrics   = agentlib.MsgMetrics
	MsgCmd       = agentlib.MsgCmd
	MsgCmdAck    = agentlib.MsgCmdAck
	MsgBye       = agentlib.MsgBye

	CmdRestart             = agentlib.CmdRestart
	CmdRefreshSubscription = agentlib.CmdRefreshSubscription
	CmdCollectNow          = agentlib.CmdCollectNow
	CmdShutdown            = agentlib.CmdShutdown

	ByeReasonVersionUnsupported = agentlib.ByeReasonVersionUnsupported
	ByeReasonServerShutdown     = agentlib.ByeReasonServerShutdown
	ByeReasonTokenRotated       = agentlib.ByeReasonTokenRotated
	ByeReasonIdleTimeout        = agentlib.ByeReasonIdleTimeout
	ByeReasonAgentDeleted       = agentlib.ByeReasonAgentDeleted

	ProtocolVersion = agentlib.ProtocolVersion
)

// majorVersion returns the integer major version from a "MAJOR.MINOR[.PATCH]"
// version string. Returns -1 on parse failure (caller treats unknown versions
// as "unsupported" per §1.8).
func majorVersion(v string) int {
	if v == "" {
		return -1
	}
	major := 0
	seenDigit := false
	for _, r := range v {
		if r == '.' {
			break
		}
		if r < '0' || r > '9' {
			return -1
		}
		major = major*10 + int(r-'0')
		seenDigit = true
	}
	if !seenDigit {
		return -1
	}
	return major
}

// IsVersionCompatible reports whether the agent-reported version is acceptable
// for the current hub. Per task spec §1.8 the rule is "same major"; we accept
// every 1.x agent against the current 1.0 hub.
func IsVersionCompatible(agentVersion string) bool {
	hubMajor := majorVersion(ProtocolVersion)
	if hubMajor < 0 {
		return false
	}
	got := majorVersion(agentVersion)
	return got == hubMajor
}
