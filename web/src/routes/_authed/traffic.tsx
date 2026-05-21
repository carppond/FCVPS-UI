import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { ThresholdConfig } from "@/components/traffic/threshold-config";
import { TrafficChart } from "@/components/traffic/traffic-chart";
import { UsageProgress } from "@/components/traffic/usage-progress";
import { useTrafficByAgentQuery, useTrafficSummaryQuery } from "@/api/traffic";
import { useAuthStore } from "@/stores/auth-store";
import i18n from "@/lib/i18n";
import zhCNTraffic from "@/locales/zh-CN/traffic.json";
import enTraffic from "@/locales/en/traffic.json";
import jaTraffic from "@/locales/ja/traffic.json";
import koTraffic from "@/locales/ko/traffic.json";

// Lazy-register the "traffic" namespace before the route mounts. Mirrors the
// pattern used by /nodes so the first-screen bundle stays slim.
function ensureTrafficNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "traffic")) {
    i18n.addResourceBundle("zh-CN", "traffic", zhCNTraffic, true, true);
    i18n.addResourceBundle("en", "traffic", enTraffic, true, true);
    i18n.addResourceBundle("ja", "traffic", jaTraffic, true, true);
    i18n.addResourceBundle("ko", "traffic", koTraffic, true, true);
  }
}

export const Route = createFileRoute("/_authed/traffic")({
  beforeLoad: () => {
    ensureTrafficNamespace();
  },
  component: TrafficPage,
});

function TrafficPage() {
  const { t } = useTranslation(["traffic", "common"]);
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";

  const summaryQ = useTrafficSummaryQuery();
  const byAgentQ = useTrafficByAgentQuery();

  return (
    <div className="flex flex-col gap-[var(--spacing-6)] p-[var(--spacing-6)]">
      <header className="flex flex-col gap-[var(--spacing-1)]">
        <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
          {t("traffic:title")}
        </h1>
        <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("traffic:subtitle")}
        </p>
      </header>

      {/* Hero: this-month usage */}
      <section className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-6)]">
        {summaryQ.isLoading ? (
          <Skeleton className="h-24 w-full" />
        ) : summaryQ.isError || !summaryQ.data ? (
          <ErrorState
            message={t("traffic:error.load_failed")}
            onRetry={() => void summaryQ.refetch()}
          />
        ) : (
          <UsageProgress
            used={summaryQ.data.total_used}
            limit={summaryQ.data.total_limit ?? 0}
            daysToReset={daysUntil(summaryQ.data.period_end)}
          />
        )}
      </section>

      {/* Mid: chart */}
      <section>
        <TrafficChart />
      </section>

      {/* Bottom: per-agent breakdown + threshold config */}
      <section className="grid grid-cols-1 gap-[var(--spacing-6)] lg:grid-cols-2">
        <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]">
          <h3 className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]">
            {t("traffic:by_agent.title")}
          </h3>
          {byAgentQ.isLoading ? (
            <Skeleton className="mt-[var(--spacing-3)] h-32 w-full" />
          ) : !byAgentQ.data || byAgentQ.data.length === 0 ? (
            <p className="mt-[var(--spacing-4)] text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
              {t("traffic:by_agent.empty")}
            </p>
          ) : (
            <ul className="mt-[var(--spacing-3)] flex flex-col gap-[var(--spacing-2)]">
              {byAgentQ.data.map((a) => {
                const totalUsed =
                  byAgentQ.data!.reduce((acc, x) => acc + x.total_used, 0) || 1;
                const share = (a.total_used / totalUsed) * 100;
                return (
                  <li
                    key={a.agent_id}
                    className="flex items-center justify-between rounded-[var(--radius-md)] border border-[var(--color-border)] p-[var(--spacing-3)]"
                  >
                    <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
                      {a.agent_name || a.agent_id}
                    </span>
                    <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                      {formatBytes(a.total_used)} ·{" "}
                      {t("traffic:by_agent.share_percent", {
                        percent: share.toFixed(1),
                      })}
                    </span>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        <ThresholdConfig
          isAdmin={Boolean(isAdmin)}
          currentLimit={summaryQ.data?.total_limit ?? 0}
        />
      </section>
    </div>
  );
}

function daysUntil(dateISO: string | undefined): number {
  if (!dateISO) return 0;
  const target = new Date(dateISO + "T00:00:00Z");
  if (Number.isNaN(target.getTime())) return 0;
  const now = new Date();
  const diff = target.getTime() - now.getTime();
  return Math.max(0, Math.ceil(diff / (1000 * 60 * 60 * 24)));
}

function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 2)} ${units[i]}`;
}
