import * as React from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { toast } from "@/components/ui/toast";
import { ChannelList, CHANNEL_KIND_ORDER } from "@/components/notify/channel-list";
import { ChannelCard } from "@/components/notify/channel-card";
import { ChannelForm } from "@/components/notify/channel-form";
import { EventSubscriptions } from "@/components/notify/event-subscriptions";
import { EventHistory } from "@/components/notify/event-history";
import { TemplateEditor } from "@/components/notify/template-editor";
import {
  useChannels,
  useDeleteChannel,
  useUpdateChannel,
} from "@/api/notify";
import { useApiError } from "@/hooks/use-api-error";
import i18n from "@/lib/i18n";
import notifyZh from "@/locales/zh-CN/notify.json";
import notifyEn from "@/locales/en/notify.json";
import notifyJa from "@/locales/ja/notify.json";
import notifyKo from "@/locales/ko/notify.json";
import type { ChannelKind, NotificationChannel } from "@/types/api";

// Lazy-register the "notify" namespace before the route mounts. Matches the
// pattern used by /scripts, /traffic, etc. so the first-screen bundle stays
// slim per tech-lead-plan §2.3.
function ensureNotifyNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "notify")) {
    i18n.addResourceBundle("zh-CN", "notify", notifyZh, true, true);
    i18n.addResourceBundle("en", "notify", notifyEn, true, true);
    i18n.addResourceBundle("ja", "notify", notifyJa, true, true);
    i18n.addResourceBundle("ko", "notify", notifyKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/notifications/")({
  beforeLoad: () => {
    ensureNotifyNamespace();
  },
  component: NotificationsPage,
});

function NotificationsPage() {
  const { t } = useTranslation(["notify", "common"]);
  const navigate = useNavigate();
  const { handle: handleError } = useApiError();

  const channelsQ = useChannels();
  const updateMutation = useUpdateChannel();
  const deleteMutation = useDeleteChannel();

  const channels = React.useMemo(() => channelsQ.data ?? [], [channelsQ.data]);

  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const selected = React.useMemo(
    () => channels.find((c) => c.id === selectedId) ?? null,
    [channels, selectedId],
  );

  // Filter by kind (top toolbar).
  const [kindFilter, setKindFilter] = React.useState<ChannelKind | "">("");

  // Create-channel dialog state.
  const [createOpen, setCreateOpen] = React.useState(false);
  const [createKind, setCreateKind] = React.useState<ChannelKind>("telegram");

  // Edit dialog state.
  const [editOpen, setEditOpen] = React.useState(false);
  const [editTarget, setEditTarget] = React.useState<NotificationChannel | null>(
    null,
  );

  // Delete-confirm dialog.
  const [deleteTarget, setDeleteTarget] =
    React.useState<NotificationChannel | null>(null);

  // Per-channel transient test status (resets on page mount).
  const [testStatus, setTestStatus] = React.useState<
    Record<string, "ok" | "failed" | "untested">
  >({});

  // Auto-select the first channel once the list loads.
  React.useEffect(() => {
    if (selectedId || channels.length === 0) return;
    setSelectedId(channels[0].id);
  }, [channels, selectedId]);

  const openCreate = (kind: ChannelKind) => {
    if (kind === "telegram") {
      // Redirect notice — full Telegram bot config lives on T-24's sub-page.
      // The basic channel still works here; we just hint where to go for the
      // 2-way command setup so users don't get confused.
      toast.message(t("notify:telegram_redirect"));
    }
    setCreateKind(kind);
    setCreateOpen(true);
  };

  const handleSelectFromList = (c: NotificationChannel) => {
    setSelectedId(c.id);
  };

  const handleEdit = (c: NotificationChannel) => {
    setEditTarget(c);
    setEditOpen(true);
  };

  const handleToggleEnabled = async (
    c: NotificationChannel,
    next: boolean,
  ) => {
    try {
      await updateMutation.mutateAsync({
        id: c.id,
        payload: { enabled: next },
      });
    } catch (err) {
      handleError(err);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;
    try {
      await deleteMutation.mutateAsync(deleteTarget.id);
      if (selectedId === deleteTarget.id) setSelectedId(null);
      setDeleteTarget(null);
      toast.success(t("notify:form.saved"));
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-[var(--spacing-6)] p-[var(--spacing-6)]">
      {/* Page header */}
      <header className="flex flex-wrap items-end justify-between gap-[var(--spacing-3)]">
        <div className="flex flex-col gap-[var(--spacing-1)]">
          <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
            {t("notify:title")}
          </h1>
          <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("notify:subtitle")}
          </p>
        </div>
        <div className="flex items-center gap-[var(--spacing-2)]">
          <label className="flex items-center gap-[var(--spacing-2)] text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
            {t("notify:page.filter_label")}
            <select
              value={kindFilter}
              onChange={(e) =>
                setKindFilter(e.target.value as ChannelKind | "")
              }
              className="h-8 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-2 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
              data-testid="notify-kind-filter"
            >
              <option value="">{t("notify:page.filter_all")}</option>
              {CHANNEL_KIND_ORDER.map((k) => (
                <option key={k} value={k}>
                  {t(`notify:kinds.${k}.name`)}
                </option>
              ))}
            </select>
          </label>
          <Button
            size="sm"
            onClick={() => openCreate(kindFilter || "telegram")}
            data-testid="notify-create-button"
          >
            <Plus className="h-4 w-4" />
            {t("notify:list.add_channel")}
          </Button>
        </div>
      </header>

      {/* Three-column layout (collapses on small screens) */}
      {channelsQ.isLoading ? (
        <div className="grid grid-cols-1 gap-[var(--spacing-4)] lg:grid-cols-3">
          <Skeleton className="h-64 w-full" />
          <Skeleton className="h-64 w-full lg:col-span-2" />
        </div>
      ) : channelsQ.isError ? (
        <ErrorState
          message={t("notify:history.load_failed")}
          onRetry={() => void channelsQ.refetch()}
          retryLabel={t("common:actions.retry")}
        />
      ) : (
        <div className="grid grid-cols-1 gap-[var(--spacing-4)] lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_minmax(0,1.4fr)]">
          {/* Left: channel list */}
          <section className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]">
            <ChannelList
              channels={channels}
              filterKind={kindFilter}
              onCreate={openCreate}
              onSelect={handleSelectFromList}
              selectedId={selected?.id}
            />
          </section>

          {/* Middle: detail card / template editor */}
          <section className="flex flex-col gap-[var(--spacing-3)]">
            {selected ? (
              <>
                <ChannelCard
                  channel={selected}
                  status={testStatus[selected.id] ?? "untested"}
                  onEdit={handleEdit}
                  onDelete={(c) => setDeleteTarget(c)}
                  onToggleEnabled={handleToggleEnabled}
                />
                <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]">
                  <TemplateEditor
                    value={selected.template ?? ""}
                    onChange={(v) => {
                      setTestStatus((prev) => ({ ...prev }));
                      // Save on explicit click only — avoids saving on every key.
                      void v;
                    }}
                    onSave={async (v) => {
                      try {
                        await updateMutation.mutateAsync({
                          id: selected.id,
                          payload: { template: v },
                        });
                        toast.success(t("notify:form.saved"));
                      } catch (err) {
                        handleError(err);
                      }
                    }}
                    onReset={async () => {
                      try {
                        await updateMutation.mutateAsync({
                          id: selected.id,
                          payload: { template: "" },
                        });
                        toast.success(t("notify:form.saved"));
                      } catch (err) {
                        handleError(err);
                      }
                    }}
                    isSaving={updateMutation.isPending}
                  />
                </div>
                <div className="flex items-center justify-end">
                  <Button
                    variant="link"
                    size="sm"
                    onClick={() =>
                      navigate({
                        to: "/notifications/$channelId",
                        params: { channelId: selected.id },
                      })
                    }
                  >
                    {t("common:actions.edit")}
                  </Button>
                </div>
              </>
            ) : (
              <EmptyState
                title={t("notify:page.select_hint")}
                description={t("notify:list.empty")}
              />
            )}
          </section>

          {/* Right: subscription matrix + history (tabs) */}
          <section className="flex flex-col gap-[var(--spacing-3)]">
            <Tabs defaultValue="matrix" className="flex flex-col gap-[var(--spacing-3)]">
              <TabsList className="self-start">
                <TabsTrigger value="matrix">
                  {t("notify:page.tab_events")}
                </TabsTrigger>
                <TabsTrigger value="history">
                  {t("notify:page.tab_history")}
                </TabsTrigger>
              </TabsList>
              <TabsContent value="matrix">
                <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]">
                  <EventSubscriptions channels={channels} />
                </div>
              </TabsContent>
              <TabsContent value="history">
                <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]">
                  <EventHistory channels={channels} channelId={selected?.id} />
                </div>
              </TabsContent>
            </Tabs>
          </section>
        </div>
      )}

      {/* Create dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("notify:list.add_channel")}</DialogTitle>
            <DialogDescription>{t("notify:form.config_section")}</DialogDescription>
          </DialogHeader>
          <ChannelForm
            channel={null}
            initialKind={createKind}
            onCancel={() => setCreateOpen(false)}
            onSaved={(c) => {
              setCreateOpen(false);
              setSelectedId(c.id);
            }}
          />
        </DialogContent>
      </Dialog>

      {/* Edit dialog */}
      <Dialog
        open={editOpen}
        onOpenChange={(o) => {
          setEditOpen(o);
          if (!o) setEditTarget(null);
        }}
      >
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("common:actions.edit")}</DialogTitle>
          </DialogHeader>
          {editTarget && (
            <ChannelForm
              channel={editTarget}
              onCancel={() => setEditOpen(false)}
              onSaved={() => {
                setEditOpen(false);
                setEditTarget(null);
              }}
            />
          )}
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <Dialog
        open={deleteTarget !== null}
        onOpenChange={(o) => {
          if (!o) setDeleteTarget(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("notify:page.delete_confirm_title")}
            </DialogTitle>
            <DialogDescription>
              {t("notify:page.delete_confirm_description", {
                name: deleteTarget?.name ?? "",
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setDeleteTarget(null)}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={handleDeleteConfirm}
              disabled={deleteMutation.isPending}
            >
              {t("notify:page.delete_confirm_submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
