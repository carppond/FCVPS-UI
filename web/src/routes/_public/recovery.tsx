import * as React from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/toast";
import { ApiError } from "@/lib/api-client";
import { useAuthStore } from "@/stores/auth-store";
import { useVerifyRecoveryMutation } from "@/api/auth";
import type { UserPublicProfile, User } from "@/types/api";

export const Route = createFileRoute("/_public/recovery")({
  component: RecoveryPage,
});

const RECOVERY_CODE_LENGTH = 8;

interface FormValues {
  code: string;
}

function profileToUser(profile: UserPublicProfile): User {
  return {
    id: profile.id,
    username: profile.username,
    role: profile.role,
    is_active: true,
    email: profile.email,
    locale: profile.locale,
    totp_enabled: profile.totp_enabled,
    created_at: profile.created_at,
    updated_at: profile.created_at,
  };
}

function RecoveryPage() {
  const { t } = useTranslation(["auth"]);
  const navigate = useNavigate();
  const pendingToken = useAuthStore((s) => s.twoFactorPending);
  const setSession = useAuthStore((s) => s.setSession);
  const verifyMutation = useVerifyRecoveryMutation();

  React.useEffect(() => {
    if (!pendingToken) {
      void navigate({ to: "/login" });
    }
  }, [pendingToken, navigate]);

  const schema = React.useMemo(
    () =>
      z.object({
        code: z
          .string()
          .min(1, t("auth:recovery.error.required"))
          .regex(/^[0-9a-fA-F]{8}$/, t("auth:recovery.error.format")),
      }),
    [t],
  );

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { code: "" },
  });

  const onSubmit = form.handleSubmit(async (values) => {
    if (!pendingToken) return;
    try {
      const data = await verifyMutation.mutateAsync({
        pending_token: pendingToken,
        code: values.code.toLowerCase(),
      });
      setSession(profileToUser(data.user));
      await navigate({ to: "/dashboard" });
    } catch (err) {
      const message =
        err instanceof ApiError &&
        (err.code === "ERR_AUTH_RECOVERY_CODE_INVALID" ||
          err.code === "ERR_AUTH_RECOVERY_CODE_EXHAUSTED")
          ? t("auth:recovery.error.invalid_or_used")
          : t("auth:recovery.error.invalid_or_used");
      toast.error(message);
    }
  });

  return (
    <div className="w-full max-w-sm px-4">
      <div className="mb-8 text-center">
        <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
          {t("auth:recovery.title")}
        </h1>
        <p className="mt-2 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("auth:recovery.description")}
        </p>
      </div>

      <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
        <div className="flex flex-col gap-2">
          <Label htmlFor="recovery-code">{t("auth:recovery.code_label")}</Label>
          <Input
            id="recovery-code"
            autoFocus
            autoComplete="off"
            spellCheck={false}
            maxLength={RECOVERY_CODE_LENGTH}
            placeholder={t("auth:recovery.code_placeholder")}
            className="font-mono tabular-nums"
            aria-invalid={!!form.formState.errors.code}
            {...form.register("code")}
          />
          {form.formState.errors.code && (
            <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
              {form.formState.errors.code.message}
            </p>
          )}
        </div>

        <Button type="submit" disabled={verifyMutation.isPending}>
          {verifyMutation.isPending
            ? t("auth:totp.submitting")
            : t("auth:recovery.submit")}
        </Button>
      </form>

      <div className="mt-6 text-center text-[var(--font-size-sm)]">
        <Link
          to="/totp"
          className="text-[var(--color-text-tertiary)] hover:text-[var(--color-text-primary)]"
        >
          {t("auth:recovery.back_to_totp")}
        </Link>
      </div>
    </div>
  );
}
