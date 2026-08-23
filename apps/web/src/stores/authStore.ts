import { create } from "zustand";
import { apiUrl, safeJson } from "@/lib/api-url";

interface User {
  id: string;
  email: string;
}

interface AuthPayload {
  access_token: string;
  expires_in: number;
}

interface AuthState {
  user: User | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  hasHydrated: boolean;
  isAuthResolved: boolean;
  isVerifyingSession: boolean;
  isLoading: boolean;
  error: string | null;

  setUser: (user: User) => void;
  bootstrapSession: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  verifySession: () => Promise<boolean>;
  refreshAccessToken: () => Promise<boolean>;
  fetchUser: () => Promise<void>;
  clearError: () => void;
}

let refreshPromise: Promise<boolean> | null = null;
let verifySessionPromise: Promise<boolean> | null = null;
let bootstrapPromise: Promise<void> | null = null;

function clearAuthState(
  set: (partial: Partial<AuthState>) => void,
  options: { resolved?: boolean } = {},
) {
  set({
    user: null,
    accessToken: null,
    isAuthenticated: false,
    isAuthResolved: options.resolved ?? true,
    isVerifyingSession: false,
  });
}

async function doRefresh(
  set: (partial: Partial<AuthState>) => void,
): Promise<boolean> {
  try {
    const response = await fetch(apiUrl("/api/v1/auth/refresh"), {
      method: "POST",
      credentials: "include",
    });

    if (!response.ok) {
      if (response.status === 401) {
        clearAuthState(set);
      }
      return false;
    }

    const data = await safeJson<AuthPayload>(response);
    set({
      accessToken: data.access_token,
      isAuthenticated: true,
      isAuthResolved: true,
      error: null,
    });
    return true;
  } catch {
    return false;
  }
}

export const useAuthStore = create<AuthState>()((set, get) => ({
  user: null,
  accessToken: null,
  isAuthenticated: false,
  hasHydrated: false,
  isAuthResolved: false,
  isVerifyingSession: false,
  isLoading: false,
  error: null,

  setUser: (user: User) => set({ user }),

  bootstrapSession: async () => {
    if (bootstrapPromise) {
      return bootstrapPromise;
    }

    bootstrapPromise = (async () => {
      set({ isVerifyingSession: true, isAuthResolved: false, error: null });
      const refreshed = await get().refreshAccessToken();
      if (refreshed) {
        await get().fetchUser();
      } else {
        clearAuthState(set);
      }
      set({
        hasHydrated: true,
        isAuthResolved: true,
        isVerifyingSession: false,
      });
    })();

    try {
      await bootstrapPromise;
    } finally {
      bootstrapPromise = null;
    }
  },

  login: async (email: string, password: string) => {
    set({ isLoading: true, error: null });

    try {
      const response = await fetch(apiUrl("/api/v1/auth/login"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ email, password }),
      });
      const data = await safeJson<AuthPayload & { message?: string }>(response);
      if (!response.ok) {
        throw new Error(data?.message || "登录失败");
      }

      set({
        accessToken: data.access_token,
        isAuthenticated: true,
        hasHydrated: true,
        isAuthResolved: true,
        isVerifyingSession: false,
        isLoading: false,
        error: null,
      });
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
        credentials: "include",
        body: JSON.stringify({ email, password }),
      });
      const data = await safeJson<AuthPayload & { message?: string }>(response);
      if (!response.ok) {
        throw new Error(data?.message || "注册失败");
      }

      set({
        accessToken: data.access_token,
        isAuthenticated: true,
        hasHydrated: true,
        isAuthResolved: true,
        isVerifyingSession: false,
        isLoading: false,
        error: null,
      });
      await get().fetchUser();
    } catch (error) {
      set({
        isLoading: false,
        error: error instanceof Error ? error.message : "注册失败",
      });
      throw error;
    }
  },

  logout: async () => {
    try {
      await fetch(apiUrl("/api/v1/auth/logout"), {
        method: "POST",
        credentials: "include",
      });
    } finally {
      clearAuthState(set);
      set({ hasHydrated: true, error: null });
    }
  },

  verifySession: async () => {
    if (verifySessionPromise) {
      return verifySessionPromise;
    }

    verifySessionPromise = (async () => {
      set({ isVerifyingSession: true, error: null });

      let token = get().accessToken;
      if (!token) {
        const refreshed = await get().refreshAccessToken();
        token = get().accessToken;
        if (!refreshed || !token) {
          clearAuthState(set);
          return false;
        }
      }

      const requestMe = (accessToken: string) =>
        fetch(apiUrl("/api/v1/me"), {
          headers: { Authorization: `Bearer ${accessToken}` },
        });

      try {
        let response = await requestMe(token);
        if (response.status === 401) {
          const refreshed = await get().refreshAccessToken();
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
      set({ isVerifyingSession: false });
    }
  },

  refreshAccessToken: async () => {
    if (refreshPromise) {
      return refreshPromise;
    }

    refreshPromise = doRefresh(set);
    try {
      return await refreshPromise;
    } finally {
      refreshPromise = null;
    }
  },

  fetchUser: async () => {
    const { accessToken } = get();
    if (!accessToken) {
      return;
    }

    const response = await fetch(apiUrl("/api/v1/me"), {
      headers: { Authorization: `Bearer ${accessToken}` },
    });
    if (!response.ok) {
      throw new Error("failed to fetch current user");
    }
    const user = await safeJson<User>(response);
    set({ user, isAuthResolved: true });
  },

  clearError: () => set({ error: null }),
}));
