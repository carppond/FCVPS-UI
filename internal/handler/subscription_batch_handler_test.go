package handler

import (
	"encoding/json"
	"net/http"
	"sort"
	"testing"

	"shiguang-vps/internal/types"
)

// createManualSub creates a manual subscription and returns its id.
func (s *subTestStack) createManualSub(t *testing.T, tok, name string, tags []string) string {
	t.Helper()
	rec := s.do(http.MethodPost, "/api/subscriptions", types.CreateSubscriptionRequest{
		Name: name, Type: types.SubTypeManual, Tags: tags,
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %s: status=%d body=%s", name, rec.Code, rec.Body.String())
	}
	var env subEnvelope[types.SubscriptionDetail]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	return env.Data.ID
}

func (s *subTestStack) getSub(t *testing.T, tok, id string) types.SubscriptionDetail {
	t.Helper()
	rec := s.do(http.MethodGet, "/api/subscriptions/"+id, nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %s: status=%d", id, rec.Code)
	}
	var env subEnvelope[types.SubscriptionDetail]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	return env.Data
}

func decodeBatch(t *testing.T, rec interface{ Bytes() []byte }) types.SubscriptionBatchResult {
	t.Helper()
	var env subEnvelope[types.SubscriptionBatchResult]
	if err := json.Unmarshal(rec.Bytes(), &env); err != nil {
		t.Fatalf("decode batch: %v", err)
	}
	return env.Data
}

func TestSubscriptionBatchDelete(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("alice")
	id1 := s.createManualSub(t, tok, "one", nil)
	id2 := s.createManualSub(t, tok, "two", nil)
	s.createManualSub(t, tok, "three", nil)

	rec := s.do(http.MethodPost, "/api/subscriptions/batch-delete",
		types.SubscriptionBatchDeleteRequest{IDs: []string{id1, id2}}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch-delete: status=%d body=%s", rec.Code, rec.Body.String())
	}
	res := decodeBatch(t, rec.Body)
	if res.SucceededCount != 2 || res.FailedCount != 0 {
		t.Fatalf("expected 2 ok/0 fail, got %+v", res)
	}
	// Only "three" should remain.
	listRec := s.do(http.MethodGet, "/api/subscriptions", nil, tok)
	var listEnv subEnvelope[types.PagedResponse[types.Subscription]]
	_ = json.Unmarshal(listRec.Body.Bytes(), &listEnv)
	if listEnv.Data.Total != 1 {
		t.Fatalf("expected 1 remaining, got %d", listEnv.Data.Total)
	}
}

func TestSubscriptionBatchTags(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("alice")
	id1 := s.createManualSub(t, tok, "one", []string{"a", "b"})
	id2 := s.createManualSub(t, tok, "two", []string{"a"})

	rec := s.do(http.MethodPost, "/api/subscriptions/batch-tags",
		types.SubscriptionBatchTagsRequest{
			IDs: []string{id1, id2}, Add: []string{"c"}, Remove: []string{"a"},
		}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch-tags: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if res := decodeBatch(t, rec.Body); res.SucceededCount != 2 {
		t.Fatalf("expected 2 ok, got %+v", res)
	}
	// id1: [a,b] +c -a => [b,c]; id2: [a] +c -a => [c]
	got1 := s.getSub(t, tok, id1).Tags
	sort.Strings(got1)
	if len(got1) != 2 || got1[0] != "b" || got1[1] != "c" {
		t.Fatalf("id1 tags = %v, want [b c]", got1)
	}
	got2 := s.getSub(t, tok, id2).Tags
	if len(got2) != 1 || got2[0] != "c" {
		t.Fatalf("id2 tags = %v, want [c]", got2)
	}
}

func TestSubscriptionBatchUpdate(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("alice")
	id1 := s.createManualSub(t, tok, "one", nil)
	id2 := s.createManualSub(t, tok, "two", nil)

	insecure := true
	interval := int32(7200)
	rec := s.do(http.MethodPost, "/api/subscriptions/batch-update",
		types.SubscriptionBatchUpdateRequest{
			IDs: []string{id1, id2}, SyncInterval: &interval, AllowInsecure: &insecure,
		}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch-update: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if res := decodeBatch(t, rec.Body); res.SucceededCount != 2 {
		t.Fatalf("expected 2 ok, got %+v", res)
	}
	got := s.getSub(t, tok, id1)
	if got.SyncInterval != 7200 || !got.AllowInsecure {
		t.Fatalf("id1 = interval %d insecure %v, want 7200/true", got.SyncInterval, got.AllowInsecure)
	}
}

func TestSubscriptionBatchUpdateRejectsEmptyPayload(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("alice")
	id1 := s.createManualSub(t, tok, "one", nil)
	rec := s.do(http.MethodPost, "/api/subscriptions/batch-update",
		types.SubscriptionBatchUpdateRequest{IDs: []string{id1}}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty payload, got %d", rec.Code)
	}
}

func TestSubscriptionBatchRejectsEmptyIDs(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("alice")
	rec := s.do(http.MethodPost, "/api/subscriptions/batch-delete",
		types.SubscriptionBatchDeleteRequest{IDs: nil}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty ids, got %d", rec.Code)
	}
}

// TestSubscriptionBatchOwnerIsolation verifies a user can't batch-delete another
// user's subscription: the id reports a failure and the record survives.
func TestSubscriptionBatchOwnerIsolation(t *testing.T) {
	s := newSubTestStack(t)
	_, tokA := s.createUserWithToken("alice")
	_, tokB := s.createUserWithToken("bob")
	aliceSub := s.createManualSub(t, tokA, "alice-sub", nil)

	rec := s.do(http.MethodPost, "/api/subscriptions/batch-delete",
		types.SubscriptionBatchDeleteRequest{IDs: []string{aliceSub}}, tokB)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch-delete: status=%d", rec.Code)
	}
	res := decodeBatch(t, rec.Body)
	if res.FailedCount != 1 || res.SucceededCount != 0 {
		t.Fatalf("expected bob's delete of alice's sub to fail, got %+v", res)
	}
	// Alice's subscription must still exist.
	if rec := s.do(http.MethodGet, "/api/subscriptions/"+aliceSub, nil, tokA); rec.Code != http.StatusOK {
		t.Fatalf("alice's sub should survive, get status=%d", rec.Code)
	}
}

func TestSubscriptionBatchSyncReturnsPerIDResults(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("alice")
	id1 := s.createManualSub(t, tok, "one", nil)
	id2 := s.createManualSub(t, tok, "two", nil)

	rec := s.do(http.MethodPost, "/api/subscriptions/batch-sync",
		types.SubscriptionBatchSyncRequest{IDs: []string{id1, id2}}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch-sync: status=%d body=%s", rec.Code, rec.Body.String())
	}
	res := decodeBatch(t, rec.Body)
	if len(res.Results) != 2 {
		t.Fatalf("expected 2 per-id results, got %d", len(res.Results))
	}
	if res.SucceededCount+res.FailedCount != 2 {
		t.Fatalf("counts should sum to 2, got %+v", res)
	}
}
