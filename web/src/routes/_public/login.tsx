import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { LoginForm } from "@/components/auth/login-form";
import { LoginArt } from "@/components/auth/login-art";

/**
 * Search params for `/login`.
 *
 * `next` is the path the user originally tried to visit; after a successful
 * login we navigate them back there (see `LoginForm.onSubmit`). We only accept
 * same-origin pathnames — absolute URLs and `//host` strings are dropped to
 * prevent open-redirect attacks.
 */
interface LoginSearch {
  next?: string;
}

function sanitizeNext(raw: unknown): string | undefined {
  if (typeof raw !== "string" || raw.length === 0) return undefined;
  // Reject anything that looks like an absolute URL or protocol-relative URL
  // (e.g. `//evil.com/foo`, `http://...`). Only allow same-origin paths.
  if (raw.startsWith("//") || raw.includes("://")) return undefined;
  if (!raw.startsWith("/")) return undefined;
  return raw;
}

export const Route = createFileRoute("/_public/login")({
  validateSearch: (search: Record<string, unknown>): LoginSearch => ({
    next: sanitizeNext(search.next),
  }),
  component: LoginPage,
});

function LoginPage() {
  const { t } = useTranslation(["auth"]);
  return (
    <div className="login-rise grid w-full max-w-3xl grid-cols-1 overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface-solid)] shadow-[var(--shadow-xl)] md:min-h-96 md:grid-cols-2">
      <LoginArt />
      <div className="flex flex-col justify-center p-8 sm:p-10">
        <h1 className="text-[var(--font-size-xl)] font-semibold text-[var(--color-text-primary)]">
          {t("auth:login.welcome")}
        </h1>
        <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("auth:login.subtitle")}
        </p>
        <div className="mt-6">
          <LoginForm />
        </div>
      </div>
    </div>
  );
}
