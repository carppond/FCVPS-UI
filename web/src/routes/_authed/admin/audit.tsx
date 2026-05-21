import * as React from "react";
import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Filter } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AuditTable } from "@/components/admin/audit-table";
import { useMeQuery } from "@/api/user";
import i18next from "@/lib/i18n";
import auditZhCN from "@/locales/zh-CN/audit.json";
import auditEn from "@/locales/en/audit.json";
import auditJa from "@/locales/ja/audit.json";
import auditKo from "@/locales/ko/audit.json";
import type { AuditListParams } from "@/api/audit";

i18next.addResourceBundle("zh-CN", "audit", auditZhCN);
i18next.addResourceBundle("en", "audit", auditEn);
i18next.addResourceBundle("ja", "audit", auditJa);
i18next.addResourceBundle("ko", "audit", auditKo);

export const Route = createFileRoute("/_authed/admin/audit")({
  component: AdminAuditPage,
});

type FilterState = Omit<AuditListParams, "page" | "pageSize">;

function AdminAuditPage() {
  const { t } = useTranslation(["audit", "common"]);
  const { data: me } = useMeQuery();

  // Draft fields the user is typing into; only applied when "Apply" is
  // clicked so the table doesn't re-fetch on every keystroke.
  const [draftUserId, setDraftUserId] = React.useState("");
  const [draftAction, setDraftAction] = React.useState("");
  const [draftFrom, setDraftFrom] = React.useState("");
  const [draftTo, setDraftTo] = React.useState("");

  const [filter, setFilter] = React.useState<FilterState>({});

  if (me && me.role !== "admin") {
    return <Navigate to="/dashboard" />;
  }

  const applyFilter = () => {
    setFilter({
      userId: draftUserId.trim() || undefined,
      action: draftAction.trim() || undefined,
      from: draftFrom ? new Date(draftFrom).getTime() : undefined,
      to: draftTo ? new Date(draftTo).getTime() : undefined,
    });
  };

  const resetFilter = () => {
    setDraftUserId("");
    setDraftAction("");
    setDraftFrom("");
    setDraftTo("");
    setFilter({});
  };

  return (
    <div className="flex flex-col gap-6">
      <header className="flex items-end justify-between">
        <div>
          <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
            {t("audit:page.title")}
          </h1>
          <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("audit:page.description")}
          </p>
        </div>
      </header>

      <section className="rounded-md border border-[var(--color-border-subtle)] bg-[var(--color-bg-elevated)] p-4">
        <div className="mb-3 flex items-center gap-2 text-[var(--font-size-sm)] font-medium text-[var(--color-text-secondary)]">
          <Filter className="h-4 w-4" />
          {t("audit:filter.title")}
        </div>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
          <div>
            <Label htmlFor="user-id">{t("audit:filter.user_id")}</Label>
            <Input
              id="user-id"
              value={draftUserId}
              onChange={(e) => setDraftUserId(e.target.value)}
              placeholder={t("audit:filter.user_id_placeholder")}
            />
          </div>
          <div>
            <Label htmlFor="action">{t("audit:filter.action")}</Label>
            <Input
              id="action"
              value={draftAction}
              onChange={(e) => setDraftAction(e.target.value)}
              placeholder={t("audit:filter.action_placeholder")}
            />
          </div>
          <div>
            <Label htmlFor="from">{t("audit:filter.from")}</Label>
            <Input
              id="from"
              type="datetime-local"
              value={draftFrom}
              onChange={(e) => setDraftFrom(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor="to">{t("audit:filter.to")}</Label>
            <Input
              id="to"
              type="datetime-local"
              value={draftTo}
              onChange={(e) => setDraftTo(e.target.value)}
            />
          </div>
        </div>
        <div className="mt-3 flex justify-end gap-2">
          <Button variant="outline" size="sm" onClick={resetFilter}>
            {t("audit:filter.reset")}
          </Button>
          <Button size="sm" onClick={applyFilter}>
            {t("audit:filter.apply")}
          </Button>
        </div>
      </section>

      <AuditTable filter={filter} />
    </div>
  );
}
