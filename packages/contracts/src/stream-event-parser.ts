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
