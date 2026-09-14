import { create } from "zustand";
import {
  getCurrentUser,
  loginAccount,
  logoutAccount,
  refreshAccountSession,
  registerAccount,
} from "@/generated/api/bodysense";
import { apiUrl } from "@/lib/api-url";
import { openApiPublicFetch } from "@/lib/openapi-client";

interface User {
  id: string;
  email: string;
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
  forgetSession: () => void;
  verifySession: () => Promise<boolean>;
  refreshAccessToken: () => Promise<boolean>;
  fetchUser: () => Promise<void>;
  clearError: () => void;
}

let refreshPromise: Promise<boolean> | null = null;
let verifySessionPromise: Promise<boolean> | null = null;
let bootstrapPromise: Promise<void> | null = null;

function generatedHttpStatus(error: unknown): number | undefined {
  if (typeof error !== "object" || error === null || !("status" in error))
    return undefined;
  return typeof error.status === "number" ? error.status : undefined;
}

function bearerFetcher(accessToken: string): typeof globalThis.fetch {
  return async (input, init) => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.toString()
          : input.url;
    const target = /^https?:\/\//i.test(raw) ? raw : apiUrl(raw);
    const inheritedHeaders = Object.fromEntries(
      new Headers(init?.headers).entries(),
    );
    return fetch(target, {
      ...init,
      headers: { ...inheritedHeaders, Authorization: `Bearer ${accessToken}` },
    });
  };
}

async function requestCurrentUser(accessToken: string): Promise<User> {
  const user = await getCurrentUser(undefined, bearerFetcher(accessToken));
  return { id: user.id, email: user.email };
}

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
    const session = await refreshAccountSession(undefined, openApiPublicFetch);
    set({
      accessToken: session.access_token,
      isAuthenticated: true,
      isAuthResolved: true,
      error: null,
    });
    return true;
  } catch (error) {
    if (generatedHttpStatus(error) === 401) {
      clearAuthState(set);
    }
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
        // Refresh is the authentication boundary. Unblock protected routes as
        // soon as it succeeds, then hydrate display-only user data in parallel
        // with route/profile loading instead of paying another serial RTT.
        set({
          hasHydrated: true,
          isAuthResolved: true,
          isVerifyingSession: false,
        });
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
      const session = await loginAccount(
        { email, password },
        undefined,
        openApiPublicFetch,
      );

      set({
        accessToken: session.access_token,
        isAuthenticated: true,
        hasHydrated: true,
        isAuthResolved: true,
        isVerifyingSession: false,
        isLoading: false,
        error: null,
      });
      await get().fetchUser();
    } catch {
      set({ isLoading: false, error: "登录失败" });
      throw new Error("登录失败");
    }
  },

  register: async (email: string, password: string) => {
    set({ isLoading: true, error: null });

    try {
      const session = await registerAccount(
        { email, password },
        undefined,
        openApiPublicFetch,
      );

      set({
        accessToken: session.access_token,
        isAuthenticated: true,
        hasHydrated: true,
        isAuthResolved: true,
        isVerifyingSession: false,
        isLoading: false,
        error: null,
      });
      await get().fetchUser();
    } catch {
      set({ isLoading: false, error: "注册失败" });
      throw new Error("注册失败");
    }
  },

  logout: async () => {
    try {
      await logoutAccount(undefined, openApiPublicFetch);
    } catch {
      // Server-side revocation may be temporarily unavailable; the local
      // session clears regardless so the browser never stays signed in.
    } finally {
      clearAuthState(set);
      set({ hasHydrated: true, error: null });
    }
  },

  forgetSession: () => {
    clearAuthState(set);
    set({ hasHydrated: true, error: null });
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

      try {
        let user: User;
        try {
          user = await requestCurrentUser(token);
        } catch (error) {
          if (generatedHttpStatus(error) !== 401) throw error;
          const refreshed = await get().refreshAccessToken();
          const nextToken = get().accessToken;
          if (!refreshed || !nextToken) {
            clearAuthState(set);
            return false;
          }
          user = await requestCurrentUser(nextToken);
        }

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

    const user = await requestCurrentUser(accessToken);
    set({ user, isAuthResolved: true });
  },

  clearError: () => set({ error: null }),
}));
