import { test as base, expect, type Page } from "@playwright/test";

/**
 * Auth fixtures for E2E tests.
 *
 * Two strategies are exposed:
 *
 *  1. `apiLogin(page, creds)` — programmatic login via /api/auth/login that
 *     drops the access_token into localStorage under the `sgvps_auth` key
 *     (matches `useAuthStore` persist config). This skips the form for any
 *     spec that just wants to land on an authed route quickly.
 *
 *  2. `authedPage` — a fixture wrapping `apiLogin` so individual specs can
 *     write `test('foo', async ({ authedPage }) => { ... })` and start on
 *     `/dashboard` already logged in as admin.
 *
 * Credentials default to `E2E_ADMIN_USER` / `E2E_ADMIN_PASS` (overridable in
 * CI workflow env). The hub backend's EnsureAdmin step prints the bootstrap
 * password to stdout on first boot, which the workflow scrapes and exposes.
 */

export interface E2ECreds {
  username: string;
  password: string;
}

export function defaultAdminCreds(): E2ECreds {
  return {
    username: process.env.E2E_ADMIN_USER || "admin",
    password: process.env.E2E_ADMIN_PASS || "ChangeMe123!",
  };
}

/** Backend base URL — vite proxy forwards /api to it during development. */
export function apiBaseURL(): string {
  return process.env.E2E_API_URL || "http://localhost:8080";
}

/**
 * Perform an API login and seed localStorage so the SPA picks up the session
 * on the next navigation. The shape mirrors zustand's `persist` middleware:
 *   { state: { user, token }, version: 0 }
 */
/**
 * Module-level session cache. With workers=1 every spec runs in the same
 * process, so one real login covers the whole suite — important because the
 * hub rate-limits login attempts per (IP|username).
 */
let cachedSession: { username: string; token: string; user: unknown } | null =
  null;

export async function apiLogin(
  page: Page,
  creds: E2ECreds = defaultAdminCreds(),
): Promise<{ token: string; user: unknown }> {
  if (cachedSession && cachedSession.username === creds.username) {
    await seedAuthStorage(page, cachedSession.user, cachedSession.token);
    return { token: cachedSession.token, user: cachedSession.user };
  }
  const res = await page.request.post(`${apiBaseURL()}/api/auth/login`, {
    data: { username: creds.username, password: creds.password },
    headers: { "Content-Type": "application/json" },
  });
  expect(res.ok(), `login failed: ${res.status()} ${await res.text()}`).toBe(
    true,
  );
  // All hub responses use the APIResponse envelope: { code, data, ... }.
  const envelope = (await res.json()) as {
    data?: {
      access_token?: string;
      user?: unknown;
      totp_required?: boolean;
    } | null;
  };
  const body = envelope.data ?? {};
  if (body.totp_required) {
    throw new Error(
      "API login returned totp_required; the admin used by E2E must have 2FA disabled",
    );
  }
  if (!body.access_token || !body.user) {
    throw new Error(`malformed login response: ${JSON.stringify(envelope)}`);
  }
  cachedSession = {
    username: creds.username,
    token: body.access_token,
    user: body.user,
  };
  await seedAuthStorage(page, body.user, body.access_token);
  return { token: body.access_token, user: body.user };
}

/**
 * Seed the zustand-persist payload BEFORE navigation so the auth store
 * initialises with a session on first paint (no flash of /login).
 */
async function seedAuthStorage(
  page: Page,
  user: unknown,
  token: string,
): Promise<void> {
  await page.addInitScript(
    (seed) => {
      try {
        window.localStorage.setItem("sgvps_auth", seed);
      } catch {
        // localStorage may be unavailable in some isolated contexts; ignore
        // so the test can still proceed to the failure assertion.
      }
    },
    JSON.stringify({ state: { user, token }, version: 0 }),
  );
}

interface AuthFixtures {
  authedPage: Page;
}

/**
 * `test` extended with an `authedPage` fixture. Specs that need an
 * authenticated page can opt in by destructuring it from the callback.
 */
export const test = base.extend<AuthFixtures>({
  authedPage: async ({ page }, use) => {
    await apiLogin(page);
    await page.goto("/dashboard");
    await use(page);
  },
});

export { expect };
