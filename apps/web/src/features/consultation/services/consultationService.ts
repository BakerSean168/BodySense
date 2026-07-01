import { authFetch } from '@/features/auth/services/authService';
import type {
  Conversation,
  ConversationListResponse,
  Message,
  ConsultationSession,
  ExtractedInfo,
  Diagnosis,
  DiagnosisAnalysis,
  TreatmentPlan,
  ConversationShare,
  SharedConversation,
} from '../types/consultation';

const API_BASE = '/api/v1';

/**
 * Parse a Response as JSON, throwing on non-ok status.
 * Skips the ok check when the caller needs the raw Response (e.g. SSE).
 */
async function parseJson<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.text().catch(() => '');
    throw new Error(`API ${res.status}: ${body || res.statusText}`);
  }
  return res.json();
}

export const consultationApi = {
  // ===== General Conversation API =====

  /**
   * Send a message (returns raw Response so the caller can handle SSE streaming).
   */
  async sendMessage(params: {
    conversationId: string | null;
    clientDraftId?: string;
    clientMessageId: string;
    requestId: string;
    message: {
      role: string;
      parts: { type: string; text: string }[];
      metadata?: Record<string, any>;
    };
    context?: { entry?: string; profileId?: string };
  }): Promise<Response> {
    return authFetch(`${API_BASE}/chat/send`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params),
    });
  },

  /**
   * List conversations with cursor-based pagination.
   */
  async listConversations(params?: {
    cursor?: string;
    limit?: number;
  }): Promise<ConversationListResponse> {
    const searchParams = new URLSearchParams();
    if (params?.cursor) searchParams.set('cursor', params.cursor);
    if (params?.limit) searchParams.set('limit', String(params.limit));
    const query = searchParams.toString();
    return authFetch(`${API_BASE}/conversations${query ? '?' + query : ''}`).then(
      (res) => parseJson<ConversationListResponse>(res),
    );
  },

  /**
   * Get a single conversation with its messages.
   */
  async getConversation(
    id: string,
  ): Promise<{ conversation: Conversation; messages: Message[] }> {
    return authFetch(`${API_BASE}/conversations/${id}`).then((res) =>
      parseJson<{ conversation: Conversation; messages: Message[] }>(res),
    );
  },

  /**
   * Delete a conversation.
   */
  async deleteConversation(id: string): Promise<void> {
    await authFetch(`${API_BASE}/conversations/${id}`, {
      method: 'DELETE',
    }).then((res) => parseJson<void>(res));
  },

  /**
   * Toggle pinned state of a conversation.
   */
  async pinConversation(id: string, pinned: boolean): Promise<void> {
    await authFetch(`${API_BASE}/conversations/${id}/pin`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ pinned }),
    }).then((res) => parseJson<void>(res));
  },

  /**
   * Trigger AI-generated title for a conversation.
   */
  async generateTitle(id: string): Promise<void> {
    await authFetch(`${API_BASE}/conversations/${id}/title`, {
      method: 'POST',
    }).then((res) => parseJson<void>(res));
  },

  /**
   * Rename a conversation title (user-initiated).
   */
  async renameTitle(id: string, title: string): Promise<void> {
    await authFetch(`${API_BASE}/conversations/${id}/title`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    }).then((res) => parseJson<void>(res));
  },

  /**
   * Generate a share link for a conversation.
   */
  async shareConversation(id: string): Promise<ConversationShare> {
    return authFetch(`${API_BASE}/conversations/${id}/share`, {
      method: 'POST',
    }).then((res) => parseJson<ConversationShare>(res));
  },

  /**
   * Revoke a share link.
   */
  async unshareConversation(id: string): Promise<void> {
    await authFetch(`${API_BASE}/conversations/${id}/share`, {
      method: 'DELETE',
    }).then((res) => parseJson<void>(res));
  },

  /**
   * Fetch shared conversation content (public, no auth required).
   */
  async getSharedConversation(token: string): Promise<SharedConversation> {
    const res = await fetch(`${API_BASE}/conversations/share/${token}`);
    return parseJson<SharedConversation>(res);
  },

  // ===== Consultation Domain API =====

  /**
   * Get consultation details for a conversation.
   */
  async getConsultation(id: string): Promise<ConsultationSession> {
    return authFetch(`${API_BASE}/consultations/${id}`).then((res) =>
      parseJson<ConsultationSession>(res),
    );
  },

  /**
   * Update extracted symptom info for a consultation.
   */
  async updateExtractedInfo(id: string, info: ExtractedInfo[]): Promise<void> {
    await authFetch(`${API_BASE}/consultations/${id}/extracted-info`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ extracted_info: info }),
    }).then((res) => parseJson<void>(res));
  },

  /**
   * Confirm a diagnosis.
   */
  async confirmDiagnosis(id: string, diagnosis: Diagnosis): Promise<void> {
    await authFetch(`${API_BASE}/consultations/${id}/confirm`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ diagnosis }),
    }).then((res) => parseJson<void>(res));
  },

  /**
   * Trigger AI diagnosis analysis.
   */
  async analyzeDiagnosis(id: string): Promise<DiagnosisAnalysis> {
    return authFetch(`${API_BASE}/consultations/${id}/diagnosis`, {
      method: 'POST',
    }).then((res) => parseJson<DiagnosisAnalysis>(res));
  },

  /**
   * Generate a treatment plan.
   */
  async generateTreatment(id: string, confirmedDiagnosis: Diagnosis): Promise<TreatmentPlan> {
    return authFetch(`${API_BASE}/consultations/${id}/treatment`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ confirmedDiagnosis }),
    }).then((res) => parseJson<TreatmentPlan>(res));
  },

  /**
   * Resume a pending interaction (ask_user).
   * Returns action info — caller should send a new chat message to continue.
   */
  async resumeInteraction(
    conversationId: string,
    interactionId: string,
    answer: unknown,
  ): Promise<{ action: string; answer_text: string }> {
    return authFetch(
      `${API_BASE}/consultations/${conversationId}/interactions/${interactionId}/resume`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ answer }),
      },
    ).then((res) => parseJson<{ action: string; answer_text: string }>(res));
  },
};

// Re-export domain types for backward compatibility with existing consumers.
// Prefer importing from '../types/consultation' in new code.
export type {
  ConsultationSession,
  ConsultationPhase,
  ExtractedInfo,
  Diagnosis,
  DiagnosisAnalysis,
  TreatmentPlan,
  Citation,
  Conversation,
  ConversationListResponse,
  Message,
  ConversationShare,
  SharedConversation,
} from '../types/consultation';
