import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react-swc";
import path from "path";

/**
 * Vitest configuration.
 *
 * Kept separate from vite.config.ts so the production build does not
 * pull in vitest's type definitions. SWC handles JSX transpile for tests.
 */
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
    css: false,
    // Playwright specs live under ./e2e and use a different test runner;
    // excluding them here prevents vitest from trying to import @playwright/test
    // hooks that aren't compatible with the jsdom environment.
    exclude: ["e2e/**", "node_modules/**", "dist/**"],
  },
});
