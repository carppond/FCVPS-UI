package substore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// fixedNodeRepo lets us inject a deterministic node set into ServeDownload
// without standing up the full nodes table.
type fixedNodeRepo struct{ nodes []*ParsedNode }

func (r fixedNodeRepo) ListForRender(ctx context.Context, subID string) ([]*ParsedNode, error) {
	return r.nodes, nil
}

func newCompatStack(t *testing.T) (*storage.SubscriptionRepo, *SubstoreCompatService, *storage.SubscriptionRecord) {
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
		ID: "u-t", Username: "u", PasswordHash: "x",
		Role: string(types.RoleUser), IsActive: true,
	})
	subs := storage.NewSubscriptionRepo(db, time.Now)
	rec, err := subs.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-target", UserID: "u-t", Name: "myname",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("subs.Create: %v", err)
	}
	repo := fixedNodeRepo{nodes: []*ParsedNode{
		{Name: "vmess", Protocol: "vmess", Server: "v.example.com", Port: 443,
			UUID: "00000000-0000-0000-0000-000000000001", Method: "auto", Network: "tcp", TLS: true},
		{Name: "ss", Protocol: "ss", Server: "s.example.com", Port: 8388,
			Method: "aes-256-gcm", Password: "pw"},
	}}
	sync, _ := NewSyncService(SyncServiceConfig{Repo: subs, NodeRepo: NoopNodeRepo{}})
	svc, err := NewSubstoreCompatService(SubstoreCompatConfig{
		Repo: subs, NodeRepo: repo, Sync: sync,
	})
	if err != nil {
		t.Fatalf("NewSubstoreCompatService: %v", err)
	}
	return subs, svc, rec
}

func TestServeDownloadByTarget(t *testing.T) {
	_, svc, rec := newCompatStack(t)
	ctx := context.Background()

	t.Run("default-clash", func(t *testing.T) {
		out, err := svc.ServeDownload(ctx, "myname", rec.ShareToken, "")
		if err != nil {
			t.Fatalf("ServeDownload: %v", err)
		}
		if !strings.HasPrefix(out.ContentType, "text/yaml") {
			t.Errorf("expected text/yaml, got %q", out.ContentType)
		}
		if !strings.Contains(string(out.Body), "proxies:") {
			t.Errorf("expected proxies: in YAML output:\n%s", out.Body)
		}
	})

	t.Run("singbox-json", func(t *testing.T) {
		out, err := svc.ServeDownload(ctx, "myname", rec.ShareToken, "singbox")
		if err != nil {
			t.Fatalf("ServeDownload: %v", err)
		}
		if !strings.HasPrefix(out.ContentType, "application/json") {
			t.Errorf("expected application/json, got %q", out.ContentType)
		}
		var doc map[string]any
		if err := json.Unmarshal(out.Body, &doc); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, out.Body)
		}
		if _, ok := doc["outbounds"]; !ok {
			t.Errorf("expected outbounds key")
		}
	})

	t.Run("v2ray-base64-uri-list", func(t *testing.T) {
		out, err := svc.ServeDownload(ctx, "myname", rec.ShareToken, "v2ray")
		if err != nil {
			t.Fatalf("ServeDownload: %v", err)
		}
		if !strings.HasPrefix(out.ContentType, "text/plain") {
			t.Errorf("expected text/plain, got %q", out.ContentType)
		}
		dec, err := base64.StdEncoding.DecodeString(string(out.Body))
		if err != nil {
			t.Fatalf("body not base64: %v", err)
		}
		if !strings.Contains(string(dec), "vmess://") || !strings.Contains(string(dec), "ss://") {
			t.Errorf("missing URIs in decoded body:\n%s", dec)
		}
	})

	t.Run("surge-conf", func(t *testing.T) {
		out, err := svc.ServeDownload(ctx, "myname", rec.ShareToken, "surge")
		if err != nil {
			t.Fatalf("ServeDownload: %v", err)
		}
		if !strings.HasPrefix(out.ContentType, "text/plain") {
			t.Errorf("expected text/plain, got %q", out.ContentType)
		}
		s := string(out.Body)
		if !strings.HasPrefix(s, "[Proxy]\n") {
			t.Errorf("missing [Proxy] section:\n%s", s)
		}
		if !strings.Contains(s, "vmess = vmess") {
			t.Errorf("missing vmess line:\n%s", s)
		}
	})

	t.Run("unknown-target-falls-back-to-clash", func(t *testing.T) {
		out, err := svc.ServeDownload(ctx, "myname", rec.ShareToken, "nosuch")
		if err != nil {
			t.Fatalf("ServeDownload: %v", err)
		}
		if !strings.HasPrefix(out.ContentType, "text/yaml") {
			t.Errorf("expected text/yaml fallback, got %q", out.ContentType)
		}
	})
}
