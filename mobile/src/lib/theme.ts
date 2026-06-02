// 拾光VPS 移动端「二次元」主题 —— 粉橙主色 + 紫光点缀。
// 保留全部原有 color key(各屏直接引用),新增 primary2 / purple / cyan 供渐变与光晕使用。

export const lightColors = {
  bg: "#fbf7ff",
  surface: "#ffffff",
  surfaceHover: "#f3eefb",
  elevated: "#faf7ff",
  border: "rgba(120,80,160,0.12)",
  borderStrong: "rgba(120,80,160,0.2)",
  primary: "#ff5d83",
  primary2: "#ff8a5c",
  purple: "#8b5cf6",
  cyan: "#2bc4d6",
  primarySoft: "rgba(255,93,131,0.12)",
  textPrimary: "#2a2336",
  textSecondary: "#6a6280",
  textTertiary: "#9a93ad",
  textDisabled: "#c3bdd0",
  success: "#22c55e",
  successBg: "rgba(34,197,94,0.12)",
  warning: "#f59e0b",
  warningBg: "rgba(245,158,11,0.12)",
  error: "#ef4444",
  errorBg: "rgba(239,68,68,0.12)",
  info: "#6b9bff",
  infoBg: "rgba(107,155,255,0.12)",
} as const;

export const darkColors = {
  bg: "#0b0a0f",
  surface: "#16131f",
  surfaceHover: "#1d1930",
  elevated: "#221d33",
  border: "rgba(255,255,255,0.08)",
  borderStrong: "rgba(255,255,255,0.15)",
  primary: "#ff6b8a",
  primary2: "#ff9a6b",
  purple: "#9b6bff",
  cyan: "#4bd6e6",
  primarySoft: "rgba(255,107,138,0.16)",
  textPrimary: "#f3eefb",
  textSecondary: "#b4adc6",
  textTertiary: "#6d6783",
  textDisabled: "#45405a",
  success: "#43e08a",
  successBg: "rgba(67,224,138,0.16)",
  warning: "#ffb44a",
  warningBg: "rgba(255,180,74,0.16)",
  error: "#ff5d73",
  errorBg: "rgba(255,93,115,0.16)",
  info: "#6b9bff",
  infoBg: "rgba(107,155,255,0.16)",
} as const;

export function getColors(mode: "light" | "dark") {
  return mode === "dark" ? darkColors : lightColors;
}

/** Type of a resolved color palette (for makeStyles factories). Values are
 * widened to `string` so both the light and dark palettes (and the union
 * `useColors()` returns) satisfy it. */
export type AppColors = Record<keyof typeof darkColors, string>;

/**
 * Backward-compatible alias — screens that haven't migrated to `useColors()`
 * import this directly. Points at the LIGHT palette so the app's default
 * appearance is light (matching the theme-store default mode). Screens migrated
 * to `useColors()` follow the live mode and support the dark toggle.
 */
export const colors = lightColors;

/** Gradient stop pairs (mode-independent) for LinearGradient. */
export const gradients = {
  primary: ["#ff6b8a", "#ff9a6b"] as const,
  purple: ["#9b6bff", "#ff6b8a"] as const,
  heroGlow: ["rgba(155,107,255,0.32)", "rgba(255,107,138,0.0)"] as const,
};

/** Soft primary glow shadow (iOS). On Android, pair with `elevation`. */
export const glow = (color = "#ff6b8a", radius = 16, opacity = 0.45) => ({
  shadowColor: color,
  shadowOpacity: opacity,
  shadowRadius: radius,
  shadowOffset: { width: 0, height: 6 },
  elevation: 8,
});

export const spacing = {
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 20,
  xxl: 24,
  xxxl: 32,
} as const;

export const radius = {
  sm: 8,
  md: 12,
  lg: 16,
  xl: 22,
} as const;

export const fontSize = {
  xs: 10,
  sm: 12,
  base: 14,
  lg: 16,
  xl: 18,
  xxl: 22,
  xxxl: 28,
} as const;
