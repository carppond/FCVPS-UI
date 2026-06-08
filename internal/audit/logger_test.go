package audit_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"shiguang-vps/internal/audit"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
)

func newTestRepo(t *testing.T) *storage.AuditRepo {
	t.Helper()
	db, err := storage.Open(config.DatabaseConfig{
		DataDir: t.TempDir(), Filename: "test.db", BusyTimeoutMs: 5000,
		MaxOpenWrite: 1, MaxOpenRead: 2,
	})
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	// audit_logs.user_id has an FK on users(id); seed referenced rows so
	// non-empty UserID entries persist instead of failing with FOREIGN KEY.
	users := storage.NewUserRepo(db, time.Now)
	for _, id := range []string{"u1", "u"} {
		if _, err := users.Create(context.Background(), storage.UserRecord{
			ID: id, Username: id, PasswordHash: "h", Role: "user", IsActive: true,
		}); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
	}
	return storage.NewAuditRepo(db, time.Now)
}

func TestLoggerLogPersists(t *testing.T) {
	repo := newTestRepo(t)
	logger := audit.New(audit.Config{Repo: repo, QueueSize: 8})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger.Start(ctx)
	defer logger.Stop()

	if err := logger.Log(ctx, middleware.AuditEntry{
		Action:  "create_subscription",
		UserID:  "u1",
		Success: true,
	}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Stop drains the queue.
	logger.Stop()

	rows, total, err := repo.List(context.Background(), storage.AuditLogFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 row, got total=%d len=%d", total, len(rows))
	}
	if rows[0].Action != "create_subscription" || rows[0].UserID != "u1" {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
}

// TestLoggerMasksSensitivePayload is the regression guard for the audit
// payload leak: the persisted audit_logs.payload must NOT contain the
// plaintext password / token — SummarizePayload must run on the persist path.
func TestLoggerMasksSensitivePayload(t *testing.T) {
	repo := newTestRepo(t)
	logger := audit.New(audit.Config{Repo: repo, QueueSize: 8})
	logger.Start(context.Background())

	raw := []byte(`{"username":"admin","password":"hunter2-SECRET","token":"tok-LEAK"}`)
	if err := logger.Log(context.Background(), middleware.AuditEntry{
		Action:  "login",
		UserID:  "u1",
		Payload: raw,
		Success: true,
	}); err != nil {
		t.Fatalf("Log: %v", err)
	}
	logger.Stop()

	rows, _, err := repo.List(context.Background(), storage.AuditLogFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	p := rows[0].Payload
	if strings.Contains(p, "hunter2-SECRET") || strings.Contains(p, "tok-LEAK") {
		t.Fatalf("sensitive value leaked into audit payload: %s", p)
	}
	if !strings.Contains(p, "admin") {
		t.Fatalf("non-sensitive field should survive masking: %s", p)
	}
}

func TestLoggerDropsWhenQueueFull(t *testing.T) {
	repo := newTestRepo(t)
	// Tiny queue + blocking worker keeps the buffer full.
	logger := audit.New(audit.Config{Repo: repo, QueueSize: 1})
	// Do NOT start the worker — the queue won't drain so the second Log
	// call must drop.
	if err := logger.Log(context.Background(), middleware.AuditEntry{Action: "a"}); err != nil {
		t.Fatalf("first Log: %v", err)
	}
	if err := logger.Log(context.Background(), middleware.AuditEntry{Action: "b"}); err != nil {
		t.Fatalf("second Log: %v", err)
	}
	if got := logger.Dropped(); got != 1 {
		t.Fatalf("Dropped() = %d, want 1", got)
	}
}

func TestLoggerGracefulShutdownDrainsQueue(t *testing.T) {
	repo := newTestRepo(t)
	logger := audit.New(audit.Config{Repo: repo, QueueSize: 16})
	logger.Start(context.Background())

	const n = 10
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = logger.Log(context.Background(), middleware.AuditEntry{
				Action: "create_pipeline", UserID: "u", Success: true,
			})
		}(i)
	}
	wg.Wait()
	logger.Stop()

	_, total, err := repo.List(context.Background(), storage.AuditLogFilter{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != int64(n) {
		t.Fatalf("expected %d persisted rows, got %d", n, total)
	}
}

func TestSummarizePayloadMasksSecrets(t *testing.T) {
	in := []byte(`{"username":"alice","password":"hunter2","token":"abc","ok":true}`)
	out := audit.SummarizePayload(in)
	s := string(out)
	if contains(s, "hunter2") {
		t.Fatalf("password leaked: %s", s)
	}
	if contains(s, "abc") {
		t.Fatalf("token leaked: %s", s)
	}
	if !contains(s, "alice") {
		t.Fatalf("expected non-sensitive field to survive: %s", s)
	}
}

func contains(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
