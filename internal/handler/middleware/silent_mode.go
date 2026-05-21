package middleware

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SilentModeReloadInterval is the polling cadence the watcher uses to detect
// silent_mode_prefix changes in the system_settings table. The 30 s window is
// short enough to feel "live" to an admin clicking the rotate button and long
// enough that the readonly query keeps a negligible DB footprint.
const SilentModeReloadInterval = 30 * time.Second

// hexPrefixPattern enforces "exactly 32 lowercase hex characters" for the
// silent-mode entry path.
var hexPrefixPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// nginx404Body mirrors the byte-for-byte default 404 response served by
// nginx 1.18.x so an unauthorised visitor cannot fingerprint our hub by
// diffing the page against a real nginx install. The HTML is intentionally
// vanilla — adding an X-Powered-By header or a different Server label would
// defeat the purpose.
const nginx404Body = "<html>\r\n" +
	"<head><title>404 Not Found</title></head>\r\n" +
	"<body>\r\n" +
	"<center><h1>404 Not Found</h1></center>\r\n" +
	"<hr><center>nginx/1.18.0</center>\r\n" +
	"</body>\r\n" +
	"</html>\r\n"

// SilentModeConfig wires the middleware to its prefix source.
//
//   - Loader is invoked from a background goroutine every
//     SilentModeReloadInterval to refresh the prefix from system_settings.
//     A returned empty string means "silent mode disabled" — the middleware
//     becomes a no-op until the next reload sees a value.
//   - InitialPrefix is the value read at startup; it short-circuits the first
//     loader poll so the server is correctly configured before accepting
//     traffic. Pass "" when the value is unknown.
//   - Logger receives reload events (info on change, warn on loader errors).
//   - Now overrides time.Now for tests.
type SilentModeConfig struct {
	InitialPrefix string
	Loader        func(ctx context.Context) (string, error)
	Logger        *slog.Logger
	Now           func() time.Time
}

// SilentMode owns the live prefix value and exposes the middleware. Use
// NewSilentMode to construct.
type SilentMode struct {
	cfg    SilentModeConfig
	prefix atomic.Pointer[string]

	once       sync.Once
	stopCh     chan struct{}
	watcherCtx context.Context
	cancel     context.CancelFunc
}

// NewSilentMode constructs a SilentMode helper. The caller must invoke Start
// (typically right before http.ListenAndServe) and Stop on shutdown to
// reclaim the watcher goroutine.
func NewSilentMode(cfg SilentModeConfig) *SilentMode {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	sm := &SilentMode{cfg: cfg, stopCh: make(chan struct{})}
	initial := cfg.InitialPrefix
	sm.prefix.Store(&initial)
	return sm
}

// Prefix returns the current 32-hex prefix (empty when silent mode is off).
func (s *SilentMode) Prefix() string {
	if s == nil {
		return ""
	}
	if p := s.prefix.Load(); p != nil {
		return *p
	}
	return ""
}

// SetPrefix overrides the active prefix immediately (used by the admin
// rotate endpoint to avoid waiting for the next poll).
func (s *SilentMode) SetPrefix(value string) {
	if s == nil {
		return
	}
	v := strings.ToLower(strings.TrimSpace(value))
	s.prefix.Store(&v)
}

// Start launches the background watcher. Safe to call multiple times; only
// the first invocation has effect.
func (s *SilentMode) Start(ctx context.Context) {
	if s == nil || s.cfg.Loader == nil {
		return
	}
	s.once.Do(func() {
		s.watcherCtx, s.cancel = context.WithCancel(ctx)
		go s.watch()
	})
}

// Stop halts the watcher goroutine. Idempotent.
func (s *SilentMode) Stop() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
}

func (s *SilentMode) watch() {
	ticker := time.NewTicker(SilentModeReloadInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.watcherCtx.Done():
			return
		case <-ticker.C:
			s.refresh()
		}
	}
}

func (s *SilentMode) refresh() {
	if s.cfg.Loader == nil {
		return
	}
	value, err := s.cfg.Loader(s.watcherCtx)
	if err != nil {
		// Missing row is "silent mode disabled" — log at debug, not warn.
		if !errors.Is(err, sql.ErrNoRows) && s.cfg.Logger != nil {
			s.cfg.Logger.Warn("silent_mode: prefix reload failed",
				slog.String("err", err.Error()))
		}
		return
	}
	current := s.Prefix()
	if value != current {
		s.SetPrefix(value)
		if s.cfg.Logger != nil {
			masked := value
			if len(masked) > 8 {
				masked = masked[:4] + "..." + masked[len(masked)-4:]
			}
			s.cfg.Logger.Info("silent_mode: prefix updated",
				slog.String("prefix_masked", masked))
		}
	}
}

// Middleware returns the http middleware enforcing the prefix rule.
func (s *SilentMode) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			prefix := s.Prefix()
			if prefix == "" {
				// Silent mode disabled — everything passes through.
				next.ServeHTTP(w, r)
				return
			}
			path := r.URL.Path
			if isSilentWhitelisted(path) {
				next.ServeHTTP(w, r)
				return
			}
			if matchesSilentPrefix(path, prefix) {
				// Strip the /_app/<prefix> portion before forwarding so that
				// downstream handlers can register canonical paths like
				// "/api/auth/login" instead of duplicating the prefix.
				stripped := "/" + strings.TrimPrefix(path[len("/_app/")+len(prefix):], "/")
				if stripped == "" {
					stripped = "/"
				}
				r2 := r.Clone(r.Context())
				r2.URL.Path = stripped
				r2.URL.RawPath = ""
				next.ServeHTTP(w, r2)
				return
			}
			Mimic404(w)
		})
	}
}

// Mimic404 writes the nginx-clone 404 body so a probe cannot distinguish the
// shiguang-vps hub from a stock nginx install. Always emits status 404 and a
// `Server: nginx/1.18.0` header. Safe to call before any other write.
func Mimic404(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Server", "nginx/1.18.0")
	h.Set("Content-Type", "text/html")
	h.Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(nginx404Body))
}

// silentWhitelist enumerates the path prefixes (or exact matches) that are
// served regardless of the silent-mode prefix. Entries match exact-equal OR
// "starts with the entry + '/'" so /api/v1/nezha also covers
// /api/v1/nezha/report.
var silentWhitelist = []string{
	"/healthz",
	"/s",
	"/dl",
	"/download",
	"/install-agent.sh",
	"/api/v1/nezha",
	"/api/notify/telegram/webhook",
	"/api/agent/ws",
}

func isSilentWhitelisted(path string) bool {
	for _, p := range silentWhitelist {
		if path == p {
			return true
		}
		if strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

// matchesSilentPrefix returns true when path begins with /_app/<prefix>/ or
// equals /_app/<prefix> exactly.
func matchesSilentPrefix(path, prefix string) bool {
	if !hexPrefixPattern.MatchString(prefix) {
		return false
	}
	full := "/_app/" + prefix
	if path == full {
		return true
	}
	return strings.HasPrefix(path, full+"/")
}
