import { beforeEach, describe, expect, it, vi } from "vitest";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { workspaceApi } from "./workspaceApi";

const validMutationResponse = {
  fact: {
    id: "4df4fc34-371e-49a3-8e74-06bad9dbe64e",
    user_id: "54dc58a5-a2fa-4883-b35a-818057f60cdd",
    kind: "symptom",
    body_region_id: "region:neck",
    value: "pain",
    details: {},
    origin: "user_reported",
    review_state: "confirmed",
    lifecycle_state: "active",
    trend: "unknown",
    provenance: {},
    excluded_from_reasoning: false,
    created_revision: 4,
    updated_revision: 4,
    created_at: "2026-09-13T07:00:00Z",
    updated_at: "2026-09-13T07:00:00Z",
  },
  revision: {
    id: "e7cbe702-a0ca-4365-b9d1-f32871f705e2",
    user_id: "54dc58a5-a2fa-4883-b35a-818057f60cdd",
    revision: 4,
    change_type: "fact_upserted",
    source: "user_edit",
    changes: {},
    created_at: "2026-09-13T07:00:00Z",
  },
};

describe("workspaceApi OpenAPI boundary", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("uses authFetch and validates a successful generated response", async () => {
    authFetchMock.mockResolvedValue(
      new Response(JSON.stringify(validMutationResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const result = await workspaceApi.addFact(3, {
      kind: "symptom",
      value: "pain",
      body_region_id: "region:neck",
    });

    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/body-state/facts",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          expected_revision: 3,
          fact: {
            kind: "symptom",
            value: "pain",
            body_region_id: "region:neck",
          },
        }),
      }),
    );
    expect(result.fact.updated_revision).toBe(4);
  });

  it("normalizes generated non-2xx errors to ApiRequestError", async () => {
    authFetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          error: {
            code: "BODY_STATE_REVISION_CONFLICT",
            message: "expected revision 3, got 4",
          },
        }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(
      workspaceApi.addFact(3, { kind: "symptom", value: "pain" }),
    ).rejects.toMatchObject({
      name: "ApiRequestError",
      status: 409,
      code: "BODY_STATE_REVISION_CONFLICT",
      message: "expected revision 3, got 4",
    });
  });

  it("fails closed when a 200 response violates the OpenAPI schema", async () => {
    authFetchMock.mockResolvedValue(
      new Response(JSON.stringify({ fact: { id: "not-enough-fields" } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(
      workspaceApi.addFact(3, { kind: "symptom", value: "pain" }),
    ).rejects.toThrow();
  });
});
