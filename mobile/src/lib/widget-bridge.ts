// Bridge to the native module that writes widget data into the iOS App Group
// shared container (and triggers a WidgetKit reload). The module ships with the
// custom development build via the apple-targets config plugin; it is ABSENT in
// Expo Go and before `expo prebuild`. So every call degrades gracefully —
// mirroring the SSH page's optional-native-module pattern — and `isAvailable`
// lets the settings UI hide / disable the widget toggle when unsupported.

let native: {
  save?: (serverUrl: string, token: string) => void;
  clear?: () => void;
  reload?: () => void;
} | null = null;

try {
  // requireOptionalNativeModule returns null (no throw) when the module isn't
  // linked, which is exactly Expo Go / pre-prebuild.
  const core = require("expo-modules-core");
  native = core?.requireOptionalNativeModule?.("ShiguangWidget") ?? null;
} catch {
  native = null;
}

/** Whether the native widget module is linked (custom dev/prod build on iOS). */
export function isWidgetSupported(): boolean {
  return native != null && typeof native.save === "function";
}

/**
 * Persist the hub URL + read-only widget token into the App Group so the
 * widget extension can fetch traffic. No-op (returns false) when unsupported.
 */
export function saveWidgetData(serverUrl: string, token: string): boolean {
  if (!isWidgetSupported()) return false;
  try {
    native!.save!(serverUrl, token);
    return true;
  } catch {
    return false;
  }
}

/** Clear the shared widget data (on disable / logout). */
export function clearWidgetData(): boolean {
  if (!native?.clear) return false;
  try {
    native.clear();
    return true;
  } catch {
    return false;
  }
}

/** Force the widget to refresh now (call on app foreground / after login). */
export function reloadWidget(): void {
  try {
    native?.reload?.();
  } catch {
    // best-effort
  }
}
