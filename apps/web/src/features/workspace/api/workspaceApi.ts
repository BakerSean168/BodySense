import {
  acceptTreatmentRevision,
  addBodyStateFact,
  correctBodyStateFact,
  generateTreatmentProposal,
  getHealthWorkspace,
  recordOutcome,
  rejectTreatmentRevision,
  resolveBodyStateSafety,
  reviewCurrentTreatment,
  reviewBodyStateFact,
  reviewBodyStateObservation,
  updateBodyStateFactTemporal,
  updateBodyStateHypothesisLifecycle,
  updateLifestyle,
} from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";
import type { BodyStateFact } from "@/features/consultation/types/consultation";
import type {
  HealthWorkspace,
  Outcome,
  WorkspaceDiagnosis,
  Treatment,
  TreatmentRevision,
  TrainingExecutionPlan,
} from "../types/workspace";

export interface AddFactInput {
  concern_key?: string;
  kind: string;
  body_region?: string;
  body_region_id?: string | null;
  value: string;
  details?: Record<string, unknown>;
  origin?: string;
  review_state?: string;
  lifecycle_state?: string;
  trend?: string;
  observed_at?: string;
}

export type LifestyleSectionKey =
  "activity" | "sleep" | "exercise" | "nutrition" | "substances" | "recovery";

export type BodyStateReviewState = "unverified" | "confirmed" | "rejected";
export type BodyStateHypothesisLifecycleState =
  "active" | "strengthened" | "weakened" | "unsupported" | "retired";
export type BodyStateSafetyResolution =
  "resolved" | "cleared_by_review" | "monitoring";

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
        body_state: response.body_state,
        diagnosis: response.diagnosis
          ? projectWorkspaceDiagnosis(response.diagnosis)
          : undefined,
        treatment: response.treatment,
        training_plan: response.training_plan,
        treatment_revisions: response.treatment_revisions,
        recent_outcomes: response.recent_outcomes,
        trends: response.trends,
        capabilities: response.capabilities,
        actions: response.actions,
      };
    }),

  addFact: async (
    expectedRevision: number,
    fact: AddFactInput,
  ): Promise<{ fact: BodyStateFact }> =>
    withOpenApiError(async () => {
      const response = await addBodyStateFact(
        { expected_revision: expectedRevision, fact },
        undefined,
        openApiAuthFetch,
      );
      return { fact: response.fact };
    }),

  reviewFact: async (
    factId: string,
    expectedRevision: number,
    reviewState: BodyStateReviewState,
  ): Promise<{ fact: BodyStateFact }> =>
    withOpenApiError(async () => {
      const response = await reviewBodyStateFact(
        factId,
        { expected_revision: expectedRevision, review_state: reviewState },
        undefined,
        openApiAuthFetch,
      );
      return { fact: response.fact };
    }),

  correctFact: async (
    factId: string,
    expectedRevision: number,
    replacement: AddFactInput,
  ): Promise<{ fact: BodyStateFact }> =>
    withOpenApiError(async () => {
      const response = await correctBodyStateFact(
        factId,
        { expected_revision: expectedRevision, replacement },
        undefined,
        openApiAuthFetch,
      );
      return { fact: response.fact };
    }),

  updateFactTemporal: async (
    factId: string,
    expectedRevision: number,
    input: { lifecycle_state?: string; trend?: string; valid_until?: string },
  ): Promise<{ fact: BodyStateFact }> =>
    withOpenApiError(async () => {
      const response = await updateBodyStateFactTemporal(
        factId,
        { expected_revision: expectedRevision, ...input },
        undefined,
        openApiAuthFetch,
      );
      return { fact: response.fact };
    }),

  reviewObservation: async (
    observationId: string,
    expectedRevision: number,
    reviewState: BodyStateReviewState,
  ): Promise<void> =>
    withOpenApiError(async () => {
      await reviewBodyStateObservation(
        observationId,
        { expected_revision: expectedRevision, review_state: reviewState },
        undefined,
        openApiAuthFetch,
      );
    }),

  updateHypothesisLifecycle: async (
    hypothesisId: string,
    expectedRevision: number,
    lifecycleState: BodyStateHypothesisLifecycleState,
  ): Promise<void> =>
    withOpenApiError(async () => {
      await updateBodyStateHypothesisLifecycle(
        hypothesisId,
        {
          expected_revision: expectedRevision,
          lifecycle_state: lifecycleState,
          counterevidence_ids: [],
        },
        undefined,
        openApiAuthFetch,
      );
    }),

  resolveSafety: async (
    expectedRevision: number,
    resolution: BodyStateSafetyResolution,
    note: string,
  ): Promise<void> =>
    withOpenApiError(async () => {
      await resolveBodyStateSafety(
        { expected_revision: expectedRevision, resolution, note },
        undefined,
        openApiAuthFetch,
      );
    }),

  generateTreatmentProposal: async (
    diagnosisAnalysisId: string,
    userConstraints: Record<string, unknown> = {},
  ): Promise<{ proposal: TreatmentRevision }> =>
    withOpenApiError(async () => {
      const response = await generateTreatmentProposal(
        {
          diagnosis_analysis_id: diagnosisAnalysisId,
          user_constraints: userConstraints,
        },
        undefined,
        openApiAuthFetch,
      );
      return { proposal: response.proposal };
    }),

  acceptTreatmentRevision: async (
    revisionId: string,
    consultationId?: string | null,
  ): Promise<{
    treatment: Treatment;
    training_plan: TrainingExecutionPlan;
  }> =>
    withOpenApiError(async () => {
      const response = await acceptTreatmentRevision(
        revisionId,
        { consultation_id: consultationId ?? null },
        undefined,
        openApiAuthFetch,
      );
      return {
        treatment: response.treatment,
        training_plan: response.training_plan,
      };
    }),

  rejectTreatmentRevision: async (revisionId: string): Promise<void> =>
    withOpenApiError(async () => {
      await rejectTreatmentRevision(revisionId, undefined, openApiAuthFetch);
    }),

  reviewCurrentTreatment: async (): Promise<{
    treatment: Treatment | null;
  }> =>
    withOpenApiError(async () => {
      const response = await reviewCurrentTreatment(
        undefined,
        openApiAuthFetch,
      );
      return { treatment: response.treatment };
    }),

  recordOutcome: async (input: {
    treatment_id?: string;
    treatment_revision_id?: string;
    intervention_id?: string;
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

  updateLifestyleCurrent: async (
    expectedRevision: number,
    section: LifestyleSectionKey,
    summary: string,
    details: Record<string, unknown> = {},
  ): Promise<void> =>
    withOpenApiError(async () => {
      await updateLifestyle(
        {
          expected_revision: expectedRevision,
          [section]: { summary, details },
        },
        undefined,
        openApiAuthFetch,
      );
    }),
};
