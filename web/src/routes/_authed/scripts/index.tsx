import * as React from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Code2, Plus, Search } from "lucide-react";
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
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError, formatApiError } from "@/hooks/use-api-error";
import { useDebounce } from "@/hooks/use-debounce";
import { ScriptList } from "@/components/script/script-list";
import {
  useCreateScript,
  useDeleteScript,
  useScripts,
} from "@/api/script";
import i18n from "@/lib/i18n";
import scriptZh from "@/locales/zh-CN/script.json";
import scriptEn from "@/locales/en/script.json";
import scriptJa from "@/locales/ja/script.json";
import scriptKo from "@/locales/ko/script.json";
import type { HookType, Script } from "@/types/api";

// Lazy-register the "script" namespace before the route mounts. Same pattern
// as /nodes — keeps first-screen i18n bundle slim per tech-lead-plan §2.3.
function ensureScriptNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "script")) {
    i18n.addResourceBundle("zh-CN", "script", scriptZh, true, true);
    i18n.addResourceBundle("en", "script", scriptEn, true, true);
    i18n.addResourceBundle("ja", "script", scriptJa, true, true);
    i18n.addResourceBundle("ko", "script", scriptKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/scripts/")({
  beforeLoad: () => {
    ensureScriptNamespace();
  },
  component: ScriptsListPage,
});

const PAGE_SIZE = 20;

function ScriptsListPage() {
  const { t } = useTranslation(["script", "common"]);
  const navigate = useNavigate();
  const { handle: handleError } = useApiError();

  const [searchInput, setSearchInput] = React.useState("");
  const keyword = useDebounce(searchInput, 300);
  const [hookFilter, setHookFilter] = React.useState<HookType | "">("");
  const [page, setPage] = React.useState(1);

  const { data, isLoading, isError, error, refetch } = useScripts({
    page,
    pageSize: PAGE_SIZE,
    keyword,
    hook: hookFilter,
  });

  const createMutation = useCreateScript();
  const deleteMutation = useDeleteScript();

  const [deleteTarget, setDeleteTarget] = React.useState<Script | null>(null);
  const [createOpen, setCreateOpen] = React.useState(false);
  const [createName, setCreateName] = React.useState("");
  const [createHook, setCreateHook] = React.useState<HookType>(
    "pre_save_nodes",
  );

  const boilerplate = React.useCallback(
    (hook: HookType) =>
      hook === "pre_save_nodes"
        ? t("script:editor.boilerplate_pre_save")
        : t("script:editor.boilerplate_post_fetch"),
    [t],
  );

  const handleCreate = async () => {
    const name = createName.trim() || t("script:editor.name_placeholder");
    try {
      const created = await createMutation.mutateAsync({
        name,
        hook: createHook,
        code: boilerplate(createHook),
        enabled: true,
      });
      toast.success(t("script:toast.created"));
      setCreateOpen(false);
      setCreateName("");
      navigate({
        to: "/scripts/$scriptId",
        params: { scriptId: created.id },
      });
    } catch (err) {
      handleError(err);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteMutation.mutateAsync(deleteTarget.id);
      toast.success(t("script:toast.deleted"));
      setDeleteTarget(null);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <header className="flex items-end justify-between">
        <div>
          <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
            {t("script:title")}
          </h1>
          <p className="mt-1 max-w-2xl text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("script:description")}
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("script:list.create")}
        </Button>
      </header>

      <div className="flex items-center gap-3">
        <div className="relative w-full max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t("script:list.search_placeholder")}
            className="pl-9"
          />
        </div>
        <select
          value={hookFilter}
          onChange={(e) =>
            setHookFilter(e.target.value as HookType | "")
          }
          aria-label={t("script:list.filter_hook")}
          className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
        >
          <option value="">{t("script:list.filter_all")}</option>
          <option value="pre_save_nodes">
            {t("script:hook.pre_save_nodes")}
          </option>
          <option value="post_fetch">{t("script:hook.post_fetch")}</option>
        </select>
      </div>

      {isLoading ? (
        <ScriptTableSkeleton />
      ) : isError ? (
        <ErrorState
          message={
            formatApiError(error, t)
          }
          onRetry={() => refetch()}
          retryLabel={t("common:actions.retry")}
        />
      ) : !data || data.items.length === 0 ? (
        <EmptyState
          icon={<Code2 />}
          title={t("script:list.empty.title")}
          description={t("script:list.empty.description")}
          ctaLabel={t("script:list.create")}
          onCta={() => setCreateOpen(true)}
        />
      ) : (
        <ScriptList
          items={data.items}
          page={page}
          total={data.total}
          pageSize={PAGE_SIZE}
          onPageChange={setPage}
          onDelete={(s) => setDeleteTarget(s)}
        />
      )}

      {/* Create dialog: minimal — name + hook choice. The editor handles
          code editing on the detail page so this dialog stays tight. */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("script:list.create")}</DialogTitle>
            <DialogDescription>{t("script:description")}</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <label className="text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
              {t("script:editor.name_placeholder")}
              <Input
                className="mt-1"
                value={createName}
                onChange={(e) => setCreateName(e.target.value)}
              />
            </label>
            <label className="text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
              {t("script:list.filter_hook")}
              <select
                className="mt-1 block h-9 w-full rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 text-[var(--color-text-primary)]"
                value={createHook}
                onChange={(e) =>
                  setCreateHook(e.target.value as HookType)
                }
              >
                <option value="pre_save_nodes">
                  {t("script:hook.pre_save_nodes")}
                </option>
                <option value="post_fetch">
                  {t("script:hook.post_fetch")}
                </option>
              </select>
            </label>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setCreateOpen(false)}
              disabled={createMutation.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              onClick={handleCreate}
              disabled={createMutation.isPending}
            >
              {t("script:list.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t("script:list.delete_confirm.title")}
            </DialogTitle>
            <DialogDescription>
              {t("script:list.delete_confirm.description", {
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
              {t("script:list.delete_confirm.submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function ScriptTableSkeleton() {
  return (
    <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]">
      <div className="flex items-center gap-4 border-b border-[var(--color-border)] px-4 py-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-4 flex-1" />
        ))}
      </div>
      {Array.from({ length: 4 }).map((_, row) => (
        <div
          key={row}
          className="flex items-center gap-4 border-b border-[var(--color-border)] px-4 py-3 last:border-0"
        >
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-4 flex-1" />
          ))}
        </div>
      ))}
    </div>
  );
}
