import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "@/components/ui/toast";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api-client";
import { useLoginMutation } from "@/api/auth";
import { useAuthStore } from "@/stores/auth-store";
import type { UserPublicProfile, User } from "@/types/api";

const SCHEMA_MIN_PASSWORD = 8;
const SCHEMA_MIN_USERNAME = 3;
const SCHEMA_MAX_USERNAME = 32;

interface FormValues {
  username: string;
  password: string;
  rememberMe: boolean;
}

function buildSchema(t: (key: string) => string) {
  return z.object({
    username: z
      .string()
      .min(1, t("auth:login.error.username_required"))
      .min(SCHEMA_MIN_USERNAME, t("auth:login.error.username_length"))
      .max(SCHEMA_MAX_USERNAME, t("auth:login.error.username_length")),
    password: z
      .string()
      .min(1, t("auth:login.error.password_required"))
      .min(SCHEMA_MIN_PASSWORD, t("auth:login.error.password_length")),
    rememberMe: z.boolean(),
  });
}

/**
 * Map a UserPublicProfile (returned by /api/auth/login) into the broader User
 * shape stored in auth-store. The public profile omits `is_active` and the
 * `updated_at` timestamp; both are filled in on the next /api/me fetch by
 * the route guard (T-6).
 */
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

export function LoginForm() {
  const { t } = useTranslation(["auth"]);
  const navigate = useNavigate();
  const loginMutation = useLoginMutation();
  const setSession = useAuthStore((s) => s.setSession);
  const setTwoFactorPending = useAuthStore((s) => s.setTwoFactorPending);

  const schema = React.useMemo(() => buildSchema(t), [t]);

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { username: "", password: "", rememberMe: false },
  });

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      const result = await loginMutation.mutateAsync({
        username: values.username,
        password: values.password,
      });
      if (result.kind === "totp_required") {
        setTwoFactorPending(result.payload.pending_token);
        await navigate({ to: "/totp" });
        return;
      }
      setSession(profileToUser(result.payload.user), result.payload.access_token);
      await navigate({ to: "/dashboard" });
    } catch (err) {
      const message = errorMessageForLogin(err, t);
      toast.error(message);
    }
  });

  const isSubmitting = loginMutation.isPending || form.formState.isSubmitting;

  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
      <div className="flex flex-col gap-2">
        <Label htmlFor="username">{t("auth:login.username_label")}</Label>
        <Input
          id="username"
          autoComplete="username"
          autoFocus
          placeholder={t("auth:login.username_placeholder")}
          aria-invalid={!!form.formState.errors.username}
          {...form.register("username")}
        />
        {form.formState.errors.username && (
          <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
            {form.formState.errors.username.message}
          </p>
        )}
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor="password">{t("auth:login.password_label")}</Label>
        <Input
          id="password"
          type="password"
          autoComplete="current-password"
          placeholder={t("auth:login.password_placeholder")}
          aria-invalid={!!form.formState.errors.password}
          {...form.register("password")}
        />
        {form.formState.errors.password && (
          <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
            {form.formState.errors.password.message}
          </p>
        )}
      </div>

      <label className="flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
        <input
          type="checkbox"
          className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
          {...form.register("rememberMe")}
        />
        {t("auth:login.remember_me")}
      </label>

      <Button type="submit" disabled={isSubmitting} className="mt-2 w-full">
        {isSubmitting ? t("auth:login.submitting") : t("auth:login.submit")}
      </Button>
    </form>
  );
}

function errorMessageForLogin(
  err: unknown,
  t: (key: string) => string,
): string {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "ERR_AUTH_INVALID_PASSWORD":
        return t("auth:login.error.invalid_credentials");
      case "ERR_AUTH_USER_INACTIVE":
        return t("auth:login.error.disabled");
      case "ERR_AUTH_RATE_LIMITED":
      case "ERR_AUTH_BRUTE_FORCE_BLOCKED":
        return t("auth:login.error.rate_limit");
      default:
        return err.message || t("auth:login.error.invalid_credentials");
    }
  }
  return t("auth:login.error.invalid_credentials");
}
