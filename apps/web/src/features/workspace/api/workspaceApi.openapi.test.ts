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

  it("generates a Treatment proposal through the generated client", async () => {
    const revisionId = "66666666-6666-4666-8666-666666666666";
    const treatmentId = "77777777-7777-4777-8777-777777777777";
    const diagnosisId = "88888888-8888-4888-8888-888888888888";
    const proposal = {
      id: revisionId,
      treatment_id: treatmentId,
      revision: 1,
      acceptance_state: "proposed",
      lifecycle_state: "active",
      source_body_state_revision: 7,
      source_diagnosis_analysis_id: diagnosisId,
      goal: "restore shoulder capacity",
      duration_weeks: 4,
      plan: {
        summary: "graded plan",
        goal: "restore shoulder capacity",
        duration_weeks: 4,
        interventions: [],
        daily_habits: [],
        expected_timeline: "4 weeks",
        warning_signs: [],
        review_triggers: [],
        safety_notes: [],
      },
      user_constraints: {},
      evidence_ids: [],
      governance: {},
      agent_configuration_id: "treatment-v2",
      agent_configuration: {},
      execution_provenance: {},
      evidence_acquisition_trace: {},
      generation_decision_trace: {},
      acceptance_decision_trace: {},
      rollout_provenance: {},
      change_reason: "",
      created_at: "2026-09-13T13:00:00Z",
      interventions: [],
    };
    authFetchMock.mockResolvedValue(
      new Response(JSON.stringify({ proposal }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const result = await workspaceApi.generateTreatmentProposal(diagnosisId, {
      equipment: "bands",
    });

    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/treatments/proposals",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          diagnosis_analysis_id: diagnosisId,
          user_constraints: { equipment: "bands" },
        }),
      }),
    );
    expect(result.proposal.id).toBe(revisionId);
    expect("user_id" in result.proposal).toBe(false);
  });

  it("accepts a Treatment revision through the atomic generated boundary", async () => {
    const revisionId = "66666666-6666-4666-8666-666666666666";
    const treatmentId = "77777777-7777-4777-8777-777777777777";
    const planId = "99999999-9999-4999-8999-999999999999";
    const consultationId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
    authFetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          treatment: {
            id: treatmentId,
            current_revision: 1,
            status: "active",
            status_reasons: [],
            created_at: "2026-09-13T13:00:00Z",
            updated_at: "2026-09-13T13:00:00Z",
          },
          training_plan: {
            id: planId,
            consultation_id: consultationId,
            treatment_id: treatmentId,
            treatment_revision_id: revisionId,
            status: "active",
            goal: "restore shoulder capacity",
            duration_weeks: 4,
            current_week: 1,
            phases: [],
            created_at: "2026-09-13T13:00:00Z",
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const result = await workspaceApi.acceptTreatmentRevision(
      revisionId,
      consultationId,
    );

    expect(authFetchMock).toHaveBeenCalledWith(
      `/api/v1/treatments/revisions/${revisionId}/accept`,
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ consultation_id: consultationId }),
      }),
    );
    expect(result.training_plan.id).toBe(planId);
    expect("user_id" in result.treatment).toBe(false);
    expect("user_id" in result.training_plan).toBe(false);
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
