package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestWebhookChannel_Send_Post_Success(t *testing.T) {
	t.Parallel()
	var capturedMethod, capturedCT, capturedBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		capturedMethod.Store(r.Method)
		capturedCT.Store(r.Header.Get("Content-Type"))
		capturedBody.Store(string(raw))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	ch := NewWebhookChannel()
	cfg := map[string]any{
		"url": srv.URL + "/hook",
	}
	err := ch.Send(context.Background(), cfg, Message{Subject: "Sub", Body: "Bod", EventType: "test.event"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if capturedMethod.Load().(string) != http.MethodPost {
		t.Fatalf("expected POST, got %s", capturedMethod.Load())
	}
	body := capturedBody.Load().(string)
	if !strings.Contains(body, "Sub") || !strings.Contains(body, "Bod") {
		t.Fatalf("body missing subject/body: %s", body)
	}
}

func TestWebhookChannel_Send_Put_Success(t *testing.T) {
	t.Parallel()
	var capturedMethod atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod.Store(r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	ch := NewWebhookChannel()
	cfg := map[string]any{"url": srv.URL + "/hook", "method": "PUT"}
	if err := ch.Send(context.Background(), cfg, Message{Subject: "S"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if capturedMethod.Load().(string) != http.MethodPut {
		t.Fatalf("expected PUT, got %s", capturedMethod.Load())
	}
}

func TestWebhookChannel_Send_BodyTemplate(t *testing.T) {
	t.Parallel()
	var capturedBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		capturedBody.Store(string(raw))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	ch := NewWebhookChannel()
	cfg := map[string]any{
		"url":           srv.URL + "/hook",
		"body_template": `{"msg":"{{.Subject}} - {{.EventType}}"}`,
	}
	err := ch.Send(context.Background(), cfg, Message{Subject: "MyAlert", EventType: "node.down"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	body := capturedBody.Load().(string)
	if !strings.Contains(body, "MyAlert") || !strings.Contains(body, "node.down") {
		t.Fatalf("template not rendered correctly: %s", body)
	}
}

func TestWebhookChannel_Send_CustomHeaders(t *testing.T) {
	t.Parallel()
	var capturedAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	ch := NewWebhookChannel()
	cfg := map[string]any{
		"url":     srv.URL + "/hook",
		"headers": map[string]any{"Authorization": "Bearer secret"},
	}
	if err := ch.Send(context.Background(), cfg, Message{Subject: "S"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if capturedAuth.Load().(string) != "Bearer secret" {
		t.Fatalf("expected auth header, got %q", capturedAuth.Load())
	}
}

func TestWebhookChannel_Send_RetriesOn503(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	ch := NewWebhookChannel()
	cfg := map[string]any{"url": srv.URL + "/hook"}
	if err := ch.Send(context.Background(), cfg, Message{Subject: "S"}); err != nil {
		t.Fatalf("send after retry: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", calls.Load())
	}
}

func TestWebhookChannel_Validate(t *testing.T) {
	t.Parallel()
	ch := NewWebhookChannel()
	if err := ch.Validate(map[string]any{"url": "http://example.com"}); err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if err := ch.Validate(map[string]any{}); err == nil {
		t.Fatal("missing url must fail")
	}
	if err := ch.Validate(map[string]any{"url": "http://x.com", "method": "DELETE"}); err == nil {
		t.Fatal("invalid method must fail")
	}
	if err := ch.Validate(nil); err == nil {
		t.Fatal("nil config must fail")
	}
}
