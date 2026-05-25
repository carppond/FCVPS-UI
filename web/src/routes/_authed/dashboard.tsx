/**
 * Dashboard v4 — Redesigned dashboard route.
 *
 * Three-section layout:
 *   1. Hero: featured online probe with real-time stats + SVG ring charts
 *   2. Stats: 4 feature-entry cards (subscriptions / nodes / traffic / alerts)
 *   3. Bottom: traffic bar chart (3fr) + recent events list (2fr)
 *
 * The dashboard locale namespace is lazy-loaded on mount so the first-screen
 * bundle (common + errors + auth) stays minimal per docs section 2.3.
 */
import { useEffect } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import i18n from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { HeroProbe } from "@/components/dashboard/hero-probe";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { DashboardTrafficChart } from "@/components/dashboard/traffic-chart";
import { RecentEvents } from "@/components/dashboard/recent-events";
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

  useEffect(() => {
    ensureDashboardBundles();
  }, []);

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Page header */}
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-bold tracking-tight text-[var(--color-text-primary)]">
            {t("dashboard:page.title")}
          </h1>
        </div>
        <div className="flex gap-1.5">
          <Button asChild>
            <Link to={"/subscriptions" as unknown as "/"}>
              <Plus className="mr-2 h-4 w-4" />
              {t("dashboard:actions.create_subscription")}
            </Link>
          </Button>
        </div>
      </header>

      {/* Section 1: Hero probe */}
      <HeroProbe />

      {/* Section 2: 4 stat cards */}
      <StatsCards />

      {/* Section 3: Bottom two columns (3:2) */}
      <div className="grid grid-cols-1 gap-3.5 lg:grid-cols-[3fr_2fr]">
        <DashboardTrafficChart />
        <RecentEvents />
      </div>
    </div>
  );
}
