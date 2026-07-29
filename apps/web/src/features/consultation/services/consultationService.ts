import { authFetch } from '@/features/auth/services/authService';
import type {
  Conversation,
  ConversationListResponse,
  Message,
  ConsultationSession,
  ConsultationThread,
  HealthFeatures,
  Diagnosis,
  DiagnosisAnalysis,
  TreatmentPlan,
  ConversationShare,
  SharedConversation,
  StreamEvent,
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
  /**
   * Start a unified consultation run (creates conversation if needed + sends message in one request).
   * Returns raw Response for SSE streaming.
   */
  async startConsultationRun(params: {
    conversationId: string | null;
    clientMessageId: string;
    requestId: string;
    message: {
      role: string;
      parts: Array<{
        type: string;
        text?: string;
        upload_id?: string;
        mime_type?: string;
        image_url?: string;
      }>;
    };
  }): Promise<Response> {
    return authFetch(`${API_BASE}/consultation-runs`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params),
    });
  },

  // ===== General Conversation API =====


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
   * Get the projection-backed consultation thread for a conversation.
   */
  async getConsultationThread(id: string): Promise<ConsultationThread> {
    return authFetch(`${API_BASE}/consultations/${id}/thread`).then((res) =>
      parseJson<ConsultationThread>(res),
    );
  },

  /**
   * Get consultation details for a conversation.
   */
  async getConsultation(id: string): Promise<ConsultationSession> {
    return authFetch(`${API_BASE}/consultations/${id}`).then((res) =>
      parseJson<ConsultationSession>(res),
    );
  },

  /**
   * Update structured health features for a consultation.
   */
  async updateHealthFeatures(id: string, healthFeatures: HealthFeatures): Promise<void> {
    await authFetch(`${API_BASE}/consultations/${id}/health-features`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ health_features: healthFeatures }),
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
   * Resume a pending interaction and continue the interrupted thread stream.
   */

  /**
   * Incremental durable event log for a run (T0-2 resume).
   * Pass afterSeq to skip already-consumed events (exclusive lower bound).
   */
  async listRunEvents(
    conversationId: string,
    runId: string,
    params?: { afterSeq?: number; limit?: number },
  ): Promise<{
    events: StreamEvent[];
    hasMore: boolean;
    nextAfterSeq: number | null;
  }> {
    const searchParams = new URLSearchParams();
    if (params?.afterSeq != null) searchParams.set('after_seq', String(params.afterSeq));
    if (params?.limit != null) searchParams.set('limit', String(params.limit));
    const query = searchParams.toString();
    const raw = await authFetch(
      `${API_BASE}/conversations/${conversationId}/runs/${runId}/events${query ? '?' + query : ''}`,
    ).then((res) =>
      parseJson<{
        events: Array<{
          seq: number;
          channel: string;
          type: string;
          ids: unknown;
          payload: unknown;
          created_at: string;
        }>;
        hasMore: boolean;
        nextAfterSeq?: number | null;
      }>(res),
    );

    const events: StreamEvent[] = raw.events.map((item) => {
      const ids =
        typeof item.ids === 'string'
          ? (JSON.parse(item.ids) as StreamEvent['ids'])
          : ((item.ids ?? {}) as StreamEvent['ids']);
      const payload =
        typeof item.payload === 'string'
          ? (JSON.parse(item.payload) as StreamEvent['payload'])
          : ((item.payload ?? {}) as StreamEvent['payload']);
      return {
        version: 1,
        seq: item.seq,
        channel: item.channel,
        type: item.type,
        ids,
        payload,
      } as StreamEvent;
    });

    return {
      events,
      hasMore: raw.hasMore,
      nextAfterSeq: raw.nextAfterSeq ?? null,
    };
  },
  async getInteractionMetrics(
    conversationId: string,
  ): Promise<{
    total: number;
    answered: number;
    expired: number;
    pending: number;
    answer_rate: number;
    expire_rate: number;
    avg_wait_seconds: number;
  }> {
    const res = await authFetch(
      `${API_BASE}/consultations/${conversationId}/interaction-metrics`,
    );
    if (!res.ok) {
      throw new Error(`interaction metrics failed: ${res.status}`);
    }
    return res.json();
  },

  async resumeInteractionStream(
    conversationId: string,
    interactionId: string,
    params: {
      requestId: string;
      answer: unknown;
    },
  ): Promise<Response> {
    return authFetch(
      `${API_BASE}/consultations/${conversationId}/interrupts/${interactionId}/answers`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(params),
      },
    );
  },
};

// Re-export domain types for backward compatibility with existing consumers.
// Prefer importing from '../types/consultation' in new code.
export type {
  ConsultationSession,
  ConsultationThread,
  ConsultationPhase,
  HealthFeatures,
  Diagnosis,
  DiagnosisAnalysis,
  TreatmentPlan,
  Citation,
  Conversation,
  ConversationListResponse,
  Message,
  ConversationShare,
  SharedConversation,
  StreamEvent,
} from '../types/consultation';
