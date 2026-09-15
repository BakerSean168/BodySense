import {
  generatedStreamEventValidationErrors,
  validateGeneratedStreamEvent,
} from "./generated-stream-event";
import type { StreamEventValidationError } from "../generated/stream-event-validator.v1";
import type { StreamEvent } from "./stream-events";

export class StreamEventParseError extends Error {
  readonly code = "INVALID_STREAM_EVENT";
  readonly issues: readonly StreamEventValidationError[];

  constructor(
    message: string,
    issues: readonly StreamEventValidationError[] = [],
  ) {
    super(message);
    this.name = "StreamEventParseError";
    this.issues = issues;
  }
}

/**
 * Runtime trust boundary for both live SSE and durable replay.
 * Validation is executed by the standalone program generated from the canonical
 * JSON Schema; this wrapper only preserves the stable application error API.
 */
export function parseStreamEvent(input: unknown): StreamEvent {
  if (!validateGeneratedStreamEvent(input)) {
    throw new StreamEventParseError(
      "StreamEvent does not match the canonical v1 schema",
      generatedStreamEventValidationErrors(),
    );
  }
  return input;
}

export function safeParseStreamEvent(
  input: unknown,
):
  | { success: true; data: StreamEvent }
  | { success: false; error: StreamEventParseError } {
  try {
    return { success: true, data: parseStreamEvent(input) };
  } catch (error) {
    return {
      success: false,
      error:
        error instanceof StreamEventParseError
          ? error
          : new StreamEventParseError(
              error instanceof Error ? error.message : String(error),
            ),
    };
  }
}

/**
 * Parse one InteractionQuestion through the canonical StreamEvent validator.
 * These sub-structure helpers intentionally reuse the full generated validator
 * so feature code never grows a second handwritten schema for shared payloads.
 */
export function parseInteractionQuestion(input: unknown) {
  const event = parseStreamEvent({
    version: 1,
    seq: 1,
    channel: "state",
    type: "state.interaction.required",
    ids: {},
    payload: {
      interaction_id: "contract-substructure-probe",
      question: input,
      created_at: "1970-01-01T00:00:00Z",
    },
  });
  if (event.type !== "state.interaction.required") {
    throw new StreamEventParseError(
      "InteractionQuestion probe resolved to wrong variant",
    );
  }
  return event.payload.question;
}

export function parseExtractedInfo(input: unknown) {
  const event = parseStreamEvent({
    version: 1,
    seq: 1,
    channel: "state",
    type: "state.extracted_info.upsert",
    ids: {},
    payload: { info: input },
  });
  if (event.type !== "state.extracted_info.upsert") {
    throw new StreamEventParseError(
      "ExtractedInfo probe resolved to wrong variant",
    );
  }
  return event.payload.info;
}

export function parseCitation(input: unknown) {
  const event = parseStreamEvent({
    version: 1,
    seq: 1,
    channel: "source",
    type: "source.citation.added",
    ids: {},
    payload: { citation: input },
  });
  if (event.type !== "source.citation.added") {
    throw new StreamEventParseError("Citation probe resolved to wrong variant");
  }
  return event.payload.citation;
}

export function parseRedFlag(input: unknown) {
  const event = parseStreamEvent({
    version: 1,
    seq: 1,
    channel: "safety",
    type: "safety.red_flag.detected",
    ids: {},
    payload: { has_red_flags: true, flags: [input] },
  });
  if (
    event.type !== "safety.red_flag.detected" ||
    event.payload.flags.length !== 1
  ) {
    throw new StreamEventParseError("RedFlag probe resolved to wrong variant");
  }
  return event.payload.flags[0];
}

export function parseRedFlagEvent(input: unknown) {
  const event = parseStreamEvent({
    version: 1,
    seq: 1,
    channel: "safety",
    type: "safety.red_flag.detected",
    ids: {},
    payload: input,
  });
  if (event.type !== "safety.red_flag.detected") {
    throw new StreamEventParseError(
      "RedFlag event probe resolved to wrong variant",
    );
  }
  return event.payload;
}
