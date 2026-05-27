import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { PagedResponse, User, AuditLog, OTAReleaseInfo, OTAHistoryItem } from "../types/api";

export function useUsersQuery() {
  return useQuery({
    queryKey: ["admin", "users"],
    queryFn: () => apiFetch<PagedResponse<User>>("/api/admin/users?page=1&page_size=100"),
  });
}

export function useAuditLogs() {
  return useQuery({
    queryKey: ["admin", "audit"],
    queryFn: () => apiFetch<PagedResponse<AuditLog>>("/api/admin/audit?page=1&page_size=50"),
  });
}

export function useOtaStatus() {
  return useQuery({
    queryKey: ["admin", "ota"],
    queryFn: () => apiFetch<OTAReleaseInfo>("/api/admin/ota/status"),
  });
}

export function useOtaHistory() {
  return useQuery({
    queryKey: ["admin", "ota-history"],
    queryFn: () => apiFetch<OTAHistoryItem[]>("/api/admin/ota/history"),
  });
}
