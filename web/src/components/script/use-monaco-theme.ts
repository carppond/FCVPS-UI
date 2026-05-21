import * as React from "react";
import { useUIStore } from "@/stores/ui-store";

/**
 * useMonacoTheme resolves the persisted theme preference into a concrete
 * light / dark value Monaco can apply directly. `system` is read off
 * `<html data-theme>` which the theme module keeps in sync with
 * `prefers-color-scheme`.
 *
 * Extracted so both the script editor and the test panel (which embeds a
 * second Monaco instance) stay in sync without duplicating the
 * MutationObserver wiring.
 */
export function useMonacoTheme(): "light" | "dark" {
  const pref = useUIStore((s) => s.theme);
  const [resolved, setResolved] = React.useState<"light" | "dark">(() =>
    readDocTheme(),
  );
  React.useEffect(() => {
    if (pref !== "system") {
      setResolved(pref);
      return;
    }
    setResolved(readDocTheme());
    const observer = new MutationObserver(() => setResolved(readDocTheme()));
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    return () => observer.disconnect();
  }, [pref]);
  return resolved;
}

function readDocTheme(): "light" | "dark" {
  if (typeof document === "undefined") return "dark";
  return document.documentElement.getAttribute("data-theme") === "light"
    ? "light"
    : "dark";
}
