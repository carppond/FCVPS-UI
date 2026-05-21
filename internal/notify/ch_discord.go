package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// DiscordChannel posts to a webhook URL. Config keys (types.DiscordConfig):
//
//	webhook_url  string (required)
//	username     string (optional override)
//	avatar_url   string (optional)
type DiscordChannel struct {
	client HTTPClient
}

// NewDiscordChannel returns a channel using the default HTTP client.
func NewDiscordChannel() *DiscordChannel {
	return &DiscordChannel{client: defaultHTTPClient}
}

// Kind returns the type identifier.
func (c *DiscordChannel) Kind() string { return "discord" }

// SetHTTPClient overrides the HTTP client (used by tests).
func (c *DiscordChannel) SetHTTPClient(client HTTPClient) { c.client = client }

// Validate ensures the webhook URL is present and non-empty.
func (c *DiscordChannel) Validate(cfg any) error {
	m, ok := cfg.(map[string]any)
	if !ok {
		return errors.New("discord: config must be an object")
	}
	if asString(m, "webhook_url") == "" {
		return errors.New("discord: webhook_url required")
	}
	return nil
}

// Send posts a content message to the webhook URL.
func (c *DiscordChannel) Send(ctx context.Context, cfg any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	m := cfg.(map[string]any)
	url := asString(m, "webhook_url")
	content := msg.Subject
	if msg.Body != "" {
		content = msg.Subject + "\n" + msg.Body
	}
	if len(content) > 1900 {
		content = content[:1900] + "…"
	}
	body := map[string]any{"content": content}
	if u := asString(m, "username"); u != "" {
		body["username"] = u
	}
	if a := asString(m, "avatar_url"); a != "" {
		body["avatar_url"] = a
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if _, _, err := retryableHTTPPost(ctx, c.client, url, headers, payload, 3); err != nil {
		return fmt.Errorf("discord send: %w", err)
	}
	return nil
}

// buildDiscord is the registry factory.
func buildDiscord(cfg map[string]any) (Channel, error) {
	ch := NewDiscordChannel()
	if err := ch.Validate(cfg); err != nil {
		return nil, err
	}
	return ch, nil
}
