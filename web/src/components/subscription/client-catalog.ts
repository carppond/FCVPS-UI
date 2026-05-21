/**
 * Catalog of supported subscription clients.
 *
 * Each entry maps a client name to:
 *  - the backend `target` query param value (drives which producer renders
 *    the body),
 *  - the platform classification used by the share UI's filter chips,
 *  - the deeplink template (when the client publishes one).
 *
 * Ordering inside the array drives the UI grid order (most-used first).
 */

export type ClientPlatform = "desktop" | "mobile" | "both";

export type ClientFormat =
  | "clash_yaml"
  | "singbox_json"
  | "v2ray_uri"
  | "surge_conf";

export interface ClientDef {
  /** Stable internal id; also used as i18n key suffix. */
  id: string;
  /** Display name; kept as raw english because the brands are well-known. */
  name: string;
  /** Backend target query parameter. */
  target: string;
  /** Output format badge shown on the card. */
  format: ClientFormat;
  platform: ClientPlatform;
  /**
   * deeplink template with `{url}` (the share URL, will be URL-encoded by the
   * caller) and `{name}` placeholders. Undefined when no standard deeplink is
   * published — the card then only exposes copy + QR.
   */
  deeplinkTemplate?: string;
  /**
   * Tells the deeplink builder whether to base64-encode the URL before
   * substitution (some clients — Shadowrocket, QX — expect this).
   */
  deeplinkEncoding?: "raw" | "base64" | "qx-resource";
}

/**
 * The 11-client catalog. Order = display order (popularity).
 */
export const CLIENT_CATALOG: ClientDef[] = [
  {
    id: "mihomo",
    name: "Mihomo",
    target: "clash",
    format: "clash_yaml",
    platform: "both",
    deeplinkTemplate: "clash://install-config?url={url}&name={name}",
    deeplinkEncoding: "raw",
  },
  {
    id: "clash_verge",
    name: "Clash Verge Rev",
    target: "clash",
    format: "clash_yaml",
    platform: "desktop",
    deeplinkTemplate: "clash://install-config?url={url}&name={name}",
    deeplinkEncoding: "raw",
  },
  {
    id: "clash_meta_android",
    name: "Clash Meta for Android",
    target: "clash",
    format: "clash_yaml",
    platform: "mobile",
    deeplinkTemplate: "clashmeta://install-config?url={url}",
    deeplinkEncoding: "raw",
  },
  {
    id: "stash",
    name: "Stash",
    target: "clash",
    format: "clash_yaml",
    platform: "mobile",
    deeplinkTemplate: "stash://install-config?url={url}",
    deeplinkEncoding: "raw",
  },
  {
    id: "singbox",
    name: "sing-box",
    target: "singbox",
    format: "singbox_json",
    platform: "both",
  },
  {
    id: "shadowrocket",
    name: "Shadowrocket",
    target: "v2ray",
    format: "v2ray_uri",
    platform: "mobile",
    deeplinkTemplate: "shadowrocket://add/sub://{url}",
    deeplinkEncoding: "base64",
  },
  {
    id: "surge_mac",
    name: "Surge for Mac",
    target: "surge",
    format: "surge_conf",
    platform: "desktop",
    deeplinkTemplate: "surge:///install-config?url={url}",
    deeplinkEncoding: "raw",
  },
  {
    id: "surge_ios",
    name: "Surge for iOS",
    target: "surge",
    format: "surge_conf",
    platform: "mobile",
    deeplinkTemplate: "surge:///install-config?url={url}",
    deeplinkEncoding: "raw",
  },
  {
    id: "qx",
    name: "Quantumult X",
    target: "v2ray",
    format: "v2ray_uri",
    platform: "mobile",
    deeplinkTemplate:
      "quantumult-x:///add-resource?remote-resource={url}",
    deeplinkEncoding: "qx-resource",
  },
  {
    id: "loon",
    name: "Loon",
    target: "v2ray",
    format: "v2ray_uri",
    platform: "mobile",
    deeplinkTemplate: "loon://import?nodelist={url}",
    deeplinkEncoding: "raw",
  },
  {
    id: "v2ray",
    name: "V2RayN / v2rayNG",
    target: "v2ray",
    format: "v2ray_uri",
    platform: "both",
  },
];

/**
 * Build the share URL for a given client by appending the target query param.
 * baseUrl already contains `?token=...`.
 */
export function buildClientShareUrl(baseUrl: string, target: string): string {
  if (!baseUrl) return "";
  const sep = baseUrl.includes("?") ? "&" : "?";
  return `${baseUrl}${sep}target=${encodeURIComponent(target)}`;
}

/**
 * Resolve the deeplink for a client, substituting the share URL and name.
 * Returns null when the client has no deeplink.
 */
export function resolveDeeplink(
  client: ClientDef,
  shareUrl: string,
  subscriptionName: string,
): string | null {
  if (!client.deeplinkTemplate || !shareUrl) return null;
  let urlValue = shareUrl;
  if (client.deeplinkEncoding === "base64") {
    urlValue = base64Url(shareUrl);
  } else if (client.deeplinkEncoding === "qx-resource") {
    // QX expects a base64-encoded JSON envelope: {"server_remote":["https://..."]}.
    const envelope = JSON.stringify({ server_remote: [shareUrl] });
    urlValue = base64Url(envelope);
  } else {
    urlValue = encodeURIComponent(shareUrl);
  }
  return client.deeplinkTemplate
    .replace("{url}", urlValue)
    .replace("{name}", encodeURIComponent(subscriptionName));
}

/**
 * btoa-based base64 encoder that tolerates Unicode characters. Browser-only;
 * the call sites are all client-side React components so SSR is not a concern.
 */
function base64Url(input: string): string {
  if (typeof window === "undefined" || typeof window.btoa !== "function") {
    return input;
  }
  // Encode UTF-8 bytes first to handle non-ASCII safely.
  const bytes = new TextEncoder().encode(input);
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return window.btoa(bin);
}
