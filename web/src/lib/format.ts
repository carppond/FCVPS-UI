import i18next from "i18next";

/** Return the current i18next language or fall back to zh-CN. */
function locale(): string {
  return i18next.language || "zh-CN";
}

const BYTE_UNITS = ["B", "KB", "MB", "GB", "TB", "PB"];

/**
 * Format a byte count into a human-readable string.
 * @example formatBytes(1536) // "1.5 KB"
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const exp = Math.min(Math.floor(Math.log(Math.abs(bytes)) / Math.log(1024)), BYTE_UNITS.length - 1);
  const value = bytes / Math.pow(1024, exp);
  return `${formatNumber(value, { maximumFractionDigits: 1 })} ${BYTE_UNITS[exp]}`;
}

const BITRATE_UNITS = ["B/s", "KB/s", "MB/s", "GB/s", "TB/s"];

/**
 * Format bytes-per-second into a human-readable rate string.
 * Uses 1024-based binary units (matching mihomo / nload conventions),
 * which is what users see in most VPS dashboards.
 * @example formatBitrate(0)        // "0 B/s"
 * @example formatBitrate(1024)     // "1.0 KB/s"
 * @example formatBitrate(1572864)  // "1.5 MB/s"
 */
export function formatBitrate(bytesPerSec: number): string {
  if (!Number.isFinite(bytesPerSec) || bytesPerSec <= 0) return "0 B/s";
  const exp = Math.min(
    Math.floor(Math.log(Math.abs(bytesPerSec)) / Math.log(1024)),
    BITRATE_UNITS.length - 1,
  );
  const value = bytesPerSec / Math.pow(1024, exp);
  // < 1024 (B/s): no fractional digits — bytes are integers by nature
  // >= 1024: 1 fractional digit
  const digits = exp === 0 ? 0 : 1;
  return `${formatNumber(value, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })} ${BITRATE_UNITS[exp]}`;
}

/**
 * Format a percentage value (0-100) with a single fractional digit.
 * Clamps out-of-range values to [0, 100].
 * @example formatPercent(12.345) // "12.3%"
 */
export function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return "—";
  const clamped = Math.max(0, Math.min(100, value));
  return `${formatNumber(clamped, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  })}%`;
}

/**
 * Format an uptime duration in seconds into a localized "Xd Yh Zm" string.
 * Picks "30 天 4 小时 15 分" for zh-CN/ja/ko (i18n) and "30d 4h 15m" for en.
 * Caller passes pre-translated unit suffixes so this stays a pure formatter.
 */
export function formatUptime(
  seconds: number,
  units: { day: string; hour: string; minute: string; separator?: string },
): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "—";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const sep = units.separator ?? " ";
  return [`${days}${units.day}`, `${hours}${units.hour}`, `${minutes}${units.minute}`].join(sep);
}

/**
 * Format a number using Intl.NumberFormat, respecting the current locale.
 */
export function formatNumber(n: number, opts?: Intl.NumberFormatOptions): string {
  return new Intl.NumberFormat(locale(), opts).format(n);
}

/**
 * Format a Date (or timestamp) using Intl.DateTimeFormat, respecting the current locale.
 */
export function formatDate(date: Date | number | string, opts?: Intl.DateTimeFormatOptions): string {
  const d = typeof date === "string" || typeof date === "number" ? new Date(date) : date;
  const defaultOpts: Intl.DateTimeFormatOptions = {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    ...opts,
  };
  return new Intl.DateTimeFormat(locale(), defaultOpts).format(d);
}

/**
 * Format a date as a relative time string (e.g., "3 minutes ago").
 */
export function formatRelativeTime(date: Date | number | string): string {
  const d = typeof date === "string" || typeof date === "number" ? new Date(date) : date;
  const diffMs = d.getTime() - Date.now();
  const diffSec = Math.round(diffMs / 1000);

  const rtf = new Intl.RelativeTimeFormat(locale(), { numeric: "auto" });

  const thresholds: [number, Intl.RelativeTimeFormatUnit][] = [
    [60, "second"],
    [3600, "minute"],
    [86400, "hour"],
    [604800, "day"],
    [2592000, "week"],
    [31536000, "month"],
    [Infinity, "year"],
  ];

  let prev = 1;
  for (const [threshold, unit] of thresholds) {
    if (Math.abs(diffSec) < threshold) {
      return rtf.format(Math.round(diffSec / prev), unit);
    }
    prev = threshold;
  }
  return rtf.format(Math.round(diffSec / 31536000), "year");
}
