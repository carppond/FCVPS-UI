import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "@/components/ui/toast";
import { ApiError, NotFoundError } from "@/lib/api-client";

/**
 * Minimal contract for the i18next `t` function used by formatApiError.
 *
 * We intentionally type the second argument very loosely (string | object)
 * so this signature is structurally assignable from both the react-i18next
 * bound version (`TFunction<[ns, ...]>`) and the raw `i18next.t`. The real
 * runtime contract we rely on: `t(key, defaultValueString)` — i18next treats
 * a string second arg as the default value (see i18next docs §Essentials).
 */
type TFunction = (
  key: string | string[],
  defaultValueOrOptions?: string | Record<string, unknown>,
) => string;

/**
 * Translate an arbitrary thrown value into a user-facing message.
 *
 * Resolution order:
 *   1. ApiError.code   → t('errors:<code>', fallbackMessage)
 *   2. NotFoundError   → t('errors:NOT_FOUND')
 *   3. Error instance  → t('errors:INTERNAL_UNKNOWN', err.message)
 *   4. Anything else   → t('errors:INTERNAL_UNKNOWN')
 *
 * Safe to call from non-React code (e.g. QueryCache.onError) — does not
 * touch React hooks; pass in any compatible t function.
 */
export function formatApiError(err: unknown, t: TFunction): string {
  if (err instanceof ApiError) {
    return t(`errors:${err.code}`, err.message);
  }
  if (err instanceof NotFoundError) {
    return t("errors:NOT_FOUND");
  }
  if (err instanceof Error) {
    return t("errors:INTERNAL_UNKNOWN", err.message);
  }
  return t("errors:INTERNAL_UNKNOWN");
}

/**
 * React hook to render API errors as toast notifications inside components.
 *
 * Usage:
 *   const { handle } = useApiError();
 *   mutation.mutate(payload, { onError: handle });
 */
export function useApiError(): { handle: (error: unknown) => void } {
  const { t } = useTranslation();

  const handle = useCallback(
    (error: unknown) => {
      const message = formatApiError(error, t as unknown as TFunction);
      toast.error(message);
    },
    [t],
  );

  return { handle };
}
