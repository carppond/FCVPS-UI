import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  Plus,
  Link2,
  Copy,
  Trash2,
  ExternalLink,
  Search,
  Clock,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { DateTimePicker } from "@/components/ui/date-picker";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import { formatDate } from "@/lib/format";
import {
  useShortLinks,
  useCreateShortLink,
  useDeleteShortLink,
} from "@/api/shortlink";
import type { ShortLink } from "@/types/api";

export const Route = createFileRoute("/_authed/shortlinks")({
  component: ShortLinksPage,
});

function ShortLinksPage() {
  const { t } = useTranslation(["shortlink", "common"]);
  const { handle: handleError } = useApiError();
  const createMutation = useCreateShortLink();
  const deleteMutation = useDeleteShortLink();
  const { data, isLoading, isError, error, refetch } = useShortLinks();

  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [targetUrl, setTargetUrl] = React.useState("");
  const [expiresAt, setExpiresAt] = React.useState("");

  const links = data ?? [];

  const openCreate = () => {
    setTargetUrl("");
    setExpiresAt("");
    setDialogOpen(true);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!targetUrl.trim()) {
      toast.error(t("shortlink:create.error_required"));
      return;
    }
    try {
      const expiresMs = expiresAt ? new Date(expiresAt).getTime() : 0;
      await createMutation.mutateAsync({
        target_url: targetUrl.trim(),
        expires_at: expiresMs,
      });
      toast.success(t("shortlink:create.success"));
      setDialogOpen(false);
    } catch (err) {
      handleError(err);
    }
  };

  const handleCopy = async (url: string) => {
    try {
      await navigator.clipboard.writeText(url);
      toast.success(t("shortlink:list.copied"));
    } catch {
      toast.error(t("shortlink:list.copy_failed"));
    }
  };

  const handleDelete = async (link: ShortLink) => {
    try {
      await deleteMutation.mutateAsync({
        fileCode: link.file_code,
        userCode: link.user_code,
      });
      toast.success(t("shortlink:list.deleted"));
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="mx-auto flex max-w-[760px] flex-col gap-6">
      <header className="flex items-end justify-between">
        <div>
          <h1 className="text-[26px] font-extrabold tracking-tight text-[var(--color-text-primary)]">
            {t("shortlink:page.title")}
          </h1>
          <p className="mt-1 text-[13px] text-[var(--color-text-tertiary)]">
            {t("shortlink:page.description")}
          </p>
        </div>
        <Button onClick={openCreate} className="h-10">
          <Plus className="mr-2 h-4 w-4" />
          {t("shortlink:page.create")}
        </Button>
      </header>

      {/* Content */}
      {isLoading && <ListSkeleton />}
      {isError && (
        <ErrorState
          message={t("shortlink:list.error_load") + (error instanceof Error ? ` (${error.message})` : "")}
          onRetry={() => refetch()}
          retryLabel={t("common:actions.retry")}
        />
      )}
      {!isLoading && !isError && links.length === 0 && (
        <EmptyState
          icon={<Link2 />}
          title={t("shortlink:list.empty_title")}
          description={t("shortlink:list.empty_description")}
          ctaLabel={t("shortlink:list.create_cta")}
          onCta={openCreate}
        />
      )}
      {!isLoading && !isError && links.length > 0 && (
        <div className="flex flex-col gap-3">
          {links.map((link) => (
            <LinkCard
              key={link.file_code + link.user_code}
              link={link}
              onCopy={handleCopy}
              onDelete={handleDelete}
              deleting={deleteMutation.isPending}
            />
          ))}
        </div>
      )}

      {/* Create Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-[480px] gap-0 overflow-hidden p-0">
          <DialogHeader className="px-7 pt-6 pb-1">
            <DialogTitle className="text-[18px] font-bold tracking-tight">
              {t("shortlink:create.title")}
            </DialogTitle>
            <p className="text-[12px] text-[var(--color-text-tertiary)]">
              {t("shortlink:create.description")}
            </p>
          </DialogHeader>
          <form onSubmit={submit}>
            <div className="flex flex-col gap-4 px-7 py-5">
              <div className="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-5">
                <div className="mb-4 flex items-center gap-2 text-[14px] font-semibold text-[var(--color-text-primary)]">
                  <span className="flex h-6 w-6 items-center justify-center rounded-md bg-[var(--color-primary-soft)] text-[var(--color-primary)]">
                    <Link2 className="h-3.5 w-3.5" />
                  </span>
                  {t("shortlink:create.target_label")}
                </div>
                <Input
                  value={targetUrl}
                  onChange={(e) => setTargetUrl(e.target.value)}
                  placeholder={t("shortlink:create.target_placeholder")}
                  className="h-11"
                  required
                />
              </div>
              <div className="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-5">
                <div className="mb-4 flex items-center gap-2 text-[14px] font-semibold text-[var(--color-text-primary)]">
                  <span className="flex h-6 w-6 items-center justify-center rounded-md bg-rgba(251,191,36,.1) text-[var(--color-warning)]" style={{ background: "rgba(251,191,36,.1)" }}>
                    <Clock className="h-3.5 w-3.5" />
                  </span>
                  {t("shortlink:create.expires_label")}
                </div>
                <DateTimePicker
                  value={expiresAt}
                  onChange={setExpiresAt}
                />
                <p className="mt-2 text-[11px] text-[var(--color-text-disabled)]">
                  {t("shortlink:create.expires_hint")}
                </p>
              </div>
            </div>
            <div className="flex justify-end gap-2.5 border-t border-[var(--color-border)] px-7 py-4">
              <Button
                type="button"
                variant="outline"
                onClick={() => setDialogOpen(false)}
                disabled={createMutation.isPending}
                className="h-10 px-6"
              >
                {t("common:actions.cancel")}
              </Button>
              <Button type="submit" disabled={createMutation.isPending} className="h-10 px-6">
                {t("common:actions.create")}
              </Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function LinkCard({
  link,
  onCopy,
  onDelete,
  deleting,
}: {
  link: ShortLink;
  onCopy: (url: string) => void;
  onDelete: (link: ShortLink) => void;
  deleting: boolean;
}) {
  const { t } = useTranslation(["shortlink", "common"]);
  const isExpired = link.expires_at ? link.expires_at < Date.now() : false;

  return (
    <div
      className={cn(
        "group overflow-hidden rounded-2xl border bg-[var(--color-surface)]",
        "transition-all duration-150 hover:-translate-y-0.5 hover:shadow-[0_8px_24px_rgba(0,0,0,0.25)]",
        isExpired ? "border-[rgba(248,113,113,0.15)]" : "border-[var(--color-border)]",
      )}
    >
      <div className="flex items-start gap-4 px-5 py-4">
        <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-[var(--color-primary-soft)] text-[var(--color-primary)]">
          <Link2 className="h-4 w-4" />
        </span>
        <div className="min-w-0 flex-1">
          {/* Short URL */}
          <div className="flex items-center gap-2">
            <code className="truncate font-mono text-[14px] font-semibold text-[var(--color-text-primary)]">
              {link.short_url}
            </code>
            <button
              type="button"
              onClick={() => onCopy(link.short_url)}
              className="shrink-0 rounded-md p-1 text-[var(--color-text-disabled)] transition hover:bg-white/[.06] hover:text-[var(--color-text-secondary)]"
              title={t("shortlink:list.copy_short_url")}
            >
              <Copy className="h-3 w-3" />
            </button>
          </div>
          {/* Target URL */}
          <a
            href={link.target_url}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-1 flex items-center gap-1 truncate text-[12px] text-[var(--color-text-tertiary)] transition hover:text-[var(--color-text-secondary)]"
          >
            <ExternalLink className="h-2.5 w-2.5 shrink-0" />
            <span className="truncate">{link.target_url}</span>
          </a>
          {/* Meta row */}
          <div className="mt-2 flex flex-wrap items-center gap-3 text-[11px] text-[var(--color-text-disabled)]">
            <span>{formatDate(link.created_at)}</span>
            <span className="h-3 w-px bg-[var(--color-border)]" />
            {link.expires_at ? (
              <span className={isExpired ? "text-[var(--color-error)]" : ""}>
                {isExpired ? "Expired" : `Expires ${formatDate(link.expires_at)}`}
              </span>
            ) : (
              <span className="rounded bg-white/[.04] px-1.5 py-0.5 text-[10px] font-medium">
                {t("shortlink:list.permanent")}
              </span>
            )}
          </div>
        </div>
        {/* Delete */}
        <button
          type="button"
          onClick={() => onDelete(link)}
          disabled={deleting}
          className="shrink-0 rounded-lg p-2 text-[var(--color-text-disabled)] opacity-0 transition group-hover:opacity-100 hover:bg-[rgba(248,113,113,0.08)] hover:text-[var(--color-error)]"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
}

function ListSkeleton() {
  return (
    <div className="flex flex-col gap-3">
      {Array.from({ length: 3 }).map((_, i) => (
        <div
          key={i}
          className="flex items-center gap-4 rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5"
        >
          <Skeleton className="h-9 w-9 rounded-xl" />
          <div className="flex-1">
            <Skeleton className="h-4 w-48" />
            <Skeleton className="mt-2 h-3 w-64" />
            <Skeleton className="mt-2 h-3 w-32" />
          </div>
        </div>
      ))}
    </div>
  );
}
