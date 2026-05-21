package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPushDeerChannel_Send_Success(t *testing.T) {
	t.Parallel()
	var capturedQuery atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery.Store(r.URL.RawQuery)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	t.Cleanup(srv.Close)

	ch := NewPushDeerChannel()
	cfg := map[string]any{
		"server_url": srv.URL,
		"pushkey":    "PDU123",
	}
	err := ch.Send(context.Background(), cfg, Message{Subject: "Alert", Body: "Node offline"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	q := capturedQuery.Load().(string)
	if !strings.Contains(q, "pushkey=PDU123") {
		t.Fatalf("expected pushkey in query: %s", q)
	}
	if !strings.Contains(q, "text=Alert") {
		t.Fatalf("expected text in query: %s", q)
	}
	if !strings.Contains(q, "type=markdown") {
		t.Fatalf("expected type=markdown in query: %s", q)
	}
}

func TestPushDeerChannel_Send_DefaultServer(t *testing.T) {
	t.Parallel()
	// Validate passes with no server_url; default is used.
	ch := NewPushDeerChannel()
	if err := ch.Validate(map[string]any{"pushkey": "X"}); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestPushDeerChannel_Send_RetriesOn503(t *testing.T) {
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

	ch := NewPushDeerChannel()
	cfg := map[string]any{"server_url": srv.URL, "pushkey": "K"}
	if err := ch.Send(context.Background(), cfg, Message{Subject: "S"}); err != nil {
		t.Fatalf("send after retry: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", calls.Load())
	}
}

func TestPushDeerChannel_Validate(t *testing.T) {
	t.Parallel()
	ch := NewPushDeerChannel()
	if err := ch.Validate(map[string]any{"pushkey": "K"}); err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if err := ch.Validate(map[string]any{}); err == nil {
		t.Fatal("missing pushkey must fail")
	}
	if err := ch.Validate(nil); err == nil {
		t.Fatal("nil config must fail")
	}
}
