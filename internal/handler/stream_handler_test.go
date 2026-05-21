package handler_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/handler"
	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/storage"
)

func newStreamTestDB(t *testing.T) *storage.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(config.DatabaseConfig{
		DataDir: dir, Filename: "test.db", BusyTimeoutMs: 5000,
		MaxOpenWrite: 1, MaxOpenRead: 2,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestStreamHandler_RejectsUnauthenticated(t *testing.T) {
	t.Parallel()
	db := newStreamTestDB(t)
	users := storage.NewUserRepo(db, time.Now)
	sessions := storage.NewSessionRepo(db, time.Now)
	tokens, err := auth.NewTokenStore(auth.TokenStoreConfig{
		Sessions: sessions, Users: users, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("token store: %v", err)
	}
	bus := notify.NewEventBus()
	h := handler.NewStreamHandler(tokens, bus, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/notify/stream", nil)
	rec := httptest.NewRecorder()
	h.Stream(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStreamHandler_DeliversSSEFrame(t *testing.T) {
	t.Parallel()
	db := newStreamTestDB(t)
	users := storage.NewUserRepo(db, time.Now)
	sessions := storage.NewSessionRepo(db, time.Now)
	if _, err := users.Create(context.Background(), storage.UserRecord{
		ID: "u1", Username: "alice", PasswordHash: "h",
		Role: "user", IsActive: true,
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	tokens, err := auth.NewTokenStore(auth.TokenStoreConfig{
		Sessions: sessions, Users: users, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("token store: %v", err)
	}
	token, _, err := tokens.Issue(context.Background(), "u1", "127.0.0.1", "ua", false)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	bus := notify.NewEventBus()
	h := handler.NewStreamHandler(tokens, bus, nil)

	srv := httptest.NewServer(http.HandlerFunc(h.Stream))
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"?token="+token, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected event-stream content type, got %q", ct)
	}
	// Wait for the initial "hello" + a published event.
	go func() {
		time.Sleep(50 * time.Millisecond)
		bus.Publish("u1", notify.SSEEvent{
			Kind: "notification_event", Payload: map[string]any{"x": 1},
		})
	}()

	reader := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(3 * time.Second)
	sawHello := false
	sawEvent := false
	for time.Now().Before(deadline) && !(sawHello && sawEvent) {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "event: system") {
			sawHello = true
		}
		if strings.HasPrefix(line, "event: notification_event") {
			sawEvent = true
		}
	}
	if !sawHello {
		t.Fatalf("did not see hello system event")
	}
	if !sawEvent {
		t.Fatalf("did not see notification_event from bus")
	}
}
