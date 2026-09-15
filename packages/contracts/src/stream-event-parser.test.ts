import { describe, expect, it } from "vitest";
import {
  parseCitation,
  parseExtractedInfo,
  parseInteractionQuestion,
  parseRedFlag,
  parseRedFlagEvent,
  parseStreamEvent,
  StreamEventParseError,
} from "./stream-event-parser";

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

describe("canonical StreamEvent sub-structure parsers", () => {
  it("parses shared payload structures through the generated validator", () => {
    expect(
      parseInteractionQuestion({ question: "哪里疼？", answer_type: "text" }),
    ).toEqual({
      question: "哪里疼？",
      answer_type: "text",
    });
    expect(
      parseExtractedInfo({ body_part: "颈部", symptom_type: "疼痛" }),
    ).toEqual({
      body_part: "颈部",
      symptom_type: "疼痛",
    });
    expect(parseCitation({ title: "Evidence" })).toEqual({ title: "Evidence" });
    expect(
      parseRedFlag({ category: "weakness", message: "new weakness" }),
    ).toEqual({
      category: "weakness",
      message: "new weakness",
    });
    expect(
      parseRedFlagEvent({
        has_red_flags: true,
        flags: [{ category: "weakness", message: "new weakness" }],
      }),
    ).toEqual({
      has_red_flags: true,
      flags: [{ category: "weakness", message: "new weakness" }],
    });
  });

  it("rejects invalid shared payloads instead of casting them", () => {
    expect(() => parseInteractionQuestion({ answer_type: "text" })).toThrow(
      StreamEventParseError,
    );
    expect(() => parseExtractedInfo({ symptom_type: "pain" })).toThrow(
      StreamEventParseError,
    );
    expect(() => parseCitation({})).toThrow(StreamEventParseError);
    expect(() => parseRedFlag({ category: "weakness" })).toThrow(
      StreamEventParseError,
    );
    expect(() =>
      parseRedFlagEvent({
        has_red_flags: true,
        flags: [{ category: "weakness" }],
      }),
    ).toThrow(StreamEventParseError);
  });
});
