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

func TestGotifyChannel_Send_Success(t *testing.T) {
	t.Parallel()
	type captured struct {
		path  string
		query string
		body  map[string]any
	}
	var got atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)
		got.Store(captured{
			path:  r.URL.Path,
			query: r.URL.RawQuery,
			body:  payload,
		})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	t.Cleanup(srv.Close)

	ch := NewGotifyChannel()
	cfg := map[string]any{
		"server_url": srv.URL,
		"app_token":  "mytoken123",
		"priority":   float64(8),
	}
	err := ch.Send(context.Background(), cfg, Message{Subject: "Alert", Body: "Node down"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	c := got.Load().(captured)
	if c.path != "/message" {
		t.Fatalf("expected path /message, got %q", c.path)
	}
	if !strings.Contains(c.query, "token=mytoken123") {
		t.Fatalf("expected token in query, got %q", c.query)
	}
	if c.body["title"] != "Alert" {
		t.Fatalf("expected title=Alert, got %v", c.body["title"])
	}
	if c.body["message"] != "Node down" {
		t.Fatalf("expected message=Node down, got %v", c.body["message"])
	}
	if c.body["priority"].(float64) != 8 {
		t.Fatalf("expected priority=8, got %v", c.body["priority"])
	}
}

func TestGotifyChannel_Send_RetriesOn503(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":2}`))
	}))
	t.Cleanup(srv.Close)

	ch := NewGotifyChannel()
	cfg := map[string]any{
		"server_url": srv.URL,
		"app_token":  "tok",
	}
	err := ch.Send(context.Background(), cfg, Message{Subject: "S"})
	if err != nil {
		t.Fatalf("send after retry: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", calls.Load())
	}
}

func TestGotifyChannel_Validate(t *testing.T) {
	t.Parallel()
	ch := NewGotifyChannel()

	// happy path
	if err := ch.Validate(map[string]any{"server_url": "http://g.example.com", "app_token": "t"}); err != nil {
		t.Fatalf("happy path: %v", err)
	}
	// missing server_url
	if err := ch.Validate(map[string]any{"app_token": "t"}); err == nil {
		t.Fatal("missing server_url must fail")
	}
	// missing app_token
	if err := ch.Validate(map[string]any{"server_url": "http://g.example.com"}); err == nil {
		t.Fatal("missing app_token must fail")
	}
	// nil config
	if err := ch.Validate(nil); err == nil {
		t.Fatal("nil config must fail")
	}
}

func TestGotifyChannel_DefaultPriority(t *testing.T) {
	t.Parallel()
	var got atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)
		got.Store(payload)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	ch := NewGotifyChannel()
	// no priority in config → default 5
	cfg := map[string]any{"server_url": srv.URL, "app_token": "t"}
	if err := ch.Send(context.Background(), cfg, Message{Subject: "S"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	body := got.Load().(map[string]any)
	if body["priority"].(float64) != 5 {
		t.Fatalf("expected default priority 5, got %v", body["priority"])
	}
}
