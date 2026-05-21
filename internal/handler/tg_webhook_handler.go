package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"shiguang-vps/internal/agent"
	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// TGWebhookHandler hosts POST /api/notify/telegram/webhook/{token}.
//
// The handler is intentionally lightweight: every authentication / routing
// decision lives in notify.Bot. The HTTP layer only:
//
//  1. Validates the URL token against the stored system_settings entry.
//  2. Decodes the JSON body into a notify.TGUpdate.
//  3. Invokes Bot.HandleUpdate (with a 30 s timeout).
//
// Errors are swallowed (200 OK is always returned) — Telegram retries every
// non-2xx update for up to 24 h, which would spam the bot owner.
type TGWebhookHandler struct {
	bot      *notify.Bot
	settings *storage.SettingsRepo
	logger   *slog.Logger

	mu          sync.RWMutex
	cachedToken string
}

// NewTGWebhookHandler wires the handler. settings is consulted on every
// request so token rotations propagate without restart; the small read-lock
// keeps the lookup off the hot path for the common case.
func NewTGWebhookHandler(bot *notify.Bot, settings *storage.SettingsRepo, logger *slog.Logger) *TGWebhookHandler {
	return &TGWebhookHandler{bot: bot, settings: settings, logger: logger}
}

// Webhook implements POST /api/notify/telegram/webhook/{token}.
func (h *TGWebhookHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h == nil || h.bot == nil || h.settings == nil {
		// 404 — same shape as silent-mode mismatch (nginx 404).
		middleware.Mimic404(w)
		return
	}
	pathToken := strings.TrimSpace(r.PathValue("token"))
	if pathToken == "" {
		middleware.Mimic404(w)
		return
	}
	stored, err := h.lookupWebhookToken(r.Context())
	if err != nil || stored == "" || stored != pathToken {
		// Wrong / missing token. Mimic an nginx 404 so the URL space looks
		// uninteresting to scanners.
		middleware.Mimic404(w)
		return
	}
	var update notify.TGUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		// Telegram only POSTs JSON; a parse failure means a probe. 200 the
		// reply to stop a retry loop and log for audit.
		if h.logger != nil {
			h.logger.Debug("tg webhook bad body",
				slog.String("err", err.Error()),
				slog.String("trace_id", traceID))
		}
		util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := h.bot.HandleUpdate(ctx, &update); err != nil && h.logger != nil {
		h.logger.Warn("tg webhook handle",
			slog.String("err", err.Error()),
			slog.String("trace_id", traceID))
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// lookupWebhookToken reads the active token from system_settings. We cache
// the value across requests so the read pool is not hit for every update.
// Cache invalidation is on-demand: SetWebhookToken updates both store +
// cache atomically.
func (h *TGWebhookHandler) lookupWebhookToken(ctx context.Context) (string, error) {
	h.mu.RLock()
	tok := h.cachedToken
	h.mu.RUnlock()
	if tok != "" {
		return tok, nil
	}
	value, err := h.settings.Get(ctx, SettingTelegramWebhookToken)
	if err != nil {
		if errors.Is(err, storage.ErrSettingNotFound) {
			return "", nil
		}
		return "", err
	}
	h.mu.Lock()
	h.cachedToken = value
	h.mu.Unlock()
	return value, nil
}

// InvalidateTokenCache drops the cached token so the next request reloads
// from system_settings. Called after a rotation.
func (h *TGWebhookHandler) InvalidateTokenCache() {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.cachedToken = ""
	h.mu.Unlock()
}

// SettingTelegramWebhookToken is the system_settings key that stores the
// per-deployment webhook token. We keep it in settings (not env / config
// file) so admins can rotate without restart.
const SettingTelegramWebhookToken = "telegram_webhook_token"

// TGBotSettingsHandler hosts /api/notify/telegram/* admin endpoints used by
// the web UI to configure the bot. The webhook itself is unauthenticated
// (token in URL); these endpoints are authenticated + admin-gated upstream.
type TGBotSettingsHandler struct {
	settings *storage.SettingsRepo
	channels *storage.NotificationChannelRepo
	bot      *notify.Bot
	logger   *slog.Logger
}

// NewTGBotSettingsHandler wires the handler. All collaborators are required
// — caller (router) guards on nil-ness.
func NewTGBotSettingsHandler(settings *storage.SettingsRepo, channels *storage.NotificationChannelRepo, bot *notify.Bot, logger *slog.Logger) *TGBotSettingsHandler {
	return &TGBotSettingsHandler{settings: settings, channels: channels, bot: bot, logger: logger}
}

// Status implements GET /api/notify/telegram/status. Returns the cached
// webhook token (so the UI can render the public URL) plus the chat-IDs
// already bound across the caller's channels.
func (h *TGBotSettingsHandler) Status(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	token, _ := h.settings.Get(r.Context(), SettingTelegramWebhookToken)
	chans, _, err := h.channels.List(r.Context(), user.ID, storage.NotificationChannelListOptions{
		PageSize: 200,
		Kind:     "telegram",
	})
	if err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "list channels", nil, traceID)
		return
	}
	bindings := make([]tgBindingDTO, 0, len(chans))
	for _, c := range chans {
		bindings = append(bindings, tgBindingDTO{
			ChannelID:   c.ID,
			ChannelName: c.Name,
			ChatIDs:     extractChatIDs(c.ConfigJSON),
			Enabled:     c.Enabled,
		})
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[tgStatusDTO]{
		Data: tgStatusDTO{
			WebhookToken: token,
			Bindings:     bindings,
		},
		RequestID: traceID,
	})
}

// RotateWebhookToken implements POST /api/notify/telegram/webhook/rotate.
// Returns the new token; admins must then update Telegram's setWebhook URL
// (see SetWebhook below).
func (h *TGBotSettingsHandler) RotateWebhookToken(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	tok := util.RandomHex32()
	if err := h.settings.Set(r.Context(), SettingTelegramWebhookToken, tok); err != nil {
		util.RespondError(w, types.ErrInternalDatabase, "persist token", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[tgRotateDTO]{
		Data:      tgRotateDTO{WebhookToken: tok},
		RequestID: traceID,
	})
}

// SetWebhook implements POST /api/notify/telegram/webhook/install. Calls
// the Telegram setWebhook API with the public URL the admin supplied.
func (h *TGBotSettingsHandler) SetWebhook(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	var req tgInstallWebhookReq
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.URL == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "url required", nil, traceID)
		return
	}
	if h.bot == nil {
		util.RespondError(w, types.ErrInternalUnknown, "bot unavailable", nil, traceID)
		return
	}
	if err := h.bot.SetWebhook(r.Context(), req.URL); err != nil {
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// tgStatusDTO mirrors the GET /api/notify/telegram/status response.
type tgStatusDTO struct {
	WebhookToken string         `json:"webhook_token"`
	Bindings     []tgBindingDTO `json:"bindings"`
}

// tgBindingDTO is one entry in the chat-ID bindings list.
type tgBindingDTO struct {
	ChannelID   string   `json:"channel_id"`
	ChannelName string   `json:"channel_name"`
	ChatIDs     []string `json:"chat_ids"`
	Enabled     bool     `json:"enabled"`
}

// tgRotateDTO is the body of the rotate response.
type tgRotateDTO struct {
	WebhookToken string `json:"webhook_token"`
}

// tgInstallWebhookReq is the body of the install request.
type tgInstallWebhookReq struct {
	URL string `json:"url"`
}

// extractChatIDs reads the chat_ids array (or single chat_id) from a stored
// channel config. Used by Status to surface the registered chats.
func extractChatIDs(raw string) []string {
	if raw == "" {
		return nil
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil
	}
	if raw, ok := cfg["chat_ids"]; ok {
		switch v := raw.(type) {
		case []any:
			out := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" {
					out = append(out, s)
				}
			}
			return out
		}
	}
	if raw, ok := cfg["chat_id"]; ok {
		if s, ok := raw.(string); ok && s != "" {
			return []string{s}
		}
	}
	return nil
}

// BuildTGWhitelistResolver returns a WhitelistResolver that walks every
// telegram channel in the database and resolves the inbound chat_id to its
// owning user. The implementation does a fresh scan per call — channel
// counts per user are small (≤ 10) and the bot's update rate is modest, so
// caching is not yet required. If profiling shows the scan as hot, add an
// in-memory cache keyed on settings.UpdatedAt.
func BuildTGWhitelistResolver(repo *storage.NotificationChannelRepo, users *storage.UserRepo) notify.WhitelistResolver {
	return func(ctx context.Context, chatID int64) (string, string, bool) {
		if repo == nil {
			return "", "", false
		}
		recs, err := repo.ListAllByKind(ctx, "telegram")
		if err != nil {
			return "", "", false
		}
		userID, ok := scanChatID(recs, chatID)
		if !ok {
			return "", "", false
		}
		locale := "zh-CN"
		if users != nil {
			if u, err := users.GetByID(ctx, userID); err == nil && u != nil && u.Locale != "" {
				locale = u.Locale
			}
		}
		return userID, locale, true
	}
}

// scanChatID matches a chatID against the chat-IDs in each record's config.
func scanChatID(recs []storage.NotificationChannelRecord, chatID int64) (string, bool) {
	target := strconv.FormatInt(chatID, 10)
	for _, r := range recs {
		for _, c := range extractChatIDs(r.ConfigJSON) {
			if c == target {
				return r.UserID, true
			}
		}
	}
	return "", false
}

// BuildAlertKeyboardForEvent is a small wrapper so handlers can compose the
// notification keyboard without importing notify.* directly. Mirrors the
// signature documented in the task spec (T-24 §A.1.3).
func BuildAlertKeyboardForEvent(eventType, resourceID string) *notify.TGInlineKeyboard {
	return notify.BuildAlertKeyboard(eventType, resourceID)
}

// AdminCheckFromUserRepo returns an AdminCheck function backed by the user
// repo. nil-safe: a nil repo yields a check that always returns false (so
// /agent_restart / /silent are unavailable in misconfigured deployments).
func AdminCheckFromUserRepo(repo *storage.UserRepo) func(ctx context.Context, userID string) bool {
	if repo == nil {
		return func(context.Context, string) bool { return false }
	}
	return func(ctx context.Context, userID string) bool {
		if userID == "" {
			return false
		}
		u, err := repo.GetByID(ctx, userID)
		if err != nil || u == nil {
			return false
		}
		return u.Role == "admin"
	}
}

// SubscriptionSyncerAdapter adapts a substore.SyncService (interface
// satisfying SyncOne) to the bot's TGSubSyncer. Kept as a type alias here
// for the wire-up code in cmd/server to avoid an import cycle (the bot
// package cannot reach substore directly without circular deps).
type SubscriptionSyncerAdapter struct {
	SyncOneFunc func(ctx context.Context, sub *storage.SubscriptionRecord) (any, error)
}

// SyncOne forwards to the wrapped func.
func (a *SubscriptionSyncerAdapter) SyncOne(ctx context.Context, sub *storage.SubscriptionRecord) (any, error) {
	if a == nil || a.SyncOneFunc == nil {
		return nil, fmt.Errorf("subscription syncer unavailable")
	}
	return a.SyncOneFunc(ctx, sub)
}

// AgentHubAdapter is a thin wrapper that adapts *agent.Hub to the bot's
// TGAgentHub interface without forcing the notify package to depend on
// internal/agent (which would create a cycle once T-31 wires the dashboard
// into notify).
type AgentHubAdapter struct {
	Hub *agent.Hub
}

// IsOnline forwards to the underlying hub.
func (a *AgentHubAdapter) IsOnline(agentID string) bool {
	if a == nil || a.Hub == nil {
		return false
	}
	return a.Hub.IsOnline(agentID)
}

// SendCommand forwards to the underlying hub.
func (a *AgentHubAdapter) SendCommand(ctx context.Context, agentID, cmdID string, payload agent.CmdPayload) error {
	if a == nil || a.Hub == nil {
		return fmt.Errorf("agent hub unavailable")
	}
	return a.Hub.SendCommand(ctx, agentID, cmdID, payload)
}
