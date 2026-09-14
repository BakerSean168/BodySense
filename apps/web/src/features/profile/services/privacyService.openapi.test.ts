import { beforeEach, describe, expect, it, vi } from "vitest";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { privacyApi } from "./privacyService";

const jsonResponse = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });

describe("privacy OpenAPI boundary", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("validates and projects the destructive erasure plan", async () => {
    authFetchMock.mockResolvedValue(
      jsonResponse({
        destructive: true,
        confirmation_phrase: "DELETE ALL BODY DATA",
        counts: [{ name: "account", count: 1 }],
        retained_audit: ["anonymous erasure request status/timestamps"],
      }),
    );

    const result = await privacyApi.getErasurePlan();

    expect(result.confirmation_phrase).toBe("DELETE ALL BODY DATA");
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/privacy/erasure-plan",
      expect.objectContaining({ method: "GET", cache: "no-store" }),
    );
  });

  it("fails closed when a privacy plan violates the generated schema", async () => {
    authFetchMock.mockResolvedValue(
      jsonResponse({
        destructive: false,
        confirmation_phrase: "DELETE ALL BODY DATA",
        counts: [{ name: "account", count: -1 }],
        retained_audit: [],
      }),
    );

    await expect(privacyApi.getErasurePlan()).rejects.toThrow();
  });

  it("sends the exact confirmation phrase and validates accepted status", async () => {
    authFetchMock.mockResolvedValue(
      jsonResponse(
        {
          request_id: "11111111-1111-4111-8111-111111111111",
          status: "completed",
          message: "accepted",
        },
        202,
      ),
    );

    const result = await privacyApi.requestErasure("DELETE ALL BODY DATA");

    expect(result.status).toBe("completed");
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/privacy/erasure",
      expect.objectContaining({
        method: "POST",
        cache: "no-store",
        body: JSON.stringify({ confirmation: "DELETE ALL BODY DATA" }),
      }),
    );
  });
});
