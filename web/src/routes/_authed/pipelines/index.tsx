import * as React from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { MoreHorizontal, Plus, Search, Workflow } from "lucide-react";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useDebounce } from "@/hooks/use-debounce";
import {
  useCreatePipeline,
  useDeletePipeline,
  usePipelines,
} from "@/api/pipeline";
import { formatDate } from "@/lib/format";
import type { Pipeline } from "@/types/api";

export const Route = createFileRoute("/_authed/pipelines/")({
  component: PipelinesListPage,
});

const PAGE_SIZE = 20;

/** Best-effort: extract operator count from `ast_json` for the list column. */
function countOperators(astJson: string): number {
  if (!astJson) return 0;
  try {
    const parsed = JSON.parse(astJson) as { operators?: unknown[] };
    return Array.isArray(parsed.operators) ? parsed.operators.length : 0;
  } catch {
    return 0;
  }
}

function PipelinesListPage() {
  const { t } = useTranslation(["pipeline", "common"]);
  const navigate = useNavigate();
  const { handle: handleError } = useApiError();

  const [searchInput, setSearchInput] = React.useState("");
  const keyword = useDebounce(searchInput, 300);
  const [page, setPage] = React.useState(1);

  const { data, isLoading, isError, error, refetch } = usePipelines({
    page,
    pageSize: PAGE_SIZE,
    keyword,
  });

  const createMutation = useCreatePipeline();
  const deleteMutation = useDeletePipeline();

  const [deleteTarget, setDeleteTarget] = React.useState<Pipeline | null>(null);

  // Default YAML scaffold for "Create" — gives the user a starting point
  // before they navigate into the editor (T-20 = list + canvas; the editor
  // can immediately surface the empty AST).
  const DEFAULT_YAML = React.useMemo(
    () =>
      [
        "apiVersion: shiguang/v1",
        "kind: Pipeline",
        "metadata:",
        "  name: untitled",
        "spec:",
        "  operators: []",
        "",
      ].join("\n"),
    [],
  );

  const handleCreate = async () => {
    try {
      const created = await createMutation.mutateAsync({
        name: t("pipeline:editor.name_placeholder"),
        yaml_content: DEFAULT_YAML,
      });
      navigate({
        to: "/pipelines/$pipelineId/edit",
        params: { pipelineId: created.id },
      });
    } catch (err) {
      handleError(err);
    }
  };

  const handleDuplicate = async (pipeline: Pipeline) => {
    try {
      const created = await createMutation.mutateAsync({
        name: `${pipeline.name} (copy)`,
        yaml_content: pipeline.yaml_content,
      });
      toast.success(t("pipeline:toast.duplicated"));
      navigate({
        to: "/pipelines/$pipelineId/edit",
        params: { pipelineId: created.id },
      });
    } catch (err) {
      handleError(err);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteMutation.mutateAsync(deleteTarget.id);
      toast.success(t("pipeline:toast.deleted"));
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
            {t("pipeline:title")}
          </h1>
          <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("pipeline:description")}
          </p>
        </div>
        <Button onClick={handleCreate} disabled={createMutation.isPending}>
          <Plus className="mr-2 h-4 w-4" />
          {t("pipeline:list.create")}
        </Button>
      </header>

      <div className="relative w-full max-w-sm">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
        <Input
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder={t("pipeline:list.search_placeholder")}
          className="pl-9"
        />
      </div>

      {isLoading ? (
        <PipelineTableSkeleton />
      ) : isError ? (
        <ErrorState
          message={
            error instanceof Error
              ? error.message
              : t("common:no_data")
          }
          onRetry={() => refetch()}
          retryLabel={t("common:actions.retry")}
        />
      ) : !data || data.items.length === 0 ? (
        <EmptyState
          icon={<Workflow />}
          title={t("pipeline:list.empty.title")}
          description={t("pipeline:list.empty.description")}
          ctaLabel={t("pipeline:list.create")}
          onCta={handleCreate}
        />
      ) : (
        <PipelineTable
          items={data.items}
          page={page}
          total={data.total}
          onPageChange={setPage}
          onDelete={(p) => setDeleteTarget(p)}
          onDuplicate={handleDuplicate}
        />
      )}

      <Dialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t("pipeline:list.delete_confirm.title")}
            </DialogTitle>
            <DialogDescription>
              {t("pipeline:list.delete_confirm.description", {
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
              {t("pipeline:list.delete_confirm.submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

// ── Table ───────────────────────────────────────────────────────────────────

interface PipelineTableProps {
  items: Pipeline[];
  page: number;
  total: number;
  onPageChange: (page: number) => void;
  onDelete: (pipeline: Pipeline) => void;
  onDuplicate: (pipeline: Pipeline) => void;
}

function PipelineTable({
  items,
  page,
  total,
  onPageChange,
  onDelete,
  onDuplicate,
}: PipelineTableProps) {
  const { t } = useTranslation(["pipeline", "common"]);
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]">
        <table className="w-full text-[var(--font-size-sm)]">
          <thead className="border-b border-[var(--color-border)] text-[var(--color-text-tertiary)]">
            <tr>
              <Th>{t("pipeline:list.columns.name")}</Th>
              <Th>{t("pipeline:list.columns.operator_count")}</Th>
              <Th>{t("pipeline:list.columns.bindings")}</Th>
              <Th>{t("pipeline:list.columns.updated_at")}</Th>
              <Th className="w-12 text-right">
                {t("pipeline:list.columns.actions")}
              </Th>
            </tr>
          </thead>
          <tbody>
            {items.map((p) => (
              <tr
                key={p.id}
                className="border-b border-[var(--color-border)] last:border-0 hover:bg-[var(--color-surface-hover)]"
              >
                <Td className="font-medium text-[var(--color-text-primary)]">
                  <Link
                    to="/pipelines/$pipelineId/edit"
                    params={{ pipelineId: p.id }}
                    className="hover:underline"
                  >
                    {p.name}
                  </Link>
                </Td>
                <Td className="tabular-nums text-[var(--color-text-secondary)]">
                  {countOperators(p.ast_json)}
                </Td>
                <Td className="text-[var(--color-text-tertiary)]">—</Td>
                <Td className="tabular-nums text-[var(--color-text-secondary)]">
                  {formatDate(p.updated_at)}
                </Td>
                <Td className="text-right">
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={t("common:aria.actions")}
                      >
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem asChild>
                        <Link
                          to="/pipelines/$pipelineId/edit"
                          params={{ pipelineId: p.id }}
                        >
                          {t("pipeline:list.actions.edit")}
                        </Link>
                      </DropdownMenuItem>
                      <DropdownMenuItem onSelect={() => onDuplicate(p)}>
                        {t("pipeline:list.actions.duplicate")}
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        onSelect={() => onDelete(p)}
                        className="text-[var(--color-error)]"
                      >
                        {t("pipeline:list.actions.delete")}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
        <span>
          {(page - 1) * PAGE_SIZE + 1} – {Math.min(page * PAGE_SIZE, total)} / {total}
        </span>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => onPageChange(Math.max(1, page - 1))}
          >
            {t("common:actions.back")}
          </Button>
          <span>
            {page} / {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => onPageChange(page + 1)}
          >
            {">"}
          </Button>
        </div>
      </div>
    </div>
  );
}

function Th({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <th
      className={`px-4 py-2 text-left text-[var(--font-size-xs)] font-medium uppercase tracking-wide ${className}`}
    >
      {children}
    </th>
  );
}

function Td({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return <td className={`px-4 py-3 align-middle ${className}`}>{children}</td>;
}

function PipelineTableSkeleton() {
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
