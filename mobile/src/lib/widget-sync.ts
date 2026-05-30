import type { TrafficSummary } from "../types/api";
import type { TrafficWidgetProps } from "../widgets/traffic-widget";

// Pushes traffic data into the home-screen widget (method 甲: app feeds the
// widget). The widget module imports `expo-widgets`, which is only linked in a
// custom dev/prod build — so we lazy-require it and no-op when absent (Expo Go
// / before prebuild), mirroring the SSH page's optional-native pattern.

// Data is considered "possibly stale" this many minutes after a push — a future
// timeline entry flips the widget into a stale state at that point, so it ages
// gracefully on its own (no background task) until the app pushes fresh data.
const STALE_AFTER_MIN = 60;

let trafficWidget: {
  updateSnapshot?: (p: TrafficWidgetProps) => void;
  updateTimeline?: (entries: { date: Date; props: TrafficWidgetProps }[]) => void;
  reload?: () => void;
} | null = null;
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

/** Map a traffic summary onto widget props. Carries up to 6 agents (enough for
 * the large family) with raw bytes, plus total count/bytes so the widget can
 * size-slice the list and format a "其余 N 个" remainder per family. */
export function summaryToWidgetProps(s: TrafficSummary, now: Date): TrafficWidgetProps {
  const top = [...s.agents]
    .sort((a, b) => b.total_used - a.total_used)
    .slice(0, 6)
    .map((a) => {
      const usedStr = formatBytes(a.total_used);
      // 方案1: per-agent "used / limit · pct%" when a monthly quota is set
      // (manual or BandwagonHost); otherwise just the used figure.
      const display =
        a.limit && a.limit > 0
          ? `${usedStr} / ${formatBytes(a.limit)} ${Math.round(
              Math.min(100, (a.total_used / a.limit) * 100),
            )}%`
          : usedStr;
      return { name: a.agent_name || a.agent_id, used: display, usedBytes: a.total_used };
    });
  return {
    used: formatBytes(s.total_used),
    limit: s.total_limit && s.total_limit > 0 ? formatBytes(s.total_limit) : "",
    percent: Math.max(0, Math.min(100, s.usage_percent || 0)),
    count: s.agents.length,
    totalUsedBytes: s.total_used,
    top,
    updatedAt: hhmm(now),
  };
}

/** Push fresh traffic into the widget. Builds a 2-entry timeline — fresh now,
 * "possibly stale" after STALE_AFTER_MIN — so the widget ages on its own until
 * the next push. No-op when the widget runtime is unavailable. */
export function pushTrafficToWidget(s: TrafficSummary): void {
  if (!isTrafficWidgetAvailable()) return;
  try {
    const now = new Date();
    const fresh = summaryToWidgetProps(s, now);
    if (typeof trafficWidget!.updateTimeline === "function") {
      const staleAt = new Date(now.getTime() + STALE_AFTER_MIN * 60_000);
      trafficWidget!.updateTimeline([
        { date: now, props: { ...fresh, stale: false } },
        { date: staleAt, props: { ...fresh, stale: true } },
      ]);
    } else {
      trafficWidget!.updateSnapshot!(fresh);
    }
  } catch {
    // best-effort; widget refresh should never break the app
  }
}
