import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  CreateScriptRequest,
  HookType,
  PagedResponse,
  Script,
  ScriptLog,
  UpdateScriptRequest,
} from "@/types/api";

// The /test endpoint returns a richer envelope than the contract DTO
// (logs + duration_ms + error). We declare the local shape here because the
// contract type ScriptTestResponse omits the logs[] channel the editor UI
// needs to surface console.log output.
export interface ScriptTestResult {
  output: string;
  logs: string[];
  duration_ms: number;
  error?: string;
}

export interface ListScriptsParams {
  page?: number;
  pageSize?: number;
  hook?: HookType | "";
  keyword?: string;
}

function buildScriptsQuery(params: ListScriptsParams): string {
  const search = new URLSearchParams();
  if (params.page !== undefined) search.set("page", String(params.page));
  if (params.pageSize !== undefined)
    search.set("page_size", String(params.pageSize));
  if (params.hook) search.set("hook", params.hook);
  if (params.keyword) search.set("keyword", params.keyword);
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

/** GET /api/scripts — paginated list of scripts owned by the current user. */
export function useScripts(params: ListScriptsParams = {}) {
  return useQuery({
    queryKey: [...queryKeys.script.list(), params],
    queryFn: () =>
      apiFetch<PagedResponse<Script>>(`/api/scripts${buildScriptsQuery(params)}`),
  });
}

/** GET /api/scripts/:id */
export function useScript(id: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.script.detail(id),
    queryFn: () => apiFetch<Script>(`/api/scripts/${id}`),
    enabled: enabled && !!id,
  });
}

/** POST /api/scripts */
export function useCreateScript() {
  return useMutation({
    mutationFn: (payload: CreateScriptRequest) =>
      apiFetch<Script>("/api/scripts", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.script.list() });
    },
  });
}

/** PATCH /api/scripts/:id (PUT is also accepted; we standardise on PATCH). */
export function useUpdateScript() {
  return useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateScriptRequest }) =>
      apiFetch<Script>(`/api/scripts/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: (script) => {
      queryClient.setQueryData(queryKeys.script.detail(script.id), script);
      queryClient.invalidateQueries({ queryKey: queryKeys.script.list() });
    },
  });
}

/** DELETE /api/scripts/:id */
export function useDeleteScript() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<null>(`/api/scripts/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.script.list() });
    },
  });
}

/**
 * POST /api/scripts/:id/test — runs the persisted script against a sample
 * input WITHOUT writing the input/output to the DB. The handler does stamp
 * last_run_at + last_error so the list view's "last status" column updates;
 * we therefore invalidate the list query on success.
 */
export function useTestScript() {
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input?: unknown }) =>
      apiFetch<ScriptTestResult>(`/api/scripts/${id}/test`, {
        method: "POST",
        body: JSON.stringify({ input: input ?? {} }),
      }),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.script.list() });
      queryClient.invalidateQueries({
        queryKey: queryKeys.script.detail(vars.id),
      });
    },
  });
}

/** GET /api/scripts/:id/logs — read the cached last-run log entry. */
export function useScriptLogs(id: string, enabled = true) {
  return useQuery({
    queryKey: [...queryKeys.script.detail(id), "logs"],
    queryFn: () => apiFetch<ScriptLog[]>(`/api/scripts/${id}/logs`),
    enabled: enabled && !!id,
  });
}
