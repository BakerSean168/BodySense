import { parseStreamEvent } from "@bodysense/contracts";
import { authFetch } from "@/features/auth/services/authService";
import {
  cancelConsultationRun as cancelConsultationRunOpenApi,
  deleteConversation as deleteConversationOpenApi,
  generateConversationTitle as generateConversationTitleOpenApi,
  getConsultation as getConsultationOpenApi,
  getConsultationInteractionMetrics as getConsultationInteractionMetricsOpenApi,
  getConsultationThread as getConsultationThreadOpenApi,
  getResumeConsultationInteractionUrl,
  getStartConsultationRunUrl,
  getConversation as getConversationOpenApi,
  getSharedConversation as getSharedConversationOpenApi,
  listConversations as listConversationsOpenApi,
  listRunEvents as listRunEventsOpenApi,
  pinConversation as pinConversationOpenApi,
  renameConversationTitle as renameConversationTitleOpenApi,
  shareConversation as shareConversationOpenApi,
  unshareConversation as unshareConversationOpenApi,
} from "@/generated/api/bodysense";
import {
  ResumeConsultationInteractionRequest as ResumeConsultationInteractionRequestSchema,
  StartConsultationRunRequest as StartConsultationRunRequestSchema,
} from "@/generated/api/model";
import type {
  AgentInteractionOutput as PublicAgentInteraction,
  ConsultationSessionResponseOutput as PublicConsultationSession,
  ConsultationThreadResponseOutput as PublicConsultationThread,
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
  PendingInteraction,
  ProjectedToolCall,
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

function toPendingInteraction(
  input: PublicAgentInteraction,
): PendingInteraction {
  return {
    id: input.id,
    run_id: input.run_id,
    conversation_id: input.conversation_id,
    tool_call_id: input.tool_call_id,
    tool_name: input.tool_name,
    question: input.question as unknown as PendingInteraction["question"],
    status: input.status,
    answer: input.answer,
    created_at: input.created_at,
    answered_at: input.answered_at ?? null,
    metadata: input.metadata,
  };
}

function toConsultationSession(
  input: PublicConsultationSession,
): ConsultationSession {
  return {
    conversation_id: input.conversation_id,
    phase: input.phase,
    extracted_info:
      input.extracted_info as unknown as ConsultationSession["extracted_info"],
    diagnosis: null,
    pending_interactions: input.pending_interactions.map(toPendingInteraction),
    created_at: input.created_at,
    updated_at: input.updated_at,
    ended_at: input.ended_at ?? null,
  };
}

function toProjectedToolCall(
  input: PublicConsultationThread["tool_calls"][number],
): ProjectedToolCall {
  return {
    tool_call_id: input.tool_call_id,
    conversation_id: input.conversation_id,
    run_id: input.run_id,
    message_id: input.message_id ?? null,
    tool_name: input.tool_name,
    arguments: input.arguments,
    status: input.status,
    result: input.result ?? null,
    error: input.error ?? null,
    created_at: input.created_at,
    started_at: input.started_at,
    finished_at: input.finished_at ?? null,
    metadata: input.metadata,
  };
}

function toConsultationThread(
  input: PublicConsultationThread,
): ConsultationThread {
  return {
    conversation_id: input.conversation_id,
    phase: input.phase,
    extracted_info:
      input.extracted_info as unknown as ConsultationThread["extracted_info"],
    body_state: input.body_state as ConsultationThread["body_state"],
    diagnosis: null,
    pending_interactions: input.pending_interactions.map(toPendingInteraction),
    interaction_history: input.interaction_history.map(toPendingInteraction),
    created_at: input.created_at,
    updated_at: input.updated_at,
    ended_at: input.ended_at ?? null,
    conversation: {
      id: input.conversation.id,
      title: input.conversation.title ?? null,
      title_status: input.conversation.title_status,
      status: input.conversation.status,
      pinned: input.conversation.pinned,
      pinned_at: input.conversation.pinned_at ?? null,
      default_model: input.conversation.default_model ?? null,
      last_message_at: input.conversation.last_message_at ?? null,
      message_count: input.conversation.message_count,
      metadata: input.conversation.metadata,
      created_at: input.conversation.created_at,
      updated_at: input.conversation.updated_at,
    },
    active_turn_run_id: input.active_turn_run_id ?? null,
    active_turn_events: input.active_turn_events.map((item) =>
      parseStreamEvent({
        version: 1,
        seq: item.seq,
        channel: item.channel,
        type: item.type,
        ids: item.ids,
        payload: item.payload,
      }),
    ),
    messages: input.messages.map(toConversationMessage),
    tool_calls: input.tool_calls.map(toProjectedToolCall),
  };
}

export const consultationApi = {
  /**
   * Start a unified consultation run. The generated request schema owns runtime
   * trust while authFetch keeps the Response body streaming for SSE consumers.
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
    const body = StartConsultationRunRequestSchema.parse(params);
    return authFetch(getStartConsultationRunUrl(), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  },

  /** Explicitly cancel a running or waiting consultation run. */
  async cancelRun(runId: string): Promise<{ status: string; run_id: string }> {
    return withOpenApiError(() =>
      cancelConsultationRunOpenApi(
        runId,
        { reason: "cancelled_by_user" },
        undefined,
        openApiAuthFetch,
      ),
    );
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

  /** Get the projection-backed consultation thread for a conversation. */
  async getConsultationThread(id: string): Promise<ConsultationThread> {
    const result = await withOpenApiError(() =>
      getConsultationThreadOpenApi(id, undefined, openApiAuthFetch),
    );
    return toConsultationThread(result);
  },

  /** Get durable consultation details for a conversation. */
  async getConsultation(id: string): Promise<ConsultationSession> {
    const result = await withOpenApiError(() =>
      getConsultationOpenApi(id, undefined, openApiAuthFetch),
    );
    return toConsultationSession(result);
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
    return withOpenApiError(() =>
      getConsultationInteractionMetricsOpenApi(
        conversationId,
        undefined,
        openApiAuthFetch,
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
    const body = ResumeConsultationInteractionRequestSchema.parse(params);
    return authFetch(
      getResumeConsultationInteractionUrl(conversationId, interactionId),
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );
  },
};
