import * as React from "react";
import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  Shield,
  User,
  BarChart2,
  Radio,
  Bell,
  Download,
  Save,
  Flame,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useMeQuery } from "@/api/user";
import { useSettings, useUpdateSettings, type SettingsMap } from "@/api/settings";
import { SilentModeSection } from "@/components/admin/silent-mode-section";
import { BackupSection } from "@/components/admin/backup-section";
import { FirewallSection } from "@/components/admin/firewall-section";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";

export const Route = createFileRoute("/_authed/admin/settings")({
  component: AdminSettingsPage,
});

type SectionId = "silent" | "account" | "traffic" | "agent" | "notify" | "firewall" | "backup";

interface NavItemDef {
  id: SectionId;
  icon: React.ReactNode;
  labelKey: string;
  iconBg: string;
  iconColor: string;
}

const NAV_ITEMS: NavItemDef[] = [
  { id: "silent", icon: <Shield className="h-[15px] w-[15px]" />, labelKey: "settings:tabs.silent_mode", iconBg: "rgba(255,99,99,.1)", iconColor: "var(--color-primary)" },
  { id: "account", icon: <User className="h-[15px] w-[15px]" />, labelKey: "settings:tabs.account", iconBg: "rgba(96,165,250,.08)", iconColor: "var(--color-info)" },
  { id: "traffic", icon: <BarChart2 className="h-[15px] w-[15px]" />, labelKey: "settings:tabs.traffic", iconBg: "rgba(52,211,153,.08)", iconColor: "var(--color-success)" },
  { id: "agent", icon: <Radio className="h-[15px] w-[15px]" />, labelKey: "settings:tabs.agent", iconBg: "rgba(251,191,36,.08)", iconColor: "var(--color-warning)" },
  { id: "notify", icon: <Bell className="h-[15px] w-[15px]" />, labelKey: "settings:tabs.notify", iconBg: "rgba(167,139,250,.08)", iconColor: "#a78bfa" },
  { id: "firewall", icon: <Flame className="h-[15px] w-[15px]" />, labelKey: "settings:tabs.firewall", iconBg: "rgba(251,146,60,.08)", iconColor: "#fb923c" },
];

const NAV_BOTTOM: NavItemDef = {
  id: "backup", icon: <Download className="h-[15px] w-[15px]" />, labelKey: "settings:tabs.backup", iconBg: "rgba(255,255,255,.04)", iconColor: "var(--color-text-secondary)",
};

function AdminSettingsPage() {
  const { t } = useTranslation(["settings", "common", "errors"]);
  const { data: me } = useMeQuery();
  const settings = useSettings();
  const [activeSection, setActiveSection] = React.useState<SectionId>("silent");

  if (me && me.role !== "admin") {
    return <Navigate to="/dashboard" />;
  }

  if (settings.isLoading) {
    return (
      <div className="mx-auto flex max-w-4xl flex-col gap-6">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-[400px] w-full rounded-2xl" />
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
    <div className="mx-auto flex max-w-[960px] flex-col gap-6">
      <header>
        <h1 className="text-[26px] font-extrabold tracking-tight text-[var(--color-text-primary)]">
          {t("settings:title")}
        </h1>
        <p className="mt-1 text-[13px] text-[var(--color-text-tertiary)]">
          {t("settings:description")}
        </p>
      </header>

      <div
        className={cn(
          "flex overflow-hidden rounded-[20px] border border-[var(--color-border)]",
          "bg-[var(--color-surface)] shadow-[0_20px_60px_rgba(0,0,0,0.4)]",
          "min-h-[560px]",
        )}
      >
        {/* Left nav */}
        <nav className="flex w-[220px] flex-shrink-0 flex-col gap-0.5 border-r border-[var(--color-border)] bg-gradient-to-b from-white/[.02] to-transparent p-5 pr-3">
          <span className="mb-1 px-3.5 text-[9px] font-bold uppercase tracking-widest text-[var(--color-text-disabled)]">
            {t("settings:title")}
          </span>
          {NAV_ITEMS.map((item) => (
            <NavButton
              key={item.id}
              item={item}
              active={activeSection === item.id}
              onClick={() => setActiveSection(item.id)}
            />
          ))}
          <div className="my-2 h-px bg-[var(--color-border)]" />
          <NavButton
            item={NAV_BOTTOM}
            active={activeSection === "backup"}
            onClick={() => setActiveSection("backup")}
          />
        </nav>

        {/* Right content */}
        <div className="flex flex-1 flex-col">
          <div key={activeSection} className="animate-in fade-in slide-in-from-bottom-1 duration-200 flex flex-1 flex-col">
            {activeSection === "silent" && <SilentPanel />}
            {activeSection === "account" && (
              <SettingsPanel
                title={t("settings:tabs.account")}
                desc={t("settings:account.session_ttl_hint")}
                iconBg="rgba(96,165,250,.1)"
                iconColor="var(--color-info)"
                icon={<User className="h-3.5 w-3.5" />}
                fields={[
                  { key: "session_ttl_seconds", label: t("settings:account.session_ttl_label"), hint: t("settings:account.session_ttl_hint"), suffix: t("settings:tabs.account") === "Account" ? "sec" : "秒", inputMode: "number" },
                  { key: "default_locale", label: t("settings:account.default_locale_label"), hint: t("settings:account.default_locale_hint"), type: "select", options: ["zh-CN", "en", "ja", "ko"] },
                ]}
                initialValues={settings.data}
              />
            )}
            {activeSection === "traffic" && (
              <SettingsPanel
                title={t("settings:tabs.traffic")}
                desc={t("settings:traffic.monthly_reset_day_hint")}
                iconBg="rgba(52,211,153,.1)"
                iconColor="var(--color-success)"
                icon={<BarChart2 className="h-3.5 w-3.5" />}
                fields={[
                  { key: "monthly_reset_day", label: t("settings:traffic.monthly_reset_day_label"), hint: t("settings:traffic.monthly_reset_day_hint"), suffix: t("settings:tabs.account") === "Account" ? "day" : "日", inputMode: "number", width: "80px" },
                  { key: "monthly_traffic_limit", label: t("settings:traffic.monthly_limit_label"), hint: t("settings:traffic.monthly_limit_hint"), suffix: t("settings:tabs.account") === "Account" ? "bytes" : "字节", inputMode: "number", width: "120px" },
                ]}
                initialValues={settings.data}
              />
            )}
            {activeSection === "agent" && (
              <SettingsPanel
                title={t("settings:tabs.agent")}
                desc={t("settings:agent.heartbeat_interval_hint")}
                iconBg="rgba(251,191,36,.1)"
                iconColor="var(--color-warning)"
                icon={<Radio className="h-3.5 w-3.5" />}
                fields={[
                  { key: "agent_heartbeat_interval", label: t("settings:agent.heartbeat_interval_label"), hint: t("settings:agent.heartbeat_interval_hint"), suffix: t("settings:tabs.account") === "Account" ? "sec" : "秒", inputMode: "number", width: "80px" },
                ]}
                initialValues={settings.data}
              />
            )}
            {activeSection === "notify" && (
              <SettingsPanel
                title={t("settings:tabs.notify")}
                desc={t("settings:notify.debounce_hint")}
                iconBg="rgba(167,139,250,.1)"
                iconColor="#a78bfa"
                icon={<Bell className="h-3.5 w-3.5" />}
                fields={[
                  { key: "notification_debounce", label: t("settings:notify.debounce_label"), hint: t("settings:notify.debounce_hint"), suffix: t("settings:tabs.account") === "Account" ? "sec" : "秒", inputMode: "number", width: "100px" },
                ]}
                initialValues={settings.data}
              />
            )}
            {activeSection === "firewall" && <FirewallSection />}
            {activeSection === "backup" && <BackupPanel />}
          </div>
        </div>
      </div>
    </div>
  );
}

function NavButton({
  item,
  active,
  onClick,
}: {
  item: NavItemDef;
  active: boolean;
  onClick: () => void;
}) {
  const { t } = useTranslation("settings");
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "relative flex items-center gap-2.5 rounded-[10px] px-3.5 py-2.5",
        "text-[13px] font-medium transition-all duration-150 text-left",
        active
          ? "bg-[var(--color-primary-soft)] text-[var(--color-text-primary)]"
          : "text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)] hover:bg-white/[.03]",
      )}
    >
      {active && (
        <span className="absolute left-0 top-1/2 h-[18px] w-[3px] -translate-y-1/2 rounded-r-sm bg-[var(--color-primary)]" />
      )}
      <span
        className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-lg transition-shadow"
        style={{
          background: item.iconBg,
          color: item.iconColor,
          boxShadow: active ? `0 0 8px ${item.iconBg}` : undefined,
        }}
      >
        {item.icon}
      </span>
      {t(item.labelKey)}
    </button>
  );
}

function SilentPanel() {
  return (
    <div className="flex flex-1 flex-col">
      <SilentModeSection />
    </div>
  );
}

function BackupPanel() {
  return (
    <div className="flex flex-1 flex-col">
      <BackupSection />
    </div>
  );
}

interface FieldDef {
  key: string;
  label: string;
  hint: string;
  suffix?: string;
  inputMode?: "number" | "text";
  type?: "select";
  options?: string[];
  width?: string;
}

function SettingsPanel({
  title,
  desc,
  iconBg,
  iconColor,
  icon,
  fields,
  initialValues,
}: {
  title: string;
  desc: string;
  iconBg: string;
  iconColor: string;
  icon: React.ReactNode;
  fields: FieldDef[];
  initialValues: SettingsMap;
}) {
  const { t } = useTranslation(["settings", "common"]);
  const { handle: handleError } = useApiError();
  const update = useUpdateSettings();

  const [values, setValues] = React.useState<SettingsMap>(() => {
    const out: SettingsMap = {};
    for (const f of fields) out[f.key] = initialValues[f.key] ?? "";
    return out;
  });

  React.useEffect(() => {
    const out: SettingsMap = {};
    for (const f of fields) out[f.key] = initialValues[f.key] ?? "";
    setValues(out);
  }, [fields, initialValues]);

  const dirty = fields.some((f) => (values[f.key] ?? "") !== (initialValues[f.key] ?? ""));

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const payload: SettingsMap = {};
    for (const f of fields) {
      const cur = values[f.key] ?? "";
      if (cur !== (initialValues[f.key] ?? "")) payload[f.key] = cur;
    }
    try {
      await update.mutateAsync(payload);
      toast.success(t("settings:actions.saved"));
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <form onSubmit={onSubmit} className="flex flex-1 flex-col">
      <div className="px-8 pt-6">
        <h2 className="flex items-center gap-2 text-[18px] font-bold tracking-tight">
          <span
            className="flex h-7 w-7 items-center justify-center rounded-lg"
            style={{ background: iconBg, color: iconColor }}
          >
            {icon}
          </span>
          {title}
        </h2>
        <p className="mt-1 text-[12px] text-[var(--color-text-tertiary)]">{desc}</p>
      </div>
      <div className="flex-1 px-8 py-5">
        {fields.map((f) => (
          <div
            key={f.key}
            className={cn(
              "flex items-center justify-between gap-5 rounded-lg py-4",
              "border-b border-white/[.04] last:border-b-0",
              "transition-colors hover:bg-white/[.01]",
            )}
          >
            <div className="flex-1 min-w-0">
              <Label className="text-[14px] font-semibold">{f.label}</Label>
              <p className="mt-0.5 text-[11px] leading-relaxed text-[var(--color-text-tertiary)]">{f.hint}</p>
            </div>
            <div className="flex shrink-0 items-center gap-1.5">
              {f.type === "select" ? (
                <select
                  value={values[f.key] ?? ""}
                  onChange={(e) => setValues((v) => ({ ...v, [f.key]: e.target.value }))}
                  className={cn(
                    "h-[38px] rounded-[10px] border border-[var(--color-border-strong)]",
                    "bg-[var(--color-bg-elevated)] px-3 text-[13px] text-[var(--color-text-primary)]",
                    "transition focus:border-[var(--color-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-primary-soft)]",
                  )}
                  style={{ width: f.width ?? "130px" }}
                >
                  {f.options?.map((o) => <option key={o} value={o}>{o}</option>)}
                </select>
              ) : (
                <Input
                  type={f.inputMode === "number" ? "number" : "text"}
                  value={values[f.key] ?? ""}
                  onChange={(e) => setValues((v) => ({ ...v, [f.key]: e.target.value }))}
                  className="h-[38px] rounded-[10px] border-[var(--color-border-strong)] bg-[var(--color-bg-elevated)] text-[13px]"
                  style={{ width: f.width ?? "150px" }}
                />
              )}
              {f.suffix && (
                <span className="text-[10px] text-[var(--color-text-disabled)]">{f.suffix}</span>
              )}
            </div>
          </div>
        ))}
      </div>
      <div className="flex justify-end border-t border-[var(--color-border)] px-8 py-4">
        <Button type="submit" disabled={!dirty || update.isPending} className="h-10 px-6">
          <Save className="mr-2 h-4 w-4" />
          {update.isPending ? t("settings:actions.saving") : t("settings:actions.save")}
        </Button>
      </div>
    </form>
  );
}
