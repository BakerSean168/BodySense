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
import { makeEvent } from "./activeTurnReducer.testSupport";

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

  describe("run-aware seq guard", () => {
    it("ignores any stale public event within the same run", () => {
      const current: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        runId: "run-1",
        sequenceRunId: "run-1",
        lastSeq: 10,
      };

      const { state } = reduceActiveTurnEvent(current, {
        ...makeEvent(
          "message.text.delta",
          { delta: "stale" },
          { run_id: "run-1" },
        ),
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
        ...makeEvent(
          "message.text.delta",
          { delta: "fresh" },
          { run_id: "run-1" },
        ),
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
        ...makeEvent(
          "source.citation.added",
          { citation: { title: "Guide" } },
          { run_id: "run-1" },
          "source",
        ),
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
        ...makeEvent(
          "run.resumed",
          { status: "running", interaction_id: "i" },
          { run_id: "run-new" },
          "run",
        ),
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

    it("keeps execution_lost user copy when message.failed follows run.failed", () => {
      const started = reduceActiveTurnEvent(
        INITIAL_ACTIVE_TURN_STATE,
        makeEvent(
          "run.started",
          { status: "running" },
          { run_id: "run-lost-message" },
          "run",
        ),
      ).state;
      const runFailed = reduceActiveTurnEvent(
        started,
        makeEvent(
          "run.failed",
          { status: "failed", reason: "execution_lost" },
          { run_id: "run-lost-message" },
          "run",
        ),
      ).state;
      const messageFailed = reduceActiveTurnEvent(
        runFailed,
        makeEvent(
          "message.failed",
          {
            status: "failed",
            error: {
              code: "execution_lost",
              message: "run execution lost; lease expired",
            },
          },
          { run_id: "run-lost-message", message_id: "msg-lost" },
          "message",
        ),
      ).state;

      expect(messageFailed.status).toBe("failed");
      expect(messageFailed.error).toContain("系统已安全回收");
      expect(messageFailed.error).not.toContain("lease expired");
    });
  });

  describe("explicit no-op public events", () => {
    it("advances the sequence watermark for validated events with no UI projection", () => {
      const current: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        runId: "run-1",
        sequenceRunId: "run-1",
        lastSeq: 10,
      };
      const event = {
        ...makeEvent(
          "usage.reported",
          { usage: { input_tokens: 10 } },
          { run_id: "run-1" },
          "usage",
        ),
        seq: 11,
      };

      const { state, effects } = reduceActiveTurnEvent(current, event);

      expect(state.lastSeq).toBe(11);
      expect(state.sequenceRunId).toBe("run-1");
      expect(effects).toEqual([]);
    });
  });
});
