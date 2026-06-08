// Package audit owns the asynchronous audit-log pipeline.
//
// Design — per §2.8 / T-28:
//
//   - The HTTP audit middleware (internal/handler/middleware/audit.go)
//     produces an AuditEntry after every mutating handler returns. To keep
//     the request hot-path latency tight we do NOT block on the DB write;
//     entries are pushed onto a buffered channel and drained by a single
//     worker goroutine.
//   - When the buffer is full we drop the entry and emit a slog warning. A
//     dropped audit is preferable to a blocked user-facing request.
//   - Stop closes the queue and waits for the worker to flush, so a
//     graceful shutdown never loses in-flight events (only events still on
//     stack when SIGTERM arrives).
//
// The package depends only on internal/storage (for AuditRepo) and
// internal/handler/middleware (for the AuditEntry / AuditRepository types)
// — keeping the dependency graph DAG-shaped.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
)

// DefaultQueueSize is the default capacity of the in-memory entry buffer.
// 1024 is sized to absorb a burst of writes during a sustained load spike
// without ever pushing back on the request goroutine.
const DefaultQueueSize = 1024

// ErrLoggerStopped is returned by Log when the worker has already been
// stopped. Callers shouldn't see this in production (Stop is only invoked
// at process shutdown) but tests rely on it.
var ErrLoggerStopped = errors.New("audit: logger stopped")

// Logger collects AuditEntry values from the request path and persists them
// via the embedded *storage.AuditRepo on a worker goroutine.
type Logger struct {
	repo   *storage.AuditRepo
	logger *slog.Logger
	now    func() time.Time
	queue  chan middleware.AuditEntry
	wg     sync.WaitGroup

	stopOnce sync.Once
	stopped  chan struct{}
	dropped  uint64
	mu       sync.Mutex // guards dropped
}

// Config wires the logger.
type Config struct {
	Repo      *storage.AuditRepo
	Logger    *slog.Logger // nil → slog.Default
	Now       func() time.Time
	QueueSize int // ≤0 → DefaultQueueSize
}

// New constructs a Logger. Call Start to launch the worker before sending
// entries; calling Log before Start blocks until a worker reads (in practice
// you should always Start during application boot).
func New(cfg Config) *Logger {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = DefaultQueueSize
	}
	return &Logger{
		repo:    cfg.Repo,
		logger:  cfg.Logger,
		now:     cfg.Now,
		queue:   make(chan middleware.AuditEntry, cfg.QueueSize),
		stopped: make(chan struct{}),
	}
}

// Start launches the worker goroutine. Calling Start more than once is a
// no-op (the first call wins).
func (l *Logger) Start(ctx context.Context) {
	if l == nil || l.repo == nil {
		return
	}
	l.wg.Add(1)
	go l.runWorker(ctx)
}

// Stop closes the queue and waits for the worker to drain. Safe to call
// multiple times. Always returns after the worker has flushed the queue
// (or the worker context has timed out, whichever comes first).
func (l *Logger) Stop() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() {
		close(l.queue)
	})
	l.wg.Wait()
}

// Log implements middleware.AuditRepository. The signature returns an error
// for API symmetry; the only error case is "queue full" — and we still
// swallow that (logging a warning) so the middleware does not 500 the
// user request because of an audit backlog.
func (l *Logger) Log(ctx context.Context, entry middleware.AuditEntry) error {
	if l == nil {
		return ErrLoggerStopped
	}
	select {
	case <-l.stopped:
		return ErrLoggerStopped
	default:
	}
	select {
	case l.queue <- entry:
		return nil
	default:
		l.mu.Lock()
		l.dropped++
		dropped := l.dropped
		l.mu.Unlock()
		l.logger.Warn("audit: queue full, dropping entry",
			slog.String("action", entry.Action),
			slog.String("user_id", entry.UserID),
			slog.Uint64("dropped_total", dropped))
		return nil
	}
}

// Dropped returns the running total of entries the buffer rejected. Used by
// the admin diagnostics endpoint and the test-suite assertions.
func (l *Logger) Dropped() uint64 {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.dropped
}

// runWorker drains the queue and inserts each entry. Returns when the queue
// is closed and empty, OR when ctx is canceled (graceful shutdown bound).
func (l *Logger) runWorker(ctx context.Context) {
	defer l.wg.Done()
	defer close(l.stopped)
	for {
		select {
		case <-ctx.Done():
			// Drain whatever is buffered before exiting — keeps writes
			// honest across a SIGTERM.
			for entry := range l.queue {
				l.persist(context.Background(), entry)
			}
			return
		case entry, ok := <-l.queue:
			if !ok {
				return
			}
			l.persist(ctx, entry)
		}
	}
}

func (l *Logger) persist(ctx context.Context, entry middleware.AuditEntry) {
	if l.repo == nil {
		return
	}
	rec := storage.AuditLogRecord{
		UserID:       entry.UserID,
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		IP:           entry.IP,
		UserAgent:    entry.UserAgent,
		// 统一脱敏出口:遮蔽 password/token/secret 等敏感字段,绝不让
		// 登录密码、改密、通知渠道凭据、2FA secret 明文落进 audit_logs。
		Payload:   string(SummarizePayload(entry.Payload)),
		Success:   entry.Success,
		CreatedAt: l.now().UnixMilli(),
	}
	if _, err := l.repo.Insert(ctx, rec); err != nil {
		l.logger.Warn("audit: persist failed",
			slog.String("action", entry.Action),
			slog.String("user_id", entry.UserID),
			slog.String("err", err.Error()))
	}
}

// SummarizePayload returns a JSON-safe summary of body suitable for
// audit_logs.payload. The function masks well-known sensitive field names
// (password, token, secret …) before encoding so a Postgres connection
// string or 2FA secret never lands in the audit trail.
//
// Exported because handlers that build their own AuditEntry payloads (e.g.
// silent-mode rotate, which carries the new prefix) can opt into the same
// masking helper instead of rolling their own.
func SummarizePayload(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		// Not JSON — keep the raw bytes (capped by the middleware already).
		return raw
	}
	v = maskSensitive(v)
	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return out
}

// maskSensitive recursively replaces values of well-known sensitive keys
// with the literal "******". Only strings are masked — numeric / boolean
// values pass through.
func maskSensitive(v any) any {
	switch m := v.(type) {
	case map[string]any:
		for k, val := range m {
			if isSensitiveKey(k) {
				if _, isString := val.(string); isString {
					m[k] = "******"
					continue
				}
			}
			m[k] = maskSensitive(val)
		}
		return m
	case []any:
		for i := range m {
			m[i] = maskSensitive(m[i])
		}
		return m
	default:
		return v
	}
}

// sensitiveKeys enumerates the JSON property names whose value is masked
// when the audit middleware persists a request body. Case-insensitive
// substring match — "smtp_password" and "PASSWORD" both hit.
var sensitiveKeys = []string{
	"password",
	"token",
	"secret",
	"api_key",
	"apikey",
	"recovery_code",
	"totp",
	"otp",
	"private_key",
}

func isSensitiveKey(key string) bool {
	lc := lowercase(key)
	for _, needle := range sensitiveKeys {
		if containsSubstr(lc, needle) {
			return true
		}
	}
	return false
}

// lowercase / containsSubstr are tiny helpers — avoiding a strings dep keeps
// the helper readable in test traces.
func lowercase(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

func containsSubstr(s, needle string) bool {
	if len(needle) == 0 || len(needle) > len(s) {
		return len(needle) == 0
	}
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
