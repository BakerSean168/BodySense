import type {
  ConversationCreatedEvent,
  MessagePersistedEvent,
  MessageCreatedEvent,
  MessageTextDeltaEvent,
  ToolCallEvent,
  ToolResultEvent,
  ExtractedInfoUpsertEvent,
  HealthFeaturesUpsertEvent,
  PhaseChangedEvent,
  CitationAddedEvent,
  KnowledgeGapEvent,
  RedFlagDetectedEvent,
  MessageCompletedEvent,
  MessageFailedEvent,
  TitleGeneratedEvent,
  StreamDoneEvent,
  StreamErrorEvent,
  StreamEvent,
} from '@bodysense/contracts';

export type { StreamEvent };


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
  run_id?: string | null;
  parent_message_id?: string | null;
  role: 'user' | 'assistant' | 'system' | 'tool';
  status: 'submitted' | 'streaming' | 'completed' | 'failed' | 'aborted';
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
  | { type: 'text'; text: string }
  | {
      type: 'source';
      title?: string;
      snippet?: string;
      url?: string;
      id?: string;
      sourceType?: 'url' | 'document';
      providerMetadata?: Record<string, unknown>;
    }
  | {
      type: 'data';
      name: string;
      data: unknown;
    }
  | {
      type: 'tool-call' | 'tool_call';
      tool?: string;
      toolName?: string;
      args: unknown;
      argsText?: string;
      toolCallId?: string;
      tool_call_id?: string;
      result?: unknown;
    }
  | {
      type: 'tool-result' | 'tool_result';
      tool?: string;
      toolName?: string;
      result: unknown;
      toolCallId?: string;
      tool_call_id?: string;
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

export interface ConsultationSession {
  conversation_id: string;
  phase: ConsultationPhase;
  extracted_info: ExtractedInfo[];
  health_features: HealthFeatures;
  diagnosis: DiagnosisAnalysis | null;
  treatment_plan: TreatmentPlan | null;
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
  confirmed?: boolean;
}

export interface HealthFeatureItem {
  label: string;
  body_part?: string;
  value?: string;
  details?: string;
  source?: string;
  confirmed?: boolean;
  metadata?: Record<string, unknown>;
}

export interface HealthFeatures {
  posture_findings: HealthFeatureItem[];
  discomforts: HealthFeatureItem[];
  negative_findings: HealthFeatureItem[];
  movement_limitations: HealthFeatureItem[];
  red_flags: HealthFeatureItem[];
  user_answers: HealthFeatureItem[];
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

export interface ConversationShare {
  shareToken: string;
  shareUrl: string;
}

export interface SharedConversation {
  title: string;
  messages: Message[];
  metadata: Record<string, unknown> | null;
}

export type SSEConversationCreated = ConversationCreatedEvent;
export type SSEMessagePersisted = MessagePersistedEvent;
export type SSEMessageCreated = MessageCreatedEvent;
export type SSETextDelta = MessageTextDeltaEvent;
export type SSEToolCall = ToolCallEvent;
export type SSEToolResult = ToolResultEvent;
export type SSEExtractedInfo = ExtractedInfoUpsertEvent;
export type SSEHealthFeatures = HealthFeaturesUpsertEvent;
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
  status: 'pending' | 'answered' | 'cancelled';
  answer?: unknown;
  created_at: string;
  answered_at?: string | null;
  metadata?: Record<string, unknown>;
}

export interface InteractionHistoryItem extends PendingInteraction {}

export interface ProjectedToolCall {
  tool_call_id: string;
  conversation_id: string;
  run_id: string;
  message_id: string | null;
  tool_name: string;
  arguments: unknown;
  status: 'running' | 'succeeded' | 'failed';
  result: unknown | null;
  error: unknown | null;
  created_at: string;
  started_at: string;
  finished_at: string | null;
  metadata: Record<string, unknown>;
}

export interface AskUserQuestion {
  question: string;
  reason?: string;
  answer_type: 'text' | 'single_choice' | 'multi_choice' | 'number' | 'date';
  options?: string[];
  allow_custom_input?: boolean;
  required?: boolean;
  context?: string;
}

export interface ToolCallInfo {
  id: string;
  tool: string;
  args: unknown;
  result?: unknown;
  status: 'running' | 'completed';
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
