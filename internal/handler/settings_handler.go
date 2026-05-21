package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/ops"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// SettingsHandler hosts /api/admin/settings* and the silent-mode rotate
// endpoint. All routes are admin-only; the router wires RequireAdmin around
// the handler methods.
type SettingsHandler struct {
	repo    *storage.SettingsRepo
	silent  *ops.SilentMode
	logger  *slog.Logger
	baseURL string // optional; used to compose the "copy login URL" hint
}

// SettingsHandlerConfig wires the handler. baseURL may be empty — when set it
// is prepended to the silent-mode prefix so the response contains a fully
// qualified login URL the admin can paste into a password manager.
type SettingsHandlerConfig struct {
	Repo    *storage.SettingsRepo
	Silent  *ops.SilentMode
	Logger  *slog.Logger
	BaseURL string
}

// NewSettingsHandler returns a handler ready to be wired into the router.
func NewSettingsHandler(cfg SettingsHandlerConfig) *SettingsHandler {
	return &SettingsHandler{
		repo:    cfg.Repo,
		silent:  cfg.Silent,
		logger:  cfg.Logger,
		baseURL: cfg.BaseURL,
	}
}

// Get implements GET /api/admin/settings. Returns the full k/v map, with
// sensitive keys (smtp_password, telegram_bot_token …) masked to "******" so
// the response is safe to screenshot.
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.repo == nil {
		util.RespondError(w, types.ErrInternalUnknown, "settings repo unavailable", nil, traceID)
		return
	}
	values, err := h.repo.GetAll(r.Context())
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, err.Error(), nil, traceID)
		return
	}
	for k := range values {
		if _, sensitive := storage.SensitiveSettingKeys[k]; sensitive && values[k] != "" {
			values[k] = storage.SettingsMask
		}
	}
	// Hide the raw silent-mode prefix to avoid shoulder-surfing — admins
	// fetch the full value through the dedicated rotate response.
	if v, ok := values[storage.SettingSilentModePrefix]; ok && len(v) > 8 {
		values[storage.SettingSilentModePrefix] = v[:4] + "..." + v[len(v)-4:]
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[map[string]string]{
		Data: values, RequestID: traceID,
	})
}

// Update implements PUT/PATCH /api/admin/settings. The request body is a
// map[string]string; any key that maps to "******" is treated as "leave the
// existing value alone" so the UI can round-trip the GET response without
// asking the admin to re-enter every secret.
func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.repo == nil {
		util.RespondError(w, types.ErrInternalUnknown, "settings repo unavailable", nil, traceID)
		return
	}
	var req map[string]string
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if len(req) == 0 {
		util.RespondJSON(w, http.StatusOK, types.APIResponse[map[string]string]{
			Data: map[string]string{}, RequestID: traceID,
		})
		return
	}
	// Drop masked entries — they signal "no change". Reject the raw prefix
	// key so admins cannot bypass the rotate endpoint (which is the only
	// path that purges sessions).
	delete(req, storage.SettingSilentModePrefix)
	for k, v := range req {
		if v == storage.SettingsMask {
			delete(req, k)
		}
	}
	// Validate the known numeric keys so the DB does not end up with garbage
	// like "abc" stored under session_ttl_seconds.
	if err := validateSettings(req); err != nil {
		util.RespondError(w, types.ErrValidationOutOfRange, err.Error(), nil, traceID)
		return
	}
	if err := h.repo.SetMany(r.Context(), req); err != nil {
		util.RespondError(w, types.ErrInternalDatabase, err.Error(), nil, traceID)
		return
	}
	if h.logger != nil {
		keys := make([]string, 0, len(req))
		for k := range req {
			keys = append(keys, k)
		}
		h.logger.Info("settings: updated",
			slog.Int("count", len(req)),
			slog.Any("keys", keys),
			slog.String("trace_id", traceID))
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[map[string]string]{
		Data: req, RequestID: traceID,
	})
}

// RotateSilent implements POST /api/admin/silent-mode/rotate. Triggers the
// ops.SilentMode.Rotate flow and returns the freshly generated 32-hex prefix
// + a paste-ready login URL. The URL is the ONLY place the cleartext prefix
// is surfaced to the client; subsequent GET /settings returns it masked.
func (h *SettingsHandler) RotateSilent(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.silent == nil {
		util.RespondError(w, types.ErrInternalUnknown, "silent mode controller unavailable", nil, traceID)
		return
	}
	newPrefix, err := h.silent.Rotate(r.Context())
	if err != nil {
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}
	resp := types.SilentModeResponse{
		Enabled:  true,
		Prefix:   newPrefix,
		LoginURL: h.composeLoginURL(r, newPrefix),
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.SilentModeResponse]{
		Data: resp, RequestID: traceID,
	})
}

// composeLoginURL builds "<scheme>://<host>/_app/<prefix>/login". When the
// admin configured a base URL we use that verbatim; otherwise we derive from
// the request which keeps reverse-proxy-fronted deployments working.
func (h *SettingsHandler) composeLoginURL(r *http.Request, prefix string) string {
	if h.baseURL != "" {
		return trimSlash(h.baseURL) + "/_app/" + prefix + "/login"
	}
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host + "/_app/" + prefix + "/login"
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// validateSettings enforces the range / format rules for the known numeric
// settings. Unknown keys are accepted as-is (forward-compat with future
// admin-only config knobs added without a code change).
func validateSettings(values map[string]string) error {
	numeric := []struct {
		key     string
		min, max int64
	}{
		{storage.SettingSessionTTLSeconds, 60, 30 * 24 * 60 * 60},
		{storage.SettingMonthlyResetDay, 1, 28},
		{storage.SettingOTACheckInterval, 60, 30 * 24 * 60 * 60},
		{storage.SettingAgentHeartbeatInterval, 5, 300},
		{storage.SettingNotificationDebounce, 0, 24 * 60 * 60},
	}
	for _, n := range numeric {
		v, ok := values[n.key]
		if !ok || v == "" {
			continue
		}
		num, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return parseError(n.key, err)
		}
		if num < n.min || num > n.max {
			return rangeError(n.key, num, n.min, n.max)
		}
	}
	if v, ok := values[storage.SettingSilentModeEnabled]; ok && v != "" {
		if v != "true" && v != "false" {
			return errors.New("silent_mode_enabled must be 'true' or 'false'")
		}
	}
	return nil
}

func parseError(key string, err error) error {
	return errors.New(key + ": " + err.Error())
}

func rangeError(key string, val, lo, hi int64) error {
	return errors.New(key + ": value " + strconv.FormatInt(val, 10) +
		" not in [" + strconv.FormatInt(lo, 10) + "," + strconv.FormatInt(hi, 10) + "]")
}

// Used so the ops package import doesn't become unused if RotateSilent is
// trimmed out at link time (paranoia; keeps the linter quiet in stripped
// builds).
var _ = context.TODO
