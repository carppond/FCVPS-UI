import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { User } from "@/types/api";

interface AuthState {
  user: User | null;
  token: string | null;
  /** Pending token issued after password auth when 2FA is required. */
  twoFactorPending: string | null;
}

interface AuthActions {
  /** Set session after successful login (with or without 2FA). */
  setSession: (user: User, token: string) => void;
  /** Clear all auth state (logout). */
  clearSession: () => void;
  /** Store the pending_token while waiting for TOTP verification. */
  setTwoFactorPending: (pendingToken: string) => void;
}

const INITIAL_STATE: AuthState = {
  user: null,
  token: null,
  twoFactorPending: null,
};

export const useAuthStore = create<AuthState & AuthActions>()(
  persist(
    (set) => ({
      ...INITIAL_STATE,

      setSession: (user, token) =>
        set({ user, token, twoFactorPending: null }),

      clearSession: () => set(INITIAL_STATE),

      setTwoFactorPending: (pendingToken) =>
        set({ twoFactorPending: pendingToken, token: null, user: null }),
    }),
    {
      name: "sgvps_auth",
      partialize: (state) => ({
        user: state.user,
        token: state.token,
        // Do not persist twoFactorPending — it should expire with the tab.
      }),
    },
  ),
);
