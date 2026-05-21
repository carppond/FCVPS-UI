import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { LoginForm } from "@/components/auth/login-form";

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
  const { t } = useTranslation(["auth", "common"]);
  return (
    <div className="w-full max-w-sm px-4">
      <div className="mb-8 text-center">
        <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
          {t("auth:login.title")}
        </h1>
        <p className="mt-2 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("auth:login.subtitle")}
        </p>
      </div>
      <LoginForm />
    </div>
  );
}
