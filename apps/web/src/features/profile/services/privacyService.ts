import {
  getPrivacyErasurePlan,
  requestPrivacyErasure,
} from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

export interface PrivacyDataCount {
  name: string;
  count: number;
}

export interface PrivacyErasurePlan {
  destructive: true;
  confirmation_phrase: "DELETE ALL BODY DATA";
  counts: PrivacyDataCount[];
  retained_audit: string[];
}

export interface PrivacyErasureRequestResult {
  request_id: string;
  status: "pending" | "running" | "retryable" | "completed";
  message: string;
}

type GeneratedPrivacyPlan = Awaited<ReturnType<typeof getPrivacyErasurePlan>>;
type GeneratedPrivacyAccepted = Awaited<
  ReturnType<typeof requestPrivacyErasure>
>;

function projectPlan(plan: GeneratedPrivacyPlan): PrivacyErasurePlan {
  return {
    destructive: plan.destructive,
    confirmation_phrase: plan.confirmation_phrase,
    counts: plan.counts.map((item) => ({ name: item.name, count: item.count })),
    retained_audit: [...plan.retained_audit],
  };
}

function projectAccepted(
  result: GeneratedPrivacyAccepted,
): PrivacyErasureRequestResult {
  return {
    request_id: result.request_id,
    status: result.status,
    message: result.message,
  };
}

export const privacyApi = {
  getErasurePlan: async (): Promise<PrivacyErasurePlan> =>
    withOpenApiError(async () =>
      projectPlan(
        await getPrivacyErasurePlan({ cache: "no-store" }, openApiAuthFetch),
      ),
    ),

  requestErasure: async (
    confirmation: PrivacyErasurePlan["confirmation_phrase"],
  ): Promise<PrivacyErasureRequestResult> =>
    withOpenApiError(async () =>
      projectAccepted(
        await requestPrivacyErasure(
          { confirmation },
          { cache: "no-store" },
          openApiAuthFetch,
        ),
      ),
    ),
};
