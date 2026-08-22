import { parseStreamEvent } from "@bodysense/contracts";
import { authFetch } from "@/features/auth/services/authService";
import { expectJson } from "@/lib/api-client";
import type {
  Conversation,
  ConversationListResponse,
  Message,
  ConsultationSession,
  ConsultationThread,
  DiagnosisAnalysis,
  DiagnosisCandidateAssessmentState,
  ConversationShare,
  SharedConversation,
  StreamEvent,
} from "../types/consultation";

const API_BASE = "/api/v1";

/**
 * Parse a Response as JSON, throwing on non-ok status.
 * Skips the ok check when the caller needs the raw Response (e.g. SSE).
 */
async function parseJson<T>(res: Response): Promise<T> {
  if (res.status === 204) return undefined as T;
  return expectJson<T>(res);
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
      method: "POST",
      headers: { "Content-Type": "application/json" },
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
    if (params?.cursor) searchParams.set("cursor", params.cursor);
    if (params?.limit) searchParams.set("limit", String(params.limit));
    const query = searchParams.toString();
    return authFetch(
      `${API_BASE}/conversations${query ? "?" + query : ""}`,
    ).then((res) => parseJson<ConversationListResponse>(res));
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
      method: "DELETE",
    }).then((res) => parseJson<void>(res));
  },

  /**
   * Toggle pinned state of a conversation.
   */
  async pinConversation(id: string, pinned: boolean): Promise<void> {
    await authFetch(`${API_BASE}/conversations/${id}/pin`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ pinned }),
    }).then((res) => parseJson<void>(res));
  },

  /**
   * Trigger AI-generated title for a conversation.
   */
  async generateTitle(id: string): Promise<void> {
    await authFetch(`${API_BASE}/conversations/${id}/title`, {
      method: "POST",
    }).then((res) => parseJson<void>(res));
  },

  /**
   * Rename a conversation title (user-initiated).
   */
  async renameTitle(id: string, title: string): Promise<void> {
    await authFetch(`${API_BASE}/conversations/${id}/title`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title }),
    }).then((res) => parseJson<void>(res));
  },

  /**
   * Generate a share link for a conversation.
   */
  async shareConversation(id: string): Promise<ConversationShare> {
    return authFetch(`${API_BASE}/conversations/${id}/share`, {
      method: "POST",
    }).then((res) => parseJson<ConversationShare>(res));
  },

  /**
   * Revoke a share link.
   */
  async unshareConversation(id: string): Promise<void> {
    await authFetch(`${API_BASE}/conversations/${id}/share`, {
      method: "DELETE",
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
   * Trigger AI diagnosis analysis.
   */
  async analyzeDiagnosis(id: string): Promise<DiagnosisAnalysis> {
    return authFetch(`${API_BASE}/consultations/${id}/diagnosis`, {
      method: "POST",
    }).then((res) => parseJson<DiagnosisAnalysis>(res));
  },

  /** Persist the user's interpretation of Diagnosis candidates without deleting any candidate. */
  async assessDiagnosisCandidates(
    analysisId: string,
    candidates: Array<{
      candidate_id: string;
      state: DiagnosisCandidateAssessmentState;
    }>,
  ): Promise<void> {
    await authFetch(`${API_BASE}/diagnosis-analyses/${analysisId}/assessment`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ candidates }),
    }).then((res) => parseJson<void>(res));
  },

  async listDiagnosisHistory(
    limit = 20,
  ): Promise<{ analyses: DiagnosisAnalysis[] }> {
    return authFetch(`${API_BASE}/diagnosis-analyses?limit=${limit}`).then(
      (res) => parseJson<{ analyses: DiagnosisAnalysis[] }>(res),
    );
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
    if (params?.afterSeq != null)
      searchParams.set("after_seq", String(params.afterSeq));
    if (params?.limit != null) searchParams.set("limit", String(params.limit));
    const query = searchParams.toString();
    const raw = await authFetch(
      `${API_BASE}/conversations/${conversationId}/runs/${runId}/events${query ? "?" + query : ""}`,
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
      const ids: unknown =
        typeof item.ids === "string" ? JSON.parse(item.ids) : (item.ids ?? {});
      const payload: unknown =
        typeof item.payload === "string"
          ? JSON.parse(item.payload)
          : (item.payload ?? {});
      return parseStreamEvent({
        version: 1,
        seq: item.seq,
        channel: item.channel,
        type: item.type,
        ids,
        payload,
      });
    });

    return {
      events,
      hasMore: raw.hasMore,
      nextAfterSeq: raw.nextAfterSeq ?? null,
    };
  },
  async getInteractionMetrics(conversationId: string): Promise<{
    total: number;
    answered: number;
    expired: number;
    pending: number;
    answer_rate: number;
    expire_rate: number;
    avg_wait_seconds: number;
  }> {
    return expectJson(
      await authFetch(
        `${API_BASE}/consultations/${conversationId}/interaction-metrics`,
      ),
    );
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
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(params),
      },
    );
  },
};
