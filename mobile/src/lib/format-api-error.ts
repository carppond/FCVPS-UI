import { ApiError } from "./api-client";
import { classifySyncError } from "./sync-error";

// Loose t so both useTranslation()'s bound t and i18next.t are assignable.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type TFunction = (key: any, defaultValueOrOptions?: any) => string;

/**
 * Translate a thrown value into a localized, user-facing message. Mirror of
 * web's formatApiError: never surfaces raw backend error strings (Go network
 * dumps, internal messages) — classifies known network failures, then falls
 * back code → family → generic.
 */
export function formatApiError(err: unknown, t: TFunction): string {
  if (err instanceof ApiError) {
    const netKind = classifySyncError(err.message);
    if (netKind) {
      return t(`errors:NET_${netKind.toUpperCase()}`);
    }
    const code = err.code.replace(/^ERR_/, "");
    const exact = t(`errors:${code}`, MISSING);
    if (exact !== MISSING) return exact;
    return t(`errors:${familyKey(code)}`, t("errors:INTERNAL_UNKNOWN"));
  }
  return t("errors:INTERNAL_UNKNOWN");
}

// i18next returns the default value verbatim when a key is missing; this
// sentinel lets us probe for key existence.
const MISSING = " missing";

/** Generic family fallback for backend codes without a dedicated entry. */
function familyKey(code: string): string {
  if (code.startsWith("NOT_FOUND")) return "NOT_FOUND";
  if (code.startsWith("VALIDATION")) return "VALIDATION_FAILED";
  if (code.startsWith("CONFLICT")) return "CONFLICT";
  if (code === "AUTH_RATE_LIMITED" || code === "AUTH_BRUTE_FORCE_BLOCKED") return "RATE_LIMITED";
  if (code === "AUTH_FORBIDDEN") return "FORBIDDEN";
  if (code.startsWith("AUTH")) return "AUTH_UNAUTHORIZED";
  if (code.startsWith("INTERNAL") || code.startsWith("HTTP_5")) return "INTERNAL";
  return "INTERNAL_UNKNOWN";
}
