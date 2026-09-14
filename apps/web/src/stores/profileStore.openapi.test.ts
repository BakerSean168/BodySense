import { beforeEach, describe, expect, it, vi } from "vitest";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { useAuthStore } from "./authStore";
import { useProfileStore } from "./profileStore";

const PROFILE_ID = "44444444-4444-4444-8444-444444444444";
const USER_ID = "55555555-5555-4555-8555-555555555555";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function validProfile() {
  return {
    id: PROFILE_ID,
    user_id: USER_ID,
    gender: "female",
    birth_date: "2000-01-02",
    age_years: 26,
    created_at: "2026-09-13T08:00:00Z",
    updated_at: "2026-09-13T08:00:00Z",
  } as const;
}

describe("profileStore OpenAPI boundary", () => {
  beforeEach(() => {
    authFetchMock.mockReset();
    useAuthStore.setState({
      accessToken: "access-profile",
      isAuthenticated: true,
      isAuthResolved: true,
    });
    useProfileStore.setState({ profile: null, isLoading: false, error: null });
  });

  it("represents a missing profile through the explicit null envelope", async () => {
    authFetchMock.mockResolvedValue(jsonResponse({ profile: null }));

    await useProfileStore.getState().fetchProfile();

    expect(useProfileStore.getState().profile).toBeNull();
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/profile",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("fails closed when profile identity metadata violates the generated schema", async () => {
    authFetchMock.mockResolvedValue(
      jsonResponse({ profile: { ...validProfile(), user_id: "not-a-uuid" } }),
    );

    await useProfileStore.getState().fetchProfile();

    expect(useProfileStore.getState().profile).toBeNull();
    expect(useProfileStore.getState().error).toBeTruthy();
  });

  it("sends only editable identity fields and validates the stored response", async () => {
    authFetchMock.mockResolvedValue(jsonResponse({ profile: validProfile() }));

    await useProfileStore.getState().updateProfile({
      gender: "female",
      birth_date: "2000-01-02",
    });

    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/profile",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ gender: "female", birth_date: "2000-01-02" }),
      }),
    );
    expect(useProfileStore.getState().profile?.id).toBe(PROFILE_ID);
  });
});
