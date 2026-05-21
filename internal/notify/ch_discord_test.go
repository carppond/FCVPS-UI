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

func TestDiscordChannel_Send_Success(t *testing.T) {
	t.Parallel()
	var captured atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		captured.Store(payload)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	ch := NewDiscordChannel()
	cfg := map[string]any{
		"webhook_url": srv.URL,
		"username":    "shiguang-bot",
	}
	err := ch.Send(context.Background(), cfg, Message{Subject: "Boom", Body: "details"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	got := captured.Load().(map[string]any)
	if got["username"] != "shiguang-bot" {
		t.Fatalf("missing username: %#v", got)
	}
	if !strings.Contains(got["content"].(string), "Boom") {
		t.Fatalf("missing subject in content: %v", got["content"])
	}
}

func TestDiscordChannel_Validate(t *testing.T) {
	t.Parallel()
	ch := NewDiscordChannel()
	if err := ch.Validate(map[string]any{"webhook_url": "https://x"}); err != nil {
		t.Fatalf("happy: %v", err)
	}
	if err := ch.Validate(map[string]any{}); err == nil {
		t.Fatalf("missing url must fail")
	}
}
