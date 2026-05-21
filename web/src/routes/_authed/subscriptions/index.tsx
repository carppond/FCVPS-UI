import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Plus, Search } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useDebounce } from "@/hooks/use-debounce";
import { SubList } from "@/components/subscription/sub-list";
import { SubCreateWizard } from "@/components/subscription/sub-create-wizard";
import { SubEditForm } from "@/components/subscription/sub-edit-form";
import { useAuthStore } from "@/stores/auth-store";
import {
  useDeleteSubscriptionMutation,
  useSyncSubscriptionMutation,
} from "@/api/subscription";
import i18n from "@/lib/i18n";
import subZh from "@/locales/zh-CN/subscription.json";
import subEn from "@/locales/en/subscription.json";
import subJa from "@/locales/ja/subscription.json";
import subKo from "@/locales/ko/subscription.json";
import type { Subscription } from "@/types/api";

function ensureSubNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "subscription")) {
    i18n.addResourceBundle("zh-CN", "subscription", subZh, true, true);
    i18n.addResourceBundle("en", "subscription", subEn, true, true);
    i18n.addResourceBundle("ja", "subscription", subJa, true, true);
    i18n.addResourceBundle("ko", "subscription", subKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/subscriptions/")({
  beforeLoad: () => {
    ensureSubNamespace();
  },
  component: SubscriptionsPage,
});

function SubscriptionsPage() {
  const { t } = useTranslation(["subscription", "common"]);
  const { handle: handleError } = useApiError();
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";

  const [searchInput, setSearchInput] = React.useState("");
  const keyword = useDebounce(searchInput, 300);

  const [allUsers, setAllUsers] = React.useState(false);
  const [wizardOpen, setWizardOpen] = React.useState(false);
  const [editTarget, setEditTarget] = React.useState<Subscription | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<Subscription | null>(null);

  const syncMutation = useSyncSubscriptionMutation();
  const deleteMutation = useDeleteSubscriptionMutation();

  const onSync = async (sub: Subscription) => {
    try {
      const res = await syncMutation.mutateAsync(sub.id);
      toast.success(
        t("subscription:detail.sync_success", {
          added: res.added_count,
          removed: res.removed_count,
        }),
      );
    } catch (err) {
      handleError(err);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteMutation.mutateAsync(deleteTarget.id);
      toast.success(t("subscription:detail.delete_confirm.success"));
      setDeleteTarget(null);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <header className="flex items-end justify-between">
        <div>
          <h1 className="text-[var(--font-size-2xl)] font-semibold tracking-tight text-[var(--color-text-primary)]">
            {t("subscription:title")}
          </h1>
          <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("subscription:subtitle")}
          </p>
        </div>
        <Button onClick={() => setWizardOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("subscription:actions.create")}
        </Button>
      </header>

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-full max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t("subscription:filters.search_placeholder")}
            className="pl-9"
          />
        </div>
        {isAdmin && (
          <label className="ml-auto flex cursor-pointer items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
            <input
              type="checkbox"
              className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
              checked={allUsers}
              onChange={(e) => setAllUsers(e.target.checked)}
            />
            {t("subscription:filters.show_all_users")}
          </label>
        )}
      </div>

      <SubList
        params={{ keyword, allUsers: isAdmin && allUsers }}
        onSync={onSync}
        onEdit={(sub) => setEditTarget(sub)}
        onDelete={(sub) => setDeleteTarget(sub)}
        onCreate={() => setWizardOpen(true)}
      />

      <SubCreateWizard
        open={wizardOpen}
        onClose={() => setWizardOpen(false)}
      />

      {editTarget && (
        <SubEditForm
          open={!!editTarget}
          subscription={editTarget}
          onClose={() => setEditTarget(null)}
        />
      )}

      <Dialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t("subscription:detail.delete_confirm.title")}
            </DialogTitle>
            <DialogDescription>
              {t("subscription:detail.delete_confirm.description", {
                name: deleteTarget?.name ?? "",
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteTarget(null)}
              disabled={deleteMutation.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={confirmDelete}
              disabled={deleteMutation.isPending}
            >
              {t("subscription:detail.delete_confirm.submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
