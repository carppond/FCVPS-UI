import * as React from "react";
import { useTranslation } from "react-i18next";
import { Copy, Link as LinkIcon, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError, formatApiError } from "@/hooks/use-api-error";
import { useDeleteShortLink, useShortLinks } from "@/api/shortlink";
import { formatDate } from "@/lib/format";
import type { ShortLink } from "@/types/api";

interface ShortLinkListProps {
  /** Triggered when the user clicks "Create" while the list is empty. */
  onCreate?: () => void;
}

/**
 * ShortLinkList — table of the current user's short links. Surfaces copy /
 * delete actions per row. Lifecycle is fully owned by TanStack Query.
 */
export function ShortLinkList({ onCreate }: ShortLinkListProps) {
  const { t } = useTranslation(["shortlink", "common"]);
  const { handle: handleError } = useApiError();
  const { data, isLoading, isError, error, refetch } = useShortLinks();
  const deleteMutation = useDeleteShortLink();

  const handleCopy = React.useCallback(
    async (url: string) => {
      try {
        await navigator.clipboard.writeText(url);
        toast.success(t("shortlink:list.copied"));
      } catch {
        toast.error(t("shortlink:list.copy_failed"));
      }
    },
    [t],
  );

  const handleDelete = React.useCallback(
    async (link: ShortLink) => {
      try {
        await deleteMutation.mutateAsync({
          fileCode: link.file_code,
          userCode: link.user_code,
        });
        toast.success(t("shortlink:list.deleted"));
      } catch (err) {
        handleError(err);
      }
    },
    [deleteMutation, handleError, t],
  );

  if (isLoading) {
    return (
      <div className="flex flex-col gap-2">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
      </div>
    );
  }

  if (isError) {
    const msg = formatApiError(error, t);
    return (
      <ErrorState
        message={t("shortlink:list.error_load") + (msg ? ` (${msg})` : "")}
        onRetry={() => refetch()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }

  const links = data ?? [];
  if (links.length === 0) {
    return (
      <EmptyState
        icon={<LinkIcon />}
        title={t("shortlink:list.empty_title")}
        description={t("shortlink:list.empty_description")}
        ctaLabel={onCreate ? t("shortlink:list.create_cta") : undefined}
        onCta={onCreate}
      />
    );
  }

  return (
    <div className="overflow-hidden rounded-md border border-[var(--color-border-subtle)]">
      <table className="w-full table-auto text-left text-[var(--font-size-sm)]">
        <thead className="bg-[var(--color-bg-subtle)] text-[var(--color-text-tertiary)]">
          <tr>
            <th className="px-4 py-2 font-medium">{t("shortlink:list.col_code")}</th>
            <th className="px-4 py-2 font-medium">{t("shortlink:list.col_target")}</th>
            <th className="px-4 py-2 font-medium">{t("shortlink:list.col_created")}</th>
            <th className="px-4 py-2 font-medium">{t("shortlink:list.col_expires")}</th>
            <th className="px-4 py-2 font-medium text-right">
              {t("shortlink:list.col_actions")}
            </th>
          </tr>
        </thead>
        <tbody>
          {links.map((link) => (
            <tr
              key={link.file_code + link.user_code}
              className="border-t border-[var(--color-border-subtle)] hover:bg-[var(--color-bg-subtle)]"
            >
              <td className="px-4 py-2 font-mono text-[var(--color-text-primary)]">
                {link.file_code}
                <span className="text-[var(--color-text-tertiary)]">/</span>
                {link.user_code}
              </td>
              <td className="max-w-xs truncate px-4 py-2 text-[var(--color-text-secondary)]">
                <a
                  href={link.target_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:underline"
                >
                  {link.target_url}
                </a>
              </td>
              <td className="px-4 py-2 text-[var(--color-text-tertiary)]">
                {formatDate(link.created_at)}
              </td>
              <td className="px-4 py-2 text-[var(--color-text-tertiary)]">
                {link.expires_at ? formatDate(link.expires_at) : t("shortlink:list.permanent")}
              </td>
              <td className="px-4 py-2">
                <div className="flex justify-end gap-1">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => handleCopy(link.short_url)}
                    title={t("shortlink:list.copy_short_url")}
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => handleDelete(link)}
                    disabled={deleteMutation.isPending}
                    title={t("common:actions.delete")}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
