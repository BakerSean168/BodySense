import { getHealthWorkspace, recordOutcome } from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";
import type { HealthWorkspace, Outcome } from "../types/workspace";

export const workspaceApi = {
  get: async (): Promise<HealthWorkspace> =>
    withOpenApiError(async () => {
      const response = await getHealthWorkspace(undefined, openApiAuthFetch);
      return {
        generated_at: response.generated_at,
        conversation_id: response.conversation_id,
        profile_ready: response.profile_ready,
        training_plan: response.training_plan,
        recent_outcomes: response.recent_outcomes,
        trends: response.trends,
        capabilities: response.capabilities,
        actions: response.actions,
      };
    }),

  recordOutcome: async (input: {
    source_type: string;
    source_key: string;
    kind: string;
    concern_key?: string;
    body_region?: string;
    value: Record<string, unknown>;
    notes?: string;
  }): Promise<{ outcome: Outcome; created: boolean }> =>
    withOpenApiError(async () => {
      const response = await recordOutcome(input, undefined, openApiAuthFetch);
      return { outcome: response.outcome, created: response.created };
    }),

};
