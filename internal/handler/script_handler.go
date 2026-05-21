package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/scriptengine"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// scriptTestTimeout caps a /test invocation. Smaller than the engine's
// 5-second default so an interactive UI does not block the user too long;
// production hook runs (inside sync_service) use the full default.
const scriptTestTimeout = 5 * time.Second

// ScriptHandler hosts the /api/scripts/* surface (T-13 / M-SCRIPT).
//
// Collaborators:
//
//   - repo persists / retrieves the scripts table.
//   - engine is the shared scriptengine.Engine (pool-backed goja runtimes).
//     Tests pass the same instance to keep timing stable.
//
// Dependencies are wired in cmd/server/main.go; tests construct the handler
// directly and route through Deps to exercise the full middleware chain.
type ScriptHandler struct {
	repo   *storage.ScriptRepo
	engine *scriptengine.Engine
	logger *slog.Logger
}

// NewScriptHandler wires a handler. engine may be nil — the constructor
// substitutes a fresh scriptengine.Engine so callers do not have to worry
// about lifecycle ordering during startup.
func NewScriptHandler(repo *storage.ScriptRepo, engine *scriptengine.Engine, logger *slog.Logger) *ScriptHandler {
	if engine == nil {
		engine = scriptengine.NewEngine(logger)
	}
	return &ScriptHandler{repo: repo, engine: engine, logger: logger}
}

// List implements GET /api/scripts. Optional ?hook= and ?keyword= filters.
func (h *ScriptHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	hook := r.URL.Query().Get("hook")
	if hook != "" && hook != string(types.HookPreSaveNodes) && hook != string(types.HookPostFetch) {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid hook filter", nil, traceID)
		return
	}
	recs, total, err := h.repo.List(r.Context(), user.ID, storage.ScriptListOptions{
		Page: page.Page, PageSize: page.PageSize,
		Hook:    hook,
		Keyword: r.URL.Query().Get("keyword"),
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	items := make([]types.Script, len(recs))
	for i := range recs {
		items[i] = scriptRecordToDTO(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.Script]]{
		Data: types.PagedResponse[types.Script]{
			Items: items, Total: total, Page: page.Page, PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// Get implements GET /api/scripts/{id}. Cross-user → 404.
func (h *ScriptHandler) Get(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.Script]{
		Data: scriptRecordToDTO(rec), RequestID: traceID,
	})
}

// Create implements POST /api/scripts.
//
// Body shape mirrors types.CreateScriptRequest; hook is validated against the
// two known constants. Code is required and non-empty.
func (h *ScriptHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.CreateScriptRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "name required", nil, traceID)
		return
	}
	if req.Hook != types.HookPreSaveNodes && req.Hook != types.HookPostFetch {
		util.RespondError(w, types.ErrValidationInvalidFormat,
			"hook must be pre_save_nodes or post_fetch", nil, traceID)
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "code required", nil, traceID)
		return
	}
	created, err := h.repo.Create(r.Context(), storage.ScriptRecord{
		ID:      util.UUIDv7(),
		UserID:  user.ID,
		Name:    req.Name,
		Hook:    string(req.Hook),
		Code:    req.Code,
		Enabled: req.Enabled,
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.Script]{
		Data: scriptRecordToDTO(created), RequestID: traceID,
	})
}

// Update implements PUT/PATCH /api/scripts/{id}. Partial update; omitted
// fields are preserved. Hook is immutable (changing it would force a
// migration in the sync chain order).
func (h *ScriptHandler) Update(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	var req types.UpdateScriptRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	upd := storage.ScriptUpdate{}
	if req.Name != "" {
		name := req.Name
		upd.Name = &name
	}
	if req.Code != "" {
		code := req.Code
		upd.Code = &code
	}
	if req.Enabled != nil {
		upd.Enabled = req.Enabled
	}
	rec, err := h.repo.Update(r.Context(), id, user.ID, upd)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.Script]{
		Data: scriptRecordToDTO(rec), RequestID: traceID,
	})
}

// Delete implements DELETE /api/scripts/{id}.
func (h *ScriptHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.repo.Delete(r.Context(), id, user.ID); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// scriptTestRequest is the local Test endpoint payload. The contract DTO
// (types.ScriptTestRequest) only knows about sample_nodes — but the engine
// accepts arbitrary input shapes, so we additionally accept "input" as a
// generic object. Both are optional; when neither is set we pass {}.
type scriptTestRequest struct {
	Input       map[string]any `json:"input,omitempty"`
	SampleNodes string         `json:"sample_nodes,omitempty"`
}

// scriptTestResponse is the local Test endpoint payload. It mirrors
// types.ScriptTestResponse but adds the logs[] channel users want to see in
// the editor's bottom rail.
type scriptTestResponse struct {
	Output     string   `json:"output"`
	Logs       []string `json:"logs"`
	DurationMs int64    `json:"duration_ms"`
	Error      string   `json:"error,omitempty"`
}

// Test implements POST /api/scripts/{id}/test. Runs the stored script
// against the supplied input without persisting anything; returns the raw
// output JSON, captured console logs and elapsed milliseconds. Failures
// (timeout / sandbox violation / runtime error) are returned in the same
// 200 envelope inside the `error` field so the editor UI can render the
// diagnostic without losing the logs that accumulated before the throw.
func (h *ScriptHandler) Test(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}

	var req scriptTestRequest
	if r.ContentLength > 0 {
		if err := util.DecodeJSONBody(r, &req); err != nil {
			util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
			return
		}
	}
	input := req.Input
	if input == nil {
		input = map[string]any{}
	}
	if req.SampleNodes != "" {
		// SampleNodes is the contract field for legacy callers — decode
		// it as JSON and expose as __input.nodes so it lines up with
		// pre_save_nodes hooks.
		var nodes any
		if err := json.Unmarshal([]byte(req.SampleNodes), &nodes); err != nil {
			util.RespondError(w, types.ErrValidationInvalidFormat,
				"sample_nodes must be JSON: "+err.Error(), nil, traceID)
			return
		}
		input["nodes"] = nodes
	}

	res, runErr := h.engine.Run(r.Context(), rec.Code, input, scriptTestTimeout)
	resp := scriptTestResponse{}
	if res != nil {
		resp.Output = res.Output
		resp.Logs = res.Logs
		resp.DurationMs = res.DurationMS
	}
	if runErr != nil {
		resp.Error = runErr.Error()
		// Stamp last_run_at so the list view's "last error" column
		// reflects the most-recent test invocation. We DO NOT short-
		// circuit on a database failure here — the user still wants to
		// see why their script failed even if the audit-write hiccups.
		_ = h.repo.RecordRun(r.Context(), rec.ID, time.Now().UnixMilli(), runErr.Error())
	} else {
		_ = h.repo.RecordRun(r.Context(), rec.ID, time.Now().UnixMilli(), "")
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[scriptTestResponse]{
		Data: resp, RequestID: traceID,
	})
}

// Logs implements GET /api/scripts/{id}/logs. The current schema only keeps
// the last failure (last_error + last_run_at), so the endpoint returns a
// single-entry slice — sufficient for the editor's "last error" panel and
// the contract's ScriptLog shape.
func (h *ScriptHandler) Logs(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	out := make([]types.ScriptLog, 0, 1)
	if rec.LastRunAt > 0 {
		entry := types.ScriptLog{
			Timestamp: rec.LastRunAt,
			Level:     "info",
			Message:   "last run",
		}
		if rec.LastError != "" {
			entry.Level = "error"
			entry.Message = "last run failed"
			entry.Error = rec.LastError
		}
		out = append(out, entry)
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.ScriptLog]{
		Data: out, RequestID: traceID,
	})
}

// respondStorageErr maps storage sentinels into the canonical error
// envelope. Anything we cannot identify is logged + reported as 500.
func (h *ScriptHandler) respondStorageErr(w http.ResponseWriter, traceID string, err error) {
	switch {
	case errors.Is(err, storage.ErrScriptNotFound):
		util.RespondError(w, types.ErrNotFoundScript, "script not found", nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Error("script handler db failed",
				slog.String("err", err.Error()), slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalDatabase, "internal error", nil, traceID)
	}
}

// scriptRecordToDTO projects the storage record into the contract DTO.
func scriptRecordToDTO(rec *storage.ScriptRecord) types.Script {
	return types.Script{
		ID:        rec.ID,
		UserID:    rec.UserID,
		Name:      rec.Name,
		Hook:      types.HookType(rec.Hook),
		Code:      rec.Code,
		Enabled:   rec.Enabled,
		LastRunAt: rec.LastRunAt,
		LastError: rec.LastError,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}
}
