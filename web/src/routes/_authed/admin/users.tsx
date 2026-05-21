import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Search, UserPlus } from "lucide-react";
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
import { useDebounce } from "@/hooks/use-debounce";
import { useApiError } from "@/hooks/use-api-error";
import { UserTable } from "@/components/admin/user-table";
import { UserForm } from "@/components/admin/user-form";
import { ResetPasswordDialog } from "@/components/admin/reset-password-dialog";
import {
  useDeleteUserMutation,
  useForceDisable2FAMutation,
  useResetUserPasswordMutation,
} from "@/api/user";
import type { User } from "@/types/api";

export const Route = createFileRoute("/_authed/admin/users")({
  component: AdminUsersPage,
});

function AdminUsersPage() {
  const { t } = useTranslation(["auth", "common"]);
  const { handle: handleError } = useApiError();

  const [searchInput, setSearchInput] = React.useState("");
  const keyword = useDebounce(searchInput, 300);

  // Dialog state (one slot per dialog).
  const [formOpen, setFormOpen] = React.useState(false);
  const [editingUser, setEditingUser] = React.useState<User | undefined>(undefined);

  const [deleteTarget, setDeleteTarget] = React.useState<User | null>(null);
  const [disable2faTarget, setDisable2faTarget] = React.useState<User | null>(null);

  const [resetTarget, setResetTarget] = React.useState<User | null>(null);
  const [resetPassword, setResetPassword] = React.useState("");
  const [resetOpen, setResetOpen] = React.useState(false);

  const deleteMutation = useDeleteUserMutation();
  const disable2faMutation = useForceDisable2FAMutation();
  const resetMutation = useResetUserPasswordMutation();

  const openCreate = () => {
    setEditingUser(undefined);
    setFormOpen(true);
  };

  const openEdit = (user: User) => {
    setEditingUser(user);
    setFormOpen(true);
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteMutation.mutateAsync(deleteTarget.id);
      toast.success(t("auth:admin_users.delete_confirm.success"));
      setDeleteTarget(null);
    } catch (err) {
      handleError(err);
    }
  };

  const confirmDisable2FA = async () => {
    if (!disable2faTarget) return;
    try {
      await disable2faMutation.mutateAsync(disable2faTarget.id);
      toast.success(t("auth:admin_users.disable_2fa_confirm.success"));
      setDisable2faTarget(null);
    } catch (err) {
      handleError(err);
    }
  };

  const handleResetPassword = async (user: User) => {
    setResetTarget(user);
    setResetPassword("");
    setResetOpen(true);
    try {
      const data = await resetMutation.mutateAsync(user.id);
      setResetPassword(data.new_password);
      toast.success(t("auth:admin_users.reset_password.success"));
    } catch (err) {
      setResetOpen(false);
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <header className="flex items-end justify-between">
        <div>
          <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
            {t("auth:admin_users.title")}
          </h1>
          <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("auth:admin_users.description")}
          </p>
        </div>
        <Button onClick={openCreate}>
          <UserPlus className="mr-2 h-4 w-4" />
          {t("auth:admin_users.create_user")}
        </Button>
      </header>

      <div className="relative w-full max-w-sm">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
        <Input
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          placeholder={t("auth:admin_users.search_placeholder")}
          className="pl-9"
        />
      </div>

      <UserTable
        keyword={keyword}
        onEdit={openEdit}
        onCreate={openCreate}
        onResetPassword={handleResetPassword}
        onForceDisable2FA={(u) => setDisable2faTarget(u)}
        onDelete={(u) => setDeleteTarget(u)}
      />

      <UserForm
        open={formOpen}
        user={editingUser}
        onClose={() => setFormOpen(false)}
      />

      <ResetPasswordDialog
        open={resetOpen}
        username={resetTarget?.username ?? ""}
        newPassword={resetPassword}
        onClose={() => setResetOpen(false)}
      />

      {/* Delete confirmation */}
      <Dialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("auth:admin_users.delete_confirm.title")}</DialogTitle>
            <DialogDescription>
              {t("auth:admin_users.delete_confirm.description", {
                username: deleteTarget?.username ?? "",
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
              {t("auth:admin_users.delete_confirm.submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Force-disable 2FA confirmation */}
      <Dialog
        open={!!disable2faTarget}
        onOpenChange={(o) => !o && setDisable2faTarget(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t("auth:admin_users.disable_2fa_confirm.title")}
            </DialogTitle>
            <DialogDescription>
              {t("auth:admin_users.disable_2fa_confirm.description", {
                username: disable2faTarget?.username ?? "",
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDisable2faTarget(null)}
              disabled={disable2faMutation.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={confirmDisable2FA}
              disabled={disable2faMutation.isPending}
            >
              {t("auth:admin_users.disable_2fa_confirm.submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
