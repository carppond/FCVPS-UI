import { create } from "zustand";
import * as SecureStore from "expo-secure-store";

type ThemeMode = "light" | "dark";

interface ThemeState {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
  toggle: () => void;
  loadFromStorage: () => Promise<void>;
}

export const useThemeStore = create<ThemeState>((set, get) => ({
  mode: "light",

  setMode: async (mode) => {
    await SecureStore.setItemAsync("theme_mode", mode);
    set({ mode });
  },

  toggle: async () => {
    const next = get().mode === "light" ? "dark" : "light";
    await SecureStore.setItemAsync("theme_mode", next);
    set({ mode: next });
  },

  loadFromStorage: async () => {
    const saved = await SecureStore.getItemAsync("theme_mode");
    if (saved === "dark" || saved === "light") set({ mode: saved });
  },
}));
