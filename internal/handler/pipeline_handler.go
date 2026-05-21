package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/pipeline"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/substore"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// pipelineRunDeadline caps the wall-clock time a single dry-run is allowed to
// spend inside the engine. Aligns with PRD M-PIPE.3 (100 nodes × 6 ops <
// 500ms); we give 4× headroom for larger node sets pasted by users.
const pipelineRunDeadline = 2 * time.Second

// PipelineHandler hosts /api/pipelines/* endpoints. The struct keeps its
// collaborators tiny so unit tests can drive every method without spinning
// up the full router: repo (CRUD), subscription repo (for run with
// subscription_id), engine (factories + 6 operators).
type PipelineHandler struct {
	repo    *storage.PipelineRepo
	subRepo *storage.SubscriptionRepo
	engine  *pipeline.Engine
	logger  *slog.Logger
}

// NewPipelineHandler wires a handler. subRepo may be nil — the run endpoint
// then refuses requests carrying subscription_id (returns 501 metadata).
func NewPipelineHandler(repo *storage.PipelineRepo, subRepo *storage.SubscriptionRepo, logger *slog.Logger) *PipelineHandler {
	return &PipelineHandler{
		repo:    repo,
		subRepo: subRepo,
		engine:  pipeline.NewEngine(),
		logger:  logger,
	}
}

// List implements GET /api/pipelines. Paginated by util.ParsePaginationQuery.
func (h *PipelineHandler) List(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	recs, total, err := h.repo.List(r.Context(), user.ID, storage.PipelineListOptions{
		Page:     page.Page,
		PageSize: page.PageSize,
		Keyword:  r.URL.Query().Get("keyword"),
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	items := make([]types.Pipeline, len(recs))
	for i := range recs {
		items[i] = pipelineRecordToDTO(&recs[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.Pipeline]]{
		Data: types.PagedResponse[types.Pipeline]{
			Items:    items,
			Total:    total,
			Page:     page.Page,
			PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// Create implements POST /api/pipelines. Accepts CreatePipelineRequest:
// the user supplies a YAML body; the handler parses it, re-marshals to AST
// JSON, and persists both columns. AST is the system of record
// (Tech Lead §1.4); YAML is regenerated from AST so the on-disk yaml_content
// is always canonical.
func (h *PipelineHandler) Create(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.CreatePipelineRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "name required", nil, traceID)
		return
	}
	if strings.TrimSpace(req.YAMLContent) == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "yaml_content required", nil, traceID)
		return
	}
	ast, err := pipeline.DecodeYAML([]byte(req.YAMLContent))
	if err != nil {
		h.respondPipelineErr(w, traceID, err)
		return
	}
	if err := pipeline.Validate(ast); err != nil {
		h.respondPipelineErr(w, traceID, err)
		return
	}
	yamlBytes, astJSON, ok := h.encodeAST(w, traceID, ast)
	if !ok {
		return
	}
	created, err := h.repo.Create(r.Context(), storage.PipelineRecord{
		ID:          util.UUIDv7(),
		UserID:      user.ID,
		Name:        req.Name,
		YAMLContent: string(yamlBytes),
		ASTJSON:     astJSON,
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusCreated, types.APIResponse[types.Pipeline]{
		Data:      pipelineRecordToDTO(created),
		RequestID: traceID,
	})
}

// Get implements GET /api/pipelines/{id}. Cross-user requests resolve to
// 404 (information hiding — never expose existence to other users).
func (h *PipelineHandler) Get(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.Pipeline]{
		Data:      pipelineRecordToDTO(rec),
		RequestID: traceID,
	})
}

// Update implements PUT /api/pipelines/{id}. Optimistic locking via
// req.Version. Accepts either yaml_content or ast_json; YAML wins when both
// supplied (it is the user-facing source) and AST is regenerated, per §6.5.
func (h *PipelineHandler) Update(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	var req types.UpdatePipelineRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.Version <= 0 {
		util.RespondError(w, types.ErrValidationRequiredField, "version required", nil, traceID)
		return
	}
	ast, ok := h.parseUpdateAST(w, traceID, &req)
	if !ok {
		return
	}
	yamlBytes, astJSON, ok := h.encodeAST(w, traceID, ast)
	if !ok {
		return
	}
	existing, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	name := existing.Name
	if strings.TrimSpace(req.Name) != "" {
		name = req.Name
	}
	if err := h.repo.Update(r.Context(), storage.PipelineRecord{
		ID: id, UserID: user.ID, Name: name,
		YAMLContent: string(yamlBytes), ASTJSON: astJSON,
		Version: req.Version,
	}); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.Pipeline]{
		Data: pipelineRecordToDTO(rec), RequestID: traceID,
	})
}

// parseUpdateAST extracts the AST from either the YAML or AST body and
// validates it. On any error it writes the response itself and returns
// (nil, false); the caller short-circuits.
func (h *PipelineHandler) parseUpdateAST(w http.ResponseWriter, traceID string, req *types.UpdatePipelineRequest) (*pipeline.AST, bool) {
	var ast *pipeline.AST
	switch {
	case strings.TrimSpace(req.YAMLContent) != "":
		parsed, err := pipeline.DecodeYAML([]byte(req.YAMLContent))
		if err != nil {
			h.respondPipelineErr(w, traceID, err)
			return nil, false
		}
		ast = parsed
	case strings.TrimSpace(req.ASTJson) != "":
		parsed, err := pipeline.UnmarshalAST(req.ASTJson)
		if err != nil {
			util.RespondError(w, types.ErrValidationInvalidFormat, err.Error(), nil, traceID)
			return nil, false
		}
		ast = parsed
	default:
		util.RespondError(w, types.ErrValidationRequiredField,
			"yaml_content or ast_json required", nil, traceID)
		return nil, false
	}
	if err := pipeline.Validate(ast); err != nil {
		h.respondPipelineErr(w, traceID, err)
		return nil, false
	}
	return ast, true
}

// encodeAST renders the AST to both YAML bytes and canonical JSON string.
// Returns (nil, "", false) on error after writing the response.
func (h *PipelineHandler) encodeAST(w http.ResponseWriter, traceID string, ast *pipeline.AST) ([]byte, string, bool) {
	yamlBytes, err := pipeline.EncodeYAML(ast)
	if err != nil {
		util.RespondError(w, types.ErrValidationYAMLParse, err.Error(), nil, traceID)
		return nil, "", false
	}
	astJSON, err := pipeline.MarshalAST(ast)
	if err != nil {
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return nil, "", false
	}
	return yamlBytes, astJSON, true
}

// Delete implements DELETE /api/pipelines/{id}.
func (h *PipelineHandler) Delete(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.repo.Delete(r.Context(), id, user.ID); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// runRequestExt extends types.RunPipelineRequest with an optional
// "input_nodes" field that lets callers run a dry-run against ad-hoc URI
// strings instead of an existing subscription. We keep the struct local to
// honour the "do not modify contract" hard constraint.
type runRequestExt struct {
	SubscriptionID string   `json:"subscription_id,omitempty"`
	Debug          bool     `json:"debug,omitempty"`
	InputNodes     []string `json:"input_nodes,omitempty"`
}

// Run implements POST /api/pipelines/{id}/run. Strategy:
//
//  1. Load the pipeline AST.
//  2. Resolve the input slice:
//     a. If req.InputNodes is provided, parse each URI via substore.ParseURI.
//     b. Else if req.SubscriptionID is set, fetch the subscription's
//     raw_content and feed it through substore.ParseBulk.
//     c. Else return 400.
//  3. Invoke pipeline.Engine.Run / Preview.
//  4. Project the result into types.RunPipelineResponse.
//
// We honour PRD M-PIPE.3 / .4 by wrapping the run in a deadline; the engine
// surfaces ErrPipelineRunTimeout for clients that should retry.
func (h *PipelineHandler) Run(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	var req runRequestExt
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	rec, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	ast, err := pipeline.UnmarshalAST(rec.ASTJSON)
	if err != nil {
		util.RespondError(w, types.ErrInternalUnknown,
			"stored ast_json invalid: "+err.Error(), nil, traceID)
		return
	}

	input, status, msg := h.resolveRunInput(r.Context(), &req, user.ID)
	if status != "" {
		util.RespondError(w, status, msg, nil, traceID)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), pipelineRunDeadline)
	defer cancel()
	resp, err := h.executeRun(ctx, ast, input, req.Debug)
	if err != nil {
		h.respondPipelineErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.RunPipelineResponse]{
		Data:      *resp,
		RequestID: traceID,
	})
}

// resolveRunInput selects between input_nodes / subscription_id. Returns
// (input, "", "") on success; (nil, errCode, message) on error.
func (h *PipelineHandler) resolveRunInput(ctx context.Context, req *runRequestExt, userID string) ([]*substore.ParsedNode, types.ErrorCode, string) {
	if len(req.InputNodes) > 0 {
		out := make([]*substore.ParsedNode, 0, len(req.InputNodes))
		for i, uri := range req.InputNodes {
			n, err := substore.ParseURI(uri)
			if err != nil {
				return nil, types.ErrValidationInvalidFormat,
					fmt.Sprintf("input_nodes[%d]: %s", i, err.Error())
			}
			out = append(out, n)
		}
		return out, "", ""
	}
	if strings.TrimSpace(req.SubscriptionID) == "" {
		return nil, types.ErrValidationRequiredField,
			"subscription_id or input_nodes required"
	}
	if h.subRepo == nil {
		return nil, types.ErrInternalUnknown,
			"subscription repo unavailable"
	}
	subRec, err := h.subRepo.GetByID(ctx, req.SubscriptionID, userID)
	if err != nil {
		if errors.Is(err, storage.ErrSubscriptionNotFound) {
			return nil, types.ErrNotFoundSubscription, "subscription not found"
		}
		return nil, types.ErrInternalDatabase, err.Error()
	}
	if len(subRec.RawContent) == 0 {
		return nil, types.ErrValidationOutOfRange,
			"subscription has no raw_content; sync first"
	}
	nodes, _ := substore.ParseBulk(string(subRec.RawContent))
	return nodes, "", ""
}

// executeRun runs / previews the AST. When debug is true we use Preview so
// the response carries before/after node-name snapshots; otherwise Run is
// faster (no name slice allocation).
func (h *PipelineHandler) executeRun(ctx context.Context, ast *pipeline.AST, input []*substore.ParsedNode, debug bool) (*types.RunPipelineResponse, error) {
	if debug {
		preview, err := h.engine.Preview(ctx, ast, input)
		if err != nil {
			return nil, err
		}
		return previewToResponse(preview), nil
	}
	res, err := h.engine.Run(ctx, ast, input)
	if err != nil {
		return nil, err
	}
	resp := &types.RunPipelineResponse{
		TotalMs:     res.DurationMS,
		OutputCount: int32(len(res.Output)),
	}
	resp.Steps = make([]types.OperatorStepResult, 0, len(res.Steps))
	for _, s := range res.Steps {
		resp.Steps = append(resp.Steps, types.OperatorStepResult{
			Operator:    s.Kind,
			InputCount:  int32(s.BeforeN),
			OutputCount: int32(s.AfterN),
			Removed:     s.Removed,
			Added:       s.Added,
			Modified:    s.Modified,
		})
	}
	return resp, nil
}

// YAMLToAST implements POST /api/pipelines/yaml-to-ast. Stateless: no DB
// access. Used by the GUI to round-trip between text editor and structured
// view (M-PIPE.5).
func (h *PipelineHandler) YAMLToAST(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	var req types.YAMLToASTRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if strings.TrimSpace(req.YAMLContent) == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "yaml_content required", nil, traceID)
		return
	}
	ast, err := pipeline.DecodeYAML([]byte(req.YAMLContent))
	if err != nil {
		h.respondPipelineErr(w, traceID, err)
		return
	}
	if err := pipeline.Validate(ast); err != nil {
		h.respondPipelineErr(w, traceID, err)
		return
	}
	astJSON, err := pipeline.MarshalAST(ast)
	if err != nil {
		util.RespondError(w, types.ErrInternalUnknown, err.Error(), nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[map[string]string]{
		Data:      map[string]string{"ast_json": astJSON},
		RequestID: traceID,
	})
}

// ASTToYAML implements POST /api/pipelines/ast-to-yaml.
func (h *PipelineHandler) ASTToYAML(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	var req types.ASTToYAMLRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if strings.TrimSpace(req.ASTJson) == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "ast_json required", nil, traceID)
		return
	}
	ast, err := pipeline.UnmarshalAST(req.ASTJson)
	if err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, err.Error(), nil, traceID)
		return
	}
	if err := pipeline.Validate(ast); err != nil {
		h.respondPipelineErr(w, traceID, err)
		return
	}
	yamlBytes, err := pipeline.EncodeYAML(ast)
	if err != nil {
		util.RespondError(w, types.ErrValidationYAMLParse, err.Error(), nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[map[string]string]{
		Data:      map[string]string{"yaml_content": string(yamlBytes)},
		RequestID: traceID,
	})
}

// Operators implements GET /api/pipelines/operators. Returns the static
// catalog of registered kinds and their declared parameter shape (a simple
// list of field names). Frontends use this to render the operator library
// without hard-coding kind names.
func (h *PipelineHandler) Operators(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	kinds := pipeline.RegisteredKinds()
	out := make([]types.OperatorSchema, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, types.OperatorSchema{
			Type:         types.OperatorType(k),
			DisplayName:  k,
			Description:  operatorDescription(k),
			ParamsSchema: operatorParamsSchema(k),
		})
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.OperatorSchema]{
		Data:      out,
		RequestID: traceID,
	})
}

