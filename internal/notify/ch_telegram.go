package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// TelegramAPIBase is the API root used by the channel; tests override this
// via TelegramChannel.SetAPIBase.
const TelegramAPIBase = "https://api.telegram.org"

// TelegramChannel sends a sendMessage request via the Bot API. Config map
// keys mirror types.TelegramConfig:
//
//	bot_token  string (required)
//	chat_id    string (required)
//	parse_mode string ("HTML" | "Markdown" | omitted)
//
// Retry policy: 3 attempts on 429 / 5xx; 1s/2s/4s exponential.
type TelegramChannel struct {
	client  HTTPClient
	apiBase string
}

// NewTelegramChannel returns a channel using the default HTTP client + API
// base. Tests use the package-level buildTelegram factory to inject mocks
// indirectly via the registry.
func NewTelegramChannel() *TelegramChannel {
	return &TelegramChannel{client: defaultHTTPClient, apiBase: TelegramAPIBase}
}

// Kind returns the type identifier.
func (c *TelegramChannel) Kind() string { return "telegram" }

// SetAPIBase overrides the API root. Used by tests to point at httptest.
func (c *TelegramChannel) SetAPIBase(base string) { c.apiBase = base }

// SetHTTPClient swaps the HTTP client (used by tests).
func (c *TelegramChannel) SetHTTPClient(client HTTPClient) { c.client = client }

// Validate ensures the required config fields are populated.
func (c *TelegramChannel) Validate(cfg any) error {
	m, ok := cfg.(map[string]any)
	if !ok {
		return errors.New("telegram: config must be an object")
	}
	if asString(m, "bot_token") == "" {
		return errors.New("telegram: bot_token required")
	}
	if asString(m, "chat_id") == "" {
		return errors.New("telegram: chat_id required")
	}
	return nil
}

// Send issues sendMessage. Parse mode is set when the config supplies it.
func (c *TelegramChannel) Send(ctx context.Context, cfg any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	m := cfg.(map[string]any)
	token := asString(m, "bot_token")
	url := fmt.Sprintf("%s/bot%s/sendMessage", c.apiBase, token)
	text := msg.Subject
	if msg.Body != "" {
		text = msg.Subject + "\n\n" + msg.Body
	}
	body := map[string]any{
		"chat_id": asString(m, "chat_id"),
		"text":    text,
	}
	if pm := asString(m, "parse_mode"); pm != "" {
		body["parse_mode"] = pm
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	headers := map[string]string{"Content-Type": "application/json"}
	_, _, err = retryableHTTPPost(ctx, c.client, url, headers, payload, 3)
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	return nil
}

// buildTelegram is the registry factory.
func buildTelegram(cfg map[string]any) (Channel, error) {
	ch := NewTelegramChannel()
	if err := ch.Validate(cfg); err != nil {
		return nil, err
	}
	return ch, nil
}

// asString returns the string at m[key], or "" when missing / wrong type.
func asString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
