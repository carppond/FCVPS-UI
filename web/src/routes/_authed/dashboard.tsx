/**
 * T-29: Dashboard route.
 *
 * Two-row layout:
 *   - Stats row (four cards).
 *   - 2×2 grid: traffic chart | agent status • recent events | sub health.
 *
 * The dashboard locale namespace is lazy-loaded on mount so the first-screen
 * bundle (common + errors + auth) stays minimal per docs §2.3.
 */
import { useEffect } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import i18n from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { RecentEvents } from "@/components/dashboard/recent-events";
import { SubHealth } from "@/components/dashboard/sub-health";
import { AgentStatusList } from "@/components/dashboard/agent-status-list";
import { TrafficChart } from "@/components/traffic/traffic-chart";
import { useAuthStore } from "@/stores/auth-store";
import zhCNDashboard from "@/locales/zh-CN/dashboard.json";
import enDashboard from "@/locales/en/dashboard.json";
import jaDashboard from "@/locales/ja/dashboard.json";
import koDashboard from "@/locales/ko/dashboard.json";

export const Route = createFileRoute("/_authed/dashboard")({
  component: DashboardPage,
});

function ensureDashboardBundles() {
  if (!i18n.hasResourceBundle("zh-CN", "dashboard")) {
    i18n.addResourceBundle("zh-CN", "dashboard", zhCNDashboard, true, true);
  }
  if (!i18n.hasResourceBundle("en", "dashboard")) {
    i18n.addResourceBundle("en", "dashboard", enDashboard, true, true);
  }
  if (!i18n.hasResourceBundle("ja", "dashboard")) {
    i18n.addResourceBundle("ja", "dashboard", jaDashboard, true, true);
  }
  if (!i18n.hasResourceBundle("ko", "dashboard")) {
    i18n.addResourceBundle("ko", "dashboard", koDashboard, true, true);
  }
}

function DashboardPage() {
  const { t } = useTranslation(["dashboard", "common"]);
  const { user } = useAuthStore();

  useEffect(() => {
    ensureDashboardBundles();
  }, []);

  return (
    <div className="flex flex-col gap-[var(--spacing-6)]">
      <header className="flex flex-col gap-[var(--spacing-2)] sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
            {t("dashboard:page.welcome", { name: user?.username ?? "" })}
          </h1>
          <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("dashboard:page.subtitle")}
          </p>
        </div>
        <Button asChild>
          <Link to={"/subscriptions" as unknown as "/"}>
            <Plus className="mr-2 h-4 w-4" />
            {t("dashboard:actions.create_subscription")}
          </Link>
        </Button>
      </header>

      <StatsCards />

      <div className="grid grid-cols-1 gap-[var(--spacing-4)] lg:grid-cols-2">
        <TrafficChart />
        <AgentStatusList />
        <RecentEvents />
        <SubHealth />
      </div>
    </div>
  );
}
