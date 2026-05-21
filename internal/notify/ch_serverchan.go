package notify

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ServerChanAPIBase is the Server酱 V3 API endpoint template. The {key}
// placeholder is replaced by the user's SendKey.
const ServerChanAPIBase = "https://sctapi.ftqq.com"

// ServerChanChannel delivers notifications via Server酱 (V3).
// Reference: https://sct.ftqq.com/
//
// Config keys (types.ServerChanConfig):
//
//	key string (required, the SendKey / SCT key)
type ServerChanChannel struct {
	client  HTTPClient
	apiBase string
}

// NewServerChanChannel returns a channel using the default HTTP client.
func NewServerChanChannel() *ServerChanChannel {
	return &ServerChanChannel{client: defaultHTTPClient, apiBase: ServerChanAPIBase}
}

// Kind returns the type identifier.
func (c *ServerChanChannel) Kind() string { return "serverchan" }

// SetHTTPClient overrides the HTTP client (used by tests).
func (c *ServerChanChannel) SetHTTPClient(client HTTPClient) { c.client = client }

// SetAPIBase overrides the API base URL (used by tests).
func (c *ServerChanChannel) SetAPIBase(base string) { c.apiBase = base }

// Validate ensures the key field is present.
func (c *ServerChanChannel) Validate(cfg any) error {
	m, ok := cfg.(map[string]any)
	if !ok {
		return errors.New("serverchan: config must be an object")
	}
	if asString(m, "key") == "" {
		return errors.New("serverchan: key required")
	}
	return nil
}

// Send posts to https://sctapi.ftqq.com/<key>.send with text + desp fields.
func (c *ServerChanChannel) Send(ctx context.Context, cfg any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	m := cfg.(map[string]any)
	key := asString(m, "key")
	base := strings.TrimRight(c.apiBase, "/")
	endpoint := fmt.Sprintf("%s/%s.send", base, key)

	body := url.Values{}
	body.Set("text", msg.Subject)
	body.Set("desp", msg.Body)
	payload := []byte(body.Encode())

	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	if _, _, err := retryableHTTPPost(ctx, c.client, endpoint, headers, payload, 3); err != nil {
		return fmt.Errorf("serverchan send (%s): %w", endpoint, err)
	}
	return nil
}

// buildServerChan is the registry factory.
func buildServerChan(cfg map[string]any) (Channel, error) {
	ch := NewServerChanChannel()
	if err := ch.Validate(cfg); err != nil {
		return nil, err
	}
	return ch, nil
}
