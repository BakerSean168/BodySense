import type {
  ConversationCreatedEvent,
  MessagePersistedEvent,
  MessageCreatedEvent,
  MessageTextDeltaEvent,
  ToolCallEvent,
  ToolResultEvent,
  ExtractedInfoUpsertEvent,
  PhaseChangedEvent,
  CitationAddedEvent,
  KnowledgeGapEvent,
  RedFlagDetectedEvent,
  MessageCompletedEvent,
  MessageFailedEvent,
  StreamDoneEvent,
  StreamErrorEvent,
  InteractionRequiredEvent,
  InteractionAnsweredEvent,
  InteractionExpiredEvent,
  StreamEvent,
  ExtractedInfo as StreamExtractedInfo,
  Citation as StreamCitation,
  RedFlag as StreamRedFlag,
  InteractionQuestion,
} from "@bodysense/contracts";

export type {
  StreamEvent,
  InteractionRequiredEvent,
  InteractionAnsweredEvent,
  InteractionExpiredEvent,
};

export interface Conversation {
  id: string;
  status: "active";
  default_model: string | null;
  last_message_at: string | null;
  message_count: number;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface Message {
  id: string;
  conversation_id: string;
  turn_id: string;
  run_id?: string | null;
  parent_message_id?: string | null;
  role: "user" | "assistant" | "system" | "tool";
  status: "submitted" | "streaming" | "completed" | "failed" | "aborted";
  seq: number;
  parts: MessagePart[];
  content_text: string;
  model: string | null;
  provider: string | null;
  provider_message_id?: string | null;
  provider_response_id?: string | null;
  input_tokens: number | null;
  output_tokens: number | null;
  total_tokens: number | null;
  error: ErrorInfo | null;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export type MessagePart =
  | { type: "text"; text: string }
  | {
      type: "image";
      upload_id: string;
      mime_type?: string;
      image_url?: string;
    }
  | {
      type: "source";
      title?: string;
      snippet?: string;
      url?: string;
      id?: string;
      sourceType?: "url" | "document";
      providerMetadata?: Record<string, unknown>;
    }
  | {
      type: "data";
      name: string;
      data: unknown;
    }
  | {
      type: "tool-call";
      toolName: string;
      args: unknown;
      argsText?: string;
      toolCallId: string;
      result?: unknown;
    }
  | {
      type: "tool-result";
      toolName: string;
      result: unknown;
      toolCallId: string;
    };

export interface ErrorInfo {
  code: string;
  message: string;
}

export interface Run {
  id: string;
  conversation_id: string;
  turn_id: string;
  request_id: string;
  status: "running" | "completed" | "failed" | "cancelled";
  model: string;
  provider: string | null;
  started_at: string;
  completed_at: string | null;
  usage: TokenUsage | null;
  error: ErrorInfo | null;
}

export interface TokenUsage {
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
}

export interface ConsultationSession {
  conversation_id: string;
  phase: ConsultationPhase;
  extracted_info: ExtractedInfo[];
  pending_interactions?: PendingInteraction[];
  interaction_history?: InteractionHistoryItem[];
  created_at: string;
  updated_at: string;
  ended_at: string | null;
  conversation?: Conversation;
}

export interface ConsultationThread extends ConsultationSession {
  conversation: Conversation;
  active_turn_run_id?: string | null;
  active_turn_events?: StreamEvent[];
  messages: Message[];
  tool_calls: ProjectedToolCall[];
}

export type ConsultationPhase = "collecting" | "ready_for_analysis";

export type ExtractedInfo = StreamExtractedInfo;

export interface ConsultationSpatialContext {
  /** Multiple selected canonical regions; selection is navigation, not evidence. */
  body_region_ids?: string[];
  reference_motion?: {
    id: string;
    label: string;
    phase: number;
    paused: boolean;
    source: "reference_animation";
  };
  body_region_id?: string;
  body_region_label?: string;
  anatomy_id?: string;
  anatomy_name?: string;
}

export type Citation = StreamCitation;

export type SSEConversationCreated = ConversationCreatedEvent;
export type SSEMessagePersisted = MessagePersistedEvent;
export type SSEMessageCreated = MessageCreatedEvent;
export type SSETextDelta = MessageTextDeltaEvent;
export type SSEToolCall = ToolCallEvent;
export type SSEToolResult = ToolResultEvent;
export type SSEExtractedInfo = ExtractedInfoUpsertEvent;
export type SSEPhaseChange = PhaseChangedEvent;
export type SSECitation = CitationAddedEvent;
export type SSEKnowledgeGap = KnowledgeGapEvent;
export type SSERedFlag = RedFlagDetectedEvent;
export type SSEMessageCompleted = MessageCompletedEvent;
export type SSEMessageFailed = MessageFailedEvent;
export type SSEStreamDone = StreamDoneEvent;
export type SSEStreamError = StreamErrorEvent;

export interface PendingInteraction {
  id: string;
  run_id: string;
  conversation_id: string;
  tool_call_id: string;
  tool_name: string;
  question: AskUserQuestion;
  status: "pending" | "answered" | "cancelled" | "expired";
  answer?: unknown;
  created_at: string;
  answered_at?: string | null;
  metadata?: Record<string, unknown>;
  /**
   * Go's commit receipt for a confirmed BodyState suggestion, from the live
   * answered milestone (null = declined, nothing written). History carries
   * the same receipt in metadata.body_state_receipt.
   */
  body_state_receipt?: BodyStateSuggestionReceipt | null;
}

/** Id-only Go receipt of a confirmed source-cited BodyState suggestion. */
export type BodyStateSuggestionReceipt = NonNullable<
  Extract<
    StreamEvent,
    { type: "state.interaction.answered" }
  >["payload"]["body_state_receipt"]
>;

export type InteractionHistoryItem = PendingInteraction;

export interface ProjectedToolCall {
  tool_call_id: string;
  conversation_id: string;
  run_id: string;
  message_id: string | null;
  tool_name: string;
  arguments: unknown;
  status: "running" | "succeeded" | "failed";
  result: unknown;
  error: unknown;
  created_at: string;
  started_at: string;
  finished_at: string | null;
  metadata: Record<string, unknown>;
}

export interface AskUserField {
  key: string;
  label: string;
  answer_type:
    | "text"
    | "single_choice"
    | "multi_choice"
    | "number"
    | "date"
    | "scale";
  options?: string[];
  exclusive_options?: string[];
  required?: boolean;
}

export interface AskUserQuestion {
  question: string;
  reason?: string;
  answer_type: "text" | "single_choice" | "multi_choice" | "number" | "date";
  options?: string[];
  allow_custom_input?: boolean;
  required?: boolean;
  context?: string;
  /** Optional multi-field form. When present, UI collects all fields once. */
  fields?: AskUserField[];
  /**
   * Go-owned binding (symptom intake safety screen or source-cited BodyState
   * suggestion). Never authored by ordinary UI input.
   */
  purpose?: "symptom_intake" | "body_state_suggestion" | (string & {});
  state_binding?: InteractionQuestion["state_binding"];
  /** Go-copied excerpt of the user's own message for a bound suggestion. */
  quote?: string;
}

export interface ToolCallInfo {
  id: string;
  tool: string;
  args: unknown;
  result?: unknown;
  status: "running" | "completed";
}

export type RedFlag = StreamRedFlag;

export type RedFlagEvent = RedFlagDetectedEvent["payload"];
