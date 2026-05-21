/**
 * T-29: Agent status panel.
 *
 * Compact, grouped list of agents bucketed by status (online / degraded /
 * offline / unknown). Each row shows the agent name, last-seen and a coloured
 * dot — clicking jumps to the agent detail.
 *
 * The list is sourced from `useAgentsQuery` and live-updated by subscribing
 * to the `/api/notify/stream` SSE feed; both the canonical `agent_status`
 * event and the legacy `agent_status_change` alias are honoured so any hub
 * variant pushes through.
 */
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/cn";
import { useAgentsQuery } from "@/api/agent";
import { useEventStream, type SSEHandlers } from "@/hooks/use-event-stream";
import type {
  AgentListItem,
  AgentStatus,
  SSEAgentStatusPayload,
} from "@/types/api";

const STATUS_ORDER: ReadonlyArray<AgentStatus | "unknown"> = [
  "online",
  "degraded",
  "offline",
  "unknown",
] as const;

function statusDotClass(status: AgentStatus | "unknown"): string {
  switch (status) {
    case "online":
      return "bg-[var(--color-status-online)]";
    case "degraded":
      return "bg-[var(--color-status-degraded)]";
    case "offline":
      return "bg-[var(--color-status-offline)]";
    case "unknown":
    default:
      return "bg-[var(--color-status-unknown)]";
  }
}

function statusLabelKey(status: AgentStatus | "unknown"): string {
  return `grid.agents.${status}`;
}

interface Bucket {
  status: AgentStatus | "unknown";
  agents: AgentListItem[];
}

function bucketize(items: AgentListItem[]): Bucket[] {
  const map = new Map<AgentStatus | "unknown", AgentListItem[]>();
  for (const a of items) {
    const key: AgentStatus | "unknown" = a.status ?? "unknown";
    const arr = map.get(key) ?? [];
    arr.push(a);
    map.set(key, arr);
  }
  return STATUS_ORDER.filter((s) => (map.get(s)?.length ?? 0) > 0).map((s) => ({
    status: s,
    agents: map.get(s) ?? [],
  }));
}

/** Live status panel. */
export function AgentStatusList() {
  const { t } = useTranslation("dashboard");
  const query = useAgentsQuery({ page: 1, pageSize: 100 });

  // SSE overrides — apply on top of the polled list so we never lose data
  // when the user blurs the tab and TanStack Query stops refetching.
  const [overrides, setOverrides] = useState<Record<string, AgentStatus>>({});
  const handlers = useMemo<SSEHandlers>(
    () => ({
      agent_status: (payload: unknown) => {
        const p = payload as SSEAgentStatusPayload | null;
        if (!p?.agent_id) return;
        setOverrides((prev) => ({ ...prev, [p.agent_id]: p.status }));
      },
      agent_status_change: (payload: unknown) => {
        const p = payload as SSEAgentStatusPayload | null;
        if (!p?.agent_id) return;
        setOverrides((prev) => ({ ...prev, [p.agent_id]: p.status }));
      },
    }),
    [],
  );
  useEventStream("/api/notify/stream", handlers);

  const mergedItems = useMemo(() => {
    const items = query.data?.items ?? [];
    if (Object.keys(overrides).length === 0) return items;
    return items.map((a) =>
      overrides[a.id] ? { ...a, status: overrides[a.id]! } : a,
    );
  }, [query.data, overrides]);

  const buckets = useMemo(() => bucketize(mergedItems), [mergedItems]);

  return (
    <Card className="flex h-full flex-col gap-[var(--spacing-3)] p-[var(--spacing-4)]">
      <header className="flex items-start justify-between">
        <div className="flex flex-col">
          <h3 className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]">
            {t("grid.agents.title")}
          </h3>
          <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("grid.agents.subtitle")}
          </span>
        </div>
        <Link
          to={"/agents" as unknown as "/"}
          className="flex items-center gap-1 text-[var(--font-size-xs)] text-[var(--color-primary)] hover:underline"
        >
          {t("grid.agents.see_all")}
          <ChevronRight className="h-3 w-3" />
        </Link>
      </header>

      {query.isLoading ? (
        <div className="flex flex-col gap-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-7 w-full" />
          ))}
        </div>
      ) : query.isError ? (
        <ErrorState message={t("error.load_failed")} />
      ) : buckets.length === 0 ? (
        <EmptyState title={t("grid.agents.no_agents")} />
      ) : (
        <div className="flex max-h-72 flex-col gap-3 overflow-y-auto">
          {buckets.map((bucket) => (
            <section key={bucket.status}>
              <div className="mb-1 flex items-center justify-between">
                <span className="text-[var(--font-size-xs)] uppercase tracking-wider text-[var(--color-text-tertiary)]">
                  {t(statusLabelKey(bucket.status))}
                </span>
                <span className="font-mono text-[var(--font-size-xs)] tabular-nums text-[var(--color-text-disabled)]">
                  {bucket.agents.length}
                </span>
              </div>
              <ul className="flex flex-col">
                {bucket.agents.map((agent) => (
                  <li
                    key={agent.id}
                    className="flex items-center gap-2 border-b border-[var(--color-border)] py-[var(--spacing-2)] last:border-b-0"
                  >
                    <span
                      aria-hidden
                      className={cn(
                        "h-2 w-2 shrink-0 rounded-full",
                        statusDotClass(bucket.status),
                      )}
                    />
                    <span className="grow truncate text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
                      {agent.name}
                    </span>
                    <span className="shrink-0 font-mono text-[var(--font-size-xs)] tabular-nums text-[var(--color-text-tertiary)]">
                      {agent.public_ip ?? "—"}
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </div>
      )}
    </Card>
  );
}
