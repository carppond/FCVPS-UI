package notify

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// PushDeerAPIDefault is the public PushDeer API endpoint used when the user
// does not supply a self-hosted server_url.
const PushDeerAPIDefault = "https://api2.pushdeer.com"

// PushDeerChannel delivers notifications via PushDeer.
// Reference: https://github.com/easychen/pushdeer
//
// Config keys (types.PushDeerConfig):
//
//	pushkey    string (required)
//	server_url string (optional; defaults to PushDeerAPIDefault)
type PushDeerChannel struct {
	client HTTPClient
}

// NewPushDeerChannel returns a channel using the default HTTP client.
func NewPushDeerChannel() *PushDeerChannel {
	return &PushDeerChannel{client: defaultHTTPClient}
}

// Kind returns the type identifier.
func (c *PushDeerChannel) Kind() string { return "pushdeer" }

// SetHTTPClient overrides the HTTP client (used by tests).
func (c *PushDeerChannel) SetHTTPClient(client HTTPClient) { c.client = client }

// Validate ensures pushkey is set.
func (c *PushDeerChannel) Validate(cfg any) error {
	m, ok := cfg.(map[string]any)
	if !ok {
		return errors.New("pushdeer: config must be an object")
	}
	if asString(m, "pushkey") == "" {
		return errors.New("pushdeer: pushkey required")
	}
	return nil
}

// Send posts to /message/push with pushkey, text, desp, and type=markdown.
func (c *PushDeerChannel) Send(ctx context.Context, cfg any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	m := cfg.(map[string]any)
	server := asString(m, "server_url")
	if server == "" {
		server = PushDeerAPIDefault
	}
	server = strings.TrimRight(server, "/")
	pushkey := asString(m, "pushkey")

	params := url.Values{}
	params.Set("pushkey", pushkey)
	params.Set("text", msg.Subject)
	params.Set("desp", msg.Body)
	params.Set("type", "markdown")

	endpoint := fmt.Sprintf("%s/message/push?%s", server, params.Encode())

	headers := map[string]string{"Content-Type": "application/json"}
	if _, _, err := retryableHTTPPost(ctx, c.client, endpoint, headers, []byte(`{}`), 3); err != nil {
		return fmt.Errorf("pushdeer send (%s): %w", server, err)
	}
	return nil
}

// buildPushDeer is the registry factory.
func buildPushDeer(cfg map[string]any) (Channel, error) {
	ch := NewPushDeerChannel()
	if err := ch.Validate(cfg); err != nil {
		return nil, err
	}
	return ch, nil
}
