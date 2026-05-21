/**
 * M-OPS Audit API client (T-28).
 *
 * Endpoints covered:
 *   GET /api/admin/audit?user_id=&action=&from=&to=&page=&page_size=
 *   GET /api/audit/logs   (user-scoped variant; same response shape)
 */
import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import type { AuditLog } from "@/types/api";

export interface AuditListParams {
  page?: number;
  pageSize?: number;
  userId?: string;
  action?: string;
  from?: number; // unix ms
  to?: number;
}

export interface AuditListResponse {
  items: AuditLog[];
  total: number;
  page: number;
  page_size: number;
}

const auditKeys = {
  all: () => ["audit"] as const,
  list: (params: AuditListParams) => ["audit", "list", params] as const,
};

function buildQuery(params: AuditListParams): string {
  const search = new URLSearchParams();
  if (params.page !== undefined) search.set("page", String(params.page));
  if (params.pageSize !== undefined) search.set("page_size", String(params.pageSize));
  if (params.userId) search.set("user_id", params.userId);
  if (params.action) search.set("action", params.action);
  if (params.from !== undefined) search.set("from", String(params.from));
  if (params.to !== undefined) search.set("to", String(params.to));
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

/** GET /api/admin/audit — admin sees every row. */
export function useAdminAuditLogs(params: AuditListParams = {}) {
  return useQuery({
    queryKey: auditKeys.list(params),
    queryFn: () =>
      apiFetch<AuditListResponse>(`/api/admin/audit${buildQuery(params)}`),
  });
}

/** GET /api/audit/logs — user-scoped fallback. */
export function useMyAuditLogs(params: AuditListParams = {}) {
  return useQuery({
    queryKey: [...auditKeys.list(params), "self"] as const,
    queryFn: () =>
      apiFetch<AuditListResponse>(`/api/audit/logs${buildQuery(params)}`),
  });
}
