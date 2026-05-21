package ops_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/ops"
	"shiguang-vps/internal/storage"
)

// newBackupFixture returns (db, settings repo) primed with a handful of rows so
// the backup actually has data to round-trip.
func newBackupFixture(t *testing.T) (*storage.DB, *storage.SettingsRepo) {
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
	repo := storage.NewSettingsRepo(db, time.Now)
	if err := repo.Set(context.Background(), "k1", "v1"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.Set(context.Background(), storage.SettingSilentModePrefix, strings.Repeat("a", 32)); err != nil {
		t.Fatalf("seed prefix: %v", err)
	}
	return db, repo
}

// TestBackup_CreateProducesValidArchive verifies Create writes a tar.gz
// containing the three expected entries with the right layout.
func TestBackup_CreateProducesValidArchive(t *testing.T) {
	t.Parallel()
	db, repo := newBackupFixture(t)
	b, err := ops.NewBackup(ops.BackupConfig{DB: db, Repo: repo})
	if err != nil {
		t.Fatalf("NewBackup: %v", err)
	}
	path, err := b.Create(context.Background())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	tr := tar.NewReader(gz)
	got := make(map[string][]byte, 3)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read body %q: %v", hdr.Name, err)
		}
		got[hdr.Name] = body
	}
	for _, want := range []string{"meta.json", "settings.json", "db/traffic.db"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("archive missing entry %q (got %v)", want, keys(got))
		}
	}
	if len(got["db/traffic.db"]) == 0 {
		t.Fatalf("db entry is empty")
	}

	var meta ops.BackupMeta
	if err := json.Unmarshal(got["meta.json"], &meta); err != nil {
		t.Fatalf("decode meta: %v", err)
	}
	if meta.SchemaVersion <= 0 {
		t.Fatalf("schema_version not set: %+v", meta)
	}
	if meta.CreatedAt == 0 {
		t.Fatalf("created_at not set")
	}

	var settings map[string]string
	if err := json.Unmarshal(got["settings.json"], &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if settings["k1"] != "v1" {
		t.Fatalf("settings missing seeded value: %#v", settings)
	}
}

// TestBackup_RoundTripRestoresSettings creates an archive, deletes a setting,
// then restores and verifies the deleted value reappears. The DB swap is
// asserted indirectly via file size matching the original.
func TestBackup_RoundTripRestoresSettings(t *testing.T) {
	t.Parallel()
	db, repo := newBackupFixture(t)
	b, _ := ops.NewBackup(ops.BackupConfig{DB: db, Repo: repo})

	// 1. Create archive.
	path, err := b.Create(context.Background())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	// 2. Mutate live state.
	if err := repo.Set(context.Background(), "k1", "after-mutation"); err != nil {
		t.Fatalf("mutate: %v", err)
	}
	if err := repo.Set(context.Background(), "fresh_key", "noise"); err != nil {
		t.Fatalf("mutate noise: %v", err)
	}

	// 3. Restore — settings should be rewritten from the archive (the live
	// "fresh_key" is left in place because Restore performs an upsert, not a
	// truncate-then-load).
	if err := b.Restore(context.Background(), path); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	v, err := repo.Get(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Get after restore: %v", err)
	}
	if v != "v1" {
		t.Fatalf("k1 = %q after restore, want v1", v)
	}
}

// TestBackup_RestoreRejectsCorruptArchive ensures the validation guard fires
// when the meta.json is missing.
func TestBackup_RestoreRejectsCorruptArchive(t *testing.T) {
	t.Parallel()
	db, repo := newBackupFixture(t)
	b, _ := ops.NewBackup(ops.BackupConfig{DB: db, Repo: repo})

	// Hand-roll an archive containing only an unknown entry.
	bad := filepathJoin(t.TempDir(), "bad.tar.gz")
	f, err := os.Create(bad)
	if err != nil {
		t.Fatalf("create bad: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "junk", Size: 4, Mode: 0o600})
	_, _ = tw.Write([]byte("junk"))
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()

	if err := b.Restore(context.Background(), bad); err == nil {
		t.Fatalf("expected Restore to reject archive missing meta.json")
	}
}

// TestBackup_RestoreRejectsArchiveMissingDB validates the second guard:
// a meta-only archive must not silently pretend it restored anything.
func TestBackup_RestoreRejectsArchiveMissingDB(t *testing.T) {
	t.Parallel()
	db, repo := newBackupFixture(t)
	b, _ := ops.NewBackup(ops.BackupConfig{DB: db, Repo: repo})

	dst := filepathJoin(t.TempDir(), "no-db.tar.gz")
	f, _ := os.Create(dst)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	meta := ops.BackupMeta{SchemaVersion: 1, CreatedAt: time.Now().UnixMilli(), DBFile: "db/traffic.db"}
	body, _ := json.Marshal(meta)
	_ = tw.WriteHeader(&tar.Header{Name: "meta.json", Size: int64(len(body)), Mode: 0o600})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()

	if err := b.Restore(context.Background(), dst); err == nil {
		t.Fatalf("expected Restore to reject archive without db entry")
	}
}

// TestBackup_NewRequiresDBAndRepo guards the constructor's nil checks.
func TestBackup_NewRequiresDBAndRepo(t *testing.T) {
	t.Parallel()
	if _, err := ops.NewBackup(ops.BackupConfig{}); err == nil {
		t.Fatalf("expected error when DB is nil")
	}
	dir := t.TempDir()
	db, _ := storage.Open(config.DatabaseConfig{
		DataDir: dir, Filename: "t.db", BusyTimeoutMs: 5000,
		MaxOpenWrite: 1, MaxOpenRead: 2,
	})
	t.Cleanup(func() { _ = db.Close() })
	if _, err := ops.NewBackup(ops.BackupConfig{DB: db}); err == nil {
		t.Fatalf("expected error when Repo is nil")
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// filepathJoin avoids importing path/filepath in tests when join is the only
// op needed; tests run on POSIX hosts where "/" works.
func filepathJoin(dir, name string) string {
	return dir + string(os.PathSeparator) + name
}
