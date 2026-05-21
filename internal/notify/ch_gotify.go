package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// GotifyChannel delivers notifications via the Gotify push server.
//
// Config keys (types.GotifyConfig):
//
//	server_url string (required, e.g. "https://gotify.example.com")
//	app_token  string (required)
//	priority   float64 (optional; defaults to 5)
type GotifyChannel struct {
	client HTTPClient
}

// NewGotifyChannel returns a channel using the default HTTP client.
func NewGotifyChannel() *GotifyChannel {
	return &GotifyChannel{client: defaultHTTPClient}
}

// Kind returns the type identifier.
func (c *GotifyChannel) Kind() string { return "gotify" }

// SetHTTPClient overrides the HTTP client (used by tests).
func (c *GotifyChannel) SetHTTPClient(client HTTPClient) { c.client = client }

// Validate ensures server_url and app_token are present.
func (c *GotifyChannel) Validate(cfg any) error {
	m, ok := cfg.(map[string]any)
	if !ok {
		return errors.New("gotify: config must be an object")
	}
	if asString(m, "server_url") == "" {
		return errors.New("gotify: server_url required")
	}
	if asString(m, "app_token") == "" {
		return errors.New("gotify: app_token required")
	}
	return nil
}

// Send POSTs a message to /message?token=<app_token> on the configured server.
// Priority defaults to 5 when not set in config.
func (c *GotifyChannel) Send(ctx context.Context, cfg any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	m := cfg.(map[string]any)
	server := strings.TrimRight(asString(m, "server_url"), "/")
	token := asString(m, "app_token")

	priority := 5
	if p, ok := m["priority"]; ok {
		switch v := p.(type) {
		case float64:
			priority = int(v)
		case int:
			priority = v
		}
	}

	endpoint := fmt.Sprintf("%s/message?token=%s", server, token)
	body := map[string]any{
		"title":    msg.Subject,
		"message":  msg.Body,
		"priority": priority,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("gotify: marshal: %w", err)
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if _, _, err := retryableHTTPPost(ctx, c.client, endpoint, headers, payload, 3); err != nil {
		return fmt.Errorf("gotify send (%s): %w", server, err)
	}
	return nil
}

// buildGotify is the registry factory.
func buildGotify(cfg map[string]any) (Channel, error) {
	ch := NewGotifyChannel()
	if err := ch.Validate(cfg); err != nil {
		return nil, err
	}
	return ch, nil
}
