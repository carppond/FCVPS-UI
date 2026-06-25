package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/substore"
	"shiguang-vps/internal/types"
)

// compatStack bundles every repo the new 4-section rendering uses so a test
// can both insert fixture rows and hit the HTTP layer through the router.
type compatStack struct {
	subs       *storage.SubscriptionRepo
	customRule *storage.CustomRuleRepo
	group      *storage.ProxyGroupRepo
	ruleSet    *storage.RuleSetProviderRepo
	mux        http.Handler
}

func newCompatTestStack(t *testing.T) (*storage.SubscriptionRepo, http.Handler) {
	t.Helper()
	stk := newCompatTestStackFull(t)
	return stk.subs, stk.mux
}

func newCompatTestStackFull(t *testing.T) *compatStack {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(config.DatabaseConfig{
		DataDir: dir, Filename: "test.db", BusyTimeoutMs: 5000,
		MaxOpenWrite: 1, MaxOpenRead: 2,
	})
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	users := storage.NewUserRepo(db, time.Now)
	_, _ = users.Create(context.Background(), storage.UserRecord{
		ID: "u-compat", Username: "u", PasswordHash: "x",
		Role: string(types.RoleUser), IsActive: true,
	})
	subs := storage.NewSubscriptionRepo(db, time.Now)
	customRule := storage.NewCustomRuleRepo(db, time.Now)
	group := storage.NewProxyGroupRepo(db, time.Now)
	ruleSet := storage.NewRuleSetProviderRepo(db, time.Now)
	syncSvc, err := substore.NewSyncService(substore.SyncServiceConfig{
		Repo: subs, NodeRepo: substore.NoopNodeRepo{},
	})
	if err != nil {
		t.Fatalf("NewSyncService: %v", err)
	}
	compatSvc, err := substore.NewSubstoreCompatService(substore.SubstoreCompatConfig{
		Repo:        subs,
		NodeRepo:    substore.NoopNodeRepo{},
		Sync:        syncSvc,
		RuleRepo:    customRule,
		GroupRepo:   group,
		RuleSetRepo: ruleSet,
	})
	if err != nil {
		t.Fatalf("NewSubstoreCompatService: %v", err)
	}
	deps := &Deps{
		DB:                    db,
		SubstoreCompatHandler: NewSubstoreCompatHandler(compatSvc, nil),
	}
	mux := NewRouter(deps)
	return &compatStack{
		subs:       subs,
		customRule: customRule,
		group:      group,
		ruleSet:    ruleSet,
		mux:        mux,
	}
}

func TestSubstoreCompatHappyPath(t *testing.T) {
	subs, mux := newCompatTestStack(t)
	created, err := subs.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-compat-1", UserID: "u-compat", Name: "myname",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet,
		"/download/myname?token="+created.ShareToken, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Total-Nodes") == "" {
		t.Fatalf("expected X-Total-Nodes header")
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/yaml") {
		t.Fatalf("expected text/yaml content type, got %q", rec.Header().Get("Content-Type"))
	}
}

func TestSubstoreCompatWrongTokenReturns404(t *testing.T) {
	subs, mux := newCompatTestStack(t)
	_, err := subs.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-compat-2", UserID: "u-compat", Name: "myname",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/download/myname?token=wrongtoken", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on bad token, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSubstoreCompatMismatchedNameReturns404(t *testing.T) {
	subs, mux := newCompatTestStack(t)
	created, err := subs.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-compat-3", UserID: "u-compat", Name: "real-name",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet,
		"/download/wrong-name?token="+created.ShareToken, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when name does not match, got %d", rec.Code)
	}
}

func TestSubstoreCompatMissingTokenReturns404(t *testing.T) {
	_, mux := newCompatTestStack(t)
	req := httptest.NewRequest(http.MethodGet, "/download/anything", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on missing token, got %d", rec.Code)
	}
}

// TestSubstoreCompatEmitsFullClashDocument exercises the bug-fix end-to-end:
// after the user configures custom_rules + proxy_group_categories +
// rule_set_providers, GET /download/{name}?token=... must return a YAML
// document with all four sections (proxies + proxy-groups + rule-providers +
// rules). Previously only `proxies:` was emitted and the client got no rules,
// no groups, no rule-sets — making the subscription effectively useless.
func TestSubstoreCompatEmitsFullClashDocument(t *testing.T) {
	stk := newCompatTestStackFull(t)
	ctx := context.Background()
	sub, err := stk.subs.Create(ctx, storage.SubscriptionRecord{
		ID: "sub-e2e", UserID: "u-compat", Name: "myname",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("subs.Create: %v", err)
	}
	if _, err := stk.customRule.Create(ctx, storage.CustomRuleRecord{
		ID: "cr-1", UserID: "u-compat",
		Name: "prepend-openai", Type: "rules", Mode: "prepend",
		Content: "DOMAIN-SUFFIX,openai.com,🚀 节点选择",
		Enabled: true, Sort: 1,
	}); err != nil {
		t.Fatalf("customRule.Create: %v", err)
	}
	if _, err := stk.group.Create(ctx, storage.ProxyGroupCategoryRecord{
		ID: "g-1", UserID: "u-compat",
		Name: "🚀 节点选择", Type: "select", SortOrder: 1,
		MemberProxies: `["DIRECT","REJECT"]`,
	}); err != nil {
		t.Fatalf("group.Create: %v", err)
	}
	if _, err := stk.ruleSet.Create(ctx, storage.RuleSetProviderRecord{
		ID: "rs-1", UserID: "u-compat",
		Name: "cn-domain", Behavior: "domain", Format: "mrs",
		URL: "https://gh-proxy.com/example/cn.mrs", IntervalSeconds: 86400,
		Enabled: true,
	}); err != nil {
		t.Fatalf("ruleSet.Create: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet,
		"/download/myname?token="+sub.ShareToken, nil)
	rec := httptest.NewRecorder()
	stk.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var doc struct {
		Proxies       []map[string]any            `yaml:"proxies"`
		ProxyGroups   []map[string]any            `yaml:"proxy-groups"`
		RuleProviders map[string]map[string]any   `yaml:"rule-providers"`
		Rules         []string                    `yaml:"rules"`
	}
	if err := yaml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode rendered YAML: %v\n%s", err, rec.Body.String())
	}
	// proxies block is allowed to be empty (NoopNodeRepo) but the field must
	// be present so the client never reports "no proxies key found".
	if doc.Proxies == nil {
		t.Errorf("expected proxies field present (even if empty)")
	}
	if len(doc.ProxyGroups) == 0 {
		t.Errorf("expected proxy-groups to be emitted")
	}
	foundGroup := false
	for _, g := range doc.ProxyGroups {
		if g["name"] == "🚀 节点选择" {
			foundGroup = true
			break
		}
	}
	if !foundGroup {
		t.Errorf("user-defined group missing in output: %v", doc.ProxyGroups)
	}
	if _, ok := doc.RuleProviders["cn-domain"]; !ok {
		t.Errorf("rule-providers.cn-domain missing: %v", doc.RuleProviders)
	}
	if doc.RuleProviders["cn-domain"]["behavior"] != "domain" ||
		doc.RuleProviders["cn-domain"]["format"] != "mrs" {
		t.Errorf("rule-provider fields wrong: %v", doc.RuleProviders["cn-domain"])
	}
	// rules: must contain the user's prepend line AND the seeded MATCH tail.
	wantPrepend := "DOMAIN-SUFFIX,openai.com,🚀 节点选择"
	wantMatch := "MATCH,🚀 节点选择"
	idxPrepend, idxMatch := -1, -1
	for i, r := range doc.Rules {
		switch r {
		case wantPrepend:
			idxPrepend = i
		case wantMatch:
			idxMatch = i
		}
	}
	if idxPrepend < 0 {
		t.Errorf("rendered rules missing user prepend %q: %v", wantPrepend, doc.Rules)
	}
	if idxMatch < 0 {
		t.Errorf("rendered rules missing default MATCH %q: %v", wantMatch, doc.Rules)
	}
	if idxPrepend > idxMatch {
		t.Errorf("prepend rule should come before MATCH tail, got %v", doc.Rules)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/yaml") {
		t.Errorf("expected text/yaml content-type, got %q", rec.Header().Get("Content-Type"))
	}
}


func TestContentDisposition_NamesProfile(t *testing.T) {
	// ASCII name appears in both the fallback and the RFC5987 form.
	got := contentDisposition("myname")
	if !strings.Contains(got, `filename="myname"`) ||
		!strings.Contains(got, "filename*=UTF-8''myname") {
		t.Fatalf("ascii name: %q", got)
	}
	// CJK name: ASCII fallback strips to the lone hyphen, filename* is encoded.
	got = contentDisposition("潇-专用")
	if !strings.HasPrefix(got, "attachment;") ||
		!strings.Contains(got, "filename*=UTF-8''%E6%BD%87") {
		t.Fatalf("cjk name not encoded: %q", got)
	}
	// Pure-CJK with no ASCII falls back to a sane default.
	if got := asciiFilenameFallback("专用"); got != "subscription" {
		t.Fatalf("empty ascii fallback = %q, want subscription", got)
	}
}
