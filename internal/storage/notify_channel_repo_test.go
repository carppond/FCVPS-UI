package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
)

func TestNotificationChannelRepo_CreateGetUpdateDelete(t *testing.T) {
	db := newTestDB(t)
	_ = seedUser(t, db, "u1")
	repo := storage.NewNotificationChannelRepo(db, time.Now)
	rec := storage.NotificationChannelRecord{
		ID:         "c1",
		UserID:     "u1",
		Kind:       "telegram",
		Name:       "primary",
		ConfigJSON: `{"bot_token":"x","chat_id":"1"}`,
		EventTypes: []string{"node_offline", "traffic_threshold"},
		Enabled:    true,
	}
	created, err := repo.Create(context.Background(), rec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.CreatedAt == 0 {
		t.Fatalf("created_at should be populated")
	}
	got, err := repo.GetByID(context.Background(), "c1", "u1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "primary" || len(got.EventTypes) != 2 || !got.Enabled {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	got.Enabled = false
	got.Name = "renamed"
	got.EventTypes = []string{"login_anomaly"}
	if err := repo.Update(context.Background(), *got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, _ := repo.GetByID(context.Background(), "c1", "u1")
	if after.Enabled || after.Name != "renamed" || len(after.EventTypes) != 1 {
		t.Fatalf("update did not persist: %+v", after)
	}

	if err := repo.Delete(context.Background(), "c1", "u1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(context.Background(), "c1", "u1"); !errors.Is(err, storage.ErrNotificationChannelNotFound) {
		t.Fatalf("expected not-found after delete, got %v", err)
	}
}

func TestNotificationChannelRepo_ListByUser_FiltersByEvent(t *testing.T) {
	db := newTestDB(t)
	_ = seedUser(t, db, "u1")
	repo := storage.NewNotificationChannelRepo(db, time.Now)
	mk := func(id, kind string, events []string, enabled bool) {
		_, err := repo.Create(context.Background(), storage.NotificationChannelRecord{
			ID: id, UserID: "u1", Kind: kind, Name: id,
			ConfigJSON: "{}", EventTypes: events, Enabled: enabled,
		})
		if err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	mk("a", "telegram", []string{"node_offline", "traffic_threshold"}, true)
	mk("b", "discord", []string{"login_anomaly"}, true)
	mk("c", "slack", []string{"node_offline"}, false) // disabled
	mk("d", "email", []string{}, true)                 // wildcard

	got, err := repo.ListByUser(context.Background(), "u1", "node_offline")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	// "a" is opted in; "c" is disabled (must be skipped). "d" has empty
	// event_types so does NOT match the opt-in filter.
	if len(got) != 1 || got[0].ID != "a" {
		ids := []string{}
		for _, r := range got {
			ids = append(ids, r.ID)
		}
		t.Fatalf("expected only channel a, got %v", ids)
	}

	// Empty eventType returns every enabled channel regardless of opt-in.
	all, err := repo.ListByUser(context.Background(), "u1", "")
	if err != nil {
		t.Fatalf("ListByUser empty: %v", err)
	}
	if len(all) != 3 { // a, b, d
		t.Fatalf("expected 3 enabled, got %d", len(all))
	}
}

func TestNotificationChannelRepo_CrossUserIsolation(t *testing.T) {
	db := newTestDB(t)
	_ = seedUser(t, db, "u1")
	seedUser(t, db, "u2")
	repo := storage.NewNotificationChannelRepo(db, time.Now)
	_, err := repo.Create(context.Background(), storage.NotificationChannelRecord{
		ID: "c1", UserID: "u1", Kind: "telegram", Name: "x",
		ConfigJSON: "{}", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// u2 must NOT see u1's channel.
	if _, err := repo.GetByID(context.Background(), "c1", "u2"); !errors.Is(err, storage.ErrNotificationChannelNotFound) {
		t.Fatalf("expected not-found for cross-user, got %v", err)
	}
}
