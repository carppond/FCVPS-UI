import { useThemeStore } from "../stores/theme-store";
import { getColors } from "./theme";

export function useColors() {
  const mode = useThemeStore((s) => s.mode);
  return getColors(mode);
}
