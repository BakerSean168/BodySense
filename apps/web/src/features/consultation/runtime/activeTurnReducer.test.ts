/**
 * Tests for ActiveTurnReducer — map-based state with upsert semantics.
 */

import { describe, it, expect } from "vitest";
import {
  reduceActiveTurnEvent,
  resetActiveTurnState,
  INITIAL_ACTIVE_TURN_STATE,
  type ActiveTurnState,
} from "./activeTurnReducer";
import type { StreamEvent } from "../types/consultation";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

let _seq = 0;

function makeEvent(
  type: string,
  payload: Record<string, unknown> = {},
  ids: Record<string, string> = {},
  channel = "message",
): StreamEvent {
  return {
    version: 1,
    seq: ++_seq,
    channel: channel as StreamEvent["channel"],
    type,
    ids: {
      conversation_id: ids.conversation_id || "conv-1",
      run_id: ids.run_id || "run-1",
      turn_id: ids.turn_id || "turn-1",
      message_id: ids.message_id || "msg-1",
      tool_call_id: ids.tool_call_id || null,
    },
    payload,
  } as StreamEvent;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("ActiveTurnReducer", () => {
  describe("aggregation", () => {
    it("aggregates text, tools, citations, and interaction into one state", () => {
      const { state: s1 } = reduceActiveTurnEvent(
        INITIAL_ACTIVE_TURN_STATE,
        makeEvent("conversation.created", {}, {}, "conversation"),
      );

      const { state: s2 } = reduceActiveTurnEvent(
        s1,
        makeEvent("message.text.delta", { delta: "Hello" }),
      );

      const { state: s3 } = reduceActiveTurnEvent(
        s2,
        makeEvent(
          "tool.call",
          { tool: "search_knowledge", args: {} },
          { tool_call_id: "tc-1" },
          "tool",
        ),
      );

      const { state: s4 } = reduceActiveTurnEvent(
        s3,
        makeEvent(
          "source.citation.added",
          { citation: { title: "Guide" } },
          {},
          "source",
        ),
      );

      const { state: s5 } = reduceActiveTurnEvent(
        s4,
        makeEvent(
          "state.interaction.required",
          {
            interaction_id: "int-1",
            created_at: "2026-08-23T00:00:00Z",
            question: {
              question: "Age?",
              answer_type: "number",
              required: true,
            },
          },
          {},
          "state",
        ),
      );

      expect(s5.text).toBe("Hello");
      expect(s5.toolCallsById["tc-1"]).toBeDefined();
      expect(s5.toolCallsById["tc-1"].tool).toBe("search_knowledge");
      expect(s5.toolCallsById["tc-1"].status).toBe("running");
      expect(s5.citationsByKey["Guide"]).toBeDefined();
      expect(s5.citationsByKey["Guide"].title).toBe("Guide");
      expect(s5.pendingInteraction).not.toBeNull();
      expect(s5.pendingInteraction!.id).toBe("int-1");
      expect(s5.status).toBe("interrupted");
    });
  });

  describe("message.completed", () => {
    it("builds finalParts on message.completed", () => {
      let state: ActiveTurnState = { ...INITIAL_ACTIVE_TURN_STATE };

      const { state: s1 } = reduceActiveTurnEvent(
        state,
        makeEvent("message.text.delta", { delta: "Hello" }),
      );
      state = s1;

      const { state: s2 } = reduceActiveTurnEvent(
        state,
        makeEvent(
          "source.citation.added",
          {
            citation: { title: "Guide", summary: "A helpful guide" },
          },
          {},
          "source",
        ),
      );
      state = s2;

      const { state: s3 } = reduceActiveTurnEvent(
        state,
        makeEvent(
          "safety.red_flag.detected",
          {
            has_red_flags: true,
            flags: [
              {
                category: "emergency",
                message: "Seek care",
                matched_text: "",
                source: "",
              },
            ],
          },
          {},
          "safety",
        ),
      );
      state = s3;

      const { state: s4 } = reduceActiveTurnEvent(
        state,
        makeEvent("message.completed", {}),
      );

      expect(s4.finalParts).toHaveLength(3);
      expect(s4.finalParts[0]).toEqual({ type: "text", text: "Hello" });
      expect(s4.finalParts[1]).toMatchObject({
        type: "source",
        sourceType: "url",
        title: "Guide",
      });
      expect(s4.finalParts[2]).toMatchObject({
        type: "data",
        name: "red_flag",
      });
      expect(s4.status).toBe("completed");
    });
  });

  describe("reset", () => {
    it("resets state after completion", () => {
      const { state: s1 } = reduceActiveTurnEvent(
        INITIAL_ACTIVE_TURN_STATE,
        makeEvent("conversation.created", {}, {}, "conversation"),
      );

      const { state: s2 } = reduceActiveTurnEvent(
        s1,
        makeEvent("message.text.delta", { delta: "Hello" }),
      );

      reduceActiveTurnEvent(s2, makeEvent("message.completed", {}));

      const resetState = resetActiveTurnState();
      expect(resetState).toEqual(INITIAL_ACTIVE_TURN_STATE);
    });
  });

  describe("state.interaction.required", () => {
    it("keeps interrupted state on interaction.required", () => {
      const { state: s1 } = reduceActiveTurnEvent(
        INITIAL_ACTIVE_TURN_STATE,
        makeEvent("conversation.created", {}, {}, "conversation"),
      );

      const { state: s2 } = reduceActiveTurnEvent(
        s1,
        makeEvent(
          "state.interaction.required",
          {
            interaction_id: "int-2",
            created_at: "2026-08-23T00:00:00Z",
            question: {
              question: "Pain level?",
              answer_type: "number",
              required: true,
            },
          },
          {},
          "state",
        ),
      );

      expect(s2.status).toBe("interrupted");
      expect(s2.pendingInteraction).not.toBeNull();
      expect(s2.pendingInteraction!.id).toBe("int-2");
      expect(s2.pendingInteraction!.status).toBe("pending");
    });

    it("preserves prior text when interaction event arrives", () => {
      const stateWithText: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        text: "some text",
      };

      const { state } = reduceActiveTurnEvent(
        stateWithText,
        makeEvent(
          "state.interaction.required",
          {
            interaction_id: "int-3",
            created_at: "2026-08-23T00:00:00Z",
            question: {
              question: "More info?",
              answer_type: "text",
              required: false,
            },
          },
          {},
          "state",
        ),
      );

      expect(state.text).toBe("some text");
      expect(state.pendingInteraction).not.toBeNull();
    });

    it("does not finalize an interrupted turn on stream.done", () => {
      const interrupted: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        status: "interrupted",
        text: "请先补充年龄信息。",
        pendingInteraction: {
          id: "int-1",
          run_id: "run-1",
          conversation_id: "conv-1",
          tool_call_id: "tc-1",
          tool_name: "ask_user",
          question: {
            question: "你的年龄是多少？",
            answer_type: "number",
            required: true,
          },
          status: "pending",
          created_at: "2026-08-23T00:00:00Z",
        },
      };

      const { state } = reduceActiveTurnEvent(
        interrupted,
        makeEvent("stream.done", {}, {}, "stream"),
      );

      expect(state.status).toBe("interrupted");
      expect(state.finalParts).toEqual([]);
      expect(state.pendingInteraction?.status).toBe("pending");
    });
  });

  describe("tool.call", () => {
    it("ignores duplicate tool.call by tool_call_id", () => {
      const stateWithTool: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        toolCallsById: {
          "tc-1": {
            id: "tc-1",
            tool: "search_knowledge",
            args: { query: "back pain" },
            status: "running",
          },
        },
      };

      const { state } = reduceActiveTurnEvent(
        stateWithTool,
        makeEvent(
          "tool.call",
          { tool: "search_knowledge", args: { query: "back pain" } },
          { tool_call_id: "tc-1" },
          "tool",
        ),
      );

      expect(Object.keys(state.toolCallsById)).toHaveLength(1);
      expect(state.toolCallsById["tc-1"]).toBeDefined();
    });
  });

  describe("tool.result", () => {
    it("creates placeholder when no prior tool.call", () => {
      const { state } = reduceActiveTurnEvent(
        INITIAL_ACTIVE_TURN_STATE,
        makeEvent(
          "tool.result",
          { tool: "search_knowledge", result: { found: 3 } },
          { tool_call_id: "tc-new" },
          "tool",
        ),
      );

      const entries = Object.values(state.toolCallsById);
      expect(entries).toHaveLength(1);
      expect(entries[0].id).toBe("tc-new");
      expect(entries[0].status).toBe("completed");
      expect(entries[0].tool).toBe("search_knowledge");
    });

    it("completes existing tool.call by exact tool_call_id", () => {
      const stateWithRunning: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        toolCallsById: {
          "tc-1": {
            id: "tc-1",
            tool: "search_knowledge",
            args: { query: "back pain" },
            status: "running",
          },
        },
      };

      const { state } = reduceActiveTurnEvent(
        stateWithRunning,
        makeEvent(
          "tool.result",
          { tool: "search_knowledge", result: { found: 3 } },
          { tool_call_id: "tc-1" },
          "tool",
        ),
      );

      expect(state.toolCallsById["tc-1"].status).toBe("completed");
      expect(state.toolCallsById["tc-1"].result).toEqual({ found: 3 });
    });

    it("does not change already-completed tool.result", () => {
      const stateWithCompleted: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        toolCallsById: {
          "tc-1": {
            id: "tc-1",
            tool: "search_knowledge",
            args: { query: "back pain" },
            status: "completed",
            result: { found: 3 },
          },
        },
      };

      const { state } = reduceActiveTurnEvent(
        stateWithCompleted,
        makeEvent(
          "tool.result",
          { tool: "search_knowledge", result: { found: 5 } },
          { tool_call_id: "tc-1" },
          "tool",
        ),
      );

      // Should keep original result
      expect(state.toolCallsById["tc-1"].result).toEqual({ found: 3 });
    });

    it("does not guess when multiple same-name tool calls are still running", () => {
      const stateWithAmbiguousRunningCalls: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        toolCallsById: {
          "tc-1": {
            id: "tc-1",
            tool: "search_knowledge",
            args: { query: "A" },
            status: "running",
          },
          "tc-2": {
            id: "tc-2",
            tool: "search_knowledge",
            args: { query: "B" },
            status: "running",
          },
        },
      };

      const { state } = reduceActiveTurnEvent(
        stateWithAmbiguousRunningCalls,
        makeEvent(
          "tool.result",
          { tool: "search_knowledge", result: { found: 5 } },
          {},
          "tool",
        ),
      );

      expect(state.toolCallsById["tc-1"].status).toBe("running");
      expect(state.toolCallsById["tc-2"].status).toBe("running");
      expect(Object.keys(state.toolCallsById)).toHaveLength(3);
    });
  });

  describe("run-aware seq guard", () => {
    it("ignores any stale public event within the same run", () => {
      const current: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        runId: "run-1",
        sequenceRunId: "run-1",
        lastSeq: 10,
      };

      const { state } = reduceActiveTurnEvent(current, {
        ...makeEvent("message.text.delta", { delta: "stale" }, { run_id: "run-1" }),
        seq: 5,
      });
      expect(state).toBe(current);
    });

    it("processes a newer public event and advances the canonical seq", () => {
      const current: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        runId: "run-1",
        sequenceRunId: "run-1",
        lastSeq: 10,
      };

      const { state } = reduceActiveTurnEvent(current, {
        ...makeEvent("message.text.delta", { delta: "fresh" }, { run_id: "run-1" }),
        seq: 15,
      });
      expect(state.text).toBe("fresh");
      expect(state.lastSeq).toBe(15);
    });

    it("rejects lower seq even when the event type differs", () => {
      const current: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        runId: "run-1",
        sequenceRunId: "run-1",
        lastSeq: 3,
      };
      const { state } = reduceActiveTurnEvent(current, {
        ...makeEvent("source.citation.added", { citation: { title: "Guide" } }, { run_id: "run-1" }, "source"),
        seq: 1,
      });
      expect(state).toBe(current);
      expect(state.citationsByKey.Guide).toBeUndefined();
    });

    it("accepts seq=1 when a new run starts", () => {
      const current: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        runId: "run-old",
        sequenceRunId: "run-old",
        lastSeq: 99,
      };
      const { state } = reduceActiveTurnEvent(current, {
        ...makeEvent("run.resumed", { status: "running", interaction_id: "i" }, { run_id: "run-new" }, "run"),
        seq: 1,
      });
      expect(state.runId).toBe("run-new");
      expect(state.sequenceRunId).toBe("run-new");
      expect(state.lastSeq).toBe(1);
    });
  });

  describe("stream.error and message.failed", () => {
    it("sets status to failed on stream.error", () => {
      const { state, effects } = reduceActiveTurnEvent(
        { ...INITIAL_ACTIVE_TURN_STATE, status: "streaming" },
        makeEvent("stream.error", { message: "connection lost" }),
      );

      expect(state.status).toBe("failed");
      expect(state.error).toBe("connection lost");
      expect(effects).toHaveLength(1);
      expect(effects[0].type).toBe("stream_error");
    });

    it("sets status to failed on message.failed", () => {
      const { state } = reduceActiveTurnEvent(
        { ...INITIAL_ACTIVE_TURN_STATE, status: "streaming" },
        makeEvent("message.failed", {
          error: { message: "generation failed" },
        }),
      );

      expect(state.status).toBe("failed");
      expect(state.error).toBe("generation failed");
    });
  });

  describe("interrupt/resume lifecycle", () => {
    it("keeps the turn interrupted until answer and clears it on run.resumed", () => {
      const { state: required } = reduceActiveTurnEvent(
        INITIAL_ACTIVE_TURN_STATE,
        makeEvent(
          "state.interaction.required",
          {
            interaction_id: "int-1",
            created_at: "2026-08-23T00:00:00Z",
            question: {
              question: "Age?",
              answer_type: "number",
              required: true,
            },
          },
          { interaction_id: "int-1", tool_call_id: "tc-ask" },
          "state",
        ),
      );

      const { state: interrupted } = reduceActiveTurnEvent(
        required,
        makeEvent(
          "run.interrupted",
          { status: "waiting_user", interaction_id: "int-1" },
          { interaction_id: "int-1" },
          "run",
        ),
      );

      const { state: answered, effects } = reduceActiveTurnEvent(
        interrupted,
        makeEvent(
          "state.interaction.answered",
          { interaction_id: "int-1", answer: { text: "35" } },
          { interaction_id: "int-1" },
          "state",
        ),
      );

      const { state: resumed } = reduceActiveTurnEvent(
        answered,
        makeEvent(
          "run.resumed",
          { status: "running", interaction_id: "int-1" },
          { interaction_id: "int-1" },
          "run",
        ),
      );

      expect(interrupted.status).toBe("interrupted");
      expect(answered.pendingInteraction?.status).toBe("answered");
      expect(effects[0].type).toBe("interaction_answered");
      expect(resumed.status).toBe("streaming");
      expect(resumed.pendingInteraction).toBeNull();
    });
  });
  describe("state.interaction.answered", () => {
    it("updates pendingInteraction status to answered", () => {
      const stateWithInteraction: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        pendingInteraction: {
          id: "int-1",
          run_id: "run-1",
          conversation_id: "conv-1",
          tool_call_id: "tc-1",
          tool_name: "ask_user",
          question: { question: "Age?", answer_type: "number", required: true },
          status: "pending",
          created_at: "2026-08-23T00:00:00Z",
        },
      };

      const { state, effects } = reduceActiveTurnEvent(
        stateWithInteraction,
        makeEvent(
          "state.interaction.answered",
          { interaction_id: "int-1" },
          {},
          "state",
        ),
      );

      expect(state.pendingInteraction?.status).toBe("answered");
      expect(effects).toHaveLength(1);
      expect(effects[0].type).toBe("interaction_answered");
    });
  });

  describe("source.knowledge_gap", () => {
    it("deduplicates by query", () => {
      const { state: s1 } = reduceActiveTurnEvent(
        INITIAL_ACTIVE_TURN_STATE,
        makeEvent(
          "source.knowledge_gap",
          { query: "posture", message: "Not found" },
          {},
          "source",
        ),
      );

      const { state: s2 } = reduceActiveTurnEvent(
        s1,
        makeEvent(
          "source.knowledge_gap",
          { query: "posture", message: "Still not found" },
          {},
          "source",
        ),
      );

      expect(Object.keys(s2.knowledgeGapsByKey)).toHaveLength(1);
      // Should keep the first entry's message
      expect(s2.knowledgeGapsByKey["posture"].message).toBe("Not found");
    });
  });

  describe("state.phase.changed", () => {
    it("emits phase_changed effect", () => {
      const { effects } = reduceActiveTurnEvent(
        INITIAL_ACTIVE_TURN_STATE,
        makeEvent(
          "state.phase.changed",
          { from: "collecting", to: "analyzing" },
          {},
          "state",
        ),
      );

      expect(effects).toHaveLength(1);
      expect(effects[0].type).toBe("phase_changed");
      expect(effects[0]).toMatchObject({ from: "collecting", to: "analyzing" });
    });
  });
});


describe("ActiveTurnReducer deterministic replay", () => {
  it("produces deep-equal state for identical public history", () => {
    const history = [
      makeEvent("run.started", { status: "running", source: "start_turn" }, { run_id: "run-det" }, "run"),
      makeEvent("message.created", { role: "assistant", status: "streaming" }, { run_id: "run-det", message_id: "m-det" }, "message"),
      makeEvent("tool.call", { tool: "search_knowledge", args: { query: "hip" } }, { run_id: "run-det" }, "tool"),
      makeEvent("state.interaction.required", {
        interaction_id: "int-det",
        created_at: "2026-08-23T00:00:00Z",
        question: { question: "Where?", answer_type: "text" },
      }, { run_id: "run-det" }, "state"),
    ];

    const replay = () => history.reduce(
      (state, event) => reduceActiveTurnEvent(state, event).state,
      resetActiveTurnState(),
    );
    expect(replay()).toEqual(replay());
  });

  it("resets sequence comparison when run identity changes", () => {
    const run1 = makeEvent("run.started", { status: "running", source: "start_turn" }, { run_id: "run-a" }, "run");
    const first = reduceActiveTurnEvent(resetActiveTurnState(), run1).state;
    const run2: StreamEvent = {
      ...makeEvent("run.resumed", { status: "running", interaction_id: "i" }, { run_id: "run-b" }, "run"),
      seq: 1,
    } as StreamEvent;
    const second = reduceActiveTurnEvent(first, run2).state;
    expect(second.runId).toBe("run-b");
    expect(second.sequenceRunId).toBe("run-b");
    expect(second.lastSeq).toBe(1);
  });

  it("keeps cancelled terminal state after stream.done", () => {
    const cancelled = reduceActiveTurnEvent(
      resetActiveTurnState(),
      makeEvent("run.cancelled", { status: "cancelled", reason: "cancelled_by_user" }, { run_id: "run-c" }, "run"),
    ).state;
    const done = reduceActiveTurnEvent(cancelled, makeEvent("stream.done", {}, { run_id: "run-c" }, "stream")).state;
    expect(done.status).toBe("cancelled");
  });
});
