export type StreamChannel =
  | 'conversation'
  | 'message'
  | 'tool'
  | 'state'
  | 'source'
  | 'safety'
  | 'usage'
  | 'stream';

export interface StreamEventIds {
  conversation_id?: string | null;
  run_id?: string | null;
  turn_id?: string | null;
  message_id?: string | null;
  tool_call_id?: string | null;
}

export interface StreamEventBase<
  TChannel extends StreamChannel,
  TType extends string,
  TPayload extends Record<string, unknown> = Record<string, never>,
> {
  version: 1;
  seq: number;
  channel: TChannel;
  type: TType;
  ids: StreamEventIds;
  payload: TPayload;
}

export type ConversationCreatedEvent = StreamEventBase<
  'conversation',
  'conversation.created',
  { replaces_draft_id?: string }
>;

export type MessagePersistedEvent = StreamEventBase<
  'message',
  'message.persisted',
  { client_message_id: string; role: string }
>;

export type MessageCreatedEvent = StreamEventBase<
  'message',
  'message.created',
  { role: string; status: string }
>;

export type MessageTextDeltaEvent = StreamEventBase<
  'message',
  'message.text.delta',
  { delta: string }
>;

export type MessageCompletedEvent = StreamEventBase<
  'message',
  'message.completed',
  { status: 'completed'; finish_reason: string; usage?: unknown }
>;

export type MessageFailedEvent = StreamEventBase<
  'message',
  'message.failed',
  { status: 'failed'; error: unknown }
>;

export type ToolCallEvent = StreamEventBase<
  'tool',
  'tool.call',
  { tool: string; args: unknown }
>;

export type ToolResultEvent = StreamEventBase<
  'tool',
  'tool.result',
  { tool: string; result: unknown }
>;

export type ExtractedInfoUpsertEvent = StreamEventBase<
  'state',
  'state.extracted_info.upsert',
  { info: unknown }
>;

export type PhaseChangedEvent = StreamEventBase<
  'state',
  'state.phase.changed',
  { from?: string; to: string; reason: string }
>;

export type DiagnosisReadyEvent = StreamEventBase<
  'state',
  'state.diagnosis.ready',
  { diagnoses: unknown[] }
>;

export type TreatmentReadyEvent = StreamEventBase<
  'state',
  'state.treatment.ready',
  { treatment_plan: unknown }
>;

export type CitationAddedEvent = StreamEventBase<
  'source',
  'source.citation.added',
  { citation: unknown }
>;

export type KnowledgeGapEvent = StreamEventBase<
  'source',
  'source.knowledge_gap',
  { query: string; message: string }
>;

export type RedFlagDetectedEvent = StreamEventBase<
  'safety',
  'safety.red_flag.detected',
  { has_red_flags: boolean; flags: unknown[] }
>;

export type UsageReportedEvent = StreamEventBase<
  'usage',
  'usage.reported',
  { usage: unknown }
>;

export type StreamDoneEvent = StreamEventBase<
  'stream',
  'stream.done',
  Record<string, never>
>;

export type StreamErrorEvent = StreamEventBase<
  'stream',
  'stream.error',
  { message: string }
>;

export type InteractionRequiredEvent = StreamEventBase<
  'state',
  'state.interaction.required',
  { interaction_id: string; question: { question: string; answer_type: string; options?: string[]; context?: string } }
>;

export type InteractionAnsweredEvent = StreamEventBase<
  'state',
  'state.interaction.answered',
  { interaction_id: string; answer: unknown }
>;

export type StreamEvent =
  | ConversationCreatedEvent
  | MessagePersistedEvent
  | MessageCreatedEvent
  | MessageTextDeltaEvent
  | MessageCompletedEvent
  | MessageFailedEvent
  | ToolCallEvent
  | ToolResultEvent
  | ExtractedInfoUpsertEvent
  | PhaseChangedEvent
  | DiagnosisReadyEvent
  | TreatmentReadyEvent
  | CitationAddedEvent
  | KnowledgeGapEvent
  | RedFlagDetectedEvent
  | UsageReportedEvent
  | StreamDoneEvent
  | StreamErrorEvent
  | InteractionRequiredEvent
  | InteractionAnsweredEvent;
