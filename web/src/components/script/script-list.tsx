import * as React from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { MoreHorizontal } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { formatDate } from "@/lib/format";
import type { Script } from "@/types/api";

export interface ScriptListProps {
  items: Script[];
  page: number;
  total: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onDelete: (script: Script) => void;
}

/**
 * ScriptList renders the table view at /scripts. Layout is identical to
 * the pipeline list (matching design system) — five columns with name as the
 * link, hook as a badge, enabled state, last_run_at and a row menu.
 */
export function ScriptList({
  items,
  page,
  total,
  pageSize,
  onPageChange,
  onDelete,
}: ScriptListProps) {
  const { t } = useTranslation(["script", "common"]);
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]">
        <table className="w-full text-[var(--font-size-sm)]">
          <thead className="border-b border-[var(--color-border)] text-[var(--color-text-tertiary)]">
            <tr>
              <Th>{t("script:list.columns.name")}</Th>
              <Th>{t("script:list.columns.hook")}</Th>
              <Th>{t("script:list.columns.enabled")}</Th>
              <Th>{t("script:list.columns.last_run_at")}</Th>
              <Th className="w-12 text-right">
                {t("script:list.columns.actions")}
              </Th>
            </tr>
          </thead>
          <tbody>
            {items.map((s) => (
              <tr
                key={s.id}
                className="border-b border-[var(--color-border)] last:border-0 hover:bg-[var(--color-surface-hover)]"
              >
                <Td className="font-medium text-[var(--color-text-primary)]">
                  <Link
                    to="/scripts/$scriptId"
                    params={{ scriptId: s.id }}
                    className="hover:underline"
                  >
                    {s.name}
                  </Link>
                  {s.last_error && (
                    <span
                      className="ml-2 inline-block text-[var(--font-size-xs)] text-[var(--color-error)]"
                      title={s.last_error}
                    >
                      *
                    </span>
                  )}
                </Td>
                <Td>
                  <Badge
                    variant={
                      s.hook === "pre_save_nodes" ? "default" : "secondary"
                    }
                  >
                    {t(`script:hook.${s.hook}`)}
                  </Badge>
                </Td>
                <Td>
                  <Badge variant={s.enabled ? "default" : "outline"}>
                    {s.enabled ? "ON" : "OFF"}
                  </Badge>
                </Td>
                <Td className="tabular-nums text-[var(--color-text-secondary)]">
                  {s.last_run_at
                    ? formatDate(s.last_run_at)
                    : t("script:list.never_run")}
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
                          to="/scripts/$scriptId"
                          params={{ scriptId: s.id }}
                        >
                          {t("script:list.actions.edit")}
                        </Link>
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        onSelect={() => onDelete(s)}
                        className="text-[var(--color-error)]"
                      >
                        {t("script:list.actions.delete")}
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
          {(page - 1) * pageSize + 1} –{" "}
          {Math.min(page * pageSize, total)} / {total}
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
