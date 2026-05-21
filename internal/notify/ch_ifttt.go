package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// IFTTTWebhookBase is the IFTTT Maker Webhooks API root.
const IFTTTWebhookBase = "https://maker.ifttt.com"

// IFTTTChannel triggers an IFTTT Maker Webhook event.
// Reference: https://ifttt.com/maker_webhooks
//
// Config keys (types.IFTTTConfig):
//
//	event_name   string (required, the IFTTT event name)
//	webhook_key  string (required, the IFTTT Maker Webhooks key)
type IFTTTChannel struct {
	client  HTTPClient
	apiBase string
}

// NewIFTTTChannel returns a channel using the default HTTP client.
func NewIFTTTChannel() *IFTTTChannel {
	return &IFTTTChannel{client: defaultHTTPClient, apiBase: IFTTTWebhookBase}
}

// Kind returns the type identifier.
func (c *IFTTTChannel) Kind() string { return "ifttt" }

// SetHTTPClient overrides the HTTP client (used by tests).
func (c *IFTTTChannel) SetHTTPClient(client HTTPClient) { c.client = client }

// SetAPIBase overrides the API base URL (used by tests).
func (c *IFTTTChannel) SetAPIBase(base string) { c.apiBase = base }

// Validate ensures event_name and webhook_key are present.
func (c *IFTTTChannel) Validate(cfg any) error {
	m, ok := cfg.(map[string]any)
	if !ok {
		return errors.New("ifttt: config must be an object")
	}
	if asString(m, "event_name") == "" {
		return errors.New("ifttt: event_name required")
	}
	if asString(m, "webhook_key") == "" {
		return errors.New("ifttt: webhook_key required")
	}
	return nil
}

// Send triggers the IFTTT Maker event with value1=subject, value2=body,
// value3=event_type.
func (c *IFTTTChannel) Send(ctx context.Context, cfg any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	m := cfg.(map[string]any)
	event := asString(m, "event_name")
	key := asString(m, "webhook_key")

	endpoint := fmt.Sprintf("%s/trigger/%s/with/key/%s", c.apiBase, event, key)
	body := map[string]any{
		"value1": msg.Subject,
		"value2": msg.Body,
		"value3": msg.EventType,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("ifttt: marshal: %w", err)
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if _, _, err := retryableHTTPPost(ctx, c.client, endpoint, headers, payload, 3); err != nil {
		return fmt.Errorf("ifttt send (event=%s): %w", event, err)
	}
	return nil
}

// buildIFTTT is the registry factory.
func buildIFTTT(cfg map[string]any) (Channel, error) {
	ch := NewIFTTTChannel()
	if err := ch.Validate(cfg); err != nil {
		return nil, err
	}
	return ch, nil
}
