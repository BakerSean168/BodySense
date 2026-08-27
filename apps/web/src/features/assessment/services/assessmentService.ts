import { authFetch } from "@/features/auth/services/authService";
import { safeJson } from "@/lib/api-url";

export interface DimensionScores {
  posture: number;
  exercise: number;
  lifestyle: number;
  injury_risk: number;
  overall: number;
}

export interface AssessmentObservation {
  observation_id: string;
  review_state: string;
  kind: string;
  body_region: string;
  label: string;
  description: string;
  severity: string;
  confidence: string;
  method: string;
  condition: Record<string, unknown>;
}

export interface AssessmentReport {
  id: string;
  user_id: string;
  status: "completed" | "insufficient_information";
  health_grade: "A" | "B" | "C" | "D";
  dimension_scores: DimensionScores;
  observations: AssessmentObservation[];
  summary: string;
  information_gaps: string[];
  safety_notes: string[];
  body_state_revision?: number;
  created_at: string;
}

export interface AssessmentListResponse {
  reports: AssessmentReport[];
  total: number;
}

export const assessmentApi = {
  async generate(): Promise<AssessmentReport> {
    const response = await authFetch("/api/v1/assessment/generate", {
      method: "POST",
    });
    if (!response.ok) throw new Error("Failed to generate assessment");
    return safeJson<AssessmentReport>(response);
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
