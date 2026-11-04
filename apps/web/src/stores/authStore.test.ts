import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "./authStore";

const client = vi.hoisted(() => ({
  getSession: vi.fn(),
  refreshSession: vi.fn(),
  signInWithPassword: vi.fn(),
  signUp: vi.fn(),
  signOut: vi.fn(),
  onAuthStateChange: vi.fn(() => ({
    data: { subscription: { unsubscribe: vi.fn() } },
  })),
}));

vi.mock("@/features/auth/services/supabase", () => ({
  getSupabaseClient: () => ({ auth: client }),
}));

const USER_ID = "11111111-1111-4111-8111-111111111111";

function response(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function mockSession(accessToken = "supabase-signed-access") {
  return {
    access_token: accessToken,
    refresh_token: "supabase-managed-refresh",
    token_type: "bearer",
    expires_in: 900,
    expires_at: Math.floor(Date.now() / 1000) + 900,
    user: { id: USER_ID, email: "test@example.com" },
  };
}

function reset() {
  useAuthStore.setState({
    user: null,
    accessToken: null,
    isAuthenticated: false,
    hasHydrated: false,
    isAuthResolved: false,
    isVerifyingSession: false,
    isLoading: false,
    error: null,
  });
}

describe("Supabase Auth browser session adapter", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    for (const mock of Object.values(client)) mock.mockClear();
    reset();
    client.getSession.mockResolvedValue({
      data: { session: null },
      error: null,
    });
    client.signOut.mockResolvedValue({ error: null });
  });

  it("restores a hosted Supabase session and verifies user via Go /me", async () => {
    client.getSession.mockResolvedValue({
      data: { session: mockSession() },
      error: null,
    });
    const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      response({
        id: USER_ID,
        email: "test@example.com",
      }),
    );
    await useAuthStore.getState().bootstrapSession();
    expect(client.getSession).toHaveBeenCalledTimes(1);
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/me",
      expect.objectContaining({
        headers: { Authorization: "Bearer supabase-signed-access" },
      }),
    );
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: "supabase-signed-access",
      isAuthenticated: true,
      user: { id: USER_ID, email: "test@example.com" },
      hasHydrated: true,
    });
  });

  it("with no hosted session fails closed and never calls the retired refresh endpoint", async () => {
    const fetcher = vi.spyOn(globalThis, "fetch");
    await useAuthStore.getState().bootstrapSession();
    expect(useAuthStore.getState()).toMatchObject({
      user: null,
      accessToken: null,
      isAuthenticated: false,
      hasHydrated: true,
    });
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("signs in using Supabase Auth and passes the Turnstile captchaToken", async () => {
    client.signInWithPassword.mockResolvedValue({
      data: { session: mockSession() },
      error: null,
    });
    const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      response({
        id: USER_ID,
        email: "test@example.com",
      }),
    );
    await useAuthStore
      .getState()
      .login("test@example.com", "password123", "turnstile-token");
    expect(client.signInWithPassword).toHaveBeenCalledWith({
      email: "test@example.com",
      password: "password123",
      options: { captchaToken: "turnstile-token" },
    });
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(useAuthStore.getState().isAuthenticated).toBe(true);
  });

  it("signs up using Supabase Auth, and refuses to pretend email confirmation was completed", async () => {
    client.signUp.mockResolvedValue({ data: { session: null }, error: null });
    await expect(
      useAuthStore
        .getState()
        .register("test@example.com", "password123", "captcha-token"),
    ).rejects.toThrow("Confirm email");
    expect(client.signUp).toHaveBeenCalledWith({
      email: "test@example.com",
      password: "password123",
      options: { captchaToken: "captcha-token" },
    });
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    client.signUp.mockResolvedValue({
      data: { session: mockSession() },
      error: null,
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      response({
        id: USER_ID,
        email: "test@example.com",
      }),
    );
    await useAuthStore.getState().register("test@example.com", "password123");
    expect(useAuthStore.getState().user?.id).toBe(USER_ID);
  });

  it("deduplicates parallel refresh requests via the hosted Auth SDK", async () => {
    let resolveRefresh: ((value: unknown) => void) | undefined;
    const pending = new Promise((resolve) => {
      resolveRefresh = resolve;
    });
    client.refreshSession.mockReturnValue(pending);
    const first = useAuthStore.getState().refreshAccessToken();
    const second = useAuthStore.getState().refreshAccessToken();
    resolveRefresh?.({
      data: { session: mockSession("refreshed") },
      error: null,
    });
    await expect(first).resolves.toBe(true);
    await expect(second).resolves.toBe(true);
    expect(client.refreshSession).toHaveBeenCalledTimes(1);
    expect(useAuthStore.getState().accessToken).toBe("refreshed");
  });

  it("fails closed for invalid Go identity responses", async () => {
    useAuthStore.setState({
      accessToken: "access-signed",
      isAuthenticated: true,
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      response({
        id: "not-a-uuid",
        email: "test@example.com",
      }),
    );
    await expect(useAuthStore.getState().verifySession()).resolves.toBe(false);
    expect(useAuthStore.getState().accessToken).toBeNull();
  });

  it("logs out through Supabase and clears the entire app auth state", async () => {
    useAuthStore.setState({
      accessToken: "token",
      user: { id: USER_ID, email: "test@example.com" },
      isAuthenticated: true,
      hasHydrated: true,
    });
    await useAuthStore.getState().logout();
    expect(client.signOut).toHaveBeenCalledWith();
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: null,
      user: null,
      isAuthenticated: false,
      hasHydrated: true,
    });
  });
});
