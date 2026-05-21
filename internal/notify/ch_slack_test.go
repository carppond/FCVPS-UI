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

func TestSlackChannel_Send_Success(t *testing.T) {
	t.Parallel()
	var captured atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		captured.Store(payload)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	ch := NewSlackChannel()
	cfg := map[string]any{
		"webhook_url": srv.URL,
		"channel":     "#alerts",
		"icon_emoji":  ":bell:",
	}
	err := ch.Send(context.Background(), cfg, Message{Subject: "Down", Body: "details"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	got := captured.Load().(map[string]any)
	if got["channel"] != "#alerts" || got["icon_emoji"] != ":bell:" {
		t.Fatalf("unexpected payload: %#v", got)
	}
	if !strings.Contains(got["text"].(string), "Down") {
		t.Fatalf("missing subject in text: %v", got["text"])
	}
}

func TestSlackChannel_Validate(t *testing.T) {
	t.Parallel()
	ch := NewSlackChannel()
	if err := ch.Validate(map[string]any{"webhook_url": "https://x"}); err != nil {
		t.Fatalf("happy: %v", err)
	}
	if err := ch.Validate(map[string]any{}); err == nil {
		t.Fatalf("missing url must fail")
	}
}
