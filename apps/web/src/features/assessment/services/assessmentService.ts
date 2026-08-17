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
  concern_key?: string;
  kind: string;
  body_region: string;
  value: Record<string, unknown>;
  condition?: Record<string, unknown>;
  evidence: string;
  confidence?: string;
}

export interface AssessmentReport {
  id: string;
  user_id: string;
  health_grade: "A" | "B" | "C" | "D";
  dimension_scores: DimensionScores;
  observations: AssessmentObservation[];
  summary: { text?: string } | string;
  information_gaps: string[];
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
