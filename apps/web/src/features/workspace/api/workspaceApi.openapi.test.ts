import { beforeEach, describe, expect, it, vi } from "vitest";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { workspaceApi } from "./workspaceApi";

const validWorkspaceResponse = {
  generated_at: "2026-09-13T08:00:00Z",
  profile_ready: true,
  recent_outcomes: [],
  trends: [],
  capabilities: {
    can_continue_consultation: true,
    can_edit_body_state: true,
    can_request_diagnosis: false,
    can_review_diagnosis: false,
    can_generate_training_plan: false,
    can_start_training: false,
    can_record_outcome: false,
    requires_safety_review: false,
    requires_diagnosis_review: false,
  },
  actions: [],
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
    expect("body_state" in result).toBe(false);
  });

  it("rejects the retired body_state field", async () => {
    authFetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          ...validWorkspaceResponse,
          body_state: {
            user_id: "legacy-lie",
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(workspaceApi.get()).rejects.toThrow();
  });

  it("records an Outcome through generated runtime validation", async () => {
    const outcomeId = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
    const input = {
      source_type: "training_log",
      source_key: "training:day-1",
      kind: "symptom_change",
      concern_key: "shoulder.right",
      body_region: "right_shoulder",
      value: { pain_delta: -2 },
      notes: "felt better after training",
    };
    authFetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          outcome: {
            id: outcomeId,
            ...input,
            association_statement: "",
            causality_level: "association_only",
            occurred_at: "2026-09-13T13:20:00Z",
            provenance: {},
            created_at: "2026-09-13T13:20:00Z",
          },
          created: true,
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );

    const result = await workspaceApi.recordOutcome(input);

    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/outcomes",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(input),
      }),
    );
    expect(result.created).toBe(true);
    expect(result.outcome.id).toBe(outcomeId);
    expect("user_id" in result.outcome).toBe(false);
  });
});
