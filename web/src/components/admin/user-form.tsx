import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { z } from "zod";
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
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useCreateUserMutation, useUpdateUserMutation } from "@/api/user";
import type { CreateUserRequest, UpdateUserRequest, User, UserRole } from "@/types/api";

interface UserFormProps {
  open: boolean;
  /** When provided, the dialog is in "edit" mode for that user. */
  user?: User;
  onClose: () => void;
}

const USERNAME_MIN = 3;
const USERNAME_MAX = 32;
const PASSWORD_MIN = 8;

interface FormValues {
  username: string;
  password: string;
  role: UserRole;
  email: string;
  isActive: boolean;
}

function buildSchema(isEdit: boolean, t: (key: string) => string) {
  // In edit mode the password field is optional (empty = unchanged).
  const passwordSchema = isEdit
    ? z.string().refine((v) => v === "" || v.length >= PASSWORD_MIN, {
        message: t("auth:login.error.password_length"),
      })
    : z
        .string()
        .refine((v) => v === "" || v.length >= PASSWORD_MIN, {
          message: t("auth:login.error.password_length"),
        });
  return z.object({
    username: z
      .string()
      .min(USERNAME_MIN, t("auth:login.error.username_length"))
      .max(USERNAME_MAX, t("auth:login.error.username_length")),
    password: passwordSchema,
    role: z.enum(["admin", "user"]),
    email: z.string().refine((v) => v === "" || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v), {
      message: "invalid email",
    }),
    isActive: z.boolean(),
  });
}

export function UserForm({ open, user, onClose }: UserFormProps) {
  const { t } = useTranslation(["auth", "common"]);
  const { handle: handleError } = useApiError();
  const createMutation = useCreateUserMutation();
  const updateMutation = useUpdateUserMutation();
  const isEdit = !!user;

  const schema = React.useMemo(() => buildSchema(isEdit, t), [isEdit, t]);

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      username: user?.username ?? "",
      password: "",
      role: user?.role ?? "user",
      email: user?.email ?? "",
      isActive: user?.is_active ?? true,
    },
  });

  React.useEffect(() => {
    if (open) {
      form.reset({
        username: user?.username ?? "",
        password: "",
        role: user?.role ?? "user",
        email: user?.email ?? "",
        isActive: user?.is_active ?? true,
      });
    }
  }, [open, user, form]);

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      if (isEdit && user) {
        const payload: UpdateUserRequest = {
          username: values.username,
          role: values.role,
          email: values.email || undefined,
          is_active: values.isActive,
        };
        await updateMutation.mutateAsync({ id: user.id, payload });
        toast.success(t("auth:admin_users.form.edit_success"));
      } else {
        const payload: CreateUserRequest = {
          username: values.username,
          password: values.password,
          role: values.role,
          email: values.email || undefined,
        };
        await createMutation.mutateAsync(payload);
        toast.success(t("auth:admin_users.form.create_success"));
      }
      onClose();
    } catch (err) {
      handleError(err);
    }
  });

  const isPending = createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={(o) => !o && !isPending && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {isEdit
              ? t("auth:admin_users.form.edit_title")
              : t("auth:admin_users.form.create_title")}
          </DialogTitle>
        </DialogHeader>

        <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
          <div className="flex flex-col gap-2">
            <Label htmlFor="user-username">
              {t("auth:admin_users.form.username_label")}
            </Label>
            <Input
              id="user-username"
              {...form.register("username")}
              aria-invalid={!!form.formState.errors.username}
            />
            {form.formState.errors.username && (
              <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
                {form.formState.errors.username.message}
              </p>
            )}
          </div>

          {!isEdit && (
            <div className="flex flex-col gap-2">
              <Label htmlFor="user-password">
                {t("auth:admin_users.form.password_label")}
              </Label>
              <Input
                id="user-password"
                type="password"
                placeholder={t("auth:admin_users.form.password_hint")}
                {...form.register("password")}
                aria-invalid={!!form.formState.errors.password}
              />
              {form.formState.errors.password && (
                <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
                  {form.formState.errors.password.message}
                </p>
              )}
            </div>
          )}

          <div className="flex flex-col gap-2">
            <Label htmlFor="user-role">
              {t("auth:admin_users.form.role_label")}
            </Label>
            <select
              id="user-role"
              {...form.register("role")}
              className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
            >
              <option value="user">{t("auth:admin_users.role.user")}</option>
              <option value="admin">{t("auth:admin_users.role.admin")}</option>
            </select>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="user-email">
              {t("auth:admin_users.form.email_label")}
            </Label>
            <Input id="user-email" type="email" {...form.register("email")} />
          </div>

          {isEdit && (
            <label className="flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
              <input
                type="checkbox"
                className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
                {...form.register("isActive")}
              />
              {t("auth:admin_users.form.is_active_label")}
            </label>
          )}

          <DialogFooter className="mt-2">
            <Button type="button" variant="outline" onClick={onClose} disabled={isPending}>
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" disabled={isPending}>
              {isEdit
                ? t("auth:admin_users.form.submit_edit")
                : t("auth:admin_users.form.submit_create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
