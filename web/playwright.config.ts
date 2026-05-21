import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright config for shiguang-vps end-to-end tests.
 *
 * Design notes:
 *  - `fullyParallel: false` + `workers: 1` keep specs serialized to avoid
 *    contention on the shared admin account / SQLite WAL.
 *  - CI mode disables the auto webServer; the workflow brings up `go run` for
 *    the hub and `pnpm dev` for vite ahead of time so we can wait-on both.
 *  - `trace: 'on-first-retry'` keeps the artefacts small during the happy
 *    path while still giving us a full trace when something breaks.
 */
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: process.env.CI ? "github" : "list",
  timeout: 30_000,
  expect: {
    timeout: 5_000,
  },
  use: {
    baseURL: process.env.E2E_BASE_URL || "http://localhost:5173",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    headless: !!process.env.CI,
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: process.env.CI
    ? undefined
    : {
        command: "pnpm dev",
        port: 5173,
        timeout: 60_000,
        reuseExistingServer: true,
      },
});
