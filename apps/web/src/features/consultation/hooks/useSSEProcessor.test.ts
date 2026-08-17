import { describe, expect, it, vi } from "vitest";
import {
  processSSELine,
  dispatchReplayEvents,
  type SSEHandlers,
} from "./useSSEProcessor";

describe("processSSELine", () => {
  it("dispatches structured text delta events by envelope type", () => {
    const onTextDelta = vi.fn();
    const handlers: SSEHandlers = { onTextDelta };
    const state = { currentEvent: "", maxSeq: 0 };

    processSSELine("event: message.text.delta", state, handlers);
    processSSELine(
      'data: {"version":1,"seq":1,"channel":"message","type":"message.text.delta","ids":{"message_id":"m1"},"payload":{"delta":"hello"}}',
      state,
      handlers,
    );

    expect(onTextDelta).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "message.text.delta",
        payload: { delta: "hello" },
      }),
    );
  });

  it("dispatches stream.done as a structured event", () => {
    const onDone = vi.fn();
    const handlers: SSEHandlers = { onDone };
    const state = { currentEvent: "", maxSeq: 0 };

    processSSELine("event: stream.done", state, handlers);
    processSSELine(
      'data: {"version":1,"seq":2,"channel":"stream","type":"stream.done","ids":{},"payload":{}}',
      state,
      handlers,
    );

    expect(onDone).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "stream.done",
        payload: {},
      }),
    );
  });
});

describe("seq tracking and replay", () => {
  it("tracks maxSeq from structured envelopes", () => {
    const onTextDelta = vi.fn();
    const handlers: SSEHandlers = { onTextDelta };
    const state = { currentEvent: "", maxSeq: 0 };

    processSSELine("event: message.text.delta", state, handlers);
    processSSELine(
      'data: {"version":1,"seq":5,"channel":"message","type":"message.text.delta","ids":{},"payload":{"delta":"a"}}',
      state,
      handlers,
    );
    expect(state.maxSeq).toBe(5);
  });

  it("dispatchReplayEvents skips already-seen seq and advances maxSeq", () => {
    const onTextDelta = vi.fn();
    const handlers: SSEHandlers = { onTextDelta };
    const state = dispatchReplayEvents(
      [
        {
          version: 1,
          seq: 3,
          channel: "message",
          type: "message.text.delta",
          ids: {},
          payload: { delta: "old" },
        } as never,
        {
          version: 1,
          seq: 4,
          channel: "message",
          type: "message.text.delta",
          ids: {},
          payload: { delta: "new" },
        } as never,
      ],
      handlers,
      { currentEvent: "", maxSeq: 3 },
    );
    expect(onTextDelta).toHaveBeenCalledTimes(1);
    expect(onTextDelta).toHaveBeenCalledWith(
      expect.objectContaining({ seq: 4, payload: { delta: "new" } }),
    );
    expect(state.maxSeq).toBe(4);
  });
});
