/**
 * sub-list.tsx — legacy DataTable removed in favour of card grid.
 *
 * This file is kept solely to export `SyncStatusBadge`, which is
 * imported by `sub-detail-header.tsx`. The rest of the list rendering
 * now lives in `subscriptions/index.tsx` + `sub-card.tsx`.
 */
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import type { SyncStatus } from "@/types/api";

export function SyncStatusBadge({ status }: { status?: SyncStatus }) {
  const { t } = useTranslation("subscription");
  if (!status) {
    return (
      <Badge variant="secondary">{t("subscription:status.never")}</Badge>
    );
  }
  const variant: Record<SyncStatus, "default" | "secondary" | "destructive" | "outline"> = {
    ok: "outline",
    pending: "secondary",
    error: "destructive",
  };
  const cls: Record<SyncStatus, string> = {
    ok: "bg-[var(--color-success-bg)] text-[var(--color-success)]",
    pending: "bg-[var(--color-info-bg)] text-[var(--color-info)]",
    error: "",
  };
  return (
    <Badge variant={variant[status]} className={cls[status]}>
      {t(`subscription:status.${status}`)}
    </Badge>
  );
}
