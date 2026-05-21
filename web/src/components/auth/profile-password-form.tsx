import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useChangePasswordMutation } from "@/api/user";

interface PasswordFormValues {
  oldPassword: string;
  newPassword: string;
  confirm: string;
}

export function ProfilePasswordForm() {
  const { t } = useTranslation(["auth"]);
  const { handle: handleError } = useApiError();
  const changeMutation = useChangePasswordMutation();

  const schema = React.useMemo(
    () =>
      z
        .object({
          oldPassword: z.string().min(1),
          newPassword: z.string().min(8, t("auth:login.error.password_length")),
          confirm: z.string().min(1),
        })
        .refine((v) => v.newPassword === v.confirm, {
          message: t("auth:profile.change_password.error_mismatch"),
          path: ["confirm"],
        })
        .refine((v) => v.newPassword !== v.oldPassword, {
          message: t("auth:profile.change_password.error_same"),
          path: ["newPassword"],
        }),
    [t],
  );

  const form = useForm<PasswordFormValues>({
    resolver: zodResolver(schema),
    defaultValues: { oldPassword: "", newPassword: "", confirm: "" },
  });

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      await changeMutation.mutateAsync({
        old_password: values.oldPassword,
        new_password: values.newPassword,
      });
      toast.success(t("auth:profile.change_password.success"));
      form.reset({ oldPassword: "", newPassword: "", confirm: "" });
    } catch (err) {
      handleError(err);
    }
  });

  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
      <PasswordField
        id="pw-old"
        label={t("auth:profile.change_password.old_label")}
        register={form.register("oldPassword")}
        error={form.formState.errors.oldPassword?.message}
      />
      <PasswordField
        id="pw-new"
        label={t("auth:profile.change_password.new_label")}
        register={form.register("newPassword")}
        error={form.formState.errors.newPassword?.message}
      />
      <PasswordField
        id="pw-confirm"
        label={t("auth:profile.change_password.confirm_label")}
        register={form.register("confirm")}
        error={form.formState.errors.confirm?.message}
      />
      <div>
        <Button type="submit" disabled={changeMutation.isPending}>
          {t("auth:profile.change_password.submit")}
        </Button>
      </div>
    </form>
  );
}

function PasswordField({
  id,
  label,
  register,
  error,
}: {
  id: string;
  label: string;
  register: ReturnType<ReturnType<typeof useForm<PasswordFormValues>>["register"]>;
  error?: string;
}) {
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={id}>{label}</Label>
      <Input id={id} type="password" autoComplete="new-password" {...register} />
      {error && (
        <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">{error}</p>
      )}
    </div>
  );
}
