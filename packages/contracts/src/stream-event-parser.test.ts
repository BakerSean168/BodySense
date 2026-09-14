import { describe, expect, it } from "vitest";
import { parseStreamEvent, StreamEventParseError } from "./stream-event-parser";

const valid = {
  version: 1,
  seq: 1,
  channel: "message",
  type: "message.text.delta",
  ids: { conversation_id: "conv-1", run_id: "run-1", message_id: "msg-1" },
  payload: { delta: "hello" },
};

function expectInvalid(input: unknown): StreamEventParseError {
  try {
    parseStreamEvent(input);
    throw new Error("expected StreamEvent validation to fail");
  } catch (error) {
    expect(error).toBeInstanceOf(StreamEventParseError);
    const parseError = error as StreamEventParseError;
    expect(parseError.code).toBe("INVALID_STREAM_EVENT");
    expect(parseError.message).toBe(
      "StreamEvent does not match the canonical v1 schema",
    );
    expect(parseError.issues.length).toBeGreaterThan(0);
    return parseError;
  }
}

describe("parseStreamEvent", () => {
  it("accepts a valid public event", () => {
    expect(parseStreamEvent(valid)).toEqual(valid);
  });

  it.each([
    { ...valid, version: 2 },
    { ...valid, seq: 0 },
    { ...valid, channel: "runtime" },
    { ...valid, type: "runtime.agent_configuration", channel: "runtime" },
    { ...valid, channel: "run" },
    { ...valid, payload: {} },
    { ...valid, ids: { conversation_id: 42 } },
    { ...valid, extra: true },
  ])("rejects malformed events via the canonical schema", (input) => {
    expectInvalid(input);
  });

  it("accepts execution_lost as a durable run.failed reason", () => {
    const event = {
      ...valid,
      channel: "run",
      type: "run.failed",
      payload: { status: "failed", reason: "execution_lost" },
    };
    expect(parseStreamEvent(event)).toEqual(event);
  });

  it("rejects run.failed without either a reason or structured error", () => {
    expectInvalid({
      ...valid,
      channel: "run",
      type: "run.failed",
      payload: { status: "failed" },
    });
  });

  it("validates authority-relevant safety payload", () => {
    expectInvalid({
      ...valid,
      channel: "safety",
      type: "safety.red_flag.detected",
      payload: { has_red_flags: "yes", flags: [] },
    });
  });
});
