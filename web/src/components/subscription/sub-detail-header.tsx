import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { ArrowLeft, ClipboardCopy, Pencil, RefreshCw, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { Subscription } from "@/types/api";
import { SyncStatusBadge } from "./sub-list";

interface SubDetailHeaderProps {
  subscription: Subscription;
  /** Pre-built share URL (or empty string when share_token is not available). */
  shareUrl: string;
  syncing: boolean;
  onSync: () => void;
  onEdit: () => void;
  onDelete: () => void;
  onCopyShareUrl: () => void;
}

export function SubDetailHeader({
  subscription,
  shareUrl,
  syncing,
  onSync,
  onEdit,
  onDelete,
  onCopyShareUrl,
}: SubDetailHeaderProps) {
  const { t } = useTranslation(["subscription", "common"]);

  return (
    <header className="flex flex-col gap-3">
      <Link
        to="/subscriptions"
        className="inline-flex items-center gap-1 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)]"
      >
        <ArrowLeft className="h-3 w-3" />
        {t("subscription:actions.back_to_list")}
      </Link>

      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-3">
            <h1 className="text-[var(--font-size-2xl)] font-semibold tracking-tight text-[var(--color-text-primary)]">
              {subscription.name}
            </h1>
            <Badge variant="outline">
              {t(`subscription:source_type.${subscription.type}`)}
            </Badge>
            <SyncStatusBadge status={subscription.last_sync_status} />
          </div>
          {subscription.remark && (
            <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
              {subscription.remark}
            </p>
          )}
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            onClick={onCopyShareUrl}
            disabled={!shareUrl}
            title={t("subscription:actions.copy_url")}
          >
            <ClipboardCopy className="mr-2 h-4 w-4" />
            {t("subscription:actions.copy_url")}
          </Button>
          <Button variant="outline" onClick={onEdit}>
            <Pencil className="mr-2 h-4 w-4" />
            {t("subscription:actions.edit")}
          </Button>
          <Button onClick={onSync} disabled={syncing}>
            <RefreshCw className={"mr-2 h-4 w-4 " + (syncing ? "animate-spin" : "")} />
            {t("subscription:actions.sync")}
          </Button>
          <Button variant="destructive" onClick={onDelete}>
            <Trash2 className="mr-2 h-4 w-4" />
            {t("subscription:actions.delete")}
          </Button>
        </div>
      </div>
    </header>
  );
}
