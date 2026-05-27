import { create } from "zustand";
import * as SecureStore from "expo-secure-store";
import { STORAGE_KEYS, DEFAULT_SERVER_URL } from "../lib/constants";
import type { UserPublicProfile } from "../types/api";

interface AuthState {
  token: string | null;
  user: UserPublicProfile | null;
  serverUrl: string;
  isReady: boolean;
  setAuth: (token: string, user: UserPublicProfile) => void;
  setServerUrl: (url: string) => void;
  clearSession: () => void;
  loadFromStorage: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: null,
  user: null,
  serverUrl: DEFAULT_SERVER_URL,
  isReady: false,

  setAuth: async (token, user) => {
    await SecureStore.setItemAsync(STORAGE_KEYS.TOKEN, token);
    await SecureStore.setItemAsync(STORAGE_KEYS.USER, JSON.stringify(user));
    set({ token, user });
  },

  setServerUrl: async (url) => {
    const cleaned = url.replace(/\/+$/, "");
    await SecureStore.setItemAsync(STORAGE_KEYS.SERVER_URL, cleaned);
    set({ serverUrl: cleaned });
  },

  clearSession: async () => {
    await SecureStore.deleteItemAsync(STORAGE_KEYS.TOKEN);
    await SecureStore.deleteItemAsync(STORAGE_KEYS.USER);
    set({ token: null, user: null });
  },

  loadFromStorage: async () => {
    const [token, userJson, serverUrl] = await Promise.all([
      SecureStore.getItemAsync(STORAGE_KEYS.TOKEN),
      SecureStore.getItemAsync(STORAGE_KEYS.USER),
      SecureStore.getItemAsync(STORAGE_KEYS.SERVER_URL),
    ]);
    const user = userJson ? JSON.parse(userJson) : null;
    set({
      token,
      user,
      serverUrl: serverUrl || DEFAULT_SERVER_URL,
      isReady: true,
    });
  },
}));
