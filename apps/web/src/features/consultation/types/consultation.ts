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
  RedFlagDetectedEvent,
  MessageCompletedEvent,
  MessageFailedEvent,
  StreamDoneEvent,
  StreamErrorEvent,
} from '@bodysense/contracts';

export type { StreamEvent } from '@bodysense/contracts';

// 通用会话类型
// Field names use snake_case to match the backend Go model JSON tags.
export interface Conversation {
  id: string;
  title: string | null;
  title_status: 'pending' | 'generating' | 'generated';
  status: 'active' | 'archived' | 'deleted';
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
  role: 'user' | 'assistant' | 'system' | 'tool';
  status: 'submitted' | 'streaming' | 'completed' | 'failed' | 'aborted';
  seq: number;
  parts: MessagePart[];
  content_text: string;
  model: string | null;
  provider: string | null;
  input_tokens: number | null;
  output_tokens: number | null;
  total_tokens: number | null;
  error: ErrorInfo | null;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export type MessagePart =
  | { type: 'text'; text: string }
  | { type: 'source'; title: string; snippet?: string; url?: string }
  | { type: 'tool-call'; tool: string; args: unknown }
  | { type: 'tool-result'; tool: string; result: unknown };

export interface ErrorInfo {
  code: string;
  message: string;
}

export interface Run {
  id: string;
  conversation_id: string;
  turn_id: string;
  request_id: string;
  status: 'running' | 'completed' | 'failed' | 'cancelled';
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

// 咨询领域扩展
export interface ConsultationSession {
  conversation_id: string;
  phase: ConsultationPhase;
  extracted_info: ExtractedInfo[];
  diagnosis: DiagnosisAnalysis | null;
  treatment_plan: TreatmentPlan | null;
  created_at: string;
  updated_at: string;
  ended_at: string | null;
  conversation?: Conversation;
}

export type ConsultationPhase =
  | 'collecting'
  | 'ready_for_analysis'
  | 'analysis_ready'
  | 'diagnosis_confirmed'
  | 'plan_ready'
  | 'completed';

export interface ExtractedInfo {
  body_part: string;
  symptom_type?: string;
  duration?: string;
  trigger?: string;
  relief?: string;
  severity?: string;
  additional_notes?: string;
  /** Whether the user has confirmed this extracted info card. */
  confirmed?: boolean;
}

export interface Diagnosis {
  name: string;
  confidence: '高' | '中' | '低';
  severity: '轻度' | '中度' | '重度';
  basis: string;
  typical_symptoms?: string;
  differential?: string;
}

export interface DiagnosisAnalysis {
  diagnoses: Diagnosis[];
  citations?: Citation[];
}

export interface TreatmentPlan {
  goal: string;
  duration_weeks: number;
  correction_exercises: ExercisePlan[];
  daily_habits: string[];
  nutrition_advice?: string;
  expected_timeline: string;
  warning_signs: string[];
  citations?: Citation[];
}

export interface ExercisePlan {
  name: string;
  description: string;
  sets?: string;
  reps?: string;
  notes?: string;
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

// 分享
export interface ConversationShare {
  shareToken: string;
  shareUrl: string;
}

export interface SharedConversation {
  title: string;
  messages: Message[];
  metadata: Record<string, unknown> | null;
}

// Structured stream event aliases.
export type SSEConversationCreated = ConversationCreatedEvent;
export type SSEMessagePersisted = MessagePersistedEvent;
export type SSEMessageCreated = MessageCreatedEvent;
export type SSETextDelta = MessageTextDeltaEvent;
export type SSEToolCall = ToolCallEvent;
export type SSEToolResult = ToolResultEvent;
export type SSEExtractedInfo = ExtractedInfoUpsertEvent;
export type SSEPhaseChange = PhaseChangedEvent;
export type SSECitation = CitationAddedEvent;
export type SSERedFlag = RedFlagDetectedEvent;
export type SSEMessageCompleted = MessageCompletedEvent;
export type SSEMessageFailed = MessageFailedEvent;
export type SSEStreamDone = StreamDoneEvent;
export type SSEStreamError = StreamErrorEvent;
export interface SSETitleGenerated {
  version: 1;
  seq: number;
  type: 'title.generated';
  channel: 'conversation';
  ids: { conversation_id?: string | null };
  payload: { title: string };
}

// Interaction types for ask_user
export interface PendingInteraction {
  id: string;
  run_id: string;
  conversation_id: string;
  tool_call_id: string;
  tool_name: string;
  question: AskUserQuestion;
  status: 'pending' | 'answered' | 'cancelled';
  created_at: string;
}

export interface AskUserQuestion {
  question: string;
  reason?: string;
  answer_type: 'text' | 'single_choice' | 'multi_choice' | 'number' | 'date';
  options?: string[];
  required?: boolean;
  context?: string;
}

export interface InteractionRequiredEvent {
  version: 1;
  seq: number;
  type: 'state.interaction.required';
  channel: 'state';
  ids: { conversation_id?: string; run_id?: string; interaction_id?: string };
  payload: {
    interaction_id: string;
    question: AskUserQuestion;
  };
}

export interface InteractionAnsweredEvent {
  version: 1;
  seq: number;
  type: 'state.interaction.answered';
  channel: 'state';
  ids: { conversation_id?: string; run_id?: string; interaction_id?: string };
  payload: {
    interaction_id: string;
    answer: unknown;
  };
}

// 列表响应
export interface ConversationListResponse {
  conversations: Conversation[];
  next_cursor: string | null;
  has_more: boolean;
}

// Red flag types (previously in useChatSSE.ts)
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
