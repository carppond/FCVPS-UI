package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTelegramChannel_Send_Success(t *testing.T) {
	t.Parallel()
	var captured atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		captured.Store(payload)
		if !strings.Contains(r.URL.Path, "/bot/sendMessage") &&
			!strings.Contains(r.URL.Path, "/botabcd:1234/sendMessage") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	ch := NewTelegramChannel()
	ch.SetAPIBase(srv.URL)
	cfg := map[string]any{
		"bot_token":  "abcd:1234",
		"chat_id":    "100",
		"parse_mode": "HTML",
	}
	err := ch.Send(context.Background(), cfg, Message{Subject: "S", Body: "B"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	val := captured.Load()
	if val == nil {
		t.Fatalf("no request captured")
	}
	got := val.(map[string]any)
	if got["chat_id"] != "100" || got["parse_mode"] != "HTML" {
		t.Fatalf("unexpected payload: %#v", got)
	}
	if !strings.Contains(got["text"].(string), "S") {
		t.Fatalf("missing subject in text: %v", got["text"])
	}
}

func TestTelegramChannel_Send_RetriesOn5xx(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	ch := NewTelegramChannel()
	ch.SetAPIBase(srv.URL)
	err := ch.Send(context.Background(), map[string]any{
		"bot_token": "x", "chat_id": "1",
	}, Message{Subject: "S"})
	if err != nil {
		t.Fatalf("send after retry: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", calls.Load())
	}
}

func TestTelegramChannel_Validate(t *testing.T) {
	t.Parallel()
	ch := NewTelegramChannel()
	if err := ch.Validate(map[string]any{"bot_token": "x", "chat_id": "y"}); err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if err := ch.Validate(map[string]any{"chat_id": "y"}); err == nil {
		t.Fatalf("missing token must fail")
	}
	if err := ch.Validate(nil); err == nil {
		t.Fatalf("nil cfg must fail")
	}
}
