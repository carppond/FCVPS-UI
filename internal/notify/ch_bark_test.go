package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBarkChannel_Send_Success(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	var lastPath atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		lastPath.Store(r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	t.Cleanup(srv.Close)

	ch := NewBarkChannel()
	cfg := map[string]any{
		"device_key": "DEVICE",
		"server_url": srv.URL,
	}
	err := ch.Send(context.Background(), cfg, Message{Subject: "Hello", Body: "World"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if calls.Load() == 0 {
		t.Fatalf("expected request to be made")
	}
	path := lastPath.Load().(string)
	if !strings.HasPrefix(path, "/DEVICE/") {
		t.Fatalf("expected device_key in path, got %q", path)
	}
	decoded, _ := url.PathUnescape(path)
	if !strings.Contains(decoded, "Hello") || !strings.Contains(decoded, "World") {
		t.Fatalf("expected subject + body in path, got decoded=%q", decoded)
	}
}

func TestBarkChannel_DefaultServerURL(t *testing.T) {
	t.Parallel()
	ch := NewBarkChannel()
	if err := ch.Validate(map[string]any{"device_key": "X"}); err != nil {
		t.Fatalf("validate: %v", err)
	}
	// We can't actually exercise the default server (api.day.app) without
	// hitting the public net; just confirm validation passes and a call
	// against an unreachable URL fails fast. The retry has 3 attempts × 1s
	// backoff so we use a short context to bound the test time.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cfg := map[string]any{"device_key": "X", "server_url": "http://127.0.0.1:1"}
	if err := ch.Send(ctx, cfg, Message{Subject: "t"}); err == nil {
		t.Fatalf("expected send to fail against an unreachable server")
	}
}
