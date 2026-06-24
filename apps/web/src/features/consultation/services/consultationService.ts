import { authFetch } from '@/features/auth/services/authService';

export interface ConsultationSession {
  id: string;
  user_id: string;
  messages: ChatMessage[];
  extracted_info: ExtractedInfo[];
  diagnosis: Diagnosis | null;
  treatment_plan: TreatmentPlan | null;
  status: 'in_progress' | 'completed';
  created_at: string;
  ended_at: string | null;
}

export interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
  timestamp?: string;
}

export interface ExtractedInfo {
  body_part: string;
  symptom_type?: string;
  duration?: string;
  trigger?: string;
  relief?: string;
  severity?: string;
  additional_notes?: string;
}

export type Diagnosis = Record<string, unknown>;

export type TreatmentPlan = Record<string, unknown>;

export interface SessionListResponse {
  sessions: ConsultationSession[];
  total: number;
  limit: number;
  offset: number;
}

export const consultationApi = {
  // Create a new consultation session
  createSession: async (): Promise<ConsultationSession> => {
    const response = await authFetch('/api/v1/consultation', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });

    if (!response.ok) {
      throw new Error('Failed to create session');
    }

    return response.json();
  },

  // Get a specific session
  getSession: async (id: string): Promise<ConsultationSession> => {
    const response = await authFetch(`/api/v1/consultation/${id}`);

    if (!response.ok) {
      throw new Error('Failed to get session');
    }

    return response.json();
  },

  // List all sessions
  listSessions: async (limit = 20, offset = 0): Promise<SessionListResponse> => {
    const response = await authFetch(
      `/api/v1/consultation?limit=${limit}&offset=${offset}`
    );

    if (!response.ok) {
      throw new Error('Failed to list sessions');
    }

    return response.json();
  },

  // Update extracted info
  updateExtractedInfo: async (
    sessionId: string,
    extractedInfo: ExtractedInfo[]
  ): Promise<void> => {
    const response = await authFetch(
      `/api/v1/consultation/${sessionId}/extracted-info`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ extracted_info: extractedInfo }),
      }
    );

    if (!response.ok) {
      throw new Error('Failed to update extracted info');
    }
  },

  // Confirm diagnosis
  confirmDiagnosis: async (
    sessionId: string,
    diagnosis: Diagnosis,
  ): Promise<void> => {
    const response = await authFetch(
      `/api/v1/consultation/${sessionId}/confirm`,
      {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ diagnosis }),
      }
    );

    if (!response.ok) {
      throw new Error('Failed to confirm diagnosis');
    }
  },
};
