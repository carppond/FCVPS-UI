package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// seedSubscription inserts a subscription belonging to userID so the FK on
// nodes(subscription_id) is satisfied. Returns the persisted subscription id.
func seedSubscription(t *testing.T, db *storage.DB, subID, userID string) string {
	t.Helper()
	repo := storage.NewSubscriptionRepo(db, time.Now)
	_, err := repo.Create(context.Background(), storage.SubscriptionRecord{
		ID:     subID,
		UserID: userID,
		Name:   subID + "-name",
		Type:   string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return subID
}

func TestNodeRepoCreateAndGet(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-node-1")
	sub := seedSubscription(t, db, "sub-node-1", user)

	repo := storage.NewNodeRepo(db, time.Now)
	created, err := repo.Create(context.Background(), storage.NodeRecord{
		ID:             "n-1",
		SubscriptionID: sub,
		RawURI:         "vmess://aaa",
		Protocol:       "vmess",
		Server:         "1.1.1.1",
		Port:           443,
		Tag:            "node-1",
		Tags:           []string{"prod"},
		ParsedConfig:   map[string]any{"network": "ws"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != "n-1" {
		t.Fatalf("expected id round-trip, got %q", created.ID)
	}

	got, err := repo.GetByID(context.Background(), "n-1", user)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Server != "1.1.1.1" || got.Port != 443 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.UserID != user {
		t.Fatalf("expected joined user_id=%s, got %s", user, got.UserID)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "prod" {
		t.Fatalf("tags round-trip: %+v", got.Tags)
	}
	if v, ok := got.ParsedConfig["network"]; !ok || v != "ws" {
		t.Fatalf("parsed_config round-trip: %+v", got.ParsedConfig)
	}
}

func TestNodeRepoCrossUserIsolation(t *testing.T) {
	db := newTestDB(t)
	owner := seedUser(t, db, "u-owner")
	intruder := seedUser(t, db, "u-intruder")
	sub := seedSubscription(t, db, "sub-iso", owner)
	repo := storage.NewNodeRepo(db, time.Now)

	_, err := repo.Create(context.Background(), storage.NodeRecord{
		ID: "n-iso", SubscriptionID: sub,
		Protocol: "ss", Server: "2.2.2.2", Port: 8388,
		RawURI: "ss://aaa", Tag: "iso",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Owner can read; intruder cannot.
	if _, err := repo.GetByID(context.Background(), "n-iso", owner); err != nil {
		t.Fatalf("owner read failed: %v", err)
	}
	if _, err := repo.GetByID(context.Background(), "n-iso", intruder); !errors.Is(err, storage.ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound for intruder, got %v", err)
	}

	// Intruder cannot update / delete.
	if err := repo.Update(context.Background(), "n-iso", intruder, storage.NodeUpdate{
		Tag: stringPtrTest("hijacked"),
	}); !errors.Is(err, storage.ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound on intruder update, got %v", err)
	}
	if err := repo.Delete(context.Background(), "n-iso", intruder); !errors.Is(err, storage.ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound on intruder delete, got %v", err)
	}
}

func TestNodeRepoListByUserFilters(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-list")
	sub := seedSubscription(t, db, "sub-list", user)
	repo := storage.NewNodeRepo(db, time.Now)

	mustCreate(t, repo, "n-a", sub, "vmess", "1.1.1.1", 443, "alpha", []string{"hk"})
	mustCreate(t, repo, "n-b", sub, "trojan", "2.2.2.2", 443, "beta", []string{"jp"})
	mustCreate(t, repo, "n-c", sub, "vmess", "3.3.3.3", 443, "gamma", []string{"hk", "fast"})

	got, total, err := repo.ListByUser(context.Background(), user, storage.NodeListOptions{
		Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if total != 3 || len(got) != 3 {
		t.Fatalf("expected 3 nodes, total=%d items=%d", total, len(got))
	}

	// protocol filter
	got, total, _ = repo.ListByUser(context.Background(), user, storage.NodeListOptions{
		Page: 1, PageSize: 10, Protocol: "vmess",
	})
	if total != 2 || len(got) != 2 {
		t.Fatalf("vmess filter expected 2, got total=%d items=%d", total, len(got))
	}

	// tag filter
	got, total, _ = repo.ListByUser(context.Background(), user, storage.NodeListOptions{
		Page: 1, PageSize: 10, Tag: "hk",
	})
	if total != 2 {
		t.Fatalf("tag hk filter expected 2, got total=%d", total)
	}

	// search filter (server)
	got, total, _ = repo.ListByUser(context.Background(), user, storage.NodeListOptions{
		Page: 1, PageSize: 10, Search: "2.2.2.2",
	})
	if total != 1 || got[0].ID != "n-b" {
		t.Fatalf("server search expected n-b, got total=%d", total)
	}
}

func TestNodeRepoUpsertBatch(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-upsert")
	sub := seedSubscription(t, db, "sub-upsert", user)
	repo := storage.NewNodeRepo(db, time.Now)

	// First sync: 3 nodes inserted.
	res, err := repo.UpsertBatch(context.Background(), sub, []storage.NodeUpsertInput{
		{RawURI: "vmess://aaa", Protocol: "vmess", Server: "1.1.1.1", Port: 443, Tag: "a"},
		{RawURI: "vmess://bbb", Protocol: "vmess", Server: "2.2.2.2", Port: 443, Tag: "b"},
		{RawURI: "vmess://ccc", Protocol: "vmess", Server: "3.3.3.3", Port: 443, Tag: "c"},
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if res.Added != 3 || res.Updated != 0 || res.Removed != 0 {
		t.Fatalf("expected 3 added, got %+v", res)
	}

	// Second sync: keep a + b (b modified), drop c, add d. Expect 1 updated,
	// 1 added, 1 removed.
	res, err = repo.UpsertBatch(context.Background(), sub, []storage.NodeUpsertInput{
		{RawURI: "vmess://aaa", Protocol: "vmess", Server: "1.1.1.1", Port: 443, Tag: "a"},
		{RawURI: "vmess://bbb", Protocol: "vmess", Server: "2.2.2.2", Port: 8443, Tag: "b-changed"},
		{RawURI: "vmess://ddd", Protocol: "vmess", Server: "4.4.4.4", Port: 443, Tag: "d"},
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if res.Added != 1 || res.Updated != 2 || res.Removed != 1 {
		t.Fatalf("expected 1 added/2 updated/1 removed, got %+v", res)
	}

	// Verify the persisted state.
	listed, err := repo.ListBySubscription(context.Background(), sub)
	if err != nil {
		t.Fatalf("ListBySubscription: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("expected 3 nodes after second upsert, got %d", len(listed))
	}
	found := map[string]bool{}
	for _, n := range listed {
		found[n.Server] = true
		if n.Server == "2.2.2.2" && n.Port != 8443 {
			t.Fatalf("expected port update on b, got %d", n.Port)
		}
		if n.Server == "2.2.2.2" && n.Tag != "b-changed" {
			t.Fatalf("expected tag update on b, got %q", n.Tag)
		}
	}
	if !found["1.1.1.1"] || !found["2.2.2.2"] || !found["4.4.4.4"] {
		t.Fatalf("expected a/b/d, found %+v", found)
	}
	if found["3.3.3.3"] {
		t.Fatalf("c should have been removed")
	}
}

func TestNodeRepoBatchUpdateLatency(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-tcp")
	sub := seedSubscription(t, db, "sub-tcp", user)
	repo := storage.NewNodeRepo(db, time.Now)

	mustCreate(t, repo, "n-1", sub, "vmess", "1.1.1.1", 443, "n1", nil)
	mustCreate(t, repo, "n-2", sub, "vmess", "2.2.2.2", 443, "n2", nil)

	now := time.Now().UnixMilli()
	if err := repo.BatchUpdateLatency(context.Background(), []storage.TCPingPersist{
		{NodeID: "n-1", LatencyMs: 42, TestedAt: now},
		{NodeID: "n-2", LatencyMs: -1, TestedAt: now},
	}); err != nil {
		t.Fatalf("BatchUpdateLatency: %v", err)
	}

	got1, _ := repo.GetByID(context.Background(), "n-1", user)
	if got1.LastLatencyMs == nil || *got1.LastLatencyMs != 42 {
		t.Fatalf("n-1 latency not set: %+v", got1.LastLatencyMs)
	}
	got2, _ := repo.GetByID(context.Background(), "n-2", user)
	if got2.LastLatencyMs == nil || *got2.LastLatencyMs != -1 {
		t.Fatalf("n-2 should be -1, got %+v", got2.LastLatencyMs)
	}
}

func TestNodeRepoDeleteAndUpdate(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "u-mod")
	sub := seedSubscription(t, db, "sub-mod", user)
	repo := storage.NewNodeRepo(db, time.Now)
	mustCreate(t, repo, "n-x", sub, "ss", "9.9.9.9", 8388, "x", nil)

	if err := repo.Update(context.Background(), "n-x", user, storage.NodeUpdate{
		Tags: &[]string{"new", "labels"},
		Tag:  stringPtrTest("renamed"),
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(context.Background(), "n-x", user)
	if len(got.Tags) != 2 || got.Tag != "renamed" {
		t.Fatalf("update did not apply: %+v", got)
	}

	if err := repo.Delete(context.Background(), "n-x", user); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(context.Background(), "n-x", user); !errors.Is(err, storage.ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound after delete, got %v", err)
	}
}

func mustCreate(t *testing.T, repo *storage.NodeRepo, id, sub, proto, server string, port int32, tag string, tags []string) {
	t.Helper()
	if _, err := repo.Create(context.Background(), storage.NodeRecord{
		ID: id, SubscriptionID: sub, RawURI: proto + "://" + id,
		Protocol: proto, Server: server, Port: port, Tag: tag, Tags: tags,
	}); err != nil {
		t.Fatalf("Create %s: %v", id, err)
	}
}

func stringPtrTest(s string) *string { return &s }
