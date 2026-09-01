import { authFetch } from "@/features/auth/services/authService";
import { safeJson } from "@/lib/api-url";

export type AssessmentEvidenceSource =
  "body_state" | "report" | "posture_analysis";

export type AssessmentEvidenceDomain =
  | "posture"
  | "exercise"
  | "lifestyle"
  | "anthropometry"
  | "health_report"
  | "injury_symptoms";

export interface AssessmentDomainCoverage {
  status: "available" | "missing";
  evidence_refs: string[];
}

export interface AssessmentEvidenceCoverage {
  status: "complete" | "partial" | "insufficient";
  available_sources: AssessmentEvidenceSource[];
  domains: Record<AssessmentEvidenceDomain, AssessmentDomainCoverage>;
}

export interface AssessmentEvidenceGap {
  dimension: AssessmentEvidenceDomain;
  /** Coverage gap only; not a clinical requirement. */
  required: false;
  description: string;
  needed_sources: AssessmentEvidenceSource[];
}

/** Historical assessment-output-v1 compatibility only. */
export interface LegacyDimensionScores {
  posture: number;
  exercise: number;
  lifestyle: number;
  injury_risk: number;
  overall: number;
}

export type AssessmentObservationKind =
  | "posture_alignment"
  | "posture_asymmetry"
  | "lifestyle_pattern"
  | "exercise_pattern"
  | "report_indicator"
  | "anthropometry";

/** Evidence-grounded observation rendered by application code, not by the model. */
export interface AssessmentObservation {
  observation_id: string;
  review_state: string;
  kind: AssessmentObservationKind;
  body_region: string;
  label: string;
  description: string;
  method: "assessment_evidence" | string;
  evidence_refs: [string];
}

/** Historical model-authored observation. Never treat this as v2 grounded data. */
export interface LegacyAssessmentObservation {
  observation_id?: string;
  review_state?: string;
  kind: string;
  body_region?: string;
  label: string;
  description: string;
  method?: string;
  severity?: string;
  confidence?: string;
  condition?: Record<string, unknown>;
}

interface AssessmentReportBase {
  id: string;
  user_id: string;
  status: "completed" | "insufficient_information";
  summary: string;
  safety_notes: string[];
  body_state_revision?: number;
  created_at: string;
}

export interface EvidenceAssessmentReport extends AssessmentReportBase {
  contract_revision: "assessment-output-v2";
  evidence_coverage: AssessmentEvidenceCoverage;
  evidence_gaps: AssessmentEvidenceGap[];
  observations: AssessmentObservation[];
  /** New reports do not carry pseudo health grades/scores. */
  health_grade?: never;
  dimension_scores?: never;
  information_gaps?: never;
}

export interface LegacyAssessmentReport extends AssessmentReportBase {
  contract_revision: "assessment-output-v1";
  /** Added by migration 000060; historical reports have no reconstructed v2 coverage. */
  evidence_coverage: Record<string, never>;
  evidence_gaps: [];
  observations: LegacyAssessmentObservation[];
  health_grade: "A" | "B" | "C" | "D";
  dimension_scores: LegacyDimensionScores;
  information_gaps: string[];
}

export type AssessmentReport =
  EvidenceAssessmentReport | LegacyAssessmentReport;

export interface AssessmentListResponse {
  reports: AssessmentReport[];
  total: number;
}

export const assessmentApi = {
  async generate(): Promise<EvidenceAssessmentReport> {
    const response = await authFetch("/api/v1/assessment/generate", {
      method: "POST",
    });
    if (!response.ok) throw new Error("Failed to generate assessment");
    return safeJson<EvidenceAssessmentReport>(response);
  },

  async getReport(id: string): Promise<AssessmentReport> {
    const response = await authFetch(`/api/v1/assessment/${id}`);
    if (!response.ok) throw new Error("Failed to load assessment");
    return safeJson<AssessmentReport>(response);
  },

  async listReports(params?: {
    limit?: number;
    offset?: number;
  }): Promise<AssessmentListResponse> {
    const search = new URLSearchParams();
    if (params?.limit != null) search.set("limit", String(params.limit));
    if (params?.offset != null) search.set("offset", String(params.offset));
    const suffix = search.size ? `?${search.toString()}` : "";
    const response = await authFetch(`/api/v1/assessment${suffix}`);
    if (!response.ok) throw new Error("Failed to load assessments");
    return safeJson<AssessmentListResponse>(response);
  },
};
