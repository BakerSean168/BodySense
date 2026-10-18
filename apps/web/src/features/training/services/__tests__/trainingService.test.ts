import { beforeEach, describe, expect, it, vi } from "vitest";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { trainingApi } from "../trainingService";

const planId = "11111111-1111-4111-8111-111111111111";
const revisionId = "22222222-2222-4222-8222-222222222222";
const sessionId = "33333333-3333-4333-8333-333333333333";

const planRecord = {
  planId,
  title: "肩胛稳定",
  goal: "restore capacity",
  status: "active",
  statusRevision: 0,
  currentAcceptedRevisionId: revisionId,
  relatedConditionIds: [],
  relatedBodyRefs: [],
  currentContent: {
    sessions: [
      {
        templateId: "t1",
        title: "Day A",
        stages: [
          {
            phase: "main",
            exercises: [
              {
                planExerciseId: "e1",
                exerciseId: "scapular-retraction",
                nameSnapshot: "Scapular control",
                sets: 2,
                target: { kind: "repetitions", min: 8, max: 12 },
              },
            ],
          },
        ],
      },
    ],
    frequency: { sessionsPerWeek: 3 },
    schedule: { mode: "frequency_only" },
    constraints: [],
  },
  review: { recommended: false, sessionFeedbackCount: 0 },
  executionGate: {
    result: "allow",
    permitsExecution: true,
    reasons: [],
    matchedRestrictionIds: [],
    requiresReview: false,
    urgentCare: false,
    safetyRevision: 4,
  },
  createdAt: "2026-10-18T08:00:00+08:00",
};

const session = {
  sessionId,
  planId,
  planRevisionId: revisionId,
  templateId: "t1",
  timeZone: "Asia/Shanghai",
  trainingDate: "2026-10-18",
  state: "ended",
  endedReason: "finished",
  revision: 2,
  startedAt: "2026-10-18T08:00:00+08:00",
  endedAt: "2026-10-18T08:30:00+08:00",
  startGate: { result: "allow", safetyRevision: 4, policyRevision: "p1" },
  actions: [],
  transitions: [],
};

function json(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

describe("trainingApi generated OpenAPI boundary", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("lists accepted plan records as the server returns them", async () => {
    authFetchMock.mockResolvedValue(json({ plans: [planRecord] }));

    const result = await trainingApi.listPlanRecords();

    expect(authFetchMock).toHaveBeenCalledWith("/api/v1/training-plans", {
      method: "GET",
    });
    expect(result[0]!.planId).toBe(planId);
    expect(result[0]!.currentContent.sessions[0]!.stages[0]!.phase).toBe("main");
  });

  it("accepts an update pinned to its base revision and the presented fingerprint", async () => {
    const proposal = {
      proposalId: "44444444-4444-4444-8444-444444444444",
      kind: "update",
      state: "pending",
      initiatedBy: "agent",
      planId,
      basedOnRevisionId: revisionId,
      draft: {
        title: "肩胛稳定",
        goal: "restore capacity",
        relatedConditionIds: [],
        relatedBodyRefs: [],
        content: planRecord.currentContent,
      },
      contentFingerprint: "b".repeat(64),
      createdAt: "2026-10-18T08:00:00+08:00",
      expiresAt: "2026-10-25T08:00:00+08:00",
    };
    authFetchMock.mockResolvedValue(
      json({
        receiptId: "55555555-5555-4555-8555-555555555555",
        proposalId: proposal.proposalId,
        decisionId: "66666666-6666-4666-8666-666666666666",
        planId,
        revisionId: "77777777-7777-4777-8777-777777777777",
        idempotencyKey: "key-1",
        decidedAt: "2026-10-18T09:00:00+08:00",
        executionGate: planRecord.executionGate,
      }),
    );

    await trainingApi.acceptProposal(proposal as never, {
      decisionId: "66666666-6666-4666-8666-666666666666",
      idempotencyKey: "key-1",
    });

    const [path, init] = authFetchMock.mock.calls[0]!;
    expect(path).toBe(`/api/v1/plan-proposals/${proposal.proposalId}/accept`);
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({
      decisionId: "66666666-6666-4666-8666-666666666666",
      presentedFingerprint: "b".repeat(64),
      targetPlanId: planId,
      expectedRevisionId: revisionId,
      idempotencyKey: "key-1",
    });
  });

  it("starts a session at the revision and Safety revision the user saw", async () => {
    authFetchMock.mockResolvedValue(
      json({
        ...session,
        state: "in_progress",
        endedReason: undefined,
        endedAt: undefined,
        revision: 0,
      }),
    );

    await trainingApi.startSession(planRecord as never, "t1");

    const [path, init] = authFetchMock.mock.calls[0]!;
    expect(path).toBe("/api/v1/training-sessions");
    const body = JSON.parse((init as RequestInit).body as string);
    expect(body).toMatchObject({
      planId,
      expectedPlanRevisionId: revisionId,
      templateId: "t1",
      expectedSafetyRevision: 4,
    });
    expect(typeof body.timeZone).toBe("string");
    expect(body.idempotencyKey).toMatch(/^[0-9a-f-]{36}$/);
  });

  it("records verbatim feedback on an ended session", async () => {
    const text = "  有点累，\n膝盖还好  ";
    authFetchMock.mockResolvedValue(
      json({
        ...session,
        feedback: {
          feedbackId: "44444444-4444-4444-8444-444444444444",
          text,
          idempotencyKey: "k",
          recordedAt: "2026-10-18T08:31:00+08:00",
        },
      }),
    );

    const result = await trainingApi.recordFeedback(sessionId, text);

    const [path, init] = authFetchMock.mock.calls[0]!;
    expect(path).toBe(`/api/v1/training-sessions/${sessionId}/feedback`);
    expect(JSON.parse((init as RequestInit).body as string).text).toBe(text);
    expect(result.feedback?.text).toBe(text);
  });

  it("never calls the retired /training surface", async () => {
    authFetchMock.mockResolvedValue(json({ sessions: [] }));
    await trainingApi.listSessions();
    for (const [path] of authFetchMock.mock.calls) {
      expect(String(path)).not.toMatch(/\/api\/v1\/training(\/|$)/);
    }
  });
});
