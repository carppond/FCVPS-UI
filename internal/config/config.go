// Package config provides typed configuration loading for the shiguang-vps hub.
//
// Precedence (highest to lowest):
//  1. Command-line flags (parsed via Load).
//  2. Process environment variables.
//  3. Defaults declared in defaults.go.
//
// .env files are intentionally NOT read; containerised users should inject
// environment via `-e` flags.
package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the merged hub configuration, populated by Load.
type Config struct {
	HTTP     HTTPConfig
	Database DatabaseConfig
	Log      LogConfig
	Session  SessionConfig
	Agent    AgentConfig
	AuthRate AuthRateConfig

	// ResetPassword, when non-empty, switches the binary into offline account
	// recovery mode: reset that username's password (plus disable TOTP and
	// re-activate the account), print the new credentials once, then exit
	// without starting the server. Meant to be run on the host (e.g. via SSH)
	// against the same --data-dir — the escape hatch when the operator is
	// locked out (forgotten password / lost authenticator).
	ResetPassword string
}

// HTTPConfig holds settings for the public HTTP server.
type HTTPConfig struct {
	Addr string
}

// DatabaseConfig holds settings for the SQLite backing store.
type DatabaseConfig struct {
	// DataDir is the directory holding the database file and other persistent state.
	DataDir string
	// Filename is the SQLite database file name (under DataDir).
	Filename string
	// BusyTimeoutMs maps to PRAGMA busy_timeout.
	BusyTimeoutMs int
	// MaxOpenWrite is the cap on writer connections (SQLite mandates 1).
	MaxOpenWrite int
	// MaxOpenRead is the cap on reader connections.
	MaxOpenRead int
}

// Path returns the resolved on-disk path to the SQLite file.
func (d DatabaseConfig) Path() string {
	return filepath.Join(d.DataDir, d.Filename)
}

// LogConfig configures the slog-based logger.
type LogConfig struct {
	// Level is one of "debug", "info", "warn", "error".
	Level string
	// Format is "json" or "text".
	Format string
	// File is the optional rotated log file path. Empty disables file output.
	File string
	// MaxSizeMB is the rotation threshold in megabytes.
	MaxSizeMB int
	// MaxAgeDays is the retention window for rotated files.
	MaxAgeDays int
	// MaxBackups is the maximum count of retained rotated files.
	MaxBackups int
}

// SessionConfig holds session / token related defaults.
type SessionConfig struct {
	// TTL is the rolling lifetime of an API access token.
	TTL time.Duration
}

// AgentConfig holds defaults for agent management.
type AgentConfig struct {
	// HeartbeatInterval is the suggested cadence at which agents should ping.
	HeartbeatInterval time.Duration
}

// AuthRateConfig tunes the per-(IP|username) login rate limiter. The defaults
// (5 attempts/hour, burst 5) suit production; E2E/CI环境用环境变量放宽，
// 否则一轮测试就会打满令牌桶。
type AuthRateConfig struct {
	// LoginPerSecond is the token-bucket refill rate for login attempts.
	LoginPerSecond float64
	// LoginBurst is the bucket size (max attempts in a quick burst).
	LoginBurst int
}

// ErrInvalidConfig is returned when a required value cannot be parsed.
var ErrInvalidConfig = errors.New("invalid configuration")

// Load returns a Config populated from CLI args, environment, and defaults.
//
// If args is nil, os.Args[1:] is used. Passing a custom args slice makes the
// function trivially testable.
func Load(args []string) (Config, error) {
	if args == nil {
		args = os.Args[1:]
	}

	cfg := defaultConfig()

	fs := flag.NewFlagSet("shiguang-vps", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	fs.StringVar(&cfg.HTTP.Addr, "http-addr", cfg.HTTP.Addr, "HTTP listen address")
	fs.StringVar(&cfg.Database.DataDir, "data-dir", cfg.Database.DataDir, "Data directory for SQLite & assets")
	fs.StringVar(&cfg.Database.Filename, "db-filename", cfg.Database.Filename, "SQLite database file name")
	fs.StringVar(&cfg.Log.Level, "log-level", cfg.Log.Level, "Log level (debug/info/warn/error)")
	fs.StringVar(&cfg.Log.Format, "log-format", cfg.Log.Format, "Log format (json/text)")
	fs.StringVar(&cfg.Log.File, "log-file", cfg.Log.File, "Optional rotated log file path")
	fs.StringVar(&cfg.ResetPassword, "reset-password", "",
		"Recovery mode: reset this username's password (and disable TOTP), print it, then exit")

	applyEnv(&cfg)

	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// defaultConfig returns a Config populated only with compiled defaults.
func defaultConfig() Config {
	return Config{
		HTTP: HTTPConfig{Addr: DefaultHTTPAddr},
		Database: DatabaseConfig{
			DataDir:       DefaultDataDir,
			Filename:      DefaultDBFilename,
			BusyTimeoutMs: DefaultDBBusyTimeoutMs,
			MaxOpenWrite:  DefaultDBMaxOpenWrite,
			MaxOpenRead:   DefaultDBMaxOpenRead,
		},
		Log: LogConfig{
			Level:      DefaultLogLevel,
			Format:     DefaultLogFormat,
			MaxSizeMB:  DefaultLogMaxSizeMB,
			MaxAgeDays: DefaultLogMaxAgeDays,
			MaxBackups: DefaultLogMaxBackups,
		},
		Session: SessionConfig{TTL: DefaultSessionTTL},
		Agent:   AgentConfig{HeartbeatInterval: DefaultAgentHeartbeatInterval},
		AuthRate: AuthRateConfig{
			LoginPerSecond: DefaultLoginRatePerSecond,
			LoginBurst:     DefaultLoginRateBurst,
		},
	}
}

// applyEnv overlays environment variables on top of cfg. Unknown variables are
// silently ignored. Parse errors return early — callers usually surface them.
func applyEnv(cfg *Config) {
	if v := os.Getenv("SHIGUANG_HTTP_ADDR"); v != "" {
		cfg.HTTP.Addr = v
	}
	if v := os.Getenv("SHIGUANG_DATA_DIR"); v != "" {
		cfg.Database.DataDir = v
	}
	if v := os.Getenv("SHIGUANG_DB_FILENAME"); v != "" {
		cfg.Database.Filename = v
	}
	if v := os.Getenv("SHIGUANG_DB_BUSY_TIMEOUT_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Database.BusyTimeoutMs = n
		}
	}
	if v := os.Getenv("SHIGUANG_DB_MAX_OPEN_READ"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Database.MaxOpenRead = n
		}
	}
	if v := os.Getenv("SHIGUANG_LOG_LEVEL"); v != "" {
		cfg.Log.Level = strings.ToLower(v)
	}
	if v := os.Getenv("SHIGUANG_LOG_FORMAT"); v != "" {
		cfg.Log.Format = strings.ToLower(v)
	}
	if v := os.Getenv("SHIGUANG_LOG_FILE"); v != "" {
		cfg.Log.File = v
	}
	if v := os.Getenv("SHIGUANG_SESSION_TTL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Session.TTL = time.Duration(n) * time.Second
		}
	}
	if v := os.Getenv("SHIGUANG_AGENT_HEARTBEAT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Agent.HeartbeatInterval = time.Duration(n) * time.Second
		}
	}
	if v := os.Getenv("SHIGUANG_LOGIN_RATE_PER_SEC"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.AuthRate.LoginPerSecond = f
		}
	}
	if v := os.Getenv("SHIGUANG_LOGIN_RATE_BURST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.AuthRate.LoginBurst = n
		}
	}
}

// validate ensures required values are well-formed. Returns ErrInvalidConfig
// wrapped with details on failure.
func (c Config) validate() error {
	if c.HTTP.Addr == "" {
		return fmt.Errorf("%w: http addr is empty", ErrInvalidConfig)
	}
	if c.Database.DataDir == "" {
		return fmt.Errorf("%w: data dir is empty", ErrInvalidConfig)
	}
	if c.Database.Filename == "" {
		return fmt.Errorf("%w: db filename is empty", ErrInvalidConfig)
	}
	if c.Database.BusyTimeoutMs <= 0 {
		return fmt.Errorf("%w: db busy_timeout must be > 0", ErrInvalidConfig)
	}
	if c.Database.MaxOpenWrite != 1 {
		return fmt.Errorf("%w: db max_open_write must equal 1 (SQLite serialised writer)", ErrInvalidConfig)
	}
	if c.Database.MaxOpenRead <= 0 {
		return fmt.Errorf("%w: db max_open_read must be > 0", ErrInvalidConfig)
	}
	switch strings.ToLower(c.Log.Level) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("%w: log level %q is not one of debug/info/warn/error", ErrInvalidConfig, c.Log.Level)
	}
	switch strings.ToLower(c.Log.Format) {
	case "json", "text":
	default:
		return fmt.Errorf("%w: log format %q is not one of json/text", ErrInvalidConfig, c.Log.Format)
	}
	if c.Session.TTL <= 0 {
		return fmt.Errorf("%w: session ttl must be > 0", ErrInvalidConfig)
	}
	if c.Agent.HeartbeatInterval <= 0 {
		return fmt.Errorf("%w: agent heartbeat must be > 0", ErrInvalidConfig)
	}
	return nil
}
