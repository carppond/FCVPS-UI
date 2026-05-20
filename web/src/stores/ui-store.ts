import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { Theme } from "@/lib/theme";
import { applyTheme } from "@/lib/theme";

interface UIState {
  theme: Theme;
  isSidebarCollapsed: boolean;
}

interface UIActions {
  setTheme: (theme: Theme) => void;
  toggleSidebar: () => void;
}

export const useUIStore = create<UIState & UIActions>()(
  persist(
    (set) => ({
      theme: "dark",
      isSidebarCollapsed: false,

      setTheme: (theme) => {
        applyTheme(theme);
        set({ theme });
      },

      toggleSidebar: () =>
        set((state) => ({ isSidebarCollapsed: !state.isSidebarCollapsed })),
    }),
    {
      name: "sgvps_ui",
      partialize: (state) => ({
        theme: state.theme,
        isSidebarCollapsed: state.isSidebarCollapsed,
      }),
    },
  ),
);
