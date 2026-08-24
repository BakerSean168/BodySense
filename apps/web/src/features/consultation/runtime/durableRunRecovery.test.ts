import { describe, expect, it, vi } from "vitest";
import type { StreamEvent } from "../types/consultation";
import { recoverDurableRunEvents } from "./durableRunRecovery";

function event(
  seq: number,
  type: StreamEvent["type"],
  payload: StreamEvent["payload"],
): StreamEvent {
  return {
    version: 1,
    seq,
    channel: type.startsWith("stream.")
      ? "stream"
      : type.startsWith("run.")
        ? "run"
        : "message",
    type,
    ids: {
      conversation_id: "conversation-1",
      run_id: "run-1",
      turn_id: "turn-1",
      message_id: "message-1",
      tool_call_id: null,
      interaction_id: null,
    },
    payload,
  } as StreamEvent;
}

describe("recoverDurableRunEvents", () => {
  it("keeps polling empty pages until a persisted terminal event arrives", async () => {
    const requestedAfterSeq: number[] = [];
    const pages = [
      { events: [], hasMore: false, nextAfterSeq: null },
      {
        events: [event(4, "message.text.delta", { delta: "hello" })],
        hasMore: false,
        nextAfterSeq: 4,
      },
      {
        events: [event(5, "stream.done", {})],
        hasMore: false,
        nextAfterSeq: 5,
      },
    ];
    let now = 0;
    const onTextDelta = vi.fn();
    const onDone = vi.fn();

    const result = await recoverDurableRunEvents({
      afterSeq: 3,
      fetchPage: async (afterSeq) => {
        requestedAfterSeq.push(afterSeq);
        return (
          pages.shift() ?? { events: [], hasMore: false, nextAfterSeq: null }
        );
      },
      handlers: { onTextDelta, onDone },
      timeoutMs: 1_000,
      pollIntervalMs: 10,
      now: () => now,
      sleep: async (ms) => {
        now += ms;
      },
    });

    expect(requestedAfterSeq).toEqual([3, 3, 4]);
    expect(onTextDelta).toHaveBeenCalledWith(
      expect.objectContaining({ seq: 4, payload: { delta: "hello" } }),
    );
    expect(onDone).toHaveBeenCalledWith(
      expect.objectContaining({ seq: 5, type: "stream.done" }),
    );
    expect(result).toEqual({ maxSeq: 5, terminalType: "stream.done" });
  });

  it("retries transient event-log failures during an API restart", async () => {
    let now = 0;
    let attempts = 0;
    const onRunFailed = vi.fn();
    const result = await recoverDurableRunEvents({
      fetchPage: async () => {
        attempts += 1;
        if (attempts < 3) {
          throw new Error("connection refused");
        }
        return {
          events: [
            event(4, "run.failed", {
              status: "failed",
              reason: "execution_lost",
            }),
          ],
          hasMore: false,
          nextAfterSeq: 4,
        };
      },
      handlers: { onRunFailed },
      timeoutMs: 1_000,
      pollIntervalMs: 100,
      now: () => now,
      sleep: async (ms) => {
        now += ms;
      },
    });

    expect(attempts).toBe(3);
    expect(result.terminalType).toBe("run.failed");
    expect(onRunFailed).toHaveBeenCalledTimes(1);
  });

  it("treats execution_lost run.failed as a durable terminal event", async () => {
    let now = 0;
    const onRunFailed = vi.fn();
    const result = await recoverDurableRunEvents({
      fetchPage: async () => ({
        events: [
          event(6, "run.failed", {
            status: "failed",
            reason: "execution_lost",
          }),
        ],
        hasMore: false,
        nextAfterSeq: 6,
      }),
      handlers: { onRunFailed },
      timeoutMs: 100,
      now: () => now,
      sleep: async (ms) => {
        now += ms;
      },
    });

    expect(onRunFailed).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "run.failed",
        payload: { status: "failed", reason: "execution_lost" },
      }),
    );
    expect(result).toEqual({ maxSeq: 6, terminalType: "run.failed" });
  });

  it("treats persisted run.cancelled as a durable terminal event", async () => {
    let now = 0;
    const result = await recoverDurableRunEvents({
      fetchPage: async () => ({
        events: [
          event(7, "run.cancelled", {
            status: "cancelled",
            reason: "cancelled_by_user",
          }),
        ],
        hasMore: false,
        nextAfterSeq: 7,
      }),
      handlers: {},
      timeoutMs: 100,
      now: () => now,
      sleep: async (ms) => { now += ms; },
    });
    expect(result).toEqual({ maxSeq: 7, terminalType: "run.cancelled" });
  });

  it("fails explicitly instead of treating an empty durable page as completion", async () => {
    let now = 0;
    await expect(
      recoverDurableRunEvents({
        fetchPage: async () => ({
          events: [],
          hasMore: false,
          nextAfterSeq: null,
        }),
        handlers: {},
        timeoutMs: 20,
        pollIntervalMs: 10,
        now: () => now,
        sleep: async (ms) => {
          now += ms;
        },
      }),
    ).rejects.toThrow(
      "恢复本次执行超时",
    );
  });
});
