import { create } from "zustand";
import { getCurrentUser } from "@/generated/api/bodysense";
import { apiUrl } from "@/lib/api-url";
import { getSupabaseClient } from "@/features/auth/services/supabase";
import type { Session } from "@supabase/supabase-js";

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
  login: (
    email: string,
    password: string,
    captchaToken?: string,
  ) => Promise<void>;
  register: (
    email: string,
    password: string,
    captchaToken?: string,
  ) => Promise<void>;
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
let subscribed = false;

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

// The Go /me endpoint validates signature/issuer/audience before we trust
// the identity. Never treat a persisted Supabase getSession() as authorization.
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

function setSessionState(
  set: (partial: Partial<AuthState>) => void,
  session: Session,
) {
  set({
    accessToken: session.access_token,
    isAuthenticated: true,
    isAuthResolved: true,
    error: null,
  });
}

async function observeSession(set: (partial: Partial<AuthState>) => void) {
  if (subscribed) return;
  const client = await getSupabaseClient();
  client.auth.onAuthStateChange((event, session) => {
    // Avoid doing asynchronous calls back into the Supabase auth client from
    // this callback (its internal session lock may still be held).
    if (event === "SIGNED_OUT" || !session) {
      clearAuthState(set);
    } else {
      setSessionState(set, session);
    }
  });
  subscribed = true;
}

async function doRefresh(
  set: (partial: Partial<AuthState>) => void,
): Promise<boolean> {
  try {
    const { data, error } = await (
      await getSupabaseClient()
    ).auth.refreshSession();
    if (error || !data.session) {
      clearAuthState(set);
      return false;
    }
    setSessionState(set, data.session);
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
    if (bootstrapPromise) return bootstrapPromise;
    bootstrapPromise = (async () => {
      set({ isVerifyingSession: true, isAuthResolved: false, error: null });
      try {
        await observeSession(set);
        const { data, error } = await (
          await getSupabaseClient()
        ).auth.getSession();
        if (error || !data.session) {
          clearAuthState(set);
        } else {
          setSessionState(set, data.session);
          // Let the existing route bootstrap continue immediately; Go still
          // gates every health-data request on signed Supabase claims.
          set({ hasHydrated: true, isVerifyingSession: false });
          await get().fetchUser();
        }
      } catch {
        clearAuthState(set);
      } finally {
        set({
          hasHydrated: true,
          isAuthResolved: true,
          isVerifyingSession: false,
        });
      }
    })();
    try {
      await bootstrapPromise;
    } finally {
      bootstrapPromise = null;
    }
  },

  login: async (email, password, captchaToken) => {
    set({ isLoading: true, error: null });
    try {
      const { data, error } = await (
        await getSupabaseClient()
      ).auth.signInWithPassword({
        email,
        password,
        options: captchaToken ? { captchaToken } : undefined,
      });
      if (error || !data.session) {
        throw error ?? new Error("登录未获得有效会话");
      }
      setSessionState(set, data.session);
      set({
        hasHydrated: true,
        isAuthResolved: true,
        isVerifyingSession: false,
      });
      await get().fetchUser();
    } catch (error) {
      set({ error: "登录失败，请检查邮箱、密码或人机验证状态" });
      throw error;
    } finally {
      set({ isLoading: false });
    }
  },

  register: async (email, password, captchaToken) => {
    set({ isLoading: true, error: null });
    try {
      const { data, error } = await (
        await getSupabaseClient()
      ).auth.signUp({
        email,
        password,
        options: captchaToken ? { captchaToken } : undefined,
      });
      if (error) throw error;
      if (!data.session) {
        throw new Error("注册未获得会话：请确认 Supabase 已关闭 Confirm email");
      }
      setSessionState(set, data.session);
      set({
        hasHydrated: true,
        isAuthResolved: true,
        isVerifyingSession: false,
      });
      await get().fetchUser();
    } catch (error) {
      set({ error: "注册失败，请检查邮箱、密码或人机验证状态" });
      throw error;
    } finally {
      set({ isLoading: false });
    }
  },

  logout: async () => {
    try {
      const { error } = await (await getSupabaseClient()).auth.signOut();
      if (error) {
        // Clear browser credentials even if remote global sign-out failed.
        await (await getSupabaseClient()).auth.signOut({ scope: "local" });
      }
    } catch {
      // The access token may remain valid until expiry; Go enforces expiry and
      // account-erasure tombstones rather than making an instant-revoke claim.
    } finally {
      clearAuthState(set);
      set({ hasHydrated: true, error: null });
    }
  },

  forgetSession: () => {
    // Called after privacy erasure: local browser credentials must be cleared.
    try {
      void getSupabaseClient()
        .then((client) => client.auth.signOut({ scope: "local" }))
        .catch(() => {});
    } catch {
      // The local app state must still be cleared if the SDK is unavailable.
    }
    clearAuthState(set);
    set({ hasHydrated: true, error: null });
  },

  verifySession: async () => {
    if (verifySessionPromise) return verifySessionPromise;
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
        set({ user, isAuthenticated: true, isAuthResolved: true });
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
    if (refreshPromise) return refreshPromise;
    refreshPromise = doRefresh(set);
    try {
      return await refreshPromise;
    } finally {
      refreshPromise = null;
    }
  },

  fetchUser: async () => {
    const { accessToken } = get();
    if (!accessToken) return;
    const user = await requestCurrentUser(accessToken);
    set({ user, isAuthResolved: true });
  },

  clearError: () => set({ error: null }),
}));
