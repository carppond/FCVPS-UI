/**
 * Action handlers extracted from cmd-k.tsx to keep the component file under
 * the size budget (coding standards §1). Pure side-effect / handler factory.
 */
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

import { toast } from "@/components/ui/toast";
import i18n from "@/lib/i18n";
import { pushRecent } from "@/hooks/use-cmd-k";
import { useAuthStore } from "@/stores/auth-store";
import { useUIStore } from "@/stores/ui-store";
import { useSubscriptionsQuery, useSyncSubscriptionMutation } from "@/api/subscription";
import { useOtaCheck } from "@/api/ota";
import { downloadBackup, useRotateSilentMode } from "@/api/settings";

const LANGS = ["zh-CN", "en", "ja", "ko"] as const;

export interface CmdActions {
  goToRecent: (entry: { to?: string }) => void;
  handleCreateSub: () => void;
  handleCreateAgent: () => void;
  handleSyncAll: () => Promise<void>;
  handleToggleTheme: () => void;
  handleToggleLang: () => Promise<void>;
  handleBackup: () => Promise<void>;
  handleOtaCheck: () => Promise<void>;
  handleSilentRotate: () => Promise<void>;
}

export function useCmdKActions(close: () => void): CmdActions {
  const { t, i18n: i18next } = useTranslation(["cmdk", "common"]);
  const navigate = useNavigate();
  const { token } = useAuthStore();
  const { theme, setTheme } = useUIStore();

  const syncOne = useSyncSubscriptionMutation();
  const otaCheck = useOtaCheck();
  const rotateSilent = useRotateSilentMode();
  const allSubsQ = useSubscriptionsQuery({ page: 1, pageSize: 200 });

  const goToRecent = (entry: { to?: string }) => {
    if (!entry.to) return;
    close();
    void navigate({ to: entry.to as never });
  };

  const handleCreateSub = () => {
    close();
    pushRecent({
      id: "action:create_sub",
      label: t("cmdk:actions.create_subscription"),
      to: "/subscriptions",
    });
    void navigate({ to: "/subscriptions" as never, search: { create: 1 } as never });
  };

  const handleCreateAgent = () => {
    close();
    pushRecent({
      id: "action:create_agent",
      label: t("cmdk:actions.create_agent"),
      to: "/agents",
    });
    void navigate({ to: "/agents" as never, search: { create: 1 } as never });
  };

  const handleSyncAll = async () => {
    close();
    const items = allSubsQ.data?.items ?? [];
    if (items.length === 0) return;
    toast.success(t("cmdk:toast.sync_all_started", { count: items.length }));
    const results = await Promise.allSettled(
      items.map((s) => syncOne.mutateAsync(s.id)),
    );
    const ok = results.filter((r) => r.status === "fulfilled").length;
    if (ok < items.length) {
      toast.message(
        t("cmdk:toast.sync_all_partial", { ok, total: items.length }),
      );
    }
  };

  const handleToggleTheme = () => {
    const next = theme === "dark" ? "light" : theme === "light" ? "system" : "dark";
    setTheme(next);
    toast.success(t("cmdk:toast.theme_changed"));
    close();
  };

  const handleToggleLang = async () => {
    const idx = LANGS.indexOf(i18next.language as (typeof LANGS)[number]);
    const next = LANGS[(idx + 1) % LANGS.length];
    await i18n.changeLanguage(next);
    toast.success(t("cmdk:toast.language_changed"));
    close();
  };

  const handleBackup = async () => {
    close();
    try {
      const blob = await downloadBackup(token ?? undefined);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `sgvps-backup-${Date.now()}.tar.gz`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      toast.success(t("cmdk:toast.backup_started"));
    } catch (err) {
      const msg = err instanceof Error ? err.message : t("cmdk:toast.sync_all_failed");
      toast.error(msg);
    }
  };

  const handleOtaCheck = async () => {
    close();
    try {
      await otaCheck.mutateAsync();
      toast.success(t("cmdk:toast.ota_started"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "OTA failed");
    }
  };

  const handleSilentRotate = async () => {
    close();
    try {
      await rotateSilent.mutateAsync();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Rotate failed");
    }
  };

  return {
    goToRecent,
    handleCreateSub,
    handleCreateAgent,
    handleSyncAll,
    handleToggleTheme,
    handleToggleLang,
    handleBackup,
    handleOtaCheck,
    handleSilentRotate,
  };
}
