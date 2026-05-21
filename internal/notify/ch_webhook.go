package notify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
)

// WebhookChannel delivers notifications to an arbitrary HTTP endpoint using a
// configurable method, headers, and Go-template body.
//
// Config keys (types.WebhookConfig):
//
//	url           string (required)
//	method        string (optional; "POST" or "PUT"; defaults to "POST")
//	headers       map[string]string (optional; merged into every request)
//	body_template string (optional; Go template; vars: .Subject .Body .EventType)
//	content_type  string (optional; defaults to "application/json")
type WebhookChannel struct {
	client HTTPClient
}

// NewWebhookChannel returns a channel using the default HTTP client.
func NewWebhookChannel() *WebhookChannel {
	return &WebhookChannel{client: defaultHTTPClient}
}

// Kind returns the type identifier.
func (c *WebhookChannel) Kind() string { return "webhook" }

// SetHTTPClient overrides the HTTP client (used by tests).
func (c *WebhookChannel) SetHTTPClient(client HTTPClient) { c.client = client }

// Validate ensures the required url field is set and that method is valid.
func (c *WebhookChannel) Validate(cfg any) error {
	m, ok := cfg.(map[string]any)
	if !ok {
		return errors.New("webhook: config must be an object")
	}
	if asString(m, "url") == "" {
		return errors.New("webhook: url required")
	}
	method := strings.ToUpper(asString(m, "method"))
	if method != "" && method != http.MethodPost && method != http.MethodPut {
		return fmt.Errorf("webhook: method must be POST or PUT, got %q", method)
	}
	return nil
}

// Send issues the configured HTTP request. body_template is rendered with the
// Message fields; if unset a default JSON payload is used. Retries are applied
// on 429 / 5xx with exponential backoff (2 retries after the first attempt).
func (c *WebhookChannel) Send(ctx context.Context, cfg any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	m := cfg.(map[string]any)
	rawURL := asString(m, "url")

	method := strings.ToUpper(asString(m, "method"))
	if method == "" {
		method = http.MethodPost
	}
	contentType := asString(m, "content_type")
	if contentType == "" {
		contentType = "application/json"
	}

	payload, err := c.renderBody(m, msg)
	if err != nil {
		return fmt.Errorf("webhook: render body: %w", err)
	}

	headers := map[string]string{"Content-Type": contentType}
	if h, ok := m["headers"]; ok {
		if hmap, ok := h.(map[string]any); ok {
			for k, v := range hmap {
				if s, ok := v.(string); ok {
					headers[k] = s
				}
			}
		}
	}

	if method == http.MethodPost {
		if _, _, err := retryableHTTPPost(ctx, c.client, rawURL, headers, payload, 3); err != nil {
			return fmt.Errorf("webhook send (%s): %w", rawURL, err)
		}
		return nil
	}

	// PUT: manual retry with same policy as retryableHTTPPost.
	if err := c.retryablePut(ctx, rawURL, headers, payload, 3); err != nil {
		return fmt.Errorf("webhook send (%s): %w", rawURL, err)
	}
	return nil
}

// renderBody renders the body_template if set, otherwise returns default JSON.
func (c *WebhookChannel) renderBody(m map[string]any, msg Message) ([]byte, error) {
	tmplStr := asString(m, "body_template")
	if tmplStr == "" {
		return defaultWebhookJSON(msg)
	}
	tmpl, err := template.New("webhook_body").Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	data := map[string]string{
		"Subject":   msg.Subject,
		"Body":      msg.Body,
		"EventType": msg.EventType,
		"Locale":    msg.Locale,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}

// defaultWebhookJSON returns a simple JSON body when no body_template is set.
func defaultWebhookJSON(msg Message) ([]byte, error) {
	// Build manually to avoid import cycle; this is the only place JSON is
	// produced without a struct so we keep it readable.
	escaped := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		s = strings.ReplaceAll(s, "\n", `\n`)
		s = strings.ReplaceAll(s, "\r", `\r`)
		s = strings.ReplaceAll(s, "\t", `\t`)
		return s
	}
	raw := fmt.Sprintf(`{"subject":%q,"body":%q,"event_type":%q}`,
		escaped(msg.Subject), escaped(msg.Body), escaped(msg.EventType))
	return []byte(raw), nil
}

// retryablePut issues an HTTP PUT with the same retry policy as
// retryableHTTPPost: up to maxAttempts on 429 / 5xx.
func (c *WebhookChannel) retryablePut(ctx context.Context, rawURL string, headers map[string]string, body []byte, maxAttempts int) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, rawURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			if !shouldRetry(ctx, attempt, maxAttempts, 0, "") {
				return fmt.Errorf("http put: %w", err)
			}
			sleepBackoff(ctx, attempt, 0)
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		if !shouldRetry(ctx, attempt, maxAttempts, resp.StatusCode, "") {
			return fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(respBody), 200))
		}
		lastErr = fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(respBody), 200))
		sleepBackoff(ctx, attempt, retryAfter)
	}
	if lastErr == nil {
		lastErr = errors.New("http put: exhausted retries")
	}
	return lastErr
}

// buildWebhook is the registry factory.
func buildWebhook(cfg map[string]any) (Channel, error) {
	ch := NewWebhookChannel()
	if err := ch.Validate(cfg); err != nil {
		return nil, err
	}
	return ch, nil
}
