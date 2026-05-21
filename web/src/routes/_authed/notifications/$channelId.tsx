import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { toast } from "@/components/ui/toast";
import { ChannelCard } from "@/components/notify/channel-card";
import { ChannelForm } from "@/components/notify/channel-form";
import { TemplateEditor } from "@/components/notify/template-editor";
import { EventHistory } from "@/components/notify/event-history";
import {
  useChannel,
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

function ensureNotifyNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "notify")) {
    i18n.addResourceBundle("zh-CN", "notify", notifyZh, true, true);
    i18n.addResourceBundle("en", "notify", notifyEn, true, true);
    i18n.addResourceBundle("ja", "notify", notifyJa, true, true);
    i18n.addResourceBundle("ko", "notify", notifyKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/notifications/$channelId")({
  beforeLoad: () => {
    ensureNotifyNamespace();
  },
  component: ChannelDetailPage,
});

function ChannelDetailPage() {
  const { channelId } = Route.useParams();
  const { t } = useTranslation(["notify", "common"]);
  const navigate = useNavigate();
  const { handle: handleError } = useApiError();

  const channelQ = useChannel(channelId);
  // The list query backs `useChannel` — make sure it's been fetched at least
  // once so the detail lookup hits a populated cache.
  useChannels();

  const updateMutation = useUpdateChannel();
  const deleteMutation = useDeleteChannel();

  const channel = channelQ.data;

  const handleToggle = async (_c: typeof channel, next: boolean) => {
    if (!channel) return;
    try {
      await updateMutation.mutateAsync({
        id: channel.id,
        payload: { enabled: next },
      });
    } catch (err) {
      handleError(err);
    }
  };

  const handleDelete = async () => {
    if (!channel) return;
    try {
      await deleteMutation.mutateAsync(channel.id);
      toast.success(t("notify:form.saved"));
      navigate({ to: "/notifications" });
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-[var(--spacing-4)] p-[var(--spacing-6)]">
      <header className="flex items-center justify-between">
        <Button asChild variant="ghost" size="sm">
          <Link to="/notifications">
            <ArrowLeft className="h-4 w-4" />
            {t("common:actions.back")}
          </Link>
        </Button>
      </header>

      {channelQ.isLoading ? (
        <Skeleton className="h-64 w-full" />
      ) : channelQ.isError || !channel ? (
        <ErrorState
          message={t("notify:history.load_failed")}
          onRetry={() => void channelQ.refetch()}
          retryLabel={t("common:actions.retry")}
        />
      ) : (
        <Tabs
          defaultValue="detail"
          className="flex flex-col gap-[var(--spacing-3)]"
        >
          <TabsList className="self-start">
            <TabsTrigger value="detail">
              {t("notify:page.tab_detail")}
            </TabsTrigger>
            <TabsTrigger value="template">
              {t("notify:page.tab_template")}
            </TabsTrigger>
            <TabsTrigger value="history">
              {t("notify:page.tab_history")}
            </TabsTrigger>
          </TabsList>

          <TabsContent
            value="detail"
            className="flex flex-col gap-[var(--spacing-4)]"
          >
            <ChannelCard
              channel={channel}
              status="untested"
              onEdit={() => {
                /* edit happens inline via the form below */
              }}
              onDelete={() => void handleDelete()}
              onToggleEnabled={handleToggle}
            />
            <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]">
              <ChannelForm
                channel={channel}
                onSaved={() => toast.success(t("notify:form.saved"))}
              />
            </div>
          </TabsContent>

          <TabsContent value="template">
            <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]">
              <TemplateEditor
                value={channel.template ?? ""}
                onChange={() => {
                  /* lifted into onSave's value parameter */
                }}
                onSave={async (v) => {
                  try {
                    await updateMutation.mutateAsync({
                      id: channel.id,
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
                      id: channel.id,
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
          </TabsContent>

          <TabsContent value="history">
            <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]">
              <EventHistory channels={[channel]} channelId={channel.id} />
            </div>
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
}
