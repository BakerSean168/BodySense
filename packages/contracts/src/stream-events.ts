/**
 * Public stream-event contract shared by the Web consumer and contract tests.
 *
 * Learning path (Thought Forest note filenames):
 * - typescript-generics-keyof-and-indexed-access.md
 * - typescript-discriminated-unions-and-exhaustiveness.md
 * - typescript-unknown-vs-any.md
 * - typescript-static-types-and-runtime-validation.md
 *
 * Important boundary: these declarations disappear at runtime. Receiving JSON
 * that is asserted as `StreamEvent` does not validate it; validation belongs at
 * the network boundary before reducers or components consume the event.
 */

export type StreamChannel =
  | 'conversation'
  | 'run'
  | 'message'
  | 'tool'
  | 'state'
  | 'source'
  | 'safety'
  | 'usage'
  | 'job'
  | 'stream'
  | 'title';

export interface StreamEventIds {
  conversation_id?: string | null;
  run_id?: string | null;
  turn_id?: string | null;
  message_id?: string | null;
  tool_call_id?: string | null;
  interaction_id?: string | null;
  job_id?: string | null;
}

export interface StreamEventBase<
  // Each concrete event supplies literal arguments such as
  // <'run', 'run.started', { status: 'running'; ... }>. Keeping those
  // literals is what later lets `event.type` discriminate the union.
  TChannel extends StreamChannel,
  TType extends string,
  // Payloads must be object-shaped. The default `Record<string, never>`
  // means “no payload keys are permitted”, not “an arbitrary object”.
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
  {
    title: string;
    title_status: 'pending' | 'generated' | 'manual';
    status: 'active';
    last_message_at: string;
    created_at: string;
    replaces_draft_id?: string;
  }
>;

export type RunStartedEvent = StreamEventBase<
  'run',
  'run.started',
  { status: 'running'; source: 'start_turn' }
>;

export type RunResumedEvent = StreamEventBase<
  'run',
  'run.resumed',
  { status: 'running'; interaction_id: string }
>;

export type RunInterruptedEvent = StreamEventBase<
  'run',
  'run.interrupted',
  { status: 'waiting_user'; interaction_id: string }
>;

export type RunCompletedEvent = StreamEventBase<
  'run',
  'run.completed',
  { status: 'completed'; usage?: unknown }
>;

export type RunFailedEvent = StreamEventBase<
  'run',
  'run.failed',
  { status: 'failed'; error: { message: string } }
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
  { status: 'failed'; error: { message: string } }
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

export type HealthFeaturesUpsertEvent = StreamEventBase<
  'state',
  'state.health_features.upsert',
  { health_features: unknown }
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

export type OutputReviewedEvent = StreamEventBase<
  'safety',
  'safety.output_reviewed',
  {
    kind: string;
    verdict: 'accepted' | 'degraded' | 'rejected';
    reasons?: string[];
    issues?: unknown[];
  }
>;

export type OutputRejectedEvent = StreamEventBase<
  'safety',
  'safety.output_rejected',
  {
    kind: string;
    verdict: 'rejected';
    reasons?: string[];
    safety_fallback?: string;
  }
>;

export type UsageReportedEvent = StreamEventBase<
  'usage',
  'usage.reported',
  { usage: unknown }
>;

export type TitleGeneratedEvent = StreamEventBase<
  'title',
  'title.generated',
  { title: string }
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

export type InteractionQuestionField = {
  key: string;
  label: string;
  answer_type?: 'text' | 'single_choice' | 'multi_choice' | 'number' | 'date' | 'scale' | 'select';
  options?: string[];
  required?: boolean;
};

export type InteractionQuestion = {
  question: string;
  answer_type?: string;
  options?: string[];
  context?: string;
  allow_custom_input?: boolean;
  required?: boolean;
  /** Optional multi-field form (≤3). Single-question path omits this. */
  fields?: InteractionQuestionField[];
};

export type InteractionRequiredEvent = StreamEventBase<
  'state',
  'state.interaction.required',
  { interaction_id: string; question: InteractionQuestion }
>;

export type InteractionAnsweredEvent = StreamEventBase<
  'state',
  'state.interaction.answered',
  { interaction_id: string; answer: unknown }
>;

export type InteractionExpiredEvent = StreamEventBase<
  'state',
  'state.interaction.expired',
  { interaction_id: string; expired_at: string; reason?: string }
>;

export type JobCreatedEvent = StreamEventBase<
  'job',
  'job.created',
  { job_type: string; status?: string }
>;

export type JobProgressEvent = StreamEventBase<
  'job',
  'job.progress',
  { progress?: unknown; stage?: string; percent?: number }
>;

export type JobCompletedEvent = StreamEventBase<
  'job',
  'job.completed',
  { result?: unknown }
>;

export type JobFailedEvent = StreamEventBase<
  'job',
  'job.failed',
  { error: unknown }
>;

export type StreamEvent =
  // This is a discriminated union: every member has a literal `type`.
  // A switch on event.type therefore narrows payload to the matching shape.
  // Adding a member here should make exhaustive consumers fail to compile
  // until they consciously handle or ignore the new protocol event.
  | ConversationCreatedEvent
  | RunStartedEvent
  | RunResumedEvent
  | RunInterruptedEvent
  | RunCompletedEvent
  | RunFailedEvent
  | MessagePersistedEvent
  | MessageCreatedEvent
  | MessageTextDeltaEvent
  | MessageCompletedEvent
  | MessageFailedEvent
  | ToolCallEvent
  | ToolResultEvent
  | ExtractedInfoUpsertEvent
  | HealthFeaturesUpsertEvent
  | PhaseChangedEvent
  | DiagnosisReadyEvent
  | TreatmentReadyEvent
  | CitationAddedEvent
  | KnowledgeGapEvent
  | RedFlagDetectedEvent
  | OutputReviewedEvent
  | OutputRejectedEvent
  | UsageReportedEvent
  | TitleGeneratedEvent
  | StreamDoneEvent
  | StreamErrorEvent
  | InteractionRequiredEvent
  | InteractionAnsweredEvent
  | InteractionExpiredEvent
  | JobCreatedEvent
  | JobProgressEvent
  | JobCompletedEvent
  | JobFailedEvent;
