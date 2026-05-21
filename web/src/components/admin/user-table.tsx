import * as React from "react";
import { useTranslation } from "react-i18next";
import { MoreHorizontal, Users } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
import { useListUsersQuery } from "@/api/user";
import { formatDate } from "@/lib/format";
import type { User } from "@/types/api";

export interface UserTableProps {
  /** Search keyword applied to /api/admin/users?keyword=. */
  keyword?: string;
  /** Triggered when the admin clicks "Edit" on a row. */
  onEdit: (user: User) => void;
  /** Triggered when the admin clicks "Reset password" on a row. */
  onResetPassword: (user: User) => void;
  /** Triggered when the admin clicks "Force disable 2FA" on a row. */
  onForceDisable2FA: (user: User) => void;
  /** Triggered when the admin clicks "Delete" on a row. */
  onDelete: (user: User) => void;
  /** Show the empty-state CTA — typically wired to the "Create user" button. */
  onCreate?: () => void;
}

const PAGE_SIZE = 20;

export function UserTable({
  keyword,
  onEdit,
  onResetPassword,
  onForceDisable2FA,
  onDelete,
  onCreate,
}: UserTableProps) {
  const { t } = useTranslation(["auth", "common"]);
  const [page, setPage] = React.useState(1);

  const { data, isLoading, isError, error, refetch } = useListUsersQuery({
    page,
    pageSize: PAGE_SIZE,
    keyword,
  });

  if (isLoading) {
    return <UserTableSkeleton />;
  }

  if (isError) {
    const errMsg =
      error instanceof Error ? error.message : String(error ?? "");
    return (
      <ErrorState
        message={t("auth:admin_users.error.load_failed") + (errMsg ? ` (${errMsg})` : "")}
        onRetry={() => refetch()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }

  const items = data?.items ?? [];
  if (items.length === 0) {
    return (
      <EmptyState
        icon={<Users />}
        title={t("auth:admin_users.empty.title")}
        description={t("auth:admin_users.empty.description")}
        ctaLabel={onCreate ? t("auth:admin_users.create_user") : undefined}
        onCta={onCreate}
      />
    );
  }

  const totalPages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1;

  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]">
        <table className="w-full text-[var(--font-size-sm)]">
          <thead className="border-b border-[var(--color-border)] text-[var(--color-text-tertiary)]">
            <tr>
              <Th>{t("auth:admin_users.columns.username")}</Th>
              <Th>{t("auth:admin_users.columns.email")}</Th>
              <Th>{t("auth:admin_users.columns.role")}</Th>
              <Th>{t("auth:admin_users.columns.status")}</Th>
              <Th>{t("auth:admin_users.columns.totp")}</Th>
              <Th>{t("auth:admin_users.columns.created_at")}</Th>
              <Th className="w-12 text-right">
                {t("auth:admin_users.columns.actions")}
              </Th>
            </tr>
          </thead>
          <tbody>
            {items.map((user) => (
              <UserRow
                key={user.id}
                user={user}
                onEdit={onEdit}
                onResetPassword={onResetPassword}
                onForceDisable2FA={onForceDisable2FA}
                onDelete={onDelete}
              />
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
        <span>
          {(page - 1) * PAGE_SIZE + 1} – {Math.min(page * PAGE_SIZE, data?.total ?? 0)}{" "}
          / {data?.total ?? 0}
        </span>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
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
            onClick={() => setPage((p) => p + 1)}
          >
            {">"}
          </Button>
        </div>
      </div>
    </div>
  );
}

interface UserRowProps {
  user: User;
  onEdit: (user: User) => void;
  onResetPassword: (user: User) => void;
  onForceDisable2FA: (user: User) => void;
  onDelete: (user: User) => void;
}

function UserRow({
  user,
  onEdit,
  onResetPassword,
  onForceDisable2FA,
  onDelete,
}: UserRowProps) {
  const { t } = useTranslation(["auth", "common"]);
  return (
    <tr className="border-b border-[var(--color-border)] last:border-0 hover:bg-[var(--color-surface-hover)]">
      <Td className="font-medium text-[var(--color-text-primary)]">
        {user.username}
      </Td>
      <Td className="text-[var(--color-text-secondary)]">{user.email ?? "—"}</Td>
      <Td>
        <Badge variant={user.role === "admin" ? "default" : "secondary"}>
          {t(`auth:admin_users.role.${user.role}`)}
        </Badge>
      </Td>
      <Td>
        <Badge variant={user.is_active ? "outline" : "destructive"}>
          {t(
            user.is_active
              ? "auth:admin_users.status.active"
              : "auth:admin_users.status.inactive",
          )}
        </Badge>
      </Td>
      <Td>
        <Badge variant={user.totp_enabled ? "outline" : "secondary"}>
          {user.totp_enabled
            ? t("auth:profile.two_factor.status_enabled")
            : t("auth:profile.two_factor.status_disabled")}
        </Badge>
      </Td>
      <Td className="text-[var(--color-text-secondary)] tabular-nums">
        {formatDate(user.created_at)}
      </Td>
      <Td className="text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label={t("common:aria.actions")}>
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => onEdit(user)}>
              {t("auth:admin_users.actions.edit")}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => onResetPassword(user)}>
              {t("auth:admin_users.actions.reset_password")}
            </DropdownMenuItem>
            {user.totp_enabled && (
              <DropdownMenuItem onSelect={() => onForceDisable2FA(user)}>
                {t("auth:admin_users.actions.disable_2fa")}
              </DropdownMenuItem>
            )}
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={() => onDelete(user)}
              className="text-[var(--color-error)]"
            >
              {t("auth:admin_users.actions.delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </Td>
    </tr>
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

function UserTableSkeleton() {
  return (
    <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]">
      <div className="flex items-center gap-4 border-b border-[var(--color-border)] px-4 py-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-4 flex-1" />
        ))}
      </div>
      {Array.from({ length: 6 }).map((_, row) => (
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
