package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Canonical keys persisted in the system_settings table. Centralising the
// strings here means handlers / ops code reference the same literal — typos
// surface at compile time instead of as silent no-ops at runtime.
//
// The set mirrors docs/04-api-contract.md §2.23. Anything missing from this
// list is treated as "unknown setting" by validation.
const (
	SettingSilentModeEnabled       = "silent_mode_enabled"
	SettingSilentModePrefix        = "silent_mode_prefix"
	SettingSessionTTLSeconds       = "session_ttl_seconds"
	SettingMonthlyResetDay         = "monthly_reset_day"
	SettingOTACheckInterval        = "ota_check_interval"
	SettingAgentHeartbeatInterval  = "agent_heartbeat_interval"
	SettingNotificationDebounce    = "notification_debounce"
	SettingSMTPHost                = "smtp_host"
	SettingSMTPPort                = "smtp_port"
	SettingSMTPUsername            = "smtp_username"
	SettingSMTPPassword            = "smtp_password"
	SettingTelegramBotToken        = "telegram_bot_token"
	SettingDefaultLocale           = "default_locale"
)

// SensitiveSettingKeys is the deny-list applied by GET /api/admin/settings —
// any key listed here is replaced with the literal "******" so screenshots /
// audit captures never expose secrets. PUT requests use the same mask: a value
// of "******" is treated as "keep the existing value".
//
// IMPORTANT: keep this list in sync with the sensitive fields above.
var SensitiveSettingKeys = map[string]struct{}{
	SettingSMTPPassword:     {},
	SettingTelegramBotToken: {},
}

// SettingsMask is the placeholder returned to GET callers (and accepted on PUT
// as "no change") for any key in SensitiveSettingKeys.
const SettingsMask = "******"

// ErrSettingNotFound is returned by Get when the row is missing. GetAll never
// returns this — it simply omits the key from the resulting map.
var ErrSettingNotFound = errors.New("storage: setting not found")

// SettingsRepo is the k/v accessor for the system_settings table. Every read
// goes through the read pool, every write through the writer pool, mirroring
// the project-wide split.
type SettingsRepo struct {
	db  *DB
	now func() time.Time
}

// NewSettingsRepo wires a repo to db. When now is nil, time.Now is used.
func NewSettingsRepo(db *DB, now func() time.Time) *SettingsRepo {
	if now == nil {
		now = time.Now
	}
	return &SettingsRepo{db: db, now: now}
}

// Get returns the raw string value for key. Returns ErrSettingNotFound when
// the row is missing.
func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("settings get: empty key")
	}
	row := r.db.Read.QueryRowContext(ctx,
		"SELECT value FROM system_settings WHERE key = ?", key)
	var value string
	if err := row.Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrSettingNotFound
		}
		return "", fmt.Errorf("scan setting %q: %w", key, err)
	}
	return value, nil
}

// Set upserts the row. value is persisted verbatim — JSON encoding / type
// conversion is the caller's responsibility.
func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	if key == "" {
		return fmt.Errorf("settings set: empty key")
	}
	now := r.now().UnixMilli()
	_, err := r.db.Write.ExecContext(ctx, `
		INSERT INTO system_settings(key, value, updated_at) VALUES(?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value,
		                                updated_at = excluded.updated_at`,
		key, value, now)
	if err != nil {
		return fmt.Errorf("upsert setting %q: %w", key, err)
	}
	return nil
}

// GetAll returns every persisted row as a key→value map. Sensitive keys are
// returned in their raw form (the masking happens in the HTTP layer so internal
// callers can still read the cleartext value).
func (r *SettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.Read.QueryContext(ctx,
		"SELECT key, value FROM system_settings")
	if err != nil {
		return nil, fmt.Errorf("select all settings: %w", err)
	}
	defer rows.Close()
	out := make(map[string]string, 16)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan setting row: %w", err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings: %w", err)
	}
	return out, nil
}

// SetMany upserts every k/v pair inside a single transaction so partial
// failures roll back. Callers that batch-update settings (settings handler PUT)
// should use this to avoid leaving the DB in a half-updated state if the
// process crashes mid-write.
func (r *SettingsRepo) SetMany(ctx context.Context, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	now := r.now().UnixMilli()
	tx, err := r.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO system_settings(key, value, updated_at) VALUES(?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value,
		                                updated_at = excluded.updated_at`)
	if err != nil {
		return fmt.Errorf("prepare upsert: %w", err)
	}
	defer stmt.Close()
	for k, v := range values {
		if k == "" {
			continue
		}
		if _, err := stmt.ExecContext(ctx, k, v, now); err != nil {
			return fmt.Errorf("upsert %q: %w", k, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit settings batch: %w", err)
	}
	return nil
}
