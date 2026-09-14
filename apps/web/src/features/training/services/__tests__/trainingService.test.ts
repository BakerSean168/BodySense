import { beforeEach, describe, expect, it, vi } from "vitest";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { trainingApi } from "../trainingService";

const planId = "11111111-1111-4111-8111-111111111111";
const validPlan = {
  id: planId,
  status: "active",
  goal: "restore capacity",
  duration_weeks: 4,
  current_week: 1,
  phases: [],
  created_at: "2026-09-13T14:00:00Z",
};

describe("trainingApi generated OpenAPI boundary", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("lists plans through generated response validation", async () => {
    authFetchMock.mockResolvedValue(
      new Response(JSON.stringify({ plans: [validPlan] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const result = await trainingApi.listPlans();

    expect(authFetchMock).toHaveBeenCalledWith("/api/v1/training", {
      method: "GET",
    });
    expect(result).toEqual([validPlan]);
    expect("user_id" in result[0]!).toBe(false);
  });

  it("updates a training log through the generated request contract", async () => {
    authFetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          message: "training log updated",
          has_proposal: false,
          result: { has_proposal: false },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const result = await trainingApi.updateLog(planId, "felt good", [
      { name: "Scapular control", completed: true },
    ]);

    expect(authFetchMock).toHaveBeenCalledWith(
      `/api/v1/training/${planId}/log`,
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({
          notes: "felt good",
          exercises: [{ name: "Scapular control", completed: true }],
        }),
      }),
    );
    expect(result.has_proposal).toBe(false);
  });

  it("submits reassessment feedback through the generated request contract", async () => {
    authFetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          review_recommended: true,
          has_proposal: false,
          requires_new_diagnosis: false,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const result = await trainingApi.submitReassessment(planId, {
      symptom_changes: "less pain",
      training_feeling: "better",
      difficulties: "none",
    });

    expect(authFetchMock).toHaveBeenCalledWith(
      `/api/v1/training/${planId}/reassess`,
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          feedback: {
            symptom_changes: "less pain",
            training_feeling: "better",
            difficulties: "none",
          },
        }),
      }),
    );
    expect(result.review_recommended).toBe(true);
  });
});
