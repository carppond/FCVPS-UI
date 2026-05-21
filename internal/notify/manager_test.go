package notify_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/util"
)

func newTestDB(t *testing.T) *storage.DB {
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

func seedUser(t *testing.T, db *storage.DB, id, locale string) {
	t.Helper()
	repo := storage.NewUserRepo(db, time.Now)
	if locale == "" {
		locale = "en"
	}
	_, err := repo.Create(context.Background(), storage.UserRecord{
		ID: id, Username: "u-" + id, PasswordHash: "h",
		Role: "user", IsActive: true, Locale: locale,
	})
	if err != nil {
		t.Fatalf("seed user %s: %v", id, err)
	}
}

func TestManager_Emit_FansOutToOptedInChannels(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1", "en")

	channels := storage.NewNotificationChannelRepo(db, time.Now)
	events := storage.NewNotificationEventRepo(db, time.Now)
	users := storage.NewUserRepo(db, time.Now)

	// Spin up a mock HTTP server that captures every POST.
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	cfg, _ := json.Marshal(map[string]any{"webhook_url": srv.URL})
	for _, kind := range []string{"discord", "slack"} {
		_, err := channels.Create(context.Background(), storage.NotificationChannelRecord{
			ID: util.UUIDv7(), UserID: "u1", Kind: kind, Name: kind,
			ConfigJSON: string(cfg), EventTypes: []string{"node_offline"},
			Enabled: true,
		})
		if err != nil {
			t.Fatalf("create channel %s: %v", kind, err)
		}
	}
	// Channel opted out of node_offline — must NOT fire.
	cfgT, _ := json.Marshal(map[string]any{"bot_token": "x", "chat_id": "y"})
	_, _ = channels.Create(context.Background(), storage.NotificationChannelRecord{
		ID: util.UUIDv7(), UserID: "u1", Kind: "telegram", Name: "muted",
		ConfigJSON: string(cfgT), EventTypes: []string{"login_anomaly"},
		Enabled: true,
	})

	reg := notify.NewRegistry()
	notify.RegisterBuiltins(reg)

	bus := notify.NewEventBus()
	mgr, err := notify.NewManager(notify.ManagerConfig{
		ChannelRepo: channels, EventRepo: events, UserRepo: users,
		Registry: reg, Bus: bus,
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	subCh, cancel := bus.Subscribe("u1")
	defer cancel()

	n, err := mgr.Emit(context.Background(), notify.Event{
		Type:       notify.EventNodeOffline,
		UserID:     "u1",
		ResourceID: "node-1",
		Payload:    notify.NodeOfflinePayload{NodeID: "node-1", NodeName: "edge"},
	})
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 channels fired, got %d", n)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 HTTP calls, got %d", calls.Load())
	}
	// SSE event published?
	select {
	case ev := <-subCh:
		if ev.Kind != "notification_event" {
			t.Fatalf("unexpected SSE kind: %s", ev.Kind)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected SSE notification_event")
	}

	// notification_events row count → 2 sent (telegram opted-out so no row).
	recs, _, err := events.ListByUser(context.Background(), "u1", storage.NotificationEventListOptions{
		Page: 1, PageSize: 100,
	})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 event rows, got %d", len(recs))
	}
}

func TestManager_Emit_Dedupe(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1", "en")
	channels := storage.NewNotificationChannelRepo(db, time.Now)
	events := storage.NewNotificationEventRepo(db, time.Now)
	users := storage.NewUserRepo(db, time.Now)

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	cfg, _ := json.Marshal(map[string]any{"webhook_url": srv.URL})
	_, err := channels.Create(context.Background(), storage.NotificationChannelRecord{
		ID: util.UUIDv7(), UserID: "u1", Kind: "discord", Name: "d",
		ConfigJSON: string(cfg), EventTypes: []string{"node_offline"},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	reg := notify.NewRegistry()
	notify.RegisterBuiltins(reg)
	mgr, err := notify.NewManager(notify.ManagerConfig{
		ChannelRepo: channels, EventRepo: events, UserRepo: users,
		Registry: reg,
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := mgr.Emit(context.Background(), notify.Event{
			Type:       notify.EventNodeOffline,
			UserID:     "u1",
			ResourceID: "node-1",
			Payload:    notify.NodeOfflinePayload{NodeID: "node-1"},
		}); err != nil {
			t.Fatalf("emit %d: %v", i, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected dedupe to keep us at 1 send, got %d", calls.Load())
	}
}

func TestManager_SendTest_FiresImmediately(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1", "en")
	channels := storage.NewNotificationChannelRepo(db, time.Now)
	events := storage.NewNotificationEventRepo(db, time.Now)
	users := storage.NewUserRepo(db, time.Now)

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	cfg, _ := json.Marshal(map[string]any{"webhook_url": srv.URL})
	ch, err := channels.Create(context.Background(), storage.NotificationChannelRecord{
		ID: util.UUIDv7(), UserID: "u1", Kind: "slack", Name: "s",
		ConfigJSON: string(cfg), Enabled: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	reg := notify.NewRegistry()
	notify.RegisterBuiltins(reg)
	mgr, _ := notify.NewManager(notify.ManagerConfig{
		ChannelRepo: channels, EventRepo: events, UserRepo: users,
		Registry: reg,
	})
	if err := mgr.SendTest(context.Background(), ch.ID, "u1"); err != nil {
		t.Fatalf("send-test: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 send, got %d", calls.Load())
	}
}
