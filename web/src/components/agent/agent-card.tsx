import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { Cpu, MemoryStick } from "lucide-react";
import { cn } from "@/lib/cn";
import { formatBytes, formatRelativeTime } from "@/lib/format";
import type { AgentListItem } from "@/types/api";
import { AgentStatusDot } from "./agent-status-dot";
import { AgentKindBadge } from "./agent-kind-badge";

interface AgentCardProps {
  agent: AgentListItem;
  /** Linkable variant — used by Dashboard (T-29) for quick navigation. */
  href?: string;
  className?: string;
}

/**
 * Compact card showing the current state of one agent. Designed to be
 * embedded both in the Dashboard's "quick glance" row and in the agent
 * list page's grid view. Looks identical regardless of role — admin
 * affordances live on the detail page.
 */
export function AgentCard({ agent, href, className }: AgentCardProps) {
  const { t } = useTranslation("agent");
  const m = agent.latest_metrics;
  const memPct =
    m && m.mem_total > 0 ? Math.min(100, (m.mem_used / m.mem_total) * 100) : 0;
  const offlineLabel = agent.last_seen_at
    ? t("card.offline_hint", {
        when: formatRelativeTime(agent.last_seen_at),
      })
    : t("card.no_metrics_hint");

  const inner = (
    <div
      className={cn(
        "flex flex-col gap-3 rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4",
        "transition-colors duration-[var(--duration-fast)] hover:border-[var(--color-border-strong)]",
        className,
      )}
    >
      <header className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex flex-col gap-1">
          <p className="truncate text-[var(--font-size-base)] font-semibold text-[var(--color-text-primary)]">
            {agent.name}
          </p>
          <div className="flex items-center gap-2">
            <AgentKindBadge kind={agent.kind} />
            <AgentStatusDot status={agent.status} withLabel />
          </div>
        </div>
      </header>

      {agent.status === "online" && m ? (
        <div className="grid grid-cols-2 gap-2 tabular-nums">
          <Metric
            icon={<Cpu className="h-3.5 w-3.5" />}
            label={t("card.cpu")}
            value={`${m.cpu_percent.toFixed(1)}%`}
          />
          <Metric
            icon={<MemoryStick className="h-3.5 w-3.5" />}
            label={t("card.memory")}
            value={`${memPct.toFixed(0)}% (${formatBytes(m.mem_used)})`}
          />
        </div>
      ) : (
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {offlineLabel}
        </p>
      )}
    </div>
  );

  if (!href) return inner;
  return (
    <Link to={href} className="block">
      {inner}
    </Link>
  );
}

function Metric({
  icon,
  label,
  value,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="flex items-center gap-1 text-[var(--font-size-xs)] uppercase tracking-wide text-[var(--color-text-tertiary)]">
        {icon}
        {label}
      </span>
      <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
        {value}
      </span>
    </div>
  );
}
