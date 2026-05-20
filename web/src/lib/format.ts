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

const BPS_UNITS = ["bps", "Kbps", "Mbps", "Gbps"];

/**
 * Format bits-per-second into a human-readable string.
 * @example formatBitrate(1500000) // "1.5 Mbps"
 */
export function formatBitrate(bps: number): string {
  if (bps === 0) return "0 bps";
  const exp = Math.min(Math.floor(Math.log(Math.abs(bps)) / Math.log(1000)), BPS_UNITS.length - 1);
  const value = bps / Math.pow(1000, exp);
  return `${formatNumber(value, { maximumFractionDigits: 1 })} ${BPS_UNITS[exp]}`;
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
