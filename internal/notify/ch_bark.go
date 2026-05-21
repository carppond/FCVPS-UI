package notify

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// BarkServerDefault is the public Bark relay used when the user does not
// supply a self-hosted server_url.
const BarkServerDefault = "https://api.day.app"

// BarkChannel pushes via Bark (https://github.com/Finb/Bark). The Bark API
// supports both GET and POST; we use POST so the title / body avoid URL
// escaping pitfalls with multi-line payloads.
//
// Config keys (types.BarkConfig):
//
//	device_key string (required)
//	server_url string (optional; defaults to BarkServerDefault)
//	sound      string (optional, e.g. "minuet")
type BarkChannel struct {
	client HTTPClient
}

// NewBarkChannel returns a channel using the default HTTP client.
func NewBarkChannel() *BarkChannel {
	return &BarkChannel{client: defaultHTTPClient}
}

// Kind returns the type identifier.
func (c *BarkChannel) Kind() string { return "bark" }

// SetHTTPClient overrides the HTTP client (used by tests).
func (c *BarkChannel) SetHTTPClient(client HTTPClient) { c.client = client }

// Validate ensures device_key is set.
func (c *BarkChannel) Validate(cfg any) error {
	m, ok := cfg.(map[string]any)
	if !ok {
		return errors.New("bark: config must be an object")
	}
	if asString(m, "device_key") == "" {
		return errors.New("bark: device_key required")
	}
	return nil
}

// Send POSTs the title + body to /<device_key>/<title>/<body>. The Bark API
// accepts both path-style and JSON-body invocations; we use the path-style
// form to minimise dependencies on the server's content negotiation.
func (c *BarkChannel) Send(ctx context.Context, cfg any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	m := cfg.(map[string]any)
	server := asString(m, "server_url")
	if server == "" {
		server = BarkServerDefault
	}
	server = strings.TrimRight(server, "/")
	deviceKey := asString(m, "device_key")
	title := url.PathEscape(msg.Subject)
	body := url.PathEscape(msg.Body)
	if body == "" {
		body = title
	}
	endpoint := fmt.Sprintf("%s/%s/%s/%s", server, deviceKey, title, body)
	if sound := asString(m, "sound"); sound != "" {
		endpoint += "?sound=" + url.QueryEscape(sound)
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if _, _, err := retryableHTTPPost(ctx, c.client, endpoint, headers, []byte(`{}`), 3); err != nil {
		return fmt.Errorf("bark send: %w", err)
	}
	return nil
}

// buildBark is the registry factory.
func buildBark(cfg map[string]any) (Channel, error) {
	ch := NewBarkChannel()
	if err := ch.Validate(cfg); err != nil {
		return nil, err
	}
	return ch, nil
}
