import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "./authStore";

const USER_ID_1 = "11111111-1111-4111-8111-111111111111";
const USER_ID_2 = "22222222-2222-4222-8222-222222222222";
const USER_ID_3 = "33333333-3333-4333-8333-333333333333";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function resetAuthState() {
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
  localStorage.clear();
}

describe("authStore secure browser session", () => {
  beforeEach(() => {
    resetAuthState();
    vi.restoreAllMocks();
  });

  it("bootstraps from the HttpOnly refresh cookie and keeps tokens out of localStorage", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        jsonResponse({ access_token: "access-1", expires_in: 900 }),
      )
      .mockResolvedValueOnce(
        jsonResponse({ id: USER_ID_1, email: "user@example.com" }),
      );

    await useAuthStore.getState().bootstrapSession();

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/v1/auth/refresh",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/v1/me",
      expect.objectContaining({
        headers: { Authorization: "Bearer access-1" },
      }),
    );
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: "access-1",
      isAuthenticated: true,
      hasHydrated: true,
      isAuthResolved: true,
      user: { id: USER_ID_1, email: "user@example.com" },
    });
    expect(localStorage.getItem("auth-storage")).toBeNull();
  });

  it("unblocks protected routes after refresh while user hydration is still pending", async () => {
    let resolveUser: ((response: Response) => void) | undefined;
    const userPromise = new Promise<Response>((resolve) => {
      resolveUser = resolve;
    });
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        jsonResponse({ access_token: "access-fast", expires_in: 900 }),
      )
      .mockReturnValueOnce(userPromise);

    const bootstrap = useAuthStore.getState().bootstrapSession();
    await vi.waitFor(() => {
      expect(useAuthStore.getState()).toMatchObject({
        accessToken: "access-fast",
        isAuthenticated: true,
        hasHydrated: true,
        isAuthResolved: true,
      });
    });
    expect(useAuthStore.getState().user).toBeNull();

    resolveUser?.(jsonResponse({ id: USER_ID_2, email: "fast@example.com" }));
    await bootstrap;
    expect(useAuthStore.getState().user).toEqual({
      id: USER_ID_2,
      email: "fast@example.com",
    });
  });

  it("fails closed when /me violates the generated identity schema", async () => {
    useAuthStore.setState({
      accessToken: "access-invalid-user",
      isAuthenticated: true,
      hasHydrated: true,
      isAuthResolved: true,
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      jsonResponse({ id: "not-a-uuid", email: "user@example.com" }),
    );

    await expect(useAuthStore.getState().verifySession()).resolves.toBe(false);
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: null,
      isAuthenticated: false,
      user: null,
    });
  });

  it("deduplicates concurrent cookie refreshes", async () => {
    let resolveResponse: ((response: Response) => void) | undefined;
    const responsePromise = new Promise<Response>((resolve) => {
      resolveResponse = resolve;
    });
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockReturnValue(responsePromise);

    const first = useAuthStore.getState().refreshAccessToken();
    const second = useAuthStore.getState().refreshAccessToken();
    resolveResponse?.(
      jsonResponse({ access_token: "access-2", expires_in: 900 }),
    );

    await expect(first).resolves.toBe(true);
    await expect(second).resolves.toBe(true);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("login accepts only the access credential in JSON", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        jsonResponse({ access_token: "access-login", expires_in: 900 }),
      )
      .mockResolvedValueOnce(
        jsonResponse({ id: USER_ID_2, email: "user@example.com" }),
      );

    await useAuthStore.getState().login("user@example.com", "password123");

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/v1/auth/login",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
      }),
    );
    expect(useAuthStore.getState().accessToken).toBe("access-login");
    expect(localStorage.length).toBe(0);
  });

  it("logout uses the cookie endpoint and clears in-memory auth state", async () => {
    useAuthStore.setState({
      accessToken: "access-live",
      isAuthenticated: true,
      hasHydrated: true,
      isAuthResolved: true,
      user: { id: USER_ID_3, email: "user@example.com" },
    });
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ message: "Logged out successfully" }));

    await useAuthStore.getState().logout();

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/auth/logout",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: null,
      isAuthenticated: false,
      user: null,
    });
  });
});
