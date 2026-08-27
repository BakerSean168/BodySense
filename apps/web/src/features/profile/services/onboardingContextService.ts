import { authFetch } from "@/features/auth/services/authService";
import { expectJson } from "@/lib/api-client";
import type { LifestyleSectionInput } from "./lifestyleService";

export interface OnboardingContextPayload {
  expected_body_state_revision?: number;
  profile: {
    gender: string;
    birth_date: string;
  };
  body_metrics: {
    height_cm: number;
    weight_kg: number;
  };
  lifestyle: {
    activity: LifestyleSectionInput;
    sleep: LifestyleSectionInput;
    exercise: LifestyleSectionInput;
    nutrition: LifestyleSectionInput;
    substances: LifestyleSectionInput;
    recovery: LifestyleSectionInput;
  };
  injury_history: string;
}

export interface OnboardingContextResult {
  body_state_revision?: number;
}

export const onboardingContextService = {
  submit: async (payload: OnboardingContextPayload) =>
    expectJson<OnboardingContextResult>(
      await authFetch("/api/v1/onboarding/context", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      }),
    ),
};
