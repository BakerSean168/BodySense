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

// SSE 事件类型
export interface SSEConversationCreated {
  conversationId: string;
  replacesDraftId: string;
}

export interface SSEMessagePersisted {
  clientMessageId: string;
  messageId: string;
  role: string;
}

export interface SSEMessageCreated {
  messageId: string;
  role: string;
  status: string;
  turnId: string;
}

export interface SSETextDelta {
  messageId: string;
  delta: string;
}

export interface SSEToolCall {
  messageId: string;
  tool: string;
  args: unknown;
}

export interface SSEToolResult {
  messageId: string;
  tool: string;
  result: unknown;
}

export interface SSEExtractedInfo {
  messageId: string;
  info: ExtractedInfo;
}

export interface SSEPhaseChange {
  messageId: string;
  from: string;
  to: string;
  reason: string;
}

export interface SSECitation {
  messageId: string;
  citation: Citation;
}

export interface SSERedFlag {
  messageId: string;
  flag: {
    type: string;
    message: string;
    severity: string;
  };
}

export interface SSEMessageCompleted {
  messageId: string;
  status: string;
  finishReason: string;
  usage: TokenUsage;
}

export interface SSEMessageFailed {
  messageId: string;
  status: string;
  error: ErrorInfo;
}

export interface SSETitleGenerated {
  conversationId: string;
  title: string;
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
