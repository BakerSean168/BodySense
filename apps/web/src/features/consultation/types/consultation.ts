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
  TitleGeneratedEvent,
  StreamDoneEvent,
  StreamErrorEvent,
  InteractionRequiredEvent,
  InteractionAnsweredEvent,
  InteractionExpiredEvent,
  StreamEvent,
} from "@bodysense/contracts";

export type {
  StreamEvent,
  InteractionRequiredEvent,
  InteractionAnsweredEvent,
  InteractionExpiredEvent,
};

export interface Conversation {
  id: string;
  title: string | null;
  title_status: "pending" | "generating" | "generated";
  status: "active" | "archived" | "deleted";
  pinned: boolean;
  pinned_at: string | null;
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
  body_state?: BodyStateSnapshot | null;
  diagnosis: DiagnosisAnalysis | null;
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

export type ConsultationPhase =
  "collecting" | "ready_for_analysis" | "analysis_ready";

export interface ExtractedInfo {
  body_part: string;
  symptom_type?: string;
  duration?: string;
  trigger?: string;
  relief?: string;
  severity?: string;
  additional_notes?: string;
  confirmed?: boolean;
}

export interface BodyStateFact {
  id: string;
  concern_key?: string;
  kind: string;
  body_region?: string;
  value: string;
  details?: Record<string, unknown>;
  origin: string;
  review_state: "unverified" | "confirmed" | "rejected" | string;
  lifecycle_state: "active" | "inactive" | "resolved" | "corrected" | string;
  trend: "unknown" | "stable" | "improving" | "worsening" | string;
  source_key?: string;
  provenance?: Record<string, unknown>;
  observed_at?: string | null;
  valid_from?: string | null;
  valid_until?: string | null;
  supersedes_fact_id?: string | null;
  created_revision?: number;
  updated_revision: number;
  created_at?: string;
  updated_at?: string;
}

export interface BodyStateObservation {
  id: string;
  concern_key?: string;
  kind: string;
  body_region?: string;
  method?: string;
  value: Record<string, unknown>;
  condition?: Record<string, unknown>;
  source_key?: string;
  provenance?: Record<string, unknown>;
  observed_at?: string | null;
  review_state: "unverified" | "confirmed" | "rejected" | string;
  lifecycle_state: string;
  excluded_from_reasoning?: boolean;
  created_revision?: number;
  updated_revision: number;
}

export interface BodyStateHypothesis {
  id: string;
  concern_key: string;
  statement: string;
  lifecycle_state:
    "active" | "strengthened" | "weakened" | "unsupported" | "retired" | string;
  confidence?: string | null;
  supporting_fact_ids: string[];
  supporting_observation_ids: string[];
  supporting_evidence_ids: string[];
  counterevidence_ids: string[];
  source_analysis_id?: string | null;
  provenance?: Record<string, unknown>;
  created_revision: number;
  updated_revision: number;
  created_at?: string;
  updated_at?: string;
}

export interface BodyStateRevision {
  id: string;
  revision: number;
  change_type: string;
  source: string;
  changes: Record<string, unknown>;
  created_at: string;
}

export interface BodyStateSnapshot {
  user_id: string;
  current_revision: number;
  safety_state: Record<string, unknown>;
  facts: BodyStateFact[];
  observations: BodyStateObservation[];
  pending_observations?: BodyStateObservation[];
  hypotheses?: BodyStateHypothesis[];
  recent_revisions?: BodyStateRevision[];
}

export type DiagnosisCandidateAssessmentState =
  "confirmed" | "unsure" | "not_applicable";

export interface DiagnosisCandidate {
  candidate_id?: string;
  concern_key?: string;
  name: string;
  confidence: "高" | "中" | "低";
  severity?: "轻度" | "中度" | "重度";
  evidence_strength?: "高" | "中" | "低";
  impact?: string;
  basis: string;
  typical_symptoms?: string;
  differential?: string;
  reasoning_summary?: string;
  basis_fact_ids?: string[];
  basis_observation_ids?: string[];
  supporting_evidence_ids?: string[];
  counterevidence_ids?: string[];
  missing_information?: string[];
  safety_notes?: string[];
}

export interface DiagnosisFreshness {
  analysis_id: string;
  state: "fresh" | "potentially_stale" | "stale";
  evaluated_against_revision: number;
  reasons: Array<{
    code: string;
    revision?: number;
    change_type?: string;
    item_id?: string;
    concern_key?: string;
    message: string;
  }>;
  checked_at: string;
}

export interface DiagnosisAnalysis {
  analysis_id?: string;
  body_state_revision?: number;
  status?:
    "completed" | "partial" | "insufficient_information" | "safety_blocked";
  scope?: string;
  summary?: string;
  candidates: DiagnosisCandidate[];
  citations?: Citation[];
  freshness?: DiagnosisFreshness;
  candidate_assessments?: Array<{
    candidate_id: string;
    state: DiagnosisCandidateAssessmentState;
    notes?: string;
  }>;
  created_at?: string;
}

export interface Citation {
  title: string;
  summary?: string;
  content?: string;
  category?: string;
  snippet?: string;
  body_markdown?: string;
  source_title?: string;
  source_author?: string;
  problem_slug?: string;
}

export interface ConversationShare {
  shareToken: string;
  shareUrl: string;
}

export interface SharedConversation {
  title: string;
  messages: Message[];
}

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
export type SSETitleGenerated = TitleGeneratedEvent;

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
}

export type InteractionHistoryItem = PendingInteraction;

export interface ProjectedToolCall {
  tool_call_id: string;
  conversation_id: string;
  run_id: string;
  message_id: string | null;
  tool_name: string;
  arguments: unknown;
  status: "running" | "succeeded" | "failed";
  result: unknown | null;
  error: unknown | null;
  created_at: string;
  started_at: string;
  finished_at: string | null;
  metadata: Record<string, unknown>;
}

export interface AskUserField {
  key: string;
  label: string;
  answer_type:
    "text" | "single_choice" | "multi_choice" | "number" | "date" | "scale";
  options?: string[];
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
  /** Optional multi-field form (T0-1). When present, UI collects all fields once. */
  fields?: AskUserField[];
}

export interface ToolCallInfo {
  id: string;
  tool: string;
  args: unknown;
  result?: unknown;
  status: "running" | "completed";
}

export interface ConversationListResponse {
  conversations: Conversation[];
  next_cursor: string | null;
  has_more: boolean;
}

export interface RedFlag {
  category: string;
  message: string;
  matched_text: string;
  source: string;
}

export interface RedFlagEvent {
  has_red_flags: boolean;
  flags: RedFlag[];
}
