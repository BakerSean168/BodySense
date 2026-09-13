import { beforeEach, describe, expect, it, vi } from "vitest";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { workspaceApi } from "./workspaceApi";

const validWorkspaceResponse = {
  generated_at: "2026-09-13T08:00:00Z",
  profile_ready: true,
  body_state: {
    current_revision: 0,
    safety_state: {},
    facts: [],
    pending_facts: [],
    observations: [],
    pending_observations: [],
    hypotheses: [],
    recent_revisions: [],
  },
  treatment_revisions: [],
  recent_outcomes: [],
  trends: [],
  capabilities: {
    can_continue_consultation: true,
    can_edit_body_state: true,
    can_request_diagnosis: false,
    can_review_diagnosis: false,
    can_generate_treatment: false,
    can_accept_treatment: false,
    can_execute_treatment: false,
    can_record_outcome: false,
    can_review_treatment: false,
    requires_safety_review: false,
    requires_diagnosis_review: false,
    requires_treatment_review: false,
  },
  actions: [],
};

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

  it("loads the workspace through generated runtime validation without inventing user_id", async () => {
    authFetchMock.mockResolvedValue(
      new Response(JSON.stringify(validWorkspaceResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const result = await workspaceApi.get();

    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/health-workspace",
      expect.objectContaining({ method: "GET" }),
    );
    expect(result.body_state.current_revision).toBe(0);
    expect("user_id" in result.body_state).toBe(false);
  });

  it("rejects the old imaginary workspace body_state.user_id shape", async () => {
    authFetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          ...validWorkspaceResponse,
          body_state: {
            ...validWorkspaceResponse.body_state,
            user_id: "legacy-lie",
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(workspaceApi.get()).rejects.toThrow();
  });

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

  it("accepts an idempotent fact mutation with revision null", async () => {
    authFetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({ ...validMutationResponse, revision: null }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    const result = await workspaceApi.addFact(4, {
      kind: "symptom",
      value: "pain",
    });

    expect(result.fact.updated_revision).toBe(4);
  });

  it("sends expected_revision when resolving safety through the generated boundary", async () => {
    authFetchMock.mockResolvedValue(
      new Response(JSON.stringify({ revision: null }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await workspaceApi.resolveSafety(9, "resolved", "reviewed by user");

    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/body-state/safety/resolve",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          expected_revision: 9,
          resolution: "resolved",
          note: "reviewed by user",
        }),
      }),
    );
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
