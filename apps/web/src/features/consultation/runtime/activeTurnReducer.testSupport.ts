/**
 * Shared StreamEvent builders for ActiveTurnReducer tests.
 */

import { parseStreamEvent } from "@bodysense/contracts";
import type { StreamEvent } from "../types/consultation";

let _seq = 0;

type EventOf<TType extends StreamEvent["type"]> = Extract<
  StreamEvent,
  { type: TType }
>;

function eventDefault<TType extends StreamEvent["type"]>(
  type: TType,
  channel: EventOf<TType>["channel"],
  payload: EventOf<TType>["payload"],
) {
  return { type, channel, payload };
}

const TEST_EVENT_DEFAULTS = {
  "conversation.created": eventDefault("conversation.created", "conversation", {
    status: "active",
    last_message_at: "2026-08-23T00:00:00Z",
    created_at: "2026-08-23T00:00:00Z",
  }),
  "run.started": eventDefault("run.started", "run", {
    status: "running",
    source: "start_turn",
  }),
  "run.resumed": eventDefault("run.resumed", "run", {
    status: "running",
    interaction_id: "interaction-1",
  }),
  "run.interrupted": eventDefault("run.interrupted", "run", {
    status: "waiting_user",
    interaction_id: "interaction-1",
  }),
  "run.failed": eventDefault("run.failed", "run", {
    status: "failed",
    reason: "test_failure",
  }),
  "run.cancelled": eventDefault("run.cancelled", "run", {
    status: "cancelled",
    reason: "cancelled_by_user",
  }),
  "message.created": eventDefault("message.created", "message", {
    role: "assistant",
    status: "streaming",
  }),
  "message.text.delta": eventDefault("message.text.delta", "message", {
    delta: "",
  }),
  "message.completed": eventDefault("message.completed", "message", {
    status: "completed",
    finish_reason: "stop",
  }),
  "message.failed": eventDefault("message.failed", "message", {
    status: "failed",
    error: { message: "generation failed" },
  }),
  "tool.call": eventDefault("tool.call", "tool", {
    tool: "test_tool",
    args: {},
  }),
  "tool.result": eventDefault("tool.result", "tool", {
    tool: "test_tool",
    result: {},
  }),
  "state.interaction.required": eventDefault(
    "state.interaction.required",
    "state",
    {
      interaction_id: "interaction-1",
      question: { question: "Test question?", answer_type: "text" },
      created_at: "2026-08-23T00:00:00Z",
    },
  ),
  "state.interaction.answered": eventDefault(
    "state.interaction.answered",
    "state",
    {
      interaction_id: "interaction-1",
      answer: { text: "test" },
    },
  ),
  "state.interaction.expired": eventDefault(
    "state.interaction.expired",
    "state",
    {
      interaction_id: "interaction-1",
      expired_at: "2026-08-23T00:05:00Z",
      reason: "ttl_elapsed",
    },
  ),
  "state.phase.changed": eventDefault("state.phase.changed", "state", {
    to: "collecting",
    reason: "test",
  }),
  "source.citation.added": eventDefault("source.citation.added", "source", {
    citation: { title: "Test source" },
  }),
  "source.knowledge_gap": eventDefault("source.knowledge_gap", "source", {
    query: "test query",
    message: "test gap",
  }),
  "safety.red_flag.detected": eventDefault(
    "safety.red_flag.detected",
    "safety",
    {
      has_red_flags: false,
      flags: [],
    },
  ),
  "usage.reported": eventDefault("usage.reported", "usage", { usage: {} }),
  "stream.done": eventDefault("stream.done", "stream", {}),
  "stream.error": eventDefault("stream.error", "stream", {
    message: "stream error",
  }),
};

export type TestEventType = keyof typeof TEST_EVENT_DEFAULTS;

export function makeEvent<TType extends TestEventType>(
  type: TType,
  payload: Record<string, unknown> = {},
  ids: Partial<StreamEvent["ids"]> = {},
  channel?: StreamEvent["channel"],
): StreamEvent {
  const defaults = TEST_EVENT_DEFAULTS[type];
  return parseStreamEvent({
    version: 1,
    seq: ++_seq,
    channel: channel ?? defaults.channel,
    type,
    ids: {
      conversation_id: ids.conversation_id ?? "conv-1",
      run_id: ids.run_id ?? "run-1",
      turn_id: ids.turn_id ?? "turn-1",
      message_id: ids.message_id ?? "msg-1",
      tool_call_id: ids.tool_call_id ?? null,
      interaction_id: ids.interaction_id ?? null,
      job_id: ids.job_id ?? null,
    },
    payload: { ...defaults.payload, ...payload },
  });
}
