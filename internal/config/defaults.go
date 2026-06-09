// Package config provides typed configuration loading for the shiguang-vps hub.
package config

import "time"

// Default values shared across the hub. These constants are the single source
// of truth for fallbacks when no env var / flag is supplied.
const (
	// DefaultHTTPAddr is the listen address for the hub HTTP server.
	DefaultHTTPAddr = ":8080"

	// DefaultDataDir is the on-disk location for SQLite database and assets.
	DefaultDataDir = "./data"

	// DefaultDBFilename is the SQLite database file name under the data dir.
	DefaultDBFilename = "shiguang.db"

	// DefaultLogLevel is the slog level used when none is configured.
	DefaultLogLevel = "info"

	// DefaultLogFormat is the default log encoder ("json" or "text").
	DefaultLogFormat = "json"

	// DefaultLogMaxSizeMB is the max rotated log file size (megabytes).
	DefaultLogMaxSizeMB = 100

	// DefaultLogMaxAgeDays is the number of days to retain rotated logs.
	DefaultLogMaxAgeDays = 7

	// DefaultLogMaxBackups is the maximum number of rotated log files retained.
	DefaultLogMaxBackups = 14

	// DefaultDBMaxOpenWrite is the write-pool max open connections (always 1 for SQLite).
	DefaultDBMaxOpenWrite = 1

	// DefaultDBMaxOpenRead is the read-pool max open connections.
	DefaultDBMaxOpenRead = 8

	// DefaultDBBusyTimeoutMs is the busy_timeout PRAGMA value (milliseconds).
	DefaultDBBusyTimeoutMs = 5000

	// DefaultSessionTTL is the rolling lifetime for an HTTP API session token.
	DefaultSessionTTL = 24 * time.Hour

	// DefaultAgentHeartbeatInterval is the suggested heartbeat cadence for agents.
	DefaultAgentHeartbeatInterval = 30 * time.Second

	// DefaultBcryptCost is the bcrypt work factor used for password hashing.
	// 12 per OWASP's current guidance (~250ms/hash). Existing cost-10 hashes
	// keep verifying — bcrypt encodes its cost in the hash — and silently
	// upgrade to 12 the next time the user changes their password.
	DefaultBcryptCost = 12

	// DefaultLoginRatePerSecond is the per-(IP|username) login bucket refill
	// rate (5 attempts per hour ≈ 0.00139/s).
	DefaultLoginRatePerSecond = 5.0 / 3600.0

	// DefaultLoginRateBurst lets honest users tolerate a quick mistyped-
	// password retry without being rate-limited.
	DefaultLoginRateBurst = 5

	// DefaultBackupKeep is how many nightly archives the scheduler retains.
	DefaultBackupKeep = 7
	// DefaultBackupHour is the UTC hour the nightly backup runs at.
	DefaultBackupHour = 4

	// DefaultPaginationPage is the default page index when none is provided.
	DefaultPaginationPage = 1

	// DefaultPaginationPageSize is the default page size when none is provided.
	DefaultPaginationPageSize = 20

	// MaxPaginationPageSize is the upper bound for the page_size query param.
	MaxPaginationPageSize = 100
)
