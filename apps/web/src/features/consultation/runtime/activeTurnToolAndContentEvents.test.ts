/**
 * Tool and content event tests for ActiveTurnReducer.
 */

import { describe, it, expect } from "vitest";
import {
  reduceActiveTurnEvent,
  INITIAL_ACTIVE_TURN_STATE,
  type ActiveTurnState,
} from "./activeTurnReducer";
import { makeEvent } from "./activeTurnReducer.testSupport";

describe("ActiveTurnReducer", () => {
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
          { from: "collecting", to: "ready_for_analysis" },
          {},
          "state",
        ),
      );

      expect(effects).toHaveLength(1);
      expect(effects[0].type).toBe("phase_changed");
      expect(effects[0]).toMatchObject({
        from: "collecting",
        to: "ready_for_analysis",
      });
    });
  });
});
