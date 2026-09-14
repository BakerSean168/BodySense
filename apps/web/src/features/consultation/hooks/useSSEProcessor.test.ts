import { describe, expect, it, vi } from "vitest";
import { parseStreamEvent } from "@bodysense/contracts";
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

  it("rejects schema-invalid live SSE before dispatch", () => {
    const onTextDelta = vi.fn();
    const onError = vi.fn();
    const handlers: SSEHandlers = { onTextDelta, onError };
    const state = { currentEvent: "", maxSeq: 0 };

    processSSELine("event: message.text.delta", state, handlers);
    processSSELine(
      'data: {"version":1,"seq":1,"channel":"message","type":"message.text.delta","ids":{},"payload":{}}',
      state,
      handlers,
    );

    expect(onTextDelta).not.toHaveBeenCalled();
    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError.mock.calls[0]?.[0]).toBeInstanceOf(Error);
    expect((onError.mock.calls[0]?.[0] as Error).message).toContain(
      "canonical v1 schema",
    );
    expect(state.maxSeq).toBe(0);
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

  const replayEvent = (input: unknown) => parseStreamEvent(input);

  it("dispatches interaction expiry through the live/replay handler map", () => {
    const onInteractionExpired = vi.fn();
    const state = dispatchReplayEvents(
      [
        replayEvent({
          version: 1,
          seq: 8,
          channel: "state",
          type: "state.interaction.expired",
          ids: { interaction_id: "interaction-1" },
          payload: {
            interaction_id: "interaction-1",
            expired_at: "2026-09-13T12:00:00Z",
            reason: "ttl_elapsed",
          },
        }),
      ],
      { onInteractionExpired },
    );

    expect(onInteractionExpired).toHaveBeenCalledTimes(1);
    expect(state.maxSeq).toBe(8);
  });

  it("dispatches run.cancelled through the live/replay handler map", () => {
    const onRunCancelled = vi.fn();
    const state = dispatchReplayEvents(
      [
        replayEvent({
          version: 1,
          seq: 9,
          channel: "run",
          type: "run.cancelled",
          ids: {
            conversation_id: "conversation-1",
            run_id: "run-1",
            turn_id: "turn-1",
          },
          payload: { status: "cancelled", reason: "cancelled_by_user" },
        }),
      ],
      { onRunCancelled },
    );

    expect(onRunCancelled).toHaveBeenCalledTimes(1);
    expect(state.maxSeq).toBe(9);
  });

  it("dispatchReplayEvents skips already-seen seq and advances maxSeq", () => {
    const onTextDelta = vi.fn();
    const handlers: SSEHandlers = { onTextDelta };
    const state = dispatchReplayEvents(
      [
        replayEvent({
          version: 1,
          seq: 3,
          channel: "message",
          type: "message.text.delta",
          ids: {},
          payload: { delta: "old" },
        }),
        replayEvent({
          version: 1,
          seq: 4,
          channel: "message",
          type: "message.text.delta",
          ids: {},
          payload: { delta: "new" },
        }),
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
