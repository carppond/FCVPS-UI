import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { User } from "@/types/api";

interface AuthState {
  user: User | null;
  /**
   * Pending token issued after password auth when 2FA is required. Kept in
   * memory only (never persisted) and sent in the verify-totp request body —
   * it is short-lived and never the long-lived session credential.
   */
  twoFactorPending: string | null;
}

interface AuthActions {
  /**
   * Record the signed-in user. The access token is NOT stored here: the server
   * sets an httpOnly `sg_session` cookie the browser sends automatically, so
   * the token never touches JavaScript-reachable storage (defends against
   * malicious extensions / XSS reading it).
   */
  setSession: (user: User) => void;
  /** Clear all auth state (logout). */
  clearSession: () => void;
  /** Store the pending_token while waiting for TOTP verification. */
  setTwoFactorPending: (pendingToken: string) => void;
}

const INITIAL_STATE: AuthState = {
  user: null,
  twoFactorPending: null,
};

export const useAuthStore = create<AuthState & AuthActions>()(
  persist(
    (set) => ({
      ...INITIAL_STATE,

      setSession: (user) => set({ user, twoFactorPending: null }),

      clearSession: () => set(INITIAL_STATE),

      setTwoFactorPending: (pendingToken) =>
        set({ twoFactorPending: pendingToken, user: null }),
    }),
    {
      name: "sgvps_auth",
      // Persist only the user for a no-flash reload; it is NOT an auth
      // credential (the httpOnly cookie is). The route guard re-verifies via
      // /api/me on every load, so a stale persisted user can't grant access.
      partialize: (state) => ({ user: state.user }),
    },
  ),
);
