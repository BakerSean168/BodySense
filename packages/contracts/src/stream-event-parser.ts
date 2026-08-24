import type { StreamChannel, StreamEvent } from "./stream-events";

export class StreamEventParseError extends Error {
  readonly code = "INVALID_STREAM_EVENT";

  constructor(message: string) {
    super(message);
    this.name = "StreamEventParseError";
  }
}

type RecordValue = Record<string, unknown>;

type EventSpec = {
  channel: StreamChannel;
  validate: (payload: RecordValue) => void;
};

function isRecord(value: unknown): value is RecordValue {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function assertRecord(value: unknown, path: string): asserts value is RecordValue {
  if (!isRecord(value)) throw new StreamEventParseError(`${path} must be an object`);
}

function requiredString(value: RecordValue, key: string, path = "payload"): string {
  const result = value[key];
  if (typeof result !== "string" || result.length === 0) {
    throw new StreamEventParseError(`${path}.${key} must be a non-empty string`);
  }
  return result;
}

function requiredBoolean(value: RecordValue, key: string, path = "payload"): boolean {
  const result = value[key];
  if (typeof result !== "boolean") {
    throw new StreamEventParseError(`${path}.${key} must be a boolean`);
  }
  return result;
}

function requiredArray(value: RecordValue, key: string, path = "payload"): unknown[] {
  const result = value[key];
  if (!Array.isArray(result)) {
    throw new StreamEventParseError(`${path}.${key} must be an array`);
  }
  return result;
}

function requiredObject(value: RecordValue, key: string, path = "payload"): RecordValue {
  const result = value[key];
  assertRecord(result, `${path}.${key}`);
  return result;
}

function literal(value: RecordValue, key: string, expected: string): void {
  if (value[key] !== expected) {
    throw new StreamEventParseError(`payload.${key} must equal ${JSON.stringify(expected)}`);
  }
}

function optionalStringArray(value: RecordValue, key: string): void {
  const result = value[key];
  if (result === undefined) return;
  if (!Array.isArray(result) || result.some((item) => typeof item !== "string")) {
    throw new StreamEventParseError(`payload.${key} must be an array of strings`);
  }
}

function validateQuestion(payload: RecordValue): void {
  const question = requiredObject(payload, "question");
  requiredString(question, "question", "payload.question");
  const options = question.options;
  if (options !== undefined && (!Array.isArray(options) || options.some((item) => typeof item !== "string"))) {
    throw new StreamEventParseError("payload.question.options must be an array of strings");
  }
  const fields = question.fields;
  if (fields !== undefined) {
    if (!Array.isArray(fields) || fields.length > 3) {
      throw new StreamEventParseError("payload.question.fields must be an array with at most 3 items");
    }
    for (const [index, field] of fields.entries()) {
      assertRecord(field, `payload.question.fields[${index}]`);
      requiredString(field, "key", `payload.question.fields[${index}]`);
      requiredString(field, "label", `payload.question.fields[${index}]`);
    }
  }
}

const noop = (_payload: RecordValue): void => {};

const EVENT_SPECS = {
  "conversation.created": { channel: "conversation", validate: (p) => { requiredString(p, "title_status"); literal(p, "status", "active"); requiredString(p, "last_message_at"); requiredString(p, "created_at"); if (typeof p.title !== "string") throw new StreamEventParseError("payload.title must be a string"); } },
  "run.started": { channel: "run", validate: (p) => { literal(p, "status", "running"); literal(p, "source", "start_turn"); } },
  "run.resumed": { channel: "run", validate: (p) => { literal(p, "status", "running"); requiredString(p, "interaction_id"); } },
  "run.interrupted": { channel: "run", validate: (p) => { literal(p, "status", "waiting_user"); requiredString(p, "interaction_id"); } },
  "run.completed": { channel: "run", validate: (p) => literal(p, "status", "completed") },
  "run.failed": { channel: "run", validate: (p) => {
    literal(p, "status", "failed");
    const reason = p.reason;
    const error = p.error;
    if (reason === undefined && error === undefined) {
      throw new StreamEventParseError("payload.run.failed requires reason or error");
    }
    if (reason !== undefined) requiredString(p, "reason");
    if (error !== undefined) requiredString(requiredObject(p, "error"), "message", "payload.error");
  } },
  "run.cancelled": { channel: "run", validate: (p) => { literal(p, "status", "cancelled"); requiredString(p, "reason"); } },
  "message.persisted": { channel: "message", validate: (p) => { requiredString(p, "client_message_id"); requiredString(p, "role"); } },
  "message.created": { channel: "message", validate: (p) => { requiredString(p, "role"); requiredString(p, "status"); } },
  "message.text.delta": { channel: "message", validate: (p) => { if (typeof p.delta !== "string") throw new StreamEventParseError("payload.delta must be a string"); } },
  "message.completed": { channel: "message", validate: (p) => { literal(p, "status", "completed"); requiredString(p, "finish_reason"); } },
  "message.failed": { channel: "message", validate: (p) => { literal(p, "status", "failed"); requiredString(requiredObject(p, "error"), "message", "payload.error"); } },
  "tool.call": { channel: "tool", validate: (p) => { requiredString(p, "tool"); if (!("args" in p)) throw new StreamEventParseError("payload.args is required"); } },
  "tool.result": { channel: "tool", validate: (p) => { requiredString(p, "tool"); if (!("result" in p)) throw new StreamEventParseError("payload.result is required"); } },
  "state.extracted_info.upsert": { channel: "state", validate: (p) => { if (!("info" in p)) throw new StreamEventParseError("payload.info is required"); } },
  "state.phase.changed": { channel: "state", validate: (p) => { requiredString(p, "to"); requiredString(p, "reason"); } },
  "source.citation.added": { channel: "source", validate: (p) => { if (!("citation" in p)) throw new StreamEventParseError("payload.citation is required"); } },
  "source.knowledge_gap": { channel: "source", validate: (p) => { requiredString(p, "query"); requiredString(p, "message"); } },
  "safety.red_flag.detected": { channel: "safety", validate: (p) => { requiredBoolean(p, "has_red_flags"); requiredArray(p, "flags"); } },
  "safety.output_reviewed": { channel: "safety", validate: (p) => { requiredString(p, "kind"); const verdict = requiredString(p, "verdict"); if (!["accepted", "degraded", "rejected"].includes(verdict)) throw new StreamEventParseError("payload.verdict is invalid"); optionalStringArray(p, "reasons"); } },
  "safety.output_rejected": { channel: "safety", validate: (p) => { requiredString(p, "kind"); literal(p, "verdict", "rejected"); optionalStringArray(p, "reasons"); } },
  "usage.reported": { channel: "usage", validate: (p) => { if (!("usage" in p)) throw new StreamEventParseError("payload.usage is required"); } },
  "title.generated": { channel: "title", validate: (p) => { requiredString(p, "title"); } },
  "stream.done": { channel: "stream", validate: (p) => { if (Object.keys(p).length !== 0) throw new StreamEventParseError("payload for stream.done must be empty"); } },
  "stream.error": { channel: "stream", validate: (p) => { requiredString(p, "message"); } },
  "state.interaction.required": { channel: "state", validate: (p) => { requiredString(p, "interaction_id"); requiredString(p, "created_at"); validateQuestion(p); } },
  "state.interaction.answered": { channel: "state", validate: (p) => { requiredString(p, "interaction_id"); if (!("answer" in p)) throw new StreamEventParseError("payload.answer is required"); } },
  "state.interaction.expired": { channel: "state", validate: (p) => { requiredString(p, "interaction_id"); requiredString(p, "expired_at"); } },
  "job.created": { channel: "job", validate: (p) => { requiredString(p, "job_type"); } },
  "job.progress": { channel: "job", validate: noop },
  "job.completed": { channel: "job", validate: noop },
  "job.failed": { channel: "job", validate: (p) => { if (!("error" in p)) throw new StreamEventParseError("payload.error is required"); } },
} satisfies Record<StreamEvent["type"], EventSpec>;

const TOP_LEVEL_KEYS = new Set(["version", "seq", "channel", "type", "ids", "payload"]);
const ID_KEYS = new Set([
  "conversation_id",
  "run_id",
  "turn_id",
  "message_id",
  "tool_call_id",
  "interaction_id",
  "job_id",
]);

export function parseStreamEvent(input: unknown): StreamEvent {
  assertRecord(input, "StreamEvent");
  for (const key of Object.keys(input)) {
    if (!TOP_LEVEL_KEYS.has(key)) throw new StreamEventParseError(`unexpected top-level field ${key}`);
  }
  if (input.version !== 1) throw new StreamEventParseError("version must equal 1");
  if (!Number.isInteger(input.seq) || (input.seq as number) < 1) {
    throw new StreamEventParseError("seq must be an integer >= 1");
  }
  if (typeof input.type !== "string" || !(input.type in EVENT_SPECS)) {
    throw new StreamEventParseError(`unsupported public event type ${String(input.type)}`);
  }
  const type = input.type as keyof typeof EVENT_SPECS;
  const spec = EVENT_SPECS[type];
  if (input.channel !== spec.channel) {
    throw new StreamEventParseError(`event ${type} must use channel ${spec.channel}`);
  }

  assertRecord(input.ids, "ids");
  for (const [key, value] of Object.entries(input.ids)) {
    if (!ID_KEYS.has(key)) throw new StreamEventParseError(`unexpected ids field ${key}`);
    if (value !== null && value !== undefined && typeof value !== "string") {
      throw new StreamEventParseError(`ids.${key} must be a string or null`);
    }
  }

  assertRecord(input.payload, "payload");
  spec.validate(input.payload);
  return input as unknown as StreamEvent;
}

export function safeParseStreamEvent(input: unknown):
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
          : new StreamEventParseError(error instanceof Error ? error.message : String(error)),
    };
  }
}
