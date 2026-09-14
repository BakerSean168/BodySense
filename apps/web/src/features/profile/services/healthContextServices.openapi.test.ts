import { beforeEach, describe, expect, it, vi } from "vitest";

const { authFetchMock } = vi.hoisted(() => ({ authFetchMock: vi.fn() }));
vi.mock("@/features/auth/services/authService", () => ({
  authFetch: authFetchMock,
}));

import { bodyMetricsService } from "./bodyMetricsService";
import { lifestyleService } from "./lifestyleService";
import { onboardingContextService } from "./onboardingContextService";

const jsonResponse = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });

const emptyLifestyleSection = (kind: string) => ({
  kind,
  summary: "",
  details: {},
});

const validLifestyle = {
  current_revision: 4,
  activity: emptyLifestyleSection("lifestyle.activity"),
  sleep: emptyLifestyleSection("lifestyle.sleep"),
  exercise: emptyLifestyleSection("lifestyle.exercise"),
  nutrition: emptyLifestyleSection("lifestyle.nutrition"),
  substances: emptyLifestyleSection("lifestyle.substances"),
  recovery: emptyLifestyleSection("lifestyle.recovery"),
  pending_updates: [],
};

describe("profile health-context OpenAPI boundaries", () => {
  beforeEach(() => authFetchMock.mockReset());

  it("validates body metrics responses before exposing the profile model", async () => {
    authFetchMock.mockResolvedValue(
      jsonResponse({
        current_revision: 4,
        height: { value: 175, unit: "cm" },
        weight: { value: 65, unit: "kg" },
        bmi: 21.2,
      }),
    );

    const result = await bodyMetricsService.get();

    expect(result.height?.value).toBe(175);
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/body-metrics",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("fails closed when body metrics response shape is malformed", async () => {
    authFetchMock.mockResolvedValue(
      jsonResponse({
        current_revision: "4",
        height: { value: 175, unit: "cm" },
      }),
    );

    await expect(bodyMetricsService.get()).rejects.toThrow();
  });

  it("sends the current revision when updating lifestyle", async () => {
    authFetchMock.mockResolvedValue(jsonResponse(validLifestyle));

    const result = await lifestyleService.update({
      expected_revision: 4,
      activity: { summary: "walks daily", details: {} },
    });

    expect(result.current_revision).toBe(4);
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/lifestyle",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({
          expected_revision: 4,
          activity: { summary: "walks daily", details: {} },
        }),
      }),
    );
  });

  it("treats onboarding as an initial revision-0 command", async () => {
    authFetchMock.mockResolvedValue(jsonResponse({ body_state_revision: 1 }));

    const result = await onboardingContextService.submit({
      expected_body_state_revision: 0,
      profile: { gender: "male", birth_date: "2004-01-01" },
      body_metrics: { height_cm: 175, weight_kg: 65 },
      lifestyle: {
        activity: { summary: "" },
        sleep: { summary: "" },
        exercise: { summary: "" },
        nutrition: { summary: "" },
        substances: { summary: "" },
        recovery: { summary: "" },
      },
      injury_history: "",
    });

    expect(result.body_state_revision).toBe(1);
    expect(authFetchMock).toHaveBeenCalledWith(
      "/api/v1/onboarding/context",
      expect.objectContaining({
        method: "PUT",
        body: expect.stringContaining('"expected_body_state_revision":0'),
      }),
    );
  });
});
