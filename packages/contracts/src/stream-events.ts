/**
 * Public StreamEvent facade.
 *
 * Static authority lives in packages/contracts/schemas/stream-event.v1.schema.json
 * and packages/contracts/generated/stream-event.v1.d.ts is generated from it.
 * Keep consumer-facing aliases here so feature code does not import generated
 * file paths directly.
 */
import type {
  BodySenseStreamEventV1,
  Citation as SchemaCitation,
  ExtractedInfo as SchemaExtractedInfo,
  InteractionQuestion as SchemaInteractionQuestion,
  InteractionQuestionField as SchemaInteractionQuestionField,
  RedFlag as SchemaRedFlag,
} from "../generated/stream-event.v1";

export type StreamEvent = BodySenseStreamEventV1;
export type StreamChannel = StreamEvent["channel"];
export type StreamEventIds = StreamEvent["ids"];
export type InteractionQuestion = SchemaInteractionQuestion;
export type InteractionQuestionField = SchemaInteractionQuestionField;
export type ExtractedInfo = SchemaExtractedInfo;
export type Citation = SchemaCitation;
export type RedFlag = SchemaRedFlag;

type EventOf<TType extends StreamEvent["type"]> = Extract<
  StreamEvent,
  { type: TType }
>;

export type ConversationCreatedEvent = EventOf<"conversation.created">;
export type RunStartedEvent = EventOf<"run.started">;
export type RunResumedEvent = EventOf<"run.resumed">;
export type RunInterruptedEvent = EventOf<"run.interrupted">;
export type RunCompletedEvent = EventOf<"run.completed">;
export type RunFailedEvent = EventOf<"run.failed">;
export type RunCancelledEvent = EventOf<"run.cancelled">;
export type MessagePersistedEvent = EventOf<"message.persisted">;
export type MessageCreatedEvent = EventOf<"message.created">;
export type MessageTextDeltaEvent = EventOf<"message.text.delta">;
export type MessageCompletedEvent = EventOf<"message.completed">;
export type MessageFailedEvent = EventOf<"message.failed">;
export type ToolCallEvent = EventOf<"tool.call">;
export type ToolResultEvent = EventOf<"tool.result">;
export type ExtractedInfoUpsertEvent = EventOf<"state.extracted_info.upsert">;
export type LifestyleContextUpsertEvent =
  EventOf<"state.lifestyle_context.upsert">;
export type PhaseChangedEvent = EventOf<"state.phase.changed">;
export type InteractionRequiredEvent = EventOf<"state.interaction.required">;
export type InteractionAnsweredEvent = EventOf<"state.interaction.answered">;
export type InteractionExpiredEvent = EventOf<"state.interaction.expired">;
export type CitationAddedEvent = EventOf<"source.citation.added">;
export type AnswerAttributionAddedEvent =
  EventOf<"source.answer_attribution.added">;
export type KnowledgeGapEvent = EventOf<"source.knowledge_gap">;
export type RedFlagDetectedEvent = EventOf<"safety.red_flag.detected">;
export type OutputReviewedEvent = EventOf<"safety.output_reviewed">;
export type OutputRejectedEvent = EventOf<"safety.output_rejected">;
export type UsageReportedEvent = EventOf<"usage.reported">;
export type TitleGeneratedEvent = EventOf<"title.generated">;
export type StreamDoneEvent = EventOf<"stream.done">;
export type StreamErrorEvent = EventOf<"stream.error">;
export type JobCreatedEvent = EventOf<"job.created">;
export type JobProgressEvent = EventOf<"job.progress">;
export type JobCompletedEvent = EventOf<"job.completed">;
export type JobFailedEvent = EventOf<"job.failed">;
