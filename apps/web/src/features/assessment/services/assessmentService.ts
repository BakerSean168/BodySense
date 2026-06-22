import { authFetch } from '@/features/auth/services/authService';

export interface DimensionScores {
  posture: number;
  exercise: number;
  lifestyle: number;
  injury_risk: number;
  overall: number;
}

export interface IdentifiedIssue {
  issue: string;
  severity: '轻度' | '中度' | '重度';
  description: string;
  priority: number;
}

export interface ImprovementSummary {
  exercise?: string;
  lifestyle?: string;
  nutrition?: string;
  general?: string;
}

export interface AssessmentReport {
  id: string;
  user_id: string;
  health_grade: string;
  dimension_scores: DimensionScores;
  identified_issues: IdentifiedIssue[];
  improvement_summary: ImprovementSummary;
  created_at: string;
}

export interface AssessmentListResponse {
  reports: AssessmentReport[];
  total: number;
  limit: number;
  offset: number;
}

export const assessmentApi = {
  // Generate a new assessment report
  generate: async (): Promise<AssessmentReport> => {
    const response = await authFetch('/api/v1/assessment/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.error || 'Failed to generate assessment');
    }

    return response.json();
  },

  // Get a specific report
  getReport: async (id: string): Promise<AssessmentReport> => {
    const response = await authFetch(`/api/v1/assessment/${id}`);

    if (!response.ok) {
      throw new Error('Failed to get report');
    }

    return response.json();
  },

  // List all reports
  listReports: async (limit = 20, offset = 0): Promise<AssessmentListResponse> => {
    const response = await authFetch(
      `/api/v1/assessment?limit=${limit}&offset=${offset}`
    );

    if (!response.ok) {
      throw new Error('Failed to list reports');
    }

    return response.json();
  },
};
