package notify

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEmailChannel_Send_BuildsMultipart(t *testing.T) {
	t.Parallel()
	var captured atomic.Value
	sender := func(ctx context.Context, cfg EmailSendConfig) error {
		captured.Store(cfg)
		return nil
	}
	tmpl := NewTemplate()
	ch := NewEmailChannelWithSender(sender, tmpl)
	cfg := map[string]any{
		"smtp_host": "smtp.example.com",
		"smtp_port": 587,
		"smtp_user": "user",
		"smtp_password": "pw",
		"smtp_tls":  true,
		"from":      "alerts@example.com",
		"to":        []any{"a@example.com", "b@example.com"},
	}
	msg := Message{
		EventType: string(EventNodeOffline),
		Subject:   "Hello",
		Body:      "Plain body line 1\nLine 2",
		Locale:    "en",
	}
	if err := ch.Send(context.Background(), cfg, msg); err != nil {
		t.Fatalf("send: %v", err)
	}
	got := captured.Load().(EmailSendConfig)
	if got.Host != "smtp.example.com" || got.Port != 587 {
		t.Fatalf("bad smtp endpoint: %#v", got)
	}
	if len(got.To) != 2 {
		t.Fatalf("expected 2 rcpts, got %d", len(got.To))
	}
	if got.Subject != "Hello" || got.Body != "Plain body line 1\nLine 2" {
		t.Fatalf("subj/body: %#v", got)
	}
}

func TestEmailChannel_Validate(t *testing.T) {
	t.Parallel()
	ch := NewEmailChannelWithSender(func(ctx context.Context, cfg EmailSendConfig) error {
		return nil
	}, nil)
	ok := map[string]any{
		"smtp_host": "x", "smtp_port": 25,
		"from": "a@b", "to": []any{"c@d"},
	}
	if err := ch.Validate(ok); err != nil {
		t.Fatalf("happy: %v", err)
	}
	bad := map[string]any{
		"smtp_host": "x", "smtp_port": 25,
		"to": []any{"c@d"},
	}
	if err := ch.Validate(bad); err == nil {
		t.Fatalf("missing from must fail")
	}
	missingPort := map[string]any{
		"smtp_host": "x", "from": "a@b", "to": []any{"c@d"},
	}
	if err := ch.Validate(missingPort); err == nil {
		t.Fatalf("missing port must fail")
	}
}

func TestEmailChannel_BuildsMIME_AsciiSubject(t *testing.T) {
	t.Parallel()
	mime := buildMIMEMessage(EmailSendConfig{
		From: "a@b", To: []string{"c@d"},
		Subject: "Plain ASCII", Body: "Hi there",
	})
	body := string(mime)
	if !strings.Contains(body, "Subject: Plain ASCII") {
		t.Fatalf("expected verbatim ascii subject, got %q", body)
	}
	if !strings.Contains(body, "text/plain") {
		t.Fatalf("expected text/plain part, got %q", body)
	}
}

func TestEmailChannel_BuildsMIME_NonASCIISubjectEncoded(t *testing.T) {
	t.Parallel()
	mime := buildMIMEMessage(EmailSendConfig{
		From: "a@b", To: []string{"c@d"},
		Subject: "中文主题", Body: "正文",
	})
	body := string(mime)
	if !strings.Contains(body, "=?UTF-8?Q?") {
		t.Fatalf("expected RFC 2047 encoded subject, got %q", body)
	}
}

func TestEmailChannel_BuildsMIME_HTMLMultipart(t *testing.T) {
	t.Parallel()
	mime := buildMIMEMessage(EmailSendConfig{
		From: "a@b", To: []string{"c@d"},
		Subject: "S", Body: "plain", HTML: "<p>html</p>",
	})
	body := string(mime)
	if !strings.Contains(body, "multipart/alternative") {
		t.Fatalf("expected multipart MIME, got %q", body)
	}
	if !strings.Contains(body, "text/html") {
		t.Fatalf("expected text/html part, got %q", body)
	}
}
