import { create } from "zustand";
import { persist } from "zustand/middleware";
import { apiUrl, safeJson } from "@/lib/api-url";

interface User {
  id: string;
  email: string;
}

interface AuthState {
  user: User | null;
  accessToken: string | null;
  refreshToken: string | null;
  isAuthenticated: boolean;
  hasHydrated: boolean;
  isAuthResolved: boolean;
  isVerifyingSession: boolean;
  isLoading: boolean;
  error: string | null;

  // Actions
  setHydrated: () => void;
  setTokens: (accessToken: string, refreshToken: string) => void;
  setUser: (user: User) => void;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => void;
  verifySession: () => Promise<boolean>;
  refreshAccessToken: () => Promise<boolean>;
  fetchUser: () => Promise<void>;
  clearError: () => void;
}

// Lock to prevent concurrent refresh token requests.
// Without this, two concurrent 401s would both call /refresh,
// and the second one would fail (token already consumed), logging the user out.
let refreshPromise: Promise<boolean> | null = null;
let verifySessionPromise: Promise<boolean> | null = null;

function clearAuthState(set: (partial: Partial<AuthState>) => void) {
  set({
    user: null,
    accessToken: null,
    refreshToken: null,
    isAuthenticated: false,
    isAuthResolved: true,
    isVerifyingSession: false,
  });
}

// Extracted refresh logic — called only through the dedup lock above.
async function doRefresh(
  get: () => AuthState,
  set: (partial: Partial<AuthState>) => void,
): Promise<boolean> {
  const { refreshToken } = get();

  if (!refreshToken) {
    return false;
  }

  try {
    const response = await fetch(apiUrl("/api/v1/auth/refresh"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });

    if (!response.ok) {
      // Refresh token invalid — clean up auth state
      clearAuthState(set);
      return false;
    }

    const data = await safeJson<{
      access_token: string;
      refresh_token: string;
    }>(response);

    set({
      accessToken: data.access_token,
      refreshToken: data.refresh_token,
    });

    return true;
  } catch {
    return false;
  }
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      accessToken: null,
      refreshToken: null,
      isAuthenticated: false,
      hasHydrated: false,
      isAuthResolved: false,
      isVerifyingSession: false,
      isLoading: false,
      error: null,

      setHydrated: () => {
        set((state) => ({
          hasHydrated: true,
          isAuthResolved: state.isAuthenticated ? state.isAuthResolved : true,
        }));
      },

      setTokens: (accessToken: string, refreshToken: string) => {
        set({
          accessToken,
          refreshToken,
          isAuthenticated: true,
          isAuthResolved: true,
        });
      },

      setUser: (user: User) => {
        set({ user });
      },

      login: async (email: string, password: string) => {
        set({ isLoading: true, error: null });

        try {
          const response = await fetch(apiUrl("/api/v1/auth/login"), {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ email, password }),
          });

          const data = await safeJson<{
            message?: string;
            access_token?: string;
            refresh_token?: string;
          }>(response);

          if (!response.ok) {
            throw new Error(
              (typeof data === "object" && data?.message) || "登录失败",
            );
          }

          set({
            accessToken: data.access_token,
            refreshToken: data.refresh_token,
            isAuthenticated: true,
            isAuthResolved: true,
            isVerifyingSession: false,
            isLoading: false,
            error: null,
          });

          // Fetch user info
          await get().fetchUser();
        } catch (error) {
          set({
            isLoading: false,
            error: error instanceof Error ? error.message : "登录失败",
          });
          throw error;
        }
      },

      register: async (email: string, password: string) => {
        set({ isLoading: true, error: null });

        try {
          const response = await fetch(apiUrl("/api/v1/auth/register"), {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ email, password }),
          });

          const data = await safeJson<{
            message?: string;
            access_token?: string;
            refresh_token?: string;
          }>(response);

          if (!response.ok) {
            throw new Error(
              (typeof data === "object" && data?.message) || "注册失败",
            );
          }

          set({
            accessToken: data.access_token,
            refreshToken: data.refresh_token,
            isAuthenticated: true,
            isAuthResolved: true,
            isVerifyingSession: false,
            isLoading: false,
            error: null,
          });

          // Fetch user info
          await get().fetchUser();
        } catch (error) {
          set({
            isLoading: false,
            error: error instanceof Error ? error.message : "注册失败",
          });
          throw error;
        }
      },

      logout: () => {
        const { refreshToken } = get();

        // Call logout endpoint to invalidate refresh token
        if (refreshToken) {
          fetch(apiUrl("/api/v1/auth/logout"), {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ refresh_token: refreshToken }),
          }).catch(console.error);
        }

        clearAuthState(set);
        set({ error: null });
      },

      verifySession: async () => {
        const { accessToken, isAuthenticated, refreshAccessToken } = get();

        if (!isAuthenticated || !accessToken) {
          clearAuthState(set);
          return false;
        }

        if (verifySessionPromise) {
          return verifySessionPromise;
        }

        verifySessionPromise = (async () => {
          set({ isVerifyingSession: true, error: null });

          const requestMe = async (token: string) =>
            fetch(apiUrl("/api/v1/me"), {
              headers: {
                Authorization: `Bearer ${token}`,
              },
            });

          try {
            let response = await requestMe(accessToken);

            if (response.status === 401) {
              const refreshed = await refreshAccessToken();
              const nextToken = get().accessToken;

              if (!refreshed || !nextToken) {
                clearAuthState(set);
                return false;
              }

              response = await requestMe(nextToken);
            }

            if (!response.ok) {
              clearAuthState(set);
              return false;
            }

            const user = await safeJson<User>(response);
            set({
              user,
              isAuthenticated: true,
              isAuthResolved: true,
              isVerifyingSession: false,
            });
            return true;
          } catch {
            clearAuthState(set);
            return false;
          }
        })();

        try {
          return await verifySessionPromise;
        } finally {
          verifySessionPromise = null;
        }
      },

      refreshAccessToken: async () => {
        // Deduplicate concurrent refresh calls — only one in-flight at a time.
        // Without this, two concurrent 401s would both call /refresh,
        // and the second one would fail (token already consumed), logging the user out.
        if (refreshPromise) return refreshPromise;

        refreshPromise = doRefresh(get, set);
        try {
          return await refreshPromise;
        } finally {
          refreshPromise = null;
        }
      },

      fetchUser: async () => {
        const { accessToken } = get();

        if (!accessToken) return;

        try {
          const response = await fetch(apiUrl("/api/v1/me"), {
            headers: {
              Authorization: `Bearer ${accessToken}`,
            },
          });

          if (response.ok) {
            const user = await safeJson<User>(response);
            set({ user, isAuthResolved: true });
          }
        } catch (error) {
          console.error("Failed to fetch user:", error);
        }
      },

      clearError: () => set({ error: null }),
    }),
    {
      name: "auth-storage",
      partialize: (state) => ({
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
        isAuthenticated: state.isAuthenticated,
        user: state.user,
      }),
      onRehydrateStorage: () => (state) => {
        state?.setHydrated();
      },
    },
  ),
);
