import { submitOnboardingContext } from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";
import type { LifestyleSectionInput } from "./lifestyleService";

export interface OnboardingContextPayload {
  expected_body_state_revision: number;
  profile: {
    gender: "male" | "female";
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
  body_state_revision: number;
}

export const onboardingContextService = {
  submit: async (
    payload: OnboardingContextPayload,
  ): Promise<OnboardingContextResult> =>
    withOpenApiError(async () => {
      const response = await submitOnboardingContext(
        payload,
        undefined,
        openApiAuthFetch,
      );
      return { body_state_revision: response.body_state_revision };
    }),
};
