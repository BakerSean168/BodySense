/**
 * Deterministic replay tests for ActiveTurnReducer.
 */

import { describe, it, expect } from "vitest";
import {
  reduceActiveTurnEvent,
  resetActiveTurnState,
  INITIAL_ACTIVE_TURN_STATE,
} from "./activeTurnReducer";
import { makeEvent } from "./activeTurnReducer.testSupport";
import type { StreamEvent } from "../types/consultation";

describe("ActiveTurnReducer deterministic replay", () => {
  it("produces deep-equal state for identical public history", () => {
    const history = [
      makeEvent(
        "run.started",
        { status: "running", source: "start_turn" },
        { run_id: "run-det" },
        "run",
      ),
      makeEvent(
        "message.created",
        { role: "assistant", status: "streaming" },
        { run_id: "run-det", message_id: "m-det" },
        "message",
      ),
      makeEvent(
        "tool.call",
        { tool: "search_knowledge", args: { query: "hip" } },
        { run_id: "run-det" },
        "tool",
      ),
      makeEvent(
        "state.interaction.required",
        {
          interaction_id: "int-det",
          created_at: "2026-08-23T00:00:00Z",
          question: { question: "Where?", answer_type: "text" },
        },
        { run_id: "run-det" },
        "state",
      ),
    ];

    const replay = () =>
      history.reduce(
        (state, event) => reduceActiveTurnEvent(state, event).state,
        resetActiveTurnState(),
      );
    expect(replay()).toEqual(replay());
  });

  it("resets sequence comparison when run identity changes", () => {
    const run1 = makeEvent(
      "run.started",
      { status: "running", source: "start_turn" },
      { run_id: "run-a" },
      "run",
    );
    const first = reduceActiveTurnEvent(resetActiveTurnState(), run1).state;
    const run2: StreamEvent = {
      ...makeEvent(
        "run.resumed",
        { status: "running", interaction_id: "i" },
        { run_id: "run-b" },
        "run",
      ),
      seq: 1,
    };
    const second = reduceActiveTurnEvent(first, run2).state;
    expect(second.runId).toBe("run-b");
    expect(second.sequenceRunId).toBe("run-b");
    expect(second.lastSeq).toBe(1);
  });

  it("keeps cancelled terminal state after stream.done", () => {
    const cancelled = reduceActiveTurnEvent(
      resetActiveTurnState(),
      makeEvent(
        "run.cancelled",
        { status: "cancelled", reason: "cancelled_by_user" },
        { run_id: "run-c" },
        "run",
      ),
    ).state;
    const done = reduceActiveTurnEvent(
      cancelled,
      makeEvent("stream.done", {}, { run_id: "run-c" }, "stream"),
    ).state;
    expect(done.status).toBe("cancelled");
  });
  it("surfaces execution_lost as a recoverable failed state", () => {
    const started = reduceActiveTurnEvent(INITIAL_ACTIVE_TURN_STATE, {
      ...makeEvent(
        "run.started",
        { status: "running", source: "start_turn" },
        {
          conversation_id: "conversation-1",
          run_id: "run-lost",
          turn_id: "turn-1",
        },
        "run",
      ),
      seq: 1,
    }).state;

    const result = reduceActiveTurnEvent(started, {
      ...makeEvent(
        "run.failed",
        { status: "failed", reason: "execution_lost" },
        {
          conversation_id: "conversation-1",
          run_id: "run-lost",
          turn_id: "turn-1",
        },
        "run",
      ),
      seq: 2,
    });

    expect(result.state.status).toBe("failed");
    expect(result.state.runId).toBe("run-lost");
    expect(result.state.error).toContain("安全回收");
    expect(result.state.pendingInteraction).toBeNull();
  });
});
