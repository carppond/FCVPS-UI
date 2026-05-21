package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TG_PARSE_MODE is the parse mode the bot uses for every outbound message.
// MarkdownV2 requires escaping of `_*[]()~`>#+-=|{}.!` so callers must use
// tgEscapeMarkdownV2 on any user / runtime-supplied text segment.
const TG_PARSE_MODE = "MarkdownV2"

// tgMaxMessageBytes mirrors Telegram's 4096-character cap on sendMessage. We
// truncate aggressively (with an ellipsis) so a misbehaving command handler
// never trips the API's hard limit.
const tgMaxMessageBytes = 3900

// TGUpdate is the subset of the Telegram Update object the bot consumes.
// Only the fields the bot actually inspects are decoded; unknown keys are
// silently ignored so future Telegram additions do not break parsing.
type TGUpdate struct {
	UpdateID      int64          `json:"update_id"`
	Message       *TGMessage     `json:"message,omitempty"`
	CallbackQuery *TGCallbackQ   `json:"callback_query,omitempty"`
}

// TGMessage carries the chat + sender + text triple every command handler
// needs. Entities is omitted — we treat the leading "/cmd" token as the
// command and the remainder as a single args string.
type TGMessage struct {
	MessageID int64  `json:"message_id"`
	Date      int64  `json:"date"`
	Text      string `json:"text"`
	Chat      TGChat `json:"chat"`
	From      *TGUser `json:"from,omitempty"`
}

// TGChat is the Telegram chat record (private / group / channel).
type TGChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// TGUser identifies the sender. We use ID for whitelist checks and Username
// (when present) only for diagnostic logging.
type TGUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
}

// TGCallbackQ is the inline-keyboard click envelope. Data is our colon-
// delimited routing tuple: "<event_type>:<resource_id>:<action>".
type TGCallbackQ struct {
	ID      string     `json:"id"`
	From    TGUser     `json:"from"`
	Message *TGMessage `json:"message,omitempty"`
	Data    string     `json:"data"`
}

// TGInlineKeyboard is the outbound inline keyboard markup. Each row is a
// slice of buttons; we use 1-3 buttons per row to keep the on-screen layout
// readable on mobile clients.
type TGInlineKeyboard struct {
	Buttons [][]TGInlineButton `json:"inline_keyboard"`
}

// TGInlineButton is one button in an inline keyboard. CallbackData is
// limited to 64 bytes by Telegram; the bot keeps tuples short.
type TGInlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

// TGSendMessageReq mirrors the Bot API sendMessage payload (subset).
type TGSendMessageReq struct {
	ChatID      int64             `json:"chat_id"`
	Text        string            `json:"text"`
	ParseMode   string            `json:"parse_mode,omitempty"`
	ReplyMarkup *TGInlineKeyboard `json:"reply_markup,omitempty"`
}

// CommandHandler is the per-command business logic invoked by the bot. It
// receives the raw args string (everything after the "/cmd " prefix) plus
// the originating chat for follow-up messages, and returns the reply text
// already formatted for MarkdownV2. The optional keyboard is rendered below
// the reply when non-nil.
type CommandHandler func(ctx context.Context, chat *TGChatContext, args string) (string, *TGInlineKeyboard, error)

// CallbackHandler executes a single inline-keyboard action. Returns the
// confirmation text shown as the answerCallbackQuery toast.
type CallbackHandler func(ctx context.Context, chat *TGChatContext, eventType, resourceID, action string) (string, error)

// TGChatContext bundles the chat + user the command runs as. UserID is the
// owning shiguang-vps user (resolved via the white-list), not the Telegram
// user ID.
type TGChatContext struct {
	ChatID   int64
	TGUserID int64
	UserID   string
	Locale   string
}

// CommandRouter owns the command/callback dispatch table. Registrations are
// made at construction time (RegisterCommand / RegisterCallback) and consumed
// by Bot.HandleUpdate. The router is goroutine-safe — concurrent webhook
// updates dispatch through it without external locking.
type CommandRouter struct {
	mu        sync.RWMutex
	commands  map[string]CommandHandler
	callbacks map[string]CallbackHandler // keyed by action
}

// NewCommandRouter returns an empty router.
func NewCommandRouter() *CommandRouter {
	return &CommandRouter{
		commands:  make(map[string]CommandHandler),
		callbacks: make(map[string]CallbackHandler),
	}
}

// RegisterCommand installs handler under name (without the leading "/").
// Re-registration overwrites — useful for test setup.
func (r *CommandRouter) RegisterCommand(name string, handler CommandHandler) {
	if r == nil || name == "" || handler == nil {
		return
	}
	r.mu.Lock()
	r.commands[strings.ToLower(name)] = handler
	r.mu.Unlock()
}

// RegisterCallback installs handler under action (the third segment of the
// "<event>:<resource>:<action>" callback_data triple).
func (r *CommandRouter) RegisterCallback(action string, handler CallbackHandler) {
	if r == nil || action == "" || handler == nil {
		return
	}
	r.mu.Lock()
	r.callbacks[strings.ToLower(action)] = handler
	r.mu.Unlock()
}

// Commands returns the list of registered command names sorted by Go's
// default map iteration order (used by the /help handler).
func (r *CommandRouter) Commands() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.commands))
	for name := range r.commands {
		out = append(out, name)
	}
	return out
}

// lookupCommand resolves a "/cmd" or "/cmd@botname" prefix to its handler.
func (r *CommandRouter) lookupCommand(name string) (CommandHandler, bool) {
	if r == nil {
		return nil, false
	}
	// Strip "@botname" suffix Telegram appends in group chats.
	if at := strings.IndexByte(name, '@'); at >= 0 {
		name = name[:at]
	}
	name = strings.ToLower(strings.TrimPrefix(name, "/"))
	r.mu.RLock()
	h, ok := r.commands[name]
	r.mu.RUnlock()
	return h, ok
}

// lookupCallback resolves the action portion to its handler.
func (r *CommandRouter) lookupCallback(action string) (CallbackHandler, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	h, ok := r.callbacks[strings.ToLower(action)]
	r.mu.RUnlock()
	return h, ok
}

// WhitelistResolver maps a Telegram chat ID to the shiguang-vps user that
// owns the bot. Implementations look up channels of kind=telegram and check
// whether chat_id matches any of the user's registered chats. Returning
// ("", "", false) signals "not authorised" and the bot silently drops the
// update (no reply — avoids leaking that the bot exists to random users).
type WhitelistResolver func(ctx context.Context, chatID int64) (userID, locale string, ok bool)

// BotConfig wires the bot to its collaborators.
type BotConfig struct {
	// BotToken is the raw Bot API token (123456:ABC...). When empty, Send
	// returns an error and webhook handling fails fast — useful for tests
	// that only exercise CommandRouter.
	BotToken string

	// APIBase is the Telegram API root. Defaults to TelegramAPIBase. Tests
	// override to point at an httptest server.
	APIBase string

	// Client is the HTTP client used for outbound sendMessage /
	// answerCallbackQuery calls. nil falls back to defaultHTTPClient.
	Client HTTPClient

	// Router holds the command + callback dispatch tables.
	Router *CommandRouter

	// Whitelist resolves chat_id → user_id. Required; nil disables every
	// update (defence-in-depth — fail closed).
	Whitelist WhitelistResolver
}

// Bot is the high-level façade the webhook handler invokes. One Bot per
// running hub; the underlying token is shared across every webhook URL the
// system mints (per-user webhook tokens authenticate the URL, not the bot).
type Bot struct {
	cfg BotConfig
}

// NewBot wires a bot. Returns an error when required fields are missing.
func NewBot(cfg BotConfig) (*Bot, error) {
	if cfg.Router == nil {
		return nil, errors.New("tg bot: router required")
	}
	if cfg.Whitelist == nil {
		return nil, errors.New("tg bot: whitelist resolver required")
	}
	if cfg.APIBase == "" {
		cfg.APIBase = TelegramAPIBase
	}
	if cfg.Client == nil {
		cfg.Client = defaultHTTPClient
	}
	return &Bot{cfg: cfg}, nil
}

// Router exposes the underlying command router so callers (handlers / tests)
// can register additional commands at runtime.
func (b *Bot) Router() *CommandRouter {
	if b == nil {
		return nil
	}
	return b.cfg.Router
}

// HandleUpdate dispatches one Telegram update. The function is idempotent
// against unknown payload shapes — it returns nil (the caller still 200s)
// rather than surfacing a parse failure to Telegram (which would cause it to
// retry indefinitely).
func (b *Bot) HandleUpdate(ctx context.Context, update *TGUpdate) error {
	if b == nil || update == nil {
		return nil
	}
	switch {
	case update.Message != nil:
		return b.handleMessage(ctx, update.Message)
	case update.CallbackQuery != nil:
		return b.handleCallback(ctx, update.CallbackQuery)
	}
	return nil
}

// handleMessage routes a /command. Non-command messages are ignored.
func (b *Bot) handleMessage(ctx context.Context, msg *TGMessage) error {
	if msg == nil || msg.Text == "" {
		return nil
	}
	text := strings.TrimSpace(msg.Text)
	if !strings.HasPrefix(text, "/") {
		return nil
	}
	userID, locale, ok := b.cfg.Whitelist(ctx, msg.Chat.ID)
	// /start is the bootstrap command: it binds chat_id → user, so the
	// whitelist may not yet contain the chat. We tolerate "ok=false" for
	// /start and let the command handler decide whether to accept the bind.
	tokenName, args := splitCommand(text)
	if !ok && !strings.EqualFold(strings.TrimPrefix(strings.SplitN(tokenName, "@", 2)[0], "/"), "start") {
		return nil // silent drop
	}
	handler, found := b.cfg.Router.lookupCommand(tokenName)
	if !found {
		return b.sendError(ctx, msg.Chat.ID, "Unknown command. Use /help.")
	}
	chatCtx := &TGChatContext{
		ChatID: msg.Chat.ID,
		UserID: userID,
		Locale: locale,
	}
	if msg.From != nil {
		chatCtx.TGUserID = msg.From.ID
	}
	reply, kb, err := handler(ctx, chatCtx, args)
	if err != nil {
		return b.sendError(ctx, msg.Chat.ID, fmt.Sprintf("Error: %s", err.Error()))
	}
	if reply == "" {
		return nil
	}
	return b.sendMessage(ctx, msg.Chat.ID, reply, kb)
}

// handleCallback routes an inline-keyboard click.
func (b *Bot) handleCallback(ctx context.Context, q *TGCallbackQ) error {
	if q == nil {
		return nil
	}
	var chatID int64
	if q.Message != nil {
		chatID = q.Message.Chat.ID
	}
	userID, locale, ok := b.cfg.Whitelist(ctx, chatID)
	if !ok {
		// Acknowledge the click silently so Telegram stops showing the
		// loading spinner on the user's button.
		return b.answerCallback(ctx, q.ID, "")
	}
	parts := strings.SplitN(q.Data, ":", 3)
	if len(parts) < 3 {
		return b.answerCallback(ctx, q.ID, "invalid callback")
	}
	eventType, resourceID, action := parts[0], parts[1], parts[2]
	handler, found := b.cfg.Router.lookupCallback(action)
	if !found {
		return b.answerCallback(ctx, q.ID, "unknown action")
	}
	chatCtx := &TGChatContext{
		ChatID:   chatID,
		TGUserID: q.From.ID,
		UserID:   userID,
		Locale:   locale,
	}
	confirm, err := handler(ctx, chatCtx, eventType, resourceID, action)
	if err != nil {
		_ = b.answerCallback(ctx, q.ID, "failed")
		return err
	}
	if confirm == "" {
		confirm = "ok"
	}
	return b.answerCallback(ctx, q.ID, confirm)
}

// sendError dispatches a one-line plain-text error reply. Used for parser
// errors / unknown commands so the user gets immediate feedback. We bypass
// MarkdownV2 to avoid an escape-loop on the error text itself.
func (b *Bot) sendError(ctx context.Context, chatID int64, text string) error {
	payload := TGSendMessageReq{
		ChatID: chatID,
		Text:   truncateForTG(text),
	}
	return b.postJSON(ctx, "sendMessage", payload)
}

// sendMessage delivers a MarkdownV2-formatted reply optionally followed by
// an inline keyboard. The text must already be escaped by the caller.
func (b *Bot) sendMessage(ctx context.Context, chatID int64, text string, keyboard *TGInlineKeyboard) error {
	payload := TGSendMessageReq{
		ChatID:      chatID,
		Text:        truncateForTG(text),
		ParseMode:   TG_PARSE_MODE,
		ReplyMarkup: keyboard,
	}
	return b.postJSON(ctx, "sendMessage", payload)
}

// answerCallback acknowledges an inline-keyboard click. text shows as a
// toast (≤ 200 chars).
func (b *Bot) answerCallback(ctx context.Context, callbackID, text string) error {
	if callbackID == "" {
		return nil
	}
	payload := map[string]any{
		"callback_query_id": callbackID,
	}
	if text != "" {
		payload["text"] = truncate(text, 190)
	}
	return b.postJSON(ctx, "answerCallbackQuery", payload)
}

// SendNotification dispatches a free-form alert (called by notify.Manager
// via a custom channel adapter — out of T-24 scope but the entry point is
// kept here so callers do not need to know the API shape).
//
// The keyboard, when supplied, is appended verbatim. Callers compose the
// "重试 / 详情 / 静音 24h" buttons via BuildAlertKeyboard.
func (b *Bot) SendNotification(ctx context.Context, chatID int64, text string, keyboard *TGInlineKeyboard) error {
	if b == nil {
		return errors.New("tg bot: nil")
	}
	return b.sendMessage(ctx, chatID, text, keyboard)
}

// SetWebhook installs the webhook URL on the Bot API side. Idempotent.
func (b *Bot) SetWebhook(ctx context.Context, url string) error {
	if b == nil {
		return errors.New("tg bot: nil")
	}
	if b.cfg.BotToken == "" {
		return errors.New("tg bot: empty bot token")
	}
	return b.postJSON(ctx, "setWebhook", map[string]any{
		"url":          url,
		"allowed_updates": []string{"message", "callback_query"},
	})
}

// postJSON marshals payload and POSTs to the Bot API. Errors from non-2xx
// responses include a truncated body to aid debugging.
func (b *Bot) postJSON(ctx context.Context, method string, payload any) error {
	if b.cfg.BotToken == "" {
		return errors.New("tg bot: empty bot token")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", method, err)
	}
	url := fmt.Sprintf("%s/bot%s/%s", b.cfg.APIBase, b.cfg.BotToken, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.cfg.Client.Do(req)
	if err != nil {
		return fmt.Errorf("tg post %s: %w", method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("tg %s status=%d body=%s", method, resp.StatusCode, truncate(string(respBody), 200))
}

// BuildAlertKeyboard composes the per-alert inline keyboard with the three
// standard actions called out in T-24: 重试 / 详情 / 静音 24h. CallbackData
// follows the "<event>:<resource>:<action>" convention.
func BuildAlertKeyboard(eventType, resourceID string) *TGInlineKeyboard {
	return &TGInlineKeyboard{
		Buttons: [][]TGInlineButton{
			{
				{Text: "🔁 重试", CallbackData: fmt.Sprintf("%s:%s:retry", eventType, resourceID)},
				{Text: "📋 详情", CallbackData: fmt.Sprintf("%s:%s:detail", eventType, resourceID)},
				{Text: "🔕 静音24h", CallbackData: fmt.Sprintf("%s:%s:mute24h", eventType, resourceID)},
			},
		},
	}
}

// splitCommand splits "/cmd args go here" into ("/cmd", "args go here"). The
// leading slash is preserved on the command so callers can keep their dispatch
// table keyed on the bare name.
func splitCommand(text string) (string, string) {
	text = strings.TrimSpace(text)
	if i := strings.IndexByte(text, ' '); i >= 0 {
		return text[:i], strings.TrimSpace(text[i+1:])
	}
	return text, ""
}

// tgEscapeMarkdownV2 escapes the punctuation Telegram's MarkdownV2 parser
// treats as syntax. Callers should run any runtime-supplied string (node
// names, error messages, …) through this before embedding into a reply.
//
// The escape set is from the Bot API docs:
//
//	_ * [ ] ( ) ~ ` > # + - = | { } . !
func tgEscapeMarkdownV2(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch r {
		case '_', '*', '[', ']', '(', ')', '~', '`', '>',
			'#', '+', '-', '=', '|', '{', '}', '.', '!', '\\':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// truncateForTG caps a message body at Telegram's per-message byte cap with
// a sentinel suffix so the user knows truncation happened.
func truncateForTG(s string) string {
	if len(s) <= tgMaxMessageBytes {
		return s
	}
	return s[:tgMaxMessageBytes] + "…"
}

// formatTGTime renders an epoch-millis timestamp in UTC RFC3339 — chosen
// because it is unambiguous across the 4 supported locales.
func formatTGTime(ms int64) string {
	if ms <= 0 {
		return "—"
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}
