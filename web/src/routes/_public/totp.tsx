import * as React from "react";
import {
  createFileRoute,
  Link,
  useNavigate,
  useSearch,
} from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { TotpInput, TOTP_CODE_LENGTH } from "@/components/auth/totp-input";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { ApiError } from "@/lib/api-client";
import { useAuthStore } from "@/stores/auth-store";
import { useVerifyTotpMutation } from "@/api/auth";
import type { UserPublicProfile, User } from "@/types/api";

/**
 * Search params for `/totp`. `next` is forwarded from `/login` so that the
 * post-TOTP redirect lands on the originally requested route.
 */
interface TotpSearch {
  next?: string;
}

function sanitizeNext(raw: unknown): string | undefined {
  if (typeof raw !== "string" || raw.length === 0) return undefined;
  if (raw.startsWith("//") || raw.includes("://")) return undefined;
  if (!raw.startsWith("/")) return undefined;
  return raw;
}

export const Route = createFileRoute("/_public/totp")({
  validateSearch: (search: Record<string, unknown>): TotpSearch => ({
    next: sanitizeNext(search.next),
  }),
  component: TotpPage,
});

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

function TotpPage() {
  const { t } = useTranslation(["auth"]);
  const navigate = useNavigate();
  const { next } = useSearch({ from: "/_public/totp" });
  const pendingToken = useAuthStore((s) => s.twoFactorPending);
  const setSession = useAuthStore((s) => s.setSession);
  const verifyMutation = useVerifyTotpMutation();

  const [code, setCode] = React.useState("");
  const [hasError, setHasError] = React.useState(false);

  React.useEffect(() => {
    if (!pendingToken) {
      void navigate({ to: "/login" });
    }
  }, [pendingToken, navigate]);

  const submitCode = React.useCallback(
    async (value: string) => {
      if (!pendingToken) return;
      try {
        const data = await verifyMutation.mutateAsync({
          pending_token: pendingToken,
          code: value,
        });
        setSession(profileToUser(data.user), data.access_token);
        if (next) {
          await navigate({ to: next } as unknown as Parameters<
            typeof navigate
          >[0]);
        } else {
          await navigate({ to: "/dashboard" });
        }
      } catch (err) {
        setHasError(true);
        setCode("");
        let messageKey = "auth:totp.error.invalid";
        if (err instanceof ApiError && err.code === "ERR_AUTH_TOTP_EXPIRED") {
          messageKey = "auth:totp.error.expired";
        }
        toast.error(t(messageKey));
      }
    },
    [pendingToken, verifyMutation, setSession, navigate, next, t],
  );

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (code.length !== TOTP_CODE_LENGTH) {
      setHasError(true);
      toast.error(t("auth:totp.error.incomplete"));
      return;
    }
    void submitCode(code);
  };

  return (
    <div className="w-full max-w-sm px-4">
      <div className="mb-8 text-center">
        <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
          {t("auth:totp.title")}
        </h1>
        <p className="mt-2 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("auth:totp.description")}
        </p>
      </div>

      <form onSubmit={handleSubmit} className="flex flex-col gap-6">
        <TotpInput
          value={code}
          onChange={(v) => {
            setCode(v);
            if (hasError) setHasError(false);
          }}
          onComplete={(v) => void submitCode(v)}
          hasError={hasError}
          disabled={verifyMutation.isPending}
          aria-label={t("auth:totp.title")}
        />
        <Button
          type="submit"
          disabled={verifyMutation.isPending || code.length !== TOTP_CODE_LENGTH}
        >
          {verifyMutation.isPending ? t("auth:totp.submitting") : t("auth:totp.submit")}
        </Button>
      </form>

      <div className="mt-6 flex items-center justify-between text-[var(--font-size-sm)]">
        <Link
          to="/login"
          className="text-[var(--color-text-tertiary)] hover:text-[var(--color-text-primary)]"
        >
          {t("auth:totp.back_to_login")}
        </Link>
        <Link
          to="/recovery"
          className="text-[var(--color-primary)] hover:underline"
        >
          {t("auth:totp.use_recovery_code")}
        </Link>
      </div>
    </div>
  );
}
