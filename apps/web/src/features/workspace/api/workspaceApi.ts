import { getHealthWorkspace, recordOutcome } from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";
import type {
  HealthWorkspace,
  Outcome,
  WorkspaceDiagnosis,
} from "../types/workspace";

function projectWorkspaceCitations(
  citations: Array<Record<string, unknown>>,
): WorkspaceDiagnosis["citations"] {
  return citations.flatMap((citation) => {
    const title = typeof citation.title === "string" ? citation.title : null;
    if (!title) return [];
    const optionalString = (key: string) =>
      typeof citation[key] === "string" ? citation[key] : undefined;
    return [
      {
        title,
        summary: optionalString("summary"),
        content: optionalString("content"),
        category: optionalString("category"),
        snippet: optionalString("snippet"),
        body_markdown: optionalString("body_markdown"),
        source_title: optionalString("source_title"),
        source_author: optionalString("source_author"),
        problem_slug: optionalString("problem_slug"),
      },
    ];
  });
}

function projectWorkspaceDiagnosis(
  diagnosis: Awaited<ReturnType<typeof getHealthWorkspace>>["diagnosis"],
): WorkspaceDiagnosis | undefined {
  if (!diagnosis) return undefined;
  if (!diagnosis.freshness || !diagnosis.candidate_assessments) {
    throw new Error(
      "health workspace diagnosis projection is missing review state",
    );
  }
  return {
    analysis_id: diagnosis.analysis_id,
    body_state_revision: diagnosis.body_state_revision,
    status: diagnosis.status,
    scope: diagnosis.scope,
    summary: diagnosis.summary,
    candidates: diagnosis.candidates,
    citations: projectWorkspaceCitations(diagnosis.citations),
    freshness: diagnosis.freshness,
    candidate_assessments: diagnosis.candidate_assessments.map(
      ({ candidate_id, state }) => ({
        candidate_id,
        state,
      }),
    ),
    created_at: diagnosis.created_at,
  };
}

export const workspaceApi = {
  get: async (): Promise<HealthWorkspace> =>
    withOpenApiError(async () => {
      const response = await getHealthWorkspace(undefined, openApiAuthFetch);
      return {
        generated_at: response.generated_at,
        conversation_id: response.conversation_id,
        profile_ready: response.profile_ready,
        diagnosis: response.diagnosis
          ? projectWorkspaceDiagnosis(response.diagnosis)
          : undefined,
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
