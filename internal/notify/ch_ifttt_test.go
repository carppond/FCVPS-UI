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

func TestIFTTTChannel_Send_Success(t *testing.T) {
	t.Parallel()
	type captured struct {
		path string
		body map[string]any
	}
	var got atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)
		got.Store(captured{path: r.URL.Path, body: payload})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`Congratulations! You've fired the test event`))
	}))
	t.Cleanup(srv.Close)

	ch := NewIFTTTChannel()
	ch.SetAPIBase(srv.URL)
	cfg := map[string]any{
		"event_name":  "my_event",
		"webhook_key": "abc123key",
	}
	err := ch.Send(context.Background(), cfg, Message{Subject: "Subject", Body: "Details", EventType: "node.offline"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	c := got.Load().(captured)
	expectedPath := "/trigger/my_event/with/key/abc123key"
	if c.path != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, c.path)
	}
	if c.body["value1"] != "Subject" {
		t.Fatalf("expected value1=Subject, got %v", c.body["value1"])
	}
	if c.body["value2"] != "Details" {
		t.Fatalf("expected value2=Details, got %v", c.body["value2"])
	}
	if c.body["value3"] != "node.offline" {
		t.Fatalf("expected value3=node.offline, got %v", c.body["value3"])
	}
}

func TestIFTTTChannel_Send_RetriesOn503(t *testing.T) {
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

	ch := NewIFTTTChannel()
	ch.SetAPIBase(srv.URL)
	cfg := map[string]any{"event_name": "e", "webhook_key": "k"}
	if err := ch.Send(context.Background(), cfg, Message{Subject: "S"}); err != nil {
		t.Fatalf("send after retry: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", calls.Load())
	}
}

func TestIFTTTChannel_Validate(t *testing.T) {
	t.Parallel()
	ch := NewIFTTTChannel()

	if err := ch.Validate(map[string]any{"event_name": "e", "webhook_key": "k"}); err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if err := ch.Validate(map[string]any{"webhook_key": "k"}); err == nil {
		t.Fatal("missing event_name must fail")
	}
	if err := ch.Validate(map[string]any{"event_name": "e"}); err == nil {
		t.Fatal("missing webhook_key must fail")
	}
	if err := ch.Validate(nil); err == nil {
		t.Fatal("nil config must fail")
	}
	// assert error message mentions field name
	err := ch.Validate(map[string]any{"webhook_key": "k"})
	if !strings.Contains(err.Error(), "event_name") {
		t.Fatalf("expected error to mention event_name, got %q", err.Error())
	}
}
