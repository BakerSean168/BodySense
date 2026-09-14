import { validateStreamEvent as validateGeneratedStreamEventArtifact } from "../generated/stream-event-validator.v1.js";
import type {
  BodySenseStreamEventV1,
  Citation,
  ConsultationAnswerAttribution,
  ExtractedInfo,
  InteractionQuestion,
  RedFlag,
} from "../generated/stream-event.v1";
import type { StreamEventValidationError } from "../generated/stream-event-validator.v1";

/**
 * Schema-generated public StreamEvent type exposed by the stable contracts facade.
 */
export type GeneratedStreamEvent = BodySenseStreamEventV1;
export type GeneratedStreamEventType = GeneratedStreamEvent["type"];
export type GeneratedCitation = Citation;
export type GeneratedConsultationAnswerAttribution =
  ConsultationAnswerAttribution;
export type GeneratedExtractedInfo = ExtractedInfo;
export type GeneratedInteractionQuestion = InteractionQuestion;
export type GeneratedRedFlag = RedFlag;

/**
 * Browser-safe standalone validator generated from the canonical JSON Schema.
 * It bundles only the compiled validation program; consumers do not ship Ajv.
 */
export function validateGeneratedStreamEvent(
  input: unknown,
): input is GeneratedStreamEvent {
  return validateGeneratedStreamEventArtifact(input);
}

export function generatedStreamEventValidationErrors(): readonly StreamEventValidationError[] {
  return validateGeneratedStreamEventArtifact.errors ?? [];
}
