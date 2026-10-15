import type { HealthWorkspace, WorkspaceDiagnosis } from "@/features/workspace/types/workspace";
import type { DiagnosisCandidate } from "@/features/consultation/types/consultation";

function candidate(overrides: Partial<DiagnosisCandidate> = {}): DiagnosisCandidate {
  return {
    candidate_id: "cand-1",
    name: "上交叉综合征倾向",
    confidence: "高",
    basis: "久坐后肩颈酸胀",
    ...overrides,
  };
}

export function makeDiagnosis(
  overrides: Partial<WorkspaceDiagnosis> = {},
): WorkspaceDiagnosis {
  return {
    analysis_id: "analysis-1",
    body_state_revision: 3,
    status: "completed",
    scope: "comprehensive",
    summary: "综合情况摘要",
    candidates: [],
    citations: [],
    freshness: {
      analysis_id: "analysis-1",
      state: "fresh",
      evaluated_against_revision: 3,
      reasons: [],
      checked_at: "2026-10-11T00:00:00Z",
    },
    candidate_assessments: [],
    created_at: "2026-10-11T00:00:00Z",
    ...overrides,
  };
}

export function makeWorkspace(
  overrides: Partial<HealthWorkspace> = {},
): HealthWorkspace {
  return {
    generated_at: "2026-10-11T00:00:00Z",
    profile_ready: true,
    body_state: {
      current_revision: 3,
      safety_state: {},
      facts: [],
      observations: [],
      pending_facts: [],
      pending_observations: [],
      recent_revisions: [],
    },
    treatment_revisions: [],
    recent_outcomes: [],
    trends: [],
    capabilities: {
      can_continue_consultation: true,
      can_edit_body_state: true,
      can_request_diagnosis: true,
      can_review_diagnosis: true,
      can_generate_treatment: true,
      can_accept_treatment: true,
      can_execute_treatment: true,
      can_record_outcome: true,
      can_review_treatment: true,
      requires_safety_review: false,
      requires_diagnosis_review: false,
      requires_treatment_review: false,
    },
    actions: [],
    ...overrides,
  };
}

export { candidate };
