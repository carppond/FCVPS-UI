// Package notify implements the M-NOTIFY backend: a channel registry, an
// Emit/SendTest manager, a 5-minute deduper, Go-template renderer and the
// SSE event bus consumed by the /api/notify/stream endpoint.
//
// Architecture references:
//   - docs/03-architecture.md §6.6 (notification debounce)
//   - docs/05-tech-lead-plan.md §T-22 (this task) + §1.5 (SSE event types)
//   - docs/04-api-contract.md §2.19 (channel kind enum + 10 Config shapes)
//
// Scope of this file: the high-level event taxonomy (EventType constants
// re-exported from internal/types) and the payload structs that handlers
// build before calling Manager.Emit.
package notify

import (
	"shiguang-vps/internal/types"
)

// EventType is re-exported from types so callers in this package can use
// notify.EventNodeOffline instead of fishing through internal/types.
type EventType = types.EventType

// Re-export the seven v1 event types for ergonomic access.
const (
	EventNodeOffline            = types.EventNodeOffline
	EventTrafficThreshold       = types.EventTrafficThreshold
	EventSubscriptionSyncFailed = types.EventSubscriptionSyncFailed
	EventBackupCompleted        = types.EventBackupCompleted
	EventLoginAnomaly           = types.EventLoginAnomaly
	EventOTAAvailable           = types.EventOTAAvailable
	EventScriptAlert            = types.EventScriptAlert
)

// EventStatus is re-exported for the manager / repo layer.
type EventStatus = types.EventStatus

const (
	EventStatusPending       = types.EventStatusPending
	EventStatusSent          = types.EventStatusSent
	EventStatusFailed        = types.EventStatusFailed
	EventStatusSkippedDedupe = types.EventStatusSkippedDedupe
)

// Event is the value handlers pass to Manager.Emit. The payload is a kind-
// specific struct (NodeOfflinePayload, TrafficThresholdPayload, etc.). It is
// rendered via Go templates so any exported field is reachable from the
// template body. ResourceID is part of the dedupe key (combined with Type) so
// repeated alerts on the same node/subscription collapse into a single notice
// within the 5-minute window.
type Event struct {
	// Type is one of the EventType constants above.
	Type EventType
	// UserID owns the notification. Channels are scanned by user; the
	// manager fans out to every enabled, opted-in channel.
	UserID string
	// ResourceID identifies the subject of the event (node ID, subscription
	// ID, agent ID, ...). It is concatenated with Type to form the dedupe
	// key (sha1).
	ResourceID string
	// Subject is a short human label used as the email Subject when the
	// channel template does not provide one.
	Subject string
	// Locale is the recipient's language code (zh-CN / en / ja / ko). When
	// empty, the manager falls back to the user's stored locale, then
	// "zh-CN".
	Locale string
	// Payload is the kind-specific data made available to templates as the
	// dot context. Templates reach fields via {{ .FieldName }}.
	Payload any
}

// NodeOfflinePayload describes a node-down alert (PRD M-NOTIFY.1).
type NodeOfflinePayload struct {
	NodeID     string
	NodeName   string
	AgentID    string
	AgentName  string
	LastSeenAt int64
	Duration   string
}

// TrafficThresholdPayload describes a flux threshold breach (PRD M-NOTIFY.2).
type TrafficThresholdPayload struct {
	UserID        string
	PeriodStart   string
	PeriodEnd     string
	TotalUsed     int64
	TotalLimit    int64
	UsagePercent  float64
	ThresholdPct  int32
}

// SubscriptionSyncFailedPayload describes a sync error (PRD M-NOTIFY.3).
type SubscriptionSyncFailedPayload struct {
	SubscriptionID   string
	SubscriptionName string
	SourceURL        string
	ErrorMessage     string
	FailedAt         int64
}

// BackupCompletedPayload describes a backup result (PRD M-NOTIFY.4).
type BackupCompletedPayload struct {
	BackupID    string
	Filename    string
	SizeBytes   int64
	DurationMs  int64
	Success     bool
	ErrorReason string
}

// LoginAnomalyPayload describes a login anomaly (PRD M-NOTIFY.5).
type LoginAnomalyPayload struct {
	Username  string
	IP        string
	UserAgent string
	Reason    string
	OccuredAt int64
}

// OTAAvailablePayload describes an OTA update notice.
type OTAAvailablePayload struct {
	LatestVersion  string
	CurrentVersion string
	ReleaseURL     string
	PublishedAt    int64
}

// ScriptAlertPayload describes a JS script runtime error.
type ScriptAlertPayload struct {
	ScriptID   string
	ScriptName string
	ErrorMsg   string
	OccuredAt  int64
}
