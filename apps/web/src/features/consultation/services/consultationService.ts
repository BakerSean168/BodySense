import { parseStreamEvent } from "@bodysense/contracts";
import { authFetch } from "@/features/auth/services/authService";
import {
  deleteConversation as deleteConversationOpenApi,
  generateConversationTitle as generateConversationTitleOpenApi,
  getConversation as getConversationOpenApi,
  getSharedConversation as getSharedConversationOpenApi,
  listConversations as listConversationsOpenApi,
  listRunEvents as listRunEventsOpenApi,
  pinConversation as pinConversationOpenApi,
  renameConversationTitle as renameConversationTitleOpenApi,
  shareConversation as shareConversationOpenApi,
  unshareConversation as unshareConversationOpenApi,
} from "@/generated/api/bodysense";
import type {
  ConversationMessageOutput as PublicConversationMessage,
  ConversationOutput as PublicConversation,
} from "@/generated/api/model";
import { expectJson } from "@/lib/api-client";
import {
  openApiAuthFetch,
  openApiPublicFetch,
  withOpenApiError,
} from "@/lib/openapi-client";
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

function toConversation(input: PublicConversation): Conversation {
  return {
    id: input.id,
    title: input.title ?? null,
    title_status: input.title_status,
    status: input.status,
    pinned: input.pinned,
    pinned_at: input.pinned_at ?? null,
    default_model: input.default_model ?? null,
    last_message_at: input.last_message_at ?? null,
    message_count: 0,
    metadata: input.metadata,
    created_at: input.created_at,
    updated_at: input.updated_at,
  };
}

function toErrorInfo(
  value: Record<string, unknown> | undefined,
): Message["error"] {
  if (!value) return null;
  const code = value.code;
  const message = value.message;
  return typeof code === "string" && typeof message === "string"
    ? { code, message }
    : null;
}

function toConversationMessage(input: PublicConversationMessage): Message {
  return {
    id: input.id,
    conversation_id: input.conversation_id,
    turn_id: input.turn_id,
    run_id: input.run_id ?? null,
    parent_message_id: input.parent_message_id ?? null,
    role: input.role,
    status: input.status,
    seq: input.seq,
    // MessagePart remains a feature-domain union. The public OpenAPI transport
    // intentionally validates the envelope while keeping part payloads as JSON
    // objects until the StreamEvent/MessagePart contract is unified in Phase 03.
    parts: input.parts as Message["parts"],
    content_text: input.content_text ?? "",
    model: input.model ?? null,
    provider: input.provider ?? null,
    provider_message_id: input.provider_message_id ?? null,
    provider_response_id: input.provider_response_id ?? null,
    input_tokens: input.input_tokens ?? null,
    output_tokens: input.output_tokens ?? null,
    total_tokens: input.total_tokens ?? null,
    error: toErrorInfo(input.error),
    metadata: input.metadata,
    created_at: input.created_at,
    updated_at: input.updated_at,
  };
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
      metadata?: Record<string, unknown>;
    };
  }): Promise<Response> {
    return authFetch(`${API_BASE}/consultation-runs`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(params),
    });
  },

  /** Explicitly cancel a running or waiting consultation run. */
  async cancelRun(runId: string): Promise<{ status: string; run_id: string }> {
    return authFetch(`${API_BASE}/consultation-runs/${runId}/cancel`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ reason: "cancelled_by_user" }),
    }).then((res) => parseJson<{ status: string; run_id: string }>(res));
  },

  // ===== General Conversation API =====

  /**
   * List conversations with cursor-based pagination.
   */
  async listConversations(params?: {
    cursor?: string;
    limit?: number;
  }): Promise<ConversationListResponse> {
    const result = await withOpenApiError(() =>
      listConversationsOpenApi(params, undefined, openApiAuthFetch),
    );
    return {
      conversations: result.conversations.map(toConversation),
      next_cursor: result.nextCursor ?? null,
      has_more: result.hasMore,
    };
  },

  /** Get one conversation with its ordered messages. */
  async getConversation(
    id: string,
  ): Promise<{ conversation: Conversation; messages: Message[] }> {
    const result = await withOpenApiError(() =>
      getConversationOpenApi(id, undefined, openApiAuthFetch),
    );
    return {
      conversation: toConversation(result.conversation),
      messages: result.messages.map(toConversationMessage),
    };
  },

  async deleteConversation(id: string): Promise<void> {
    await withOpenApiError(() =>
      deleteConversationOpenApi(id, undefined, openApiAuthFetch),
    );
  },

  async pinConversation(id: string, pinned: boolean): Promise<void> {
    await withOpenApiError(() =>
      pinConversationOpenApi(id, { pinned }, undefined, openApiAuthFetch),
    );
  },

  async generateTitle(id: string): Promise<void> {
    await withOpenApiError(() =>
      generateConversationTitleOpenApi(id, undefined, openApiAuthFetch),
    );
  },

  async renameTitle(id: string, title: string): Promise<void> {
    await withOpenApiError(() =>
      renameConversationTitleOpenApi(
        id,
        { title },
        undefined,
        openApiAuthFetch,
      ),
    );
  },

  async shareConversation(id: string): Promise<ConversationShare> {
    return withOpenApiError(() =>
      shareConversationOpenApi(id, undefined, openApiAuthFetch),
    );
  },

  async unshareConversation(id: string): Promise<void> {
    await withOpenApiError(() =>
      unshareConversationOpenApi(id, undefined, openApiAuthFetch),
    );
  },

  /** Fetch shared conversation content through the generated public client. */
  async getSharedConversation(token: string): Promise<SharedConversation> {
    const result = await withOpenApiError(() =>
      getSharedConversationOpenApi(token, undefined, openApiPublicFetch),
    );
    return {
      title: result.title,
      messages: result.messages.map(toConversationMessage),
    };
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
    const raw = await withOpenApiError(() =>
      listRunEventsOpenApi(
        conversationId,
        runId,
        { after_seq: params?.afterSeq, limit: params?.limit },
        undefined,
        openApiAuthFetch,
      ),
    );

    const events: StreamEvent[] = raw.events.map((item) =>
      parseStreamEvent({
        version: 1,
        seq: item.seq,
        channel: item.channel,
        type: item.type,
        ids: item.ids,
        payload: item.payload,
      }),
    );

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
