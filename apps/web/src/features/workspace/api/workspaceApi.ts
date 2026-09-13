import { authFetch } from "@/features/auth/services/authService";
import { expectEmpty, expectJson } from "@/lib/api-client";
import {
  addBodyStateFact,
  getHealthWorkspace,
} from "@/generated/api/bodysense";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";
import type { BodyStateFact } from "@/features/consultation/types/consultation";
import type {
  HealthWorkspace,
  Outcome,
  WorkspaceDiagnosis,
  Treatment,
  TreatmentRevision,
} from "../types/workspace";

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  return expectJson<T>(await authFetch(url, init));
}

async function requestEmpty(url: string, init?: RequestInit): Promise<void> {
  return expectEmpty(await authFetch(url, init));
}

const jsonHeaders = { "Content-Type": "application/json" };

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

  reviewFact: (factId: string, expectedRevision: number, reviewState: string) =>
    request<{ fact: BodyStateFact }>(
      `/api/v1/body-state/facts/${factId}/review`,
      {
        method: "PATCH",
        headers: jsonHeaders,
        body: JSON.stringify({
          expected_revision: expectedRevision,
          review_state: reviewState,
        }),
      },
    ),

  correctFact: (
    factId: string,
    expectedRevision: number,
    replacement: AddFactInput,
  ) =>
    request<{ fact: BodyStateFact }>(
      `/api/v1/body-state/facts/${factId}/correct`,
      {
        method: "POST",
        headers: jsonHeaders,
        body: JSON.stringify({
          expected_revision: expectedRevision,
          replacement,
        }),
      },
    ),

  updateFactTemporal: (
    factId: string,
    expectedRevision: number,
    input: { lifecycle_state?: string; trend?: string; valid_until?: string },
  ) =>
    request<{ fact: BodyStateFact }>(
      `/api/v1/body-state/facts/${factId}/temporal`,
      {
        method: "PATCH",
        headers: jsonHeaders,
        body: JSON.stringify({ expected_revision: expectedRevision, ...input }),
      },
    ),

  reviewObservation: (
    observationId: string,
    expectedRevision: number,
    reviewState: "confirmed" | "rejected",
  ) =>
    request(`/api/v1/body-state/observations/${observationId}/review`, {
      method: "PATCH",
      headers: jsonHeaders,
      body: JSON.stringify({
        expected_revision: expectedRevision,
        review_state: reviewState,
      }),
    }),

  updateHypothesisLifecycle: (
    hypothesisId: string,
    expectedRevision: number,
    lifecycleState: string,
  ) =>
    request(`/api/v1/body-state/hypotheses/${hypothesisId}/lifecycle`, {
      method: "PATCH",
      headers: jsonHeaders,
      body: JSON.stringify({
        expected_revision: expectedRevision,
        lifecycle_state: lifecycleState,
        counterevidence_ids: [],
      }),
    }),

  resolveSafety: (expectedRevision: number, resolution: string, note: string) =>
    request("/api/v1/body-state/safety/resolve", {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify({
        expected_revision: expectedRevision,
        resolution,
        note,
      }),
    }),

  generateTreatmentProposal: (
    diagnosisAnalysisId: string,
    userConstraints: Record<string, unknown> = {},
  ) =>
    request<{ proposal: TreatmentRevision }>("/api/v1/treatments/proposals", {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify({
        diagnosis_analysis_id: diagnosisAnalysisId,
        user_constraints: userConstraints,
      }),
    }),

  acceptTreatmentRevision: (
    revisionId: string,
    consultationId?: string | null,
  ) =>
    request<{ treatment: Treatment; training_plan?: { id: string } | null }>(
      `/api/v1/treatments/revisions/${revisionId}/accept`,
      {
        method: "POST",
        headers: jsonHeaders,
        body: JSON.stringify({ consultation_id: consultationId || null }),
      },
    ),

  rejectTreatmentRevision: (revisionId: string) =>
    requestEmpty(`/api/v1/treatments/revisions/${revisionId}/reject`, {
      method: "POST",
    }),

  reviewCurrentTreatment: () =>
    request<{ treatment: Treatment | null }>(
      "/api/v1/treatments/current/review",
      {
        method: "POST",
      },
    ),

  recordOutcome: (input: {
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
  }) =>
    request<{ outcome: Outcome; created: boolean }>("/api/v1/outcomes", {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify(input),
    }),

  updateLifestyleCurrent: (
    expectedRevision: number,
    section: LifestyleSectionKey,
    summary: string,
    details: Record<string, unknown> = {},
  ) =>
    request("/api/v1/lifestyle", {
      method: "PUT",
      headers: jsonHeaders,
      body: JSON.stringify({
        expected_revision: expectedRevision,
        [section]: {
          summary,
          details,
        },
      }),
    }),
};
