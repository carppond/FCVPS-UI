import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { Eye, EyeOff } from "lucide-react";
import { toast } from "@/components/ui/toast";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api-client";
import { formatApiError } from "@/hooks/use-api-error";
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
  // `/login` validates and sanitizes `next` (see `_public/login.tsx`), so the
  // value here is guaranteed to be a same-origin pathname or undefined.
  const { next } = useSearch({ from: "/_public/login" });
  const loginMutation = useLoginMutation();
  const setSession = useAuthStore((s) => s.setSession);
  const setTwoFactorPending = useAuthStore((s) => s.setTwoFactorPending);

  const [showPassword, setShowPassword] = React.useState(false);

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
        // Forward `next` through the 2FA step so the post-TOTP navigation
        // still lands on the originally requested page.
        await navigate({ to: "/totp", search: next ? { next } : undefined });
        return;
      }
      setSession(profileToUser(result.payload.user), result.payload.access_token);
      if (next) {
        // `next` is sanitized in the route's validateSearch and may be any
        // valid pathname (e.g. dynamic detail routes), so we can't satisfy
        // the strict literal `to` type — cast through `unknown`.
        await navigate({ to: next } as unknown as Parameters<
          typeof navigate
        >[0]);
      } else {
        await navigate({ to: "/dashboard" });
      }
    } catch (err) {
      const message = errorMessageForLogin(err, t);
      toast.error(message);
    }
  });

  const isSubmitting = loginMutation.isPending || form.formState.isSubmitting;

  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-5" noValidate>
      <div
        className="login-fade-up flex flex-col gap-1.5"
        style={{ animationDelay: "0.12s" }}
      >
        <label
          htmlFor="username"
          className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]"
        >
          {t("auth:login.username_label")}
        </label>
        <div className="relative">
          <input
            id="username"
            autoComplete="username"
            autoFocus
            placeholder={t("auth:login.username_placeholder")}
            aria-invalid={!!form.formState.errors.username}
            className="peer h-10 w-full border-0 border-b border-[var(--color-border-strong)] bg-transparent px-1 text-[var(--font-size-sm)] text-[var(--color-text-primary)] outline-none transition-colors placeholder:text-[var(--color-text-tertiary)]"
            {...form.register("username")}
          />
          <span className="pointer-events-none absolute -bottom-px left-0 h-0.5 w-0 rounded-full bg-[var(--color-primary)] transition-[width] duration-[var(--duration-normal)] peer-focus:w-full" />
        </div>
        {form.formState.errors.username && (
          <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
            {form.formState.errors.username.message}
          </p>
        )}
      </div>

      <div
        className="login-fade-up flex flex-col gap-1.5"
        style={{ animationDelay: "0.22s" }}
      >
        <label
          htmlFor="password"
          className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]"
        >
          {t("auth:login.password_label")}
        </label>
        <div className="relative">
          <input
            id="password"
            type={showPassword ? "text" : "password"}
            autoComplete="current-password"
            placeholder={t("auth:login.password_placeholder")}
            aria-invalid={!!form.formState.errors.password}
            className="peer h-10 w-full border-0 border-b border-[var(--color-border-strong)] bg-transparent pl-1 pr-8 text-[var(--font-size-sm)] text-[var(--color-text-primary)] outline-none transition-colors placeholder:text-[var(--color-text-tertiary)]"
            {...form.register("password")}
          />
          <span className="pointer-events-none absolute -bottom-px left-0 h-0.5 w-0 rounded-full bg-[var(--color-primary)] transition-[width] duration-[var(--duration-normal)] peer-focus:w-full" />
          <button
            type="button"
            onClick={() => setShowPassword((s) => !s)}
            aria-label={
              showPassword
                ? t("auth:login.hide_password")
                : t("auth:login.show_password")
            }
            className="absolute bottom-1.5 right-0 flex h-7 w-7 items-center justify-center text-[var(--color-text-tertiary)] transition-colors duration-[var(--duration-fast)] hover:text-[var(--color-text-primary)]"
          >
            {showPassword ? (
              <EyeOff className="h-4 w-4" aria-hidden />
            ) : (
              <Eye className="h-4 w-4" aria-hidden />
            )}
          </button>
        </div>
        {form.formState.errors.password && (
          <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
            {form.formState.errors.password.message}
          </p>
        )}
      </div>

      <label
        className="login-fade-up flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]"
        style={{ animationDelay: "0.3s" }}
      >
        <input
          type="checkbox"
          className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)] accent-[var(--color-primary)]"
          {...form.register("rememberMe")}
        />
        {t("auth:login.remember_me")}
      </label>

      <div
        className="login-fade-up mt-1"
        style={{ animationDelay: "0.36s" }}
      >
        <Button type="submit" disabled={isSubmitting} className="w-full">
          {isSubmitting ? t("auth:login.submitting") : t("auth:login.submit")}
        </Button>
      </div>
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
        // Never surface the raw backend message — fall through to the
        // localized generic handler (family/code based).
        return formatApiError(err, t);
    }
  }
  return t("auth:login.error.invalid_credentials");
}
