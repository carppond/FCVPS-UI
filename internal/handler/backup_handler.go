package handler

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/ops"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// backupUploadLimit caps multipart bodies at 256 MiB so a hostile admin can't
// fill /tmp with a 100 GB tar. The number is generous enough for the largest
// realistic database (a few GB of traffic_records) plus settings dump.
const backupUploadLimit = 256 << 20

// BackupHandler hosts /api/admin/backup and /api/admin/backup/restore.
// All routes are admin-only; mounted via auth.RequireAdmin in the router.
type BackupHandler struct {
	backup *ops.Backup
	logger *slog.Logger
}

// NewBackupHandler returns a handler ready to wire up. backup may be nil —
// the endpoints respond with ErrInternalUnknown so misconfiguration is
// visible to admins immediately instead of silently 404ing.
func NewBackupHandler(backup *ops.Backup, logger *slog.Logger) *BackupHandler {
	return &BackupHandler{backup: backup, logger: logger}
}

// Create implements POST /api/admin/backup. Produces a tar.gz snapshot and
// streams it back to the caller; the temp file is removed once the response
// is fully written (or when the connection drops).
//
// Browser-friendly behaviour: Content-Disposition: attachment forces a
// download dialog instead of inline preview.
func (h *BackupHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.backup == nil {
		util.RespondError(w, types.ErrInternalUnknown, "backup unavailable", nil, traceID)
		return
	}
	path, err := h.backup.Create(r.Context())
	if err != nil {
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}
	defer func() {
		// Best-effort cleanup; logging suffices if the file lingers in /tmp.
		if err := os.Remove(path); err != nil && h.logger != nil && !os.IsNotExist(err) {
			h.logger.Warn("backup: temp file cleanup failed",
				slog.String("path", path),
				slog.String("err", err.Error()))
		}
	}()

	f, err := os.Open(path)
	if err != nil {
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}
	filename := ops.SuggestedFilename(time.Now())
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, f); err != nil && h.logger != nil {
		h.logger.Warn("backup: stream copy failed",
			slog.String("err", err.Error()),
			slog.String("trace_id", traceID))
	}
}

// Restore implements POST /api/admin/backup/restore. The request is a
// multipart upload with a single file field named "archive". After a
// successful restore the response includes restart_required=true so the UI
// can show the "service is restarting" banner; the actual process restart is
// the caller's responsibility (managed externally — see ops doc).
func (h *BackupHandler) Restore(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.backup == nil {
		util.RespondError(w, types.ErrInternalUnknown, "backup unavailable", nil, traceID)
		return
	}
	// Limit the body BEFORE parsing so a hostile client doesn't exhaust RAM.
	r.Body = http.MaxBytesReader(w, r.Body, backupUploadLimit)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		util.RespondError(w, types.ErrValidationOutOfRange, err.Error(), nil, traceID)
		return
	}
	file, _, err := r.FormFile("archive")
	if err != nil {
		util.RespondError(w, types.ErrValidationRequiredField, "archive field missing", nil, traceID)
		return
	}
	defer file.Close()

	// Spool to a temp file so ops.Backup.Restore can stat / seek the input.
	tmp, err := os.CreateTemp("", "shiguang-restore-*.tar.gz")
	if err != nil {
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}
	if err := tmp.Close(); err != nil {
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}

	if err := h.backup.Restore(r.Context(), tmpPath); err != nil {
		util.RespondError(w, types.ErrValidationSchemaMismatch, err.Error(), nil, traceID)
		return
	}
	if h.logger != nil {
		h.logger.Warn("backup: restore completed via API; restart required",
			slog.String("trace_id", traceID))
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[map[string]any]{
		Data: map[string]any{
			"restored":         true,
			"restart_required": true,
		},
		RequestID: traceID,
	})
}
