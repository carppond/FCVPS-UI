package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// SlackChannel posts to a Slack incoming webhook. Config keys
// (types.SlackConfig):
//
//	webhook_url string (required)
//	channel     string (optional override, e.g. "#alerts")
//	username    string (optional)
//	icon_emoji  string (optional, e.g. ":bell:")
type SlackChannel struct {
	client HTTPClient
}

// NewSlackChannel returns a channel using the default HTTP client.
func NewSlackChannel() *SlackChannel {
	return &SlackChannel{client: defaultHTTPClient}
}

// Kind returns the type identifier.
func (c *SlackChannel) Kind() string { return "slack" }

// SetHTTPClient overrides the HTTP client (used by tests).
func (c *SlackChannel) SetHTTPClient(client HTTPClient) { c.client = client }

// Validate ensures the webhook URL is populated.
func (c *SlackChannel) Validate(cfg any) error {
	m, ok := cfg.(map[string]any)
	if !ok {
		return errors.New("slack: config must be an object")
	}
	if asString(m, "webhook_url") == "" {
		return errors.New("slack: webhook_url required")
	}
	return nil
}

// Send posts the message to Slack's incoming webhook.
func (c *SlackChannel) Send(ctx context.Context, cfg any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	m := cfg.(map[string]any)
	url := asString(m, "webhook_url")
	text := msg.Subject
	if msg.Body != "" {
		text = "*" + msg.Subject + "*\n" + msg.Body
	}
	body := map[string]any{"text": text}
	if ch := asString(m, "channel"); ch != "" {
		body["channel"] = ch
	}
	if u := asString(m, "username"); u != "" {
		body["username"] = u
	}
	if e := asString(m, "icon_emoji"); e != "" {
		body["icon_emoji"] = e
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if _, _, err := retryableHTTPPost(ctx, c.client, url, headers, payload, 3); err != nil {
		return fmt.Errorf("slack send: %w", err)
	}
	return nil
}

// buildSlack is the registry factory.
func buildSlack(cfg map[string]any) (Channel, error) {
	ch := NewSlackChannel()
	if err := ch.Validate(cfg); err != nil {
		return nil, err
	}
	return ch, nil
}
