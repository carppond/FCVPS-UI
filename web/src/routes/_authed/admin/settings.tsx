import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { useMeQuery } from "@/api/user";
import { useSettings } from "@/api/settings";
import { SilentModeSection } from "@/components/admin/silent-mode-section";
import { BackupSection } from "@/components/admin/backup-section";
import {
  SettingsForm,
  type SettingsFieldDescriptor,
} from "@/components/admin/settings-form";
import i18n from "@/lib/i18n";
import settingsZh from "@/locales/zh-CN/settings.json";
import settingsEn from "@/locales/en/settings.json";
import settingsJa from "@/locales/ja/settings.json";
import settingsKo from "@/locales/ko/settings.json";

// Lazy-register the "settings" namespace before mount; mirrors /rules and
// /nodes — first-screen bundle stays under the 30 KB budget called out in
// tech-lead-plan §2.3.
function ensureSettingsNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "settings")) {
    i18n.addResourceBundle("zh-CN", "settings", settingsZh, true, true);
    i18n.addResourceBundle("en", "settings", settingsEn, true, true);
    i18n.addResourceBundle("ja", "settings", settingsJa, true, true);
    i18n.addResourceBundle("ko", "settings", settingsKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/admin/settings")({
  beforeLoad: () => {
    ensureSettingsNamespace();
  },
  component: AdminSettingsPage,
});

// Field descriptors per-tab. Centralised here so the order on screen is
// obvious at a glance; the SettingsForm component reads each descriptor at
// runtime to render labels + hints + validators.
const ACCOUNT_FIELDS: SettingsFieldDescriptor[] = [
  {
    key: "session_ttl_seconds",
    labelKey: "account.session_ttl_label",
    hintKey: "account.session_ttl_hint",
    inputMode: "number",
    min: 60,
    max: 2592000,
  },
  {
    key: "default_locale",
    labelKey: "account.default_locale_label",
    hintKey: "account.default_locale_hint",
  },
];

const TRAFFIC_FIELDS: SettingsFieldDescriptor[] = [
  {
    key: "monthly_reset_day",
    labelKey: "traffic.monthly_reset_day_label",
    hintKey: "traffic.monthly_reset_day_hint",
    inputMode: "number",
    min: 1,
    max: 28,
  },
  {
    key: "monthly_traffic_limit",
    labelKey: "traffic.monthly_limit_label",
    hintKey: "traffic.monthly_limit_hint",
    inputMode: "number",
    min: 0,
  },
];

const AGENT_FIELDS: SettingsFieldDescriptor[] = [
  {
    key: "agent_heartbeat_interval",
    labelKey: "agent.heartbeat_interval_label",
    hintKey: "agent.heartbeat_interval_hint",
    inputMode: "number",
    min: 5,
    max: 300,
  },
];

const NOTIFY_FIELDS: SettingsFieldDescriptor[] = [
  {
    key: "notification_debounce",
    labelKey: "notify.debounce_label",
    hintKey: "notify.debounce_hint",
    inputMode: "number",
    min: 0,
    max: 86400,
  },
];

/**
 * /admin/settings — admin-only system configuration page. Six tabs grouped by
 * concern; the silent-mode and backup sections own their full UI (destructive
 * confirms, post-action dialogs). The remaining tabs share the generic
 * SettingsForm component.
 *
 * Authorisation is enforced server-side (RequireAdmin on every endpoint) but
 * we also bounce non-admins client-side to avoid the confusing 403 flash on
 * first paint.
 */
function AdminSettingsPage() {
  const { t } = useTranslation(["settings", "common", "errors"]);
  const { data: me } = useMeQuery();
  const settings = useSettings();

  if (me && me.role !== "admin") {
    return <Navigate to="/dashboard" />;
  }

  if (settings.isLoading) {
    return (
      <div className="mx-auto flex max-w-4xl flex-col gap-6">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (settings.isError || !settings.data) {
    return (
      <ErrorState
        message={t("settings:errors.load_failed")}
        onRetry={() => settings.refetch()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }

  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-6">
      <header>
        <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
          {t("settings:title")}
        </h1>
        <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("settings:description")}
        </p>
      </header>

      <Tabs defaultValue="silent_mode" className="w-full">
        <TabsList className="flex w-full flex-wrap gap-1">
          <TabsTrigger value="silent_mode">
            {t("settings:tabs.silent_mode")}
          </TabsTrigger>
          <TabsTrigger value="account">{t("settings:tabs.account")}</TabsTrigger>
          <TabsTrigger value="traffic">{t("settings:tabs.traffic")}</TabsTrigger>
          <TabsTrigger value="agent">{t("settings:tabs.agent")}</TabsTrigger>
          <TabsTrigger value="notify">{t("settings:tabs.notify")}</TabsTrigger>
          <TabsTrigger value="backup">{t("settings:tabs.backup")}</TabsTrigger>
        </TabsList>

        <TabsContent value="silent_mode">
          <SilentModeSection />
        </TabsContent>

        <TabsContent value="account">
          <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6">
            <SettingsForm
              fields={ACCOUNT_FIELDS}
              initialValues={settings.data}
            />
          </div>
        </TabsContent>

        <TabsContent value="traffic">
          <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6">
            <SettingsForm
              fields={TRAFFIC_FIELDS}
              initialValues={settings.data}
            />
          </div>
        </TabsContent>

        <TabsContent value="agent">
          <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6">
            <SettingsForm
              fields={AGENT_FIELDS}
              initialValues={settings.data}
            />
          </div>
        </TabsContent>

        <TabsContent value="notify">
          <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6">
            <SettingsForm
              fields={NOTIFY_FIELDS}
              initialValues={settings.data}
            />
          </div>
        </TabsContent>

        <TabsContent value="backup">
          <BackupSection />
        </TabsContent>
      </Tabs>
    </div>
  );
}
