import { create } from "zustand";
import * as SecureStore from "expo-secure-store";
import i18n, { resolveLanguage, type LanguagePreference } from "../lib/i18n";

interface LocaleState {
  /** 用户偏好:跟随系统 / 强制中文 / 强制英文。 */
  preference: LanguagePreference;
  setPreference: (pref: LanguagePreference) => Promise<void>;
  loadFromStorage: () => Promise<void>;
}

const STORAGE_KEY = "locale_preference";

function isPreference(v: string | null): v is LanguagePreference {
  return v === "system" || v === "zh-CN" || v === "en";
}

export const useLocaleStore = create<LocaleState>((set) => ({
  preference: "zh-CN", // 默认中文;「跟随系统」是显式选项

  setPreference: async (pref) => {
    await SecureStore.setItemAsync(STORAGE_KEY, pref);
    await i18n.changeLanguage(resolveLanguage(pref));
    set({ preference: pref });
  },

  loadFromStorage: async () => {
    const saved = await SecureStore.getItemAsync(STORAGE_KEY);
    if (isPreference(saved)) {
      await i18n.changeLanguage(resolveLanguage(saved));
      set({ preference: saved });
    }
  },
}));
