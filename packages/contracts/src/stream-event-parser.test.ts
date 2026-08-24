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

describe("parseStreamEvent", () => {
  it("accepts a valid public event", () => {
    expect(parseStreamEvent(valid)).toEqual(valid);
  });

  it.each([
    [{ ...valid, version: 2 }, "version"],
    [{ ...valid, seq: 0 }, "seq"],
    [{ ...valid, channel: "runtime" }, "channel"],
    [{ ...valid, type: "runtime.agent_configuration", channel: "runtime" }, "unsupported public event type"],
    [{ ...valid, channel: "run" }, "must use channel"],
    [{ ...valid, payload: {} }, "payload.delta"],
    [{ ...valid, ids: { conversation_id: 42 } }, "ids.conversation_id"],
    [{ ...valid, extra: true }, "unexpected top-level field"],
  ])("rejects malformed events", (input, message) => {
    expect(() => parseStreamEvent(input)).toThrow(StreamEventParseError);
    expect(() => parseStreamEvent(input)).toThrow(message as string);
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
    expect(() =>
      parseStreamEvent({
        ...valid,
        channel: "run",
        type: "run.failed",
        payload: { status: "failed" },
      }),
    ).toThrow("requires reason or error");
  });

  it("validates authority-relevant safety payload", () => {
    expect(() =>
      parseStreamEvent({
        ...valid,
        channel: "safety",
        type: "safety.red_flag.detected",
        payload: { has_red_flags: "yes", flags: [] },
      }),
    ).toThrow("payload.has_red_flags");
  });
});
