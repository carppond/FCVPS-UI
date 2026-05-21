package handler

import (
	"shiguang-vps/internal/pipeline"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// previewToResponse maps a PreviewResult into the contract RunPipelineResponse.
func previewToResponse(pr *pipeline.PreviewResult) *types.RunPipelineResponse {
	resp := &types.RunPipelineResponse{
		TotalMs:     pr.DurationMS,
		OutputCount: int32(pr.TotalOut),
	}
	resp.Steps = make([]types.OperatorStepResult, 0, len(pr.Steps))
	for _, s := range pr.Steps {
		resp.Steps = append(resp.Steps, types.OperatorStepResult{
			Operator:    s.Kind,
			InputCount:  int32(len(s.BeforeIDs)),
			OutputCount: int32(len(s.AfterIDs)),
			Removed:     s.Removed,
			Added:       s.Added,
			Modified:    s.Modified,
		})
	}
	return resp
}

// pipelineRecordToDTO projects a storage record into the API DTO.
func pipelineRecordToDTO(rec *storage.PipelineRecord) types.Pipeline {
	if rec == nil {
		return types.Pipeline{}
	}
	return types.Pipeline{
		ID:            rec.ID,
		UserID:        rec.UserID,
		Name:          rec.Name,
		YAMLContent:   rec.YAMLContent,
		ASTJson:       rec.ASTJSON,
		Version:       rec.Version,
		SchemaVersion: rec.SchemaVersion,
		CreatedAt:     rec.CreatedAt,
		UpdatedAt:     rec.UpdatedAt,
	}
}

// pipeline_schema.go owns the static metadata served by
// GET /api/pipelines/operators. Keeping it out of pipeline_handler.go
// preserves the project's "≤ 500 lines per file" cap from the coding
// standards.

// operatorDescription returns a short human-readable line per kind. The
// frontend overrides these with i18n strings; the description is a fallback
// for tools (curl / OpenAPI generators) that need a single source of truth.
func operatorDescription(kind string) string {
	switch kind {
	case pipeline.KindFilter:
		return "Keep nodes that satisfy a boolean expression."
	case pipeline.KindMap:
		return "Rewrite a single node field with a template value."
	case pipeline.KindSort:
		return "Sort nodes by a single key, ascending or descending."
	case pipeline.KindDedupe:
		return "Remove duplicate nodes by a composite key."
	case pipeline.KindRegexRename:
		return "Apply a Go RE2 replacement to a name / tag field."
	case pipeline.KindOutput:
		return "Terminal step: declares the desired output format."
	}
	return ""
}

// operatorParamsSchema returns a hand-rolled JSON-Schema-lite descriptor for
// each operator's parameters. The shape is intentionally permissive — we use
// it for documentation, not validation (the operator factory remains the
// authoritative validator at run-time).
func operatorParamsSchema(kind string) any {
	switch kind {
	case pipeline.KindFilter:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expr": map[string]any{"type": "string"},
			},
			"required": []string{"expr"},
		}
	case pipeline.KindMap:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"field": map[string]any{"type": "string"},
				"value": map[string]any{"type": "string"},
			},
			"required": []string{"field", "value"},
		}
	case pipeline.KindSort:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key":   map[string]any{"type": "string"},
				"order": map[string]any{"type": "string", "enum": []string{"asc", "desc"}},
			},
			"required": []string{"key"},
		}
	case pipeline.KindDedupe:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fields": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
		}
	case pipeline.KindRegexRename:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":     map[string]any{"type": "string"},
				"replacement": map[string]any{"type": "string"},
				"field":       map[string]any{"type": "string", "enum": []string{"name", "tag"}},
			},
			"required": []string{"pattern", "replacement"},
		}
	case pipeline.KindOutput:
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format":    map[string]any{"type": "string", "enum": []string{"clash", "clash_meta", "raw"}},
				"max_nodes": map[string]any{"type": "integer", "minimum": 0},
			},
			"required": []string{"format"},
		}
	}
	return map[string]any{}
}
