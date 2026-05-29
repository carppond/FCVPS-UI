import type { TrafficSummary } from "../types/api";
import type { TrafficWidgetProps } from "../widgets/traffic-widget";

// Pushes traffic data into the home-screen widget (method 甲: app feeds the
// widget). The widget module imports `expo-widgets`, which is only linked in a
// custom dev/prod build — so we lazy-require it and no-op when absent (Expo Go
// / before prebuild), mirroring the SSH page's optional-native pattern.

let trafficWidget: { updateSnapshot?: (p: TrafficWidgetProps) => void; reload?: () => void } | null = null;
try {
  // Importing the widget module triggers createWidget(); guard so Expo Go
  // (no expo-widgets native module) doesn't crash on load.
  trafficWidget = require("../widgets/traffic-widget").trafficWidget ?? null;
} catch {
  trafficWidget = null;
}

/** Whether the widget runtime is available (custom build on a supported OS). */
export function isTrafficWidgetAvailable(): boolean {
  return trafficWidget != null && typeof trafficWidget.updateSnapshot === "function";
}

/** Human-readable bytes (binary units), e.g. 1318554000000 → "1.2 TB". */
export function formatBytes(bytes: number): string {
  if (!bytes || bytes < 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  let v = bytes;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`;
}

function hhmm(d: Date): string {
  const p = (n: number) => String(n).padStart(2, "0");
  return `${p(d.getHours())}:${p(d.getMinutes())}`;
}

/** Map a traffic summary onto widget props (top 3 agents by usage). */
export function summaryToWidgetProps(s: TrafficSummary, now: Date): TrafficWidgetProps {
  const top = [...s.agents]
    .sort((a, b) => b.total_used - a.total_used)
    .slice(0, 3)
    .map((a) => ({ name: a.agent_name || a.agent_id, used: formatBytes(a.total_used) }));
  return {
    used: formatBytes(s.total_used),
    limit: s.total_limit && s.total_limit > 0 ? formatBytes(s.total_limit) : "",
    percent: Math.max(0, Math.min(100, s.usage_percent || 0)),
    top,
    updatedAt: hhmm(now),
  };
}

/** Push a fresh snapshot into the widget. No-op when the widget is unavailable. */
export function pushTrafficToWidget(s: TrafficSummary): void {
  if (!isTrafficWidgetAvailable()) return;
  try {
    trafficWidget!.updateSnapshot!(summaryToWidgetProps(s, new Date()));
  } catch {
    // best-effort; widget refresh should never break the app
  }
}
