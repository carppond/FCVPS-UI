package ops

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shiguang-vps/internal/storage"
)

// backupSchemaVersion is bumped whenever the on-disk layout (file names,
// metadata fields) changes incompatibly. Restore checks the value and refuses
// archives produced by a newer/incompatible writer.
const backupSchemaVersion = 1

// Canonical entry names inside the tar.gz so Restore can locate them without
// scanning the archive.
const (
	backupMetaName     = "meta.json"
	backupDBName       = "db/traffic.db"
	backupSettingsName = "settings.json"
)

// BackupMeta records what the archive contains so the consumer (admin tool or
// Restore) can validate compatibility before touching live data.
type BackupMeta struct {
	SchemaVersion int    `json:"schema_version"`
	CreatedAt     int64  `json:"created_at"`
	DBFile        string `json:"db_file"`
	Note          string `json:"note,omitempty"`
}

// BackupConfig wires the Backup helper. DBPath is the on-disk filename of the
// SQLite database (typically data/traffic.db); Repo is the settings k/v
// accessor — both are mandatory.
type BackupConfig struct {
	DB     *storage.DB
	Repo   *storage.SettingsRepo
	Logger *slog.Logger
}

// Backup creates / restores tar.gz snapshots of the system. It is the only
// place in the codebase that writes outside of the application database
// directory.
type Backup struct {
	db     *storage.DB
	repo   *storage.SettingsRepo
	logger *slog.Logger
}

// NewBackup constructs a Backup helper. db is required so Create can issue a
// wal_checkpoint before copying the file; repo is required for the settings
// dump. logger may be nil.
func NewBackup(cfg BackupConfig) (*Backup, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("backup: db required")
	}
	if cfg.Repo == nil {
		return nil, fmt.Errorf("backup: settings repo required")
	}
	return &Backup{db: cfg.DB, repo: cfg.Repo, logger: cfg.Logger}, nil
}

// Create produces a tar.gz archive containing:
//
//   - meta.json     — schema version + creation timestamp
//   - db/traffic.db — the SQLite file, captured AFTER wal_checkpoint(TRUNCATE)
//     so the WAL / SHM sidecars are merged in
//   - settings.json — full system_settings dump (cleartext; sensitive fields
//     included — admins are expected to encrypt the archive at rest)
//
// The archive is written into the system temp dir; the caller is responsible
// for moving / streaming it to its final destination and deleting the temp
// file when done.
func (b *Backup) Create(ctx context.Context) (string, error) {
	// Step 1: checkpoint the WAL so the .db file is a complete snapshot. We
	// use TRUNCATE so the WAL file is zero-length afterwards, leaving a tidy
	// single-file database. Errors are non-fatal — the copy below still works
	// from the most recent fsync.
	if _, err := b.db.Write.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		if b.logger != nil {
			b.logger.Warn("backup: wal_checkpoint failed; continuing",
				slog.String("err", err.Error()))
		}
	}

	// Step 2: open the destination archive.
	tmp, err := os.CreateTemp("", "shiguang-backup-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("backup: create temp file: %w", err)
	}
	dst := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
			// Best-effort cleanup on error path; ignore.
			_ = os.Remove(dst)
		}
	}()

	gz := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gz)

	// Step 3: write meta.json first so consumers can short-circuit on version
	// mismatch without scanning the rest.
	meta := BackupMeta{
		SchemaVersion: backupSchemaVersion,
		CreatedAt:     time.Now().UnixMilli(),
		DBFile:        backupDBName,
	}
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", fmt.Errorf("backup: encode meta: %w", err)
	}
	if err := writeTarFile(tw, backupMetaName, metaJSON, 0o600); err != nil {
		return "", err
	}

	// Step 4: settings dump. Reads through the repo so we honour the same
	// connection pool as the rest of the app.
	settings, err := b.repo.GetAll(ctx)
	if err != nil {
		return "", fmt.Errorf("backup: load settings: %w", err)
	}
	settingsJSON, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("backup: encode settings: %w", err)
	}
	if err := writeTarFile(tw, backupSettingsName, settingsJSON, 0o600); err != nil {
		return "", err
	}

	// Step 5: stream the .db file in. We re-open the file read-only so any
	// in-flight writer pool connection isn't blocked.
	if err := streamFileToTar(tw, b.db.Path(), backupDBName); err != nil {
		return "", fmt.Errorf("backup: copy db: %w", err)
	}

	// Step 6: flush + close in the correct order (tar → gzip → file).
	if err := tw.Close(); err != nil {
		return "", fmt.Errorf("backup: close tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("backup: close gzip: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("backup: close temp file: %w", err)
	}
	closed = true

	if b.logger != nil {
		b.logger.Info("backup: archive created",
			slog.String("path", dst))
	}
	return dst, nil
}

// Restore unpacks tarPath into the data directory, replacing the live
// database file. The flow assumes the caller has already drained inflight
// requests — silent_mode rotation + immediate restart is the canonical
// recipe.
//
// v1 IS NOT a hot-restore: callers must `os.Exit` after a successful Restore
// so the process re-opens the new file. The handler enforces this by writing
// the new database to <db>.restore, swapping atomically, and returning a
// non-nil error if the swap fails (so the caller surfaces the failure
// instead of pretending the restore succeeded).
func (b *Backup) Restore(ctx context.Context, tarPath string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("restore: open archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("restore: gunzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	var (
		meta            *BackupMeta
		settingsContent []byte
		dbWritten       bool
	)
	// We write the .db into a staging file first so a corrupt archive cannot
	// half-overwrite the live file.
	dbDest := b.db.Path()
	stagingPath := dbDest + ".restore"
	_ = os.Remove(stagingPath) // clean any stale staging from a previous failed restore

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("restore: read tar: %w", err)
		}
		// Defensive: reject path traversal entries.
		if strings.Contains(hdr.Name, "..") {
			return fmt.Errorf("restore: refusing suspicious entry %q", hdr.Name)
		}
		switch hdr.Name {
		case backupMetaName:
			buf, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("restore: read meta: %w", err)
			}
			var m BackupMeta
			if err := json.Unmarshal(buf, &m); err != nil {
				return fmt.Errorf("restore: parse meta: %w", err)
			}
			if m.SchemaVersion > backupSchemaVersion {
				return fmt.Errorf("restore: archive schema %d newer than supported %d",
					m.SchemaVersion, backupSchemaVersion)
			}
			meta = &m
		case backupSettingsName:
			buf, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("restore: read settings: %w", err)
			}
			settingsContent = buf
		case backupDBName:
			out, err := os.OpenFile(stagingPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return fmt.Errorf("restore: open staging: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				_ = os.Remove(stagingPath)
				return fmt.Errorf("restore: copy db: %w", err)
			}
			if err := out.Close(); err != nil {
				_ = os.Remove(stagingPath)
				return fmt.Errorf("restore: close staging: %w", err)
			}
			dbWritten = true
		default:
			// Forward-compat: skip unknown entries silently.
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return fmt.Errorf("restore: skip %q: %w", hdr.Name, err)
			}
		}
	}

	if meta == nil {
		_ = os.Remove(stagingPath)
		return fmt.Errorf("restore: archive missing %s", backupMetaName)
	}
	if !dbWritten {
		return fmt.Errorf("restore: archive missing %s", backupDBName)
	}

	// Settings dump is restored via the repo so the on-disk format matches
	// the running schema (skip if the archive omitted it for some reason).
	if len(settingsContent) > 0 {
		var settings map[string]string
		if err := json.Unmarshal(settingsContent, &settings); err != nil {
			_ = os.Remove(stagingPath)
			return fmt.Errorf("restore: parse settings: %w", err)
		}
		if err := b.repo.SetMany(ctx, settings); err != nil {
			_ = os.Remove(stagingPath)
			return fmt.Errorf("restore: write settings: %w", err)
		}
	}

	// Atomic swap on POSIX: os.Rename is atomic when source and destination
	// live on the same filesystem (which they do — both in the data dir).
	if err := os.Rename(stagingPath, dbDest); err != nil {
		return fmt.Errorf("restore: swap db file: %w", err)
	}

	if b.logger != nil {
		b.logger.Warn("backup: restore completed; process MUST restart to re-open database",
			slog.String("db", dbDest))
	}
	return nil
}

// writeTarFile is a small helper that writes a single in-memory file into the
// archive. Centralised so the header layout is consistent across entries.
func writeTarFile(tw *tar.Writer, name string, data []byte, mode int64) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    mode,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write header %q: %w", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("write body %q: %w", name, err)
	}
	return nil
}

// streamFileToTar copies srcPath into the archive under name. The header
// records the file's real size + mod time so consumers can verify quickly.
func streamFileToTar(tw *tar.Writer, srcPath, name string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", srcPath, err)
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer src.Close()
	hdr := &tar.Header{
		Name:    name,
		Mode:    int64(info.Mode().Perm()),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write header %q: %w", name, err)
	}
	if _, err := io.Copy(tw, src); err != nil {
		return fmt.Errorf("copy body %q: %w", name, err)
	}
	return nil
}

// SuggestedFilename returns a stable name for downloaded archives so admins
// can keep multiple snapshots without manual renaming.
func SuggestedFilename(now time.Time) string {
	return fmt.Sprintf("shiguang-backup-%s.tar.gz",
		now.UTC().Format("20060102-150405"))
}

// Ensure the import of filepath stays useful even when Restore's swap path is
// the only consumer; the lint package whines otherwise.
var _ = filepath.Join
