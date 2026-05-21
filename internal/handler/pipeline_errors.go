package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"shiguang-vps/internal/pipeline"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// pipeline_errors.go owns the storage / pipeline error → ErrorCode mapping
// for /api/pipelines/*. Split from pipeline_handler.go so the latter stays
// under the 500-line cap.

// respondStorageErr translates pipeline_repo errors into the canonical envelope.
func (h *PipelineHandler) respondStorageErr(w http.ResponseWriter, traceID string, err error) {
	switch {
	case errors.Is(err, storage.ErrPipelineNotFound):
		util.RespondError(w, types.ErrNotFoundPipeline, "pipeline not found", nil, traceID)
	case errors.Is(err, storage.ErrPipelineVersionConflict):
		util.RespondError(w, types.ErrConflictPipelineVersion,
			"pipeline version conflict", nil, traceID)
	case errors.Is(err, storage.ErrSubscriptionNotFound):
		util.RespondError(w, types.ErrNotFoundSubscription, "subscription not found", nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Error("pipeline handler db failed",
				slog.String("err", err.Error()), slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalDatabase, "internal error", nil, traceID)
	}
}

// respondPipelineErr translates engine / validator / codec errors into the
// canonical envelope. The mapping is exhaustive against the sentinels in
// internal/pipeline and the contract error codes in internal/types.
func (h *PipelineHandler) respondPipelineErr(w http.ResponseWriter, traceID string, err error) {
	var pe *pipeline.PipelineError
	if errors.As(err, &pe) {
		h.respondPipelineStepErr(w, traceID, pe)
		return
	}
	switch {
	case errors.Is(err, pipeline.ErrSchemaMismatch):
		util.RespondError(w, types.ErrValidationSchemaMismatch, err.Error(), nil, traceID)
	case errors.Is(err, pipeline.ErrUnknownOperator):
		util.RespondError(w, types.ErrPipelineOperatorUnknown, err.Error(), nil, traceID)
	case errors.Is(err, pipeline.ErrOutputRequired),
		errors.Is(err, pipeline.ErrTooManyOperators),
		errors.Is(err, pipeline.ErrInvalidArgs),
		errors.Is(err, pipeline.ErrInvalidExpression):
		util.RespondError(w, types.ErrPipelineOperatorParams, err.Error(), nil, traceID)
	case errors.Is(err, pipeline.ErrInvalidRegex):
		util.RespondError(w, types.ErrValidationRegexCompile, err.Error(), nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Warn("pipeline error mapped to YAML parse",
				slog.String("err", err.Error()), slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrValidationYAMLParse, err.Error(), nil, traceID)
	}
}

// respondPipelineStepErr handles the PipelineError wrapper produced by the
// engine when an individual step's Apply fails. Separated so the parent
// function stays under the 80-line cap.
func (h *PipelineHandler) respondPipelineStepErr(w http.ResponseWriter, traceID string, pe *pipeline.PipelineError) {
	switch {
	case errors.Is(pe.Err, context.DeadlineExceeded):
		util.RespondError(w, types.ErrPipelineRunTimeout, pe.Error(), nil, traceID)
	case errors.Is(pe.Err, pipeline.ErrUnknownOperator):
		util.RespondError(w, types.ErrPipelineOperatorUnknown, pe.Error(), nil, traceID)
	case errors.Is(pe.Err, pipeline.ErrInvalidArgs),
		errors.Is(pe.Err, pipeline.ErrInvalidExpression):
		util.RespondError(w, types.ErrPipelineOperatorParams, pe.Error(), nil, traceID)
	case errors.Is(pe.Err, pipeline.ErrInvalidRegex):
		util.RespondError(w, types.ErrValidationRegexCompile, pe.Error(), nil, traceID)
	default:
		util.RespondError(w, types.ErrPipelineOperatorParams, pe.Error(), nil, traceID)
	}
}
