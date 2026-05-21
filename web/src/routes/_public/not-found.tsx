import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/_public/not-found")({
  component: NotFoundPage,
});

/**
 * 404 / silent-mode placeholder page.
 *
 * Mimics an nginx default 404 page to reduce fingerprinting. Colors are mapped
 * to design tokens (text-tertiary / border) so the page still respects the
 * light/dark theme without revealing it's an SPA fallback. The `nginx/1.27.0`
 * footer is intentionally a hardcoded literal (server fingerprint imitation)
 * and is NOT translated; the user-facing 404 title still goes through i18n.
 */
function NotFoundPage() {
  const { t } = useTranslation(["common"]);
  return (
    <div className="mx-auto max-w-2xl px-6 py-16 text-center text-[var(--color-text-tertiary)]">
      <h1 className="m-0 text-[var(--font-size-4xl)] text-[var(--color-text-secondary)]">
        404
      </h1>
      <h2 className="mt-0 mb-4 text-[var(--font-size-lg)]">
        {t("common:not_found.title")}
      </h2>
      <hr className="mx-auto my-4 w-3/5 border-0 border-t border-[var(--color-border)]" />
      <p className="text-[var(--font-size-xs)]">nginx/1.27.0</p>
    </div>
  );
}
