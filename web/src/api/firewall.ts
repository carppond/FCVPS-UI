import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import type { FirewallStatusResponse } from "@/types/api";

// ─── Query keys ──────────────────────────────────────────────────────────────
// Firewall is admin-only and self-contained; allow/delete mutations prime the
// status cache from their response so the list refreshes without a refetch.

const firewallKeys = {
  all: () => ["firewall"] as const,
  status: () => ["firewall", "status"] as const,
};

/** GET /api/admin/firewall/status — ufw status + allow-rule list + note. */
export function useFirewallStatus() {
  return useQuery({
    queryKey: firewallKeys.status(),
    queryFn: () => apiFetch<FirewallStatusResponse>("/api/admin/firewall/status"),
  });
}

interface PortPayload {
  port: number;
  proto: "tcp" | "udp";
}

/** POST /api/admin/firewall/allow — open a port. Returns refreshed status. */
export function useAllowPort() {
  return useMutation({
    mutationFn: (payload: PortPayload) =>
      apiFetch<FirewallStatusResponse>("/api/admin/firewall/allow", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: (data) => {
      queryClient.setQueryData(firewallKeys.status(), data);
    },
  });
}

/**
 * POST /api/admin/firewall/delete — remove an allow-rule by port/proto. The
 * backend refuses protected ports (SSH / panel access) with 403.
 */
export function useDeletePort() {
  return useMutation({
    mutationFn: (payload: { port: number; proto: "tcp" | "udp" | "" }) =>
      apiFetch<FirewallStatusResponse>("/api/admin/firewall/delete", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: (data) => {
      queryClient.setQueryData(firewallKeys.status(), data);
    },
  });
}
