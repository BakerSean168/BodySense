import { submitOnboardingContext } from "@/generated/api/bodysense";
import type { ReportedDiscomfortAreaCode } from "@/generated/api/model/reportedDiscomfortAreaCode.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";
import type { LifestyleSectionInput } from "./lifestyleService";

/** Step 05 coarse area code (or "none"), re-exported so callers stay off generated types. */
export type DiscomfortAreaCode = ReportedDiscomfortAreaCode;

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
  /** Step 05 self-report: coarse areas or ["none"]; omitted when unanswered. Not a symptom record. */
  discomfort_areas?: DiscomfortAreaCode[];
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
