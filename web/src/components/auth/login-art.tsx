import * as React from "react";
import { useTranslation } from "react-i18next";
import { Activity } from "lucide-react";

/**
 * Login left-hand brand panel: a dark hero with the "拾光娘" mascot illustration.
 *
 * The illustration is served from `public/login-art.png` so a deployer can drop
 * in their own character without touching code. If that asset is missing or
 * fails to load, we fall back to a token-built emblem so the panel never breaks.
 *
 * Colours are composed from theme-independent tokens (neutral + chart scale) so
 * the panel stays dark in both light and dark themes.
 */
const HERO_BG =
  "radial-gradient(125% 125% at 0% 0%, color-mix(in srgb, var(--color-chart-2) 50%, var(--color-neutral-200)) 0%, var(--color-neutral-200) 64%)";
const HALO_BG =
  "radial-gradient(circle, color-mix(in srgb, var(--color-primary) 45%, transparent), color-mix(in srgb, var(--color-chart-2) 28%, transparent) 45%, transparent 70%)";

export function LoginArt() {
  const { t } = useTranslation(["auth"]);
  const [fallback, setFallback] = React.useState(false);
  const stageRef = React.useRef<HTMLDivElement>(null);

  // Subtle pointer parallax — the mascot drifts toward the cursor.
  const onMove = React.useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const el = stageRef.current;
    if (!el) return;
    const r = e.currentTarget.getBoundingClientRect();
    const x = (e.clientX - r.left - r.width / 2) / r.width;
    const y = (e.clientY - r.top - r.height / 2) / r.height;
    el.style.transform = `translate(${x * 14}px, ${y * 12}px)`;
  }, []);
  const onLeave = React.useCallback(() => {
    if (stageRef.current) stageRef.current.style.transform = "";
  }, []);

  return (
    <div
      className="relative hidden overflow-hidden text-[var(--color-neutral-950)] md:flex md:flex-col"
      style={{ background: HERO_BG }}
      onMouseMove={onMove}
      onMouseLeave={onLeave}
    >
      <div
        className="login-halo pointer-events-none absolute left-1/2 top-[42%] h-96 w-96 blur-2xl"
        style={{ background: HALO_BG }}
        aria-hidden
      />
      <span
        className="login-sparkle pointer-events-none absolute left-[20%] top-[22%] h-1.5 w-1.5 rounded-full bg-[var(--color-neutral-950)]"
        aria-hidden
      />
      <span
        className="login-sparkle pointer-events-none absolute right-[18%] top-[32%] h-1.5 w-1.5 rounded-full bg-[var(--color-neutral-950)]"
        style={{ animationDelay: "0.8s" }}
        aria-hidden
      />
      <span
        className="login-sparkle pointer-events-none absolute right-[26%] bottom-[30%] h-1 w-1 rounded-full bg-[var(--color-neutral-950)]"
        style={{ animationDelay: "1.6s" }}
        aria-hidden
      />

      <div
        ref={stageRef}
        className="relative flex flex-1 items-end justify-center transition-transform duration-[var(--duration-fast)]"
      >
        {fallback ? (
          <div className="login-float mb-12 flex h-32 w-32 items-center justify-center rounded-[var(--radius-xl)] border border-[var(--color-border-strong)] bg-[var(--color-neutral-100)]">
            <Activity className="h-14 w-14 text-[var(--color-primary)]" aria-hidden />
          </div>
        ) : (
          <img
            src="/login-art.png"
            alt={t("auth:login.character_alt")}
            onError={() => setFallback(true)}
            className="login-float pointer-events-none max-h-[80%] w-auto object-contain object-bottom drop-shadow-[var(--shadow-xl)]"
          />
        )}
      </div>

      <div className="login-fade-up relative z-10 px-8 pb-8 text-center">
        <p className="text-[var(--font-size-lg)] font-semibold tracking-wide">
          {t("auth:login.brand_name")}
        </p>
        <p className="mt-1 text-[var(--font-size-xs)] text-[var(--color-neutral-800)]">
          {t("auth:login.brand_tagline")}
        </p>
      </div>
    </div>
  );
}
