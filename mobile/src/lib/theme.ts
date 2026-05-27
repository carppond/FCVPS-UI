export const lightColors = {
  bg: "#f5f5f7",
  surface: "#ffffff",
  surfaceHover: "#f0f0f2",
  elevated: "#f8f8fa",
  border: "rgba(0,0,0,0.08)",
  borderStrong: "rgba(0,0,0,0.12)",
  primary: "#ff6363",
  primarySoft: "rgba(255,99,99,0.08)",
  textPrimary: "#1a1a1e",
  textSecondary: "#4a4a55",
  textTertiary: "#8a8a96",
  textDisabled: "#b8b8c2",
  success: "#22c55e",
  successBg: "rgba(34,197,94,0.08)",
  warning: "#f59e0b",
  warningBg: "rgba(245,158,11,0.08)",
  error: "#ef4444",
  errorBg: "rgba(239,68,68,0.08)",
  info: "#3b82f6",
  infoBg: "rgba(59,130,246,0.08)",
} as const;

export const darkColors = {
  bg: "#0a0a0c",
  surface: "#111113",
  surfaceHover: "#161618",
  elevated: "#1c1c1f",
  border: "rgba(255,255,255,0.07)",
  borderStrong: "rgba(255,255,255,0.12)",
  primary: "#ff6363",
  primarySoft: "rgba(255,99,99,0.08)",
  textPrimary: "#f0f0f2",
  textSecondary: "#a0a0ad",
  textTertiary: "#5c5c6a",
  textDisabled: "#3a3a44",
  success: "#22c55e",
  successBg: "rgba(34,197,94,0.08)",
  warning: "#f59e0b",
  warningBg: "rgba(245,158,11,0.08)",
  error: "#ef4444",
  errorBg: "rgba(239,68,68,0.08)",
  info: "#3b82f6",
  infoBg: "rgba(59,130,246,0.08)",
} as const;

export function getColors(mode: "light" | "dark") {
  return mode === "dark" ? darkColors : lightColors;
}

/** Backward-compatible alias — existing code imports `colors` directly. */
export const colors = lightColors;

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
  sm: 6,
  md: 10,
  lg: 14,
  xl: 20,
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
