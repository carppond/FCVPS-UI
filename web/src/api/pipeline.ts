import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  CreatePipelineRequest,
  PagedResponse,
  Pipeline,
  RunPipelineRequest,
  RunPipelineResponse,
  UpdatePipelineRequest,
} from "@/types/api";

// ─── Pipeline CRUD ───────────────────────────────────────────────────────────

export interface ListPipelinesParams {
  page?: number;
  pageSize?: number;
  keyword?: string;
}

function buildPipelinesQuery(params: ListPipelinesParams): string {
  const search = new URLSearchParams();
  if (params.page !== undefined) search.set("page", String(params.page));
  if (params.pageSize !== undefined)
    search.set("page_size", String(params.pageSize));
  if (params.keyword) search.set("keyword", params.keyword);
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

/** GET /api/pipelines — paginated list of pipelines owned by the current user. */
export function usePipelines(params: ListPipelinesParams = {}) {
  return useQuery({
    queryKey: [...queryKeys.pipeline.list(), params],
    queryFn: () =>
      apiFetch<PagedResponse<Pipeline>>(
        `/api/pipelines${buildPipelinesQuery(params)}`,
      ),
  });
}

/** GET /api/pipelines/:id — detail (yaml_content + ast_json). */
export function usePipeline(id: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.pipeline.detail(id),
    queryFn: () => apiFetch<Pipeline>(`/api/pipelines/${id}`),
    enabled: enabled && !!id,
  });
}

/** POST /api/pipelines — create a new pipeline. */
export function useCreatePipeline() {
  return useMutation({
    mutationFn: (payload: CreatePipelineRequest) =>
      apiFetch<Pipeline>("/api/pipelines", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.pipeline.list() });
    },
  });
}

/** PUT /api/pipelines/:id — full save (yaml + ast + version). */
export function useUpdatePipeline() {
  return useMutation({
    mutationFn: ({
      id,
      payload,
    }: {
      id: string;
      payload: UpdatePipelineRequest;
    }) =>
      apiFetch<Pipeline>(`/api/pipelines/${id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      }),
    onSuccess: (pipeline) => {
      queryClient.setQueryData(
        queryKeys.pipeline.detail(pipeline.id),
        pipeline,
      );
      queryClient.invalidateQueries({ queryKey: queryKeys.pipeline.list() });
    },
  });
}

/** DELETE /api/pipelines/:id — remove a pipeline. */
export function useDeletePipeline() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<null>(`/api/pipelines/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.pipeline.list() });
    },
  });
}

// ─── Preview / YAML round-trip ───────────────────────────────────────────────

/**
 * POST /api/pipelines/:id/run — debug preview against a real subscription.
 * Returns per-operator input/output counts + node-id diffs. The full debug
 * dialog is implemented in T-21; this hook is wired up here so the canvas
 * page can populate the editor store once T-21 lands.
 */
export function useRunPreview() {
  return useMutation({
    mutationFn: ({
      id,
      payload,
    }: {
      id: string;
      payload: RunPipelineRequest;
    }) =>
      apiFetch<RunPipelineResponse>(`/api/pipelines/${id}/run`, {
        method: "POST",
        body: JSON.stringify({ ...payload, debug: payload.debug ?? true }),
      }),
  });
}

/** POST /api/pipelines/yaml-to-ast — stateless YAML → AST conversion. */
export function useParseYaml() {
  return useMutation({
    mutationFn: (yaml_content: string) =>
      apiFetch<{ ast_json: string }>("/api/pipelines/yaml-to-ast", {
        method: "POST",
        body: JSON.stringify({ yaml_content }),
      }),
  });
}

/** POST /api/pipelines/ast-to-yaml — stateless AST → YAML conversion. */
export function useToYaml() {
  return useMutation({
    mutationFn: (ast_json: string) =>
      apiFetch<{ yaml_content: string }>("/api/pipelines/ast-to-yaml", {
        method: "POST",
        body: JSON.stringify({ ast_json }),
      }),
  });
}
