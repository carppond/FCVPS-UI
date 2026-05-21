package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestServerChanChannel_Send_Success(t *testing.T) {
	t.Parallel()
	var capturedPath, capturedBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		capturedPath.Store(r.URL.Path)
		capturedBody.Store(string(raw))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	t.Cleanup(srv.Close)

	ch := NewServerChanChannel()
	ch.SetAPIBase(srv.URL)
	cfg := map[string]any{"key": "SCTXXX123"}
	err := ch.Send(context.Background(), cfg, Message{Subject: "Title", Body: "Long body"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	path := capturedPath.Load().(string)
	if !strings.HasSuffix(path, "/SCTXXX123.send") {
		t.Fatalf("unexpected path: %q", path)
	}
	rawBody := capturedBody.Load().(string)
	vals, _ := url.ParseQuery(rawBody)
	if vals.Get("text") != "Title" {
		t.Fatalf("expected text=Title, got %q", vals.Get("text"))
	}
	if vals.Get("desp") != "Long body" {
		t.Fatalf("expected desp=Long body, got %q", vals.Get("desp"))
	}
}

func TestServerChanChannel_Send_RetriesOn503(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	t.Cleanup(srv.Close)

	ch := NewServerChanChannel()
	ch.SetAPIBase(srv.URL)
	cfg := map[string]any{"key": "K"}
	if err := ch.Send(context.Background(), cfg, Message{Subject: "S"}); err != nil {
		t.Fatalf("send after retry: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", calls.Load())
	}
}

func TestServerChanChannel_Validate(t *testing.T) {
	t.Parallel()
	ch := NewServerChanChannel()
	if err := ch.Validate(map[string]any{"key": "K"}); err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if err := ch.Validate(map[string]any{}); err == nil {
		t.Fatal("missing key must fail")
	}
	if err := ch.Validate(nil); err == nil {
		t.Fatal("nil config must fail")
	}
}
