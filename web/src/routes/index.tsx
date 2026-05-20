import { createFileRoute } from "@tanstack/react-router";

/**
 * Root index "/" — renders welcome content outside of any layout group.
 * Once auth is implemented (T-6), this will redirect authenticated users to /dashboard.
 */
export const Route = createFileRoute("/")({
  component: WelcomePage,
});

function WelcomePage() {
  return (
    <div
      className="flex min-h-screen items-center justify-center bg-[var(--color-bg)]"
    >
      <div className="text-center">
        <h1 className="text-[var(--font-size-2xl)] font-bold text-[var(--color-text-primary)]">
          Welcome to 拾光VPS
        </h1>
        <p className="mt-2 text-[var(--color-text-tertiary)]">
          Self-hosted VPS panel
        </p>
      </div>
    </div>
  );
}
