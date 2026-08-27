import { authFetch } from "@/features/auth/services/authService";
import { expectEmpty, expectJson } from "@/lib/api-client";
import type { BodyStateFact } from "@/features/consultation/types/consultation";
import type {
  HealthWorkspace,
  Outcome,
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

export const workspaceApi = {
  get: () => request<HealthWorkspace>("/api/v1/health-workspace"),

  addFact: (expectedRevision: number, fact: AddFactInput) =>
    request<{ fact: BodyStateFact }>("/api/v1/body-state/facts", {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify({ expected_revision: expectedRevision, fact }),
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
};
