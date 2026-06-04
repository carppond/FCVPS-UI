/**
 * Dashboard v4 — Hero probe section.
 *
 * Displays the first online agent's real-time metrics:
 *   - Name + pulsing green dot + meta info (os/arch/uptime/total probes)
 *   - 4 stat tiles: net up / net down / disk / connections
 *   - 2 SVG ring charts: CPU% / Memory%
 *
 * Falls back to an empty state when no agents are online.
 */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { useAgentsQuery } from "@/api/agent";
import { formatBitrate, formatBytes, formatUptime } from "@/lib/format";
import type { AgentListItem } from "@/types/api";

// ── SVG Ring Chart ──────────────────────────────────────────────────────────

interface RingProps {
  value: number; // 0..100
  label: string;
  color: string; // CSS variable like var(--color-primary)
  glowColor: string; // rgba for the drop-shadow
}

const RING_RADIUS = 58;
const RING_CIRCUMFERENCE = 2 * Math.PI * RING_RADIUS; // ~364.42

function Ring({ value, label, color, glowColor }: RingProps) {
  const clamped = Math.max(0, Math.min(100, value));
  const dashLen = (clamped / 100) * RING_CIRCUMFERENCE;
  const gapLen = RING_CIRCUMFERENCE - dashLen;

  return (
    <div className="relative flex h-[140px] w-[140px] flex-shrink-0 flex-col items-center justify-center">
      <svg
        width="140"
        height="140"
        viewBox="0 0 140 140"
        className="absolute inset-0 -rotate-90"
      >
        <circle
          cx="70"
          cy="70"
          r={RING_RADIUS}
          fill="none"
          stroke="var(--color-border)"
          strokeWidth="7"
        />
        <circle
          cx="70"
          cy="70"
          r={RING_RADIUS}
          fill="none"
          stroke={color}
          strokeWidth="7"
          strokeDasharray={`${dashLen} ${gapLen}`}
          strokeLinecap="round"
          style={{ filter: `drop-shadow(0 0 6px ${glowColor})` }}
        />
      </svg>
      <span
        className="text-[30px] font-bold tabular-nums leading-none"
        style={{ color }}
      >
        {clamped.toFixed(clamped >= 100 ? 0 : 1)}
      </span>
      <span className="mt-1 text-[10px] font-semibold uppercase tracking-wider text-[var(--color-text-tertiary)]">
        {label}
      </span>
    </div>
  );
}

// ── Stat tile ───────────────────────────────────────────────────────────────

interface HeroStatProps {
  label: string;
  children: React.ReactNode;
}

function HeroStat({ label, children }: HeroStatProps) {
  return (
    <div className="flex min-h-[100px] flex-1 flex-col items-center justify-center rounded-[10px] bg-[var(--color-bg-elevated)] px-4 py-3.5 text-center">
      <div className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-[var(--color-text-tertiary)]">
        {label}
      </div>
      {children}
    </div>
  );
}

// ── Main component ──────────────────────────────────────────────────────────

export function HeroProbe() {
  const { t, i18n } = useTranslation("dashboard");
  const query = useAgentsQuery({ page: 1, pageSize: 200 });
  const items = useMemo<AgentListItem[]>(() => query.data?.items ?? [], [query.data]);

  const totalAgents = items.length;
  const onlineAgents = items.filter(
    (a) => a.online || a.status === "online",
  ).length;

  // Pick the first online agent to feature
  const featured = useMemo(
    () => items.find((a) => a.online || a.status === "online") ?? null,
    [items],
  );

  const m = featured?.latest_metrics;

  const uptimeUnits = useMemo(() => {
    const lang = i18n.language;
    if (lang.startsWith("zh") || lang === "ja" || lang === "ko") {
      return { day: "天", hour: "小时", minute: "分" };
    }
    return { day: "d", hour: "h", minute: "m" };
  }, [i18n.language]);

  // Loading
  if (query.isLoading) {
    return (
      <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6">
        <Skeleton className="mb-4 h-6 w-40" />
        <div className="flex gap-3.5">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-[100px] flex-1" />
          ))}
          <Skeleton className="h-[140px] w-[140px] rounded-full" />
          <Skeleton className="h-[140px] w-[140px] rounded-full" />
        </div>
      </div>
    );
  }

  // Empty state: no online agents
  if (!featured) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] px-6 py-10">
        <p className="text-sm text-[var(--color-text-tertiary)]">
          {t("hero.no_agent")}
        </p>
        <Link
          to={"/agents" as unknown as "/"}
          className="inline-flex items-center gap-1.5 text-sm font-medium text-[var(--color-primary)] hover:underline"
        >
          <Plus className="h-4 w-4" />
          {t("hero.no_agent_hint")}
        </Link>
      </div>
    );
  }

  const cpuPercent = m?.cpu_percent ?? 0;
  const memPercent =
    m && m.mem_total > 0 ? (m.mem_used / m.mem_total) * 100 : 0;
  const diskPercent =
    m && m.disk_total > 0 ? (m.disk_used / m.disk_total) * 100 : 0;
  const connTotal = (m?.conn_tcp ?? 0) + (m?.conn_udp ?? 0);

  const allOnlineText =
    onlineAgents >= totalAgents
      ? t("hero.all_online")
      : t("hero.partial_online", { online: onlineAgents });

  const metaText = t("hero.meta", {
    os: featured.os ?? "—",
    arch: featured.arch ?? "—",
    version: featured.version ?? "—",
    uptime: m ? formatUptime(m.uptime, uptimeUnits) : "—",
    total: totalAgents,
    allOnline: allOnlineText,
  });

  return (
    <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6 backdrop-blur-2xl">
      {/* Top: name + pulsing dot */}
      <div className="mb-1.5 flex items-center gap-2">
        <span className="h-2.5 w-2.5 animate-pulse rounded-full bg-[var(--color-success)] shadow-[0_0_10px_var(--color-success)]" />
        <h2 className="text-lg font-semibold text-[var(--color-text-primary)]">
          {featured.name}
        </h2>
      </div>

      {/* Meta line */}
      <p className="mb-4 text-xs text-[var(--color-text-tertiary)]">
        {metaText}
      </p>

      {/* Row: 4 stats + 2 rings */}
      <div className="flex items-center gap-3.5">
        {/* 4 stat tiles */}
        <HeroStat label={t("hero.net_up")}>
          <div className="text-xl font-bold tabular-nums text-[var(--color-info)]">
            {formatBitrate(m?.net_out_speed ?? 0)}
          </div>
        </HeroStat>

        <HeroStat label={t("hero.net_down")}>
          <div className="text-xl font-bold tabular-nums text-[var(--color-success)]">
            {formatBitrate(m?.net_in_speed ?? 0)}
          </div>
        </HeroStat>

        <HeroStat label={t("hero.disk")}>
          <div className="text-xl font-bold tabular-nums text-[var(--color-text-primary)]">
            {formatBytes(m?.disk_used ?? 0)}
          </div>
          <div className="mt-1.5 h-1 w-full overflow-hidden rounded-sm bg-[var(--color-border)]">
            <div
              className="h-full rounded-sm bg-[var(--color-info)]"
              style={{ width: `${Math.min(100, diskPercent)}%` }}
            />
          </div>
          <div className="mt-1 text-[10px] text-[var(--color-text-tertiary)]">
            / {formatBytes(m?.disk_total ?? 0)}
          </div>
        </HeroStat>

        <HeroStat label={t("hero.connections")}>
          <div className="text-xl font-bold tabular-nums text-[var(--color-text-primary)]">
            {connTotal}
          </div>
          <div className="mt-1 text-[10px] text-[var(--color-text-tertiary)]">
            TCP {m?.conn_tcp ?? 0} · UDP {m?.conn_udp ?? 0}
          </div>
        </HeroStat>

        {/* 2 ring charts */}
        <Ring
          value={cpuPercent}
          label={t("hero.cpu")}
          color="var(--color-primary)"
          glowColor="rgba(255,99,99,.4)"
        />
        <Ring
          value={memPercent}
          label={t("hero.memory")}
          color="var(--color-warning)"
          glowColor="rgba(255,159,10,.3)"
        />
      </div>
    </div>
  );
}
