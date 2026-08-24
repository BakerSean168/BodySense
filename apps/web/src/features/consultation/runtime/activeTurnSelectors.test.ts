/**
 * Tests for ActiveTurnSelectors — view model derivation from ActiveTurnState.
 */

import { describe, it, expect } from "vitest";
import {
  selectActiveTurnViewModel,
  selectVisibleToolCalls,
  selectStreamingMarkdown,
  selectIsInterrupted,
  selectIsComposerLocked,
  selectHasVisibleContent,
  selectHasRenderableContent,
  shouldApplyInitialActiveTurn,
} from "./activeTurnSelectors";
import {
  INITIAL_ACTIVE_TURN_STATE,
  type ActiveTurnState,
} from "./activeTurnReducer";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function stateWith(overrides: Partial<ActiveTurnState> = {}): ActiveTurnState {
  return { ...INITIAL_ACTIVE_TURN_STATE, ...overrides };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("activeTurnSelectors", () => {
  describe("selectVisibleToolCalls", () => {
    it("returns empty array for empty state", () => {
      expect(selectVisibleToolCalls(INITIAL_ACTIVE_TURN_STATE)).toEqual([]);
    });

    it("filters out ask_user tools", () => {
      const state = stateWith({
        toolCallsById: {
          "tc-1": {
            id: "tc-1",
            tool: "search_knowledge",
            args: { query: "back" },
            status: "running",
          },
          "tc-2": {
            id: "tc-2",
            tool: "ask_user",
            args: { question: "age?" },
            status: "running",
          },
        },
      });
      const result = selectVisibleToolCalls(state);
      expect(result).toHaveLength(1);
      expect(result[0].id).toBe("tc-1");
    });

    it("sorts running tools before completed", () => {
      const state = stateWith({
        toolCallsById: {
          "tc-a": {
            id: "tc-a",
            tool: "search_knowledge",
            args: { query: "a" },
            status: "completed",
            result: {},
          },
          "tc-b": {
            id: "tc-b",
            tool: "search_knowledge",
            args: { query: "b" },
            status: "running",
          },
          "tc-c": {
            id: "tc-c",
            tool: "extract_symptom_info",
            args: { body_part: "c" },
            status: "completed",
            result: {},
          },
        },
      });
      const result = selectVisibleToolCalls(state);
      expect(result).toHaveLength(3);
      expect(result[0].status).toBe("running");
      expect(result[1].status).toBe("completed");
      expect(result[2].status).toBe("completed");
    });
  });

  describe("selectStreamingMarkdown", () => {
    it("returns the text field", () => {
      const state = stateWith({ text: "# Hello\n\nWorld" });
      expect(selectStreamingMarkdown(state)).toBe("# Hello\n\nWorld");
    });
  });

  describe("selectIsInterrupted", () => {
    it("returns true when status is interrupted", () => {
      expect(selectIsInterrupted(stateWith({ status: "interrupted" }))).toBe(
        true,
      );
    });

    it("returns false when status is streaming", () => {
      expect(selectIsInterrupted(stateWith({ status: "streaming" }))).toBe(
        false,
      );
    });
  });

  describe("selectIsComposerLocked", () => {
    it("returns true while streaming", () => {
      expect(selectIsComposerLocked(stateWith({ status: "streaming" }))).toBe(
        true,
      );
    });

    it("returns true while interrupted", () => {
      expect(selectIsComposerLocked(stateWith({ status: "interrupted" }))).toBe(
        true,
      );
    });

    it("returns false after the turn is completed", () => {
      expect(selectIsComposerLocked(stateWith({ status: "completed" }))).toBe(
        false,
      );
    });
  });

  describe("selectHasVisibleContent", () => {
    it("returns false for empty state", () => {
      expect(selectHasVisibleContent(INITIAL_ACTIVE_TURN_STATE)).toBe(false);
    });

    it("returns true when text is non-empty", () => {
      expect(selectHasVisibleContent(stateWith({ text: "Hello" }))).toBe(true);
    });

    it("returns true when tool calls exist", () => {
      expect(
        selectHasVisibleContent(
          stateWith({
            toolCallsById: {
              "tc-1": {
                id: "tc-1",
                tool: "search",
                args: {},
                status: "running",
              },
            },
          }),
        ),
      ).toBe(true);
    });

    it("returns true for a pending interaction even without bubble content", () => {
      expect(
        selectHasVisibleContent(
          stateWith({
            pendingInteraction: {
              id: "ia-1",
              run_id: "run-1",
              conversation_id: "conv-1",
              tool_call_id: "tc-1",
              tool_name: "ask_user",
              question: {
                question: "是否有颈肩不适？",
                answer_type: "single_choice",
                options: ["有", "无"],
              },
              status: "pending",
              created_at: "2026-07-06T00:00:00Z",
            },
          }),
        ),
      ).toBe(true);
    });
  });

  describe("selectHasRenderableContent", () => {
    it("returns false for ask_user-only active turns", () => {
      expect(
        selectHasRenderableContent(
          stateWith({
            pendingInteraction: {
              id: "ia-1",
              run_id: "run-1",
              conversation_id: "conv-1",
              tool_call_id: "tc-1",
              tool_name: "ask_user",
              question: {
                question: "是否有颈肩不适？",
                answer_type: "single_choice",
                options: ["有", "无"],
              },
              status: "pending",
              created_at: "2026-07-06T00:00:00Z",
            },
            toolCallsById: {
              "tc-1": {
                id: "tc-1",
                tool: "ask_user",
                args: { question: "是否有颈肩不适？" },
                status: "running",
              },
            },
          }),
        ),
      ).toBe(false);
    });
  });

  describe("shouldApplyInitialActiveTurn", () => {
    it("keeps a locally recovered failed terminal when the server active seed disappears", () => {
      const current = stateWith({
        runId: "run-1",
        sequenceRunId: "run-1",
        lastSeq: 5,
        status: "failed",
      });

      expect(shouldApplyInitialActiveTurn(current, null)).toBe(false);
    });

    it("rejects an older server seed for the same run", () => {
      const current = stateWith({
        runId: "run-1",
        sequenceRunId: "run-1",
        lastSeq: 5,
        status: "failed",
      });
      const stale = stateWith({
        runId: "run-1",
        sequenceRunId: "run-1",
        lastSeq: 4,
        status: "streaming",
      });

      expect(shouldApplyInitialActiveTurn(current, stale)).toBe(false);
    });

    it("accepts a newer seed or a different run", () => {
      const current = stateWith({
        runId: "run-1",
        sequenceRunId: "run-1",
        lastSeq: 5,
        status: "failed",
      });
      const newer = stateWith({
        runId: "run-1",
        sequenceRunId: "run-1",
        lastSeq: 6,
        status: "streaming",
      });
      const otherRun = stateWith({
        runId: "run-2",
        sequenceRunId: "run-2",
        lastSeq: 1,
        status: "streaming",
      });

      expect(shouldApplyInitialActiveTurn(current, newer)).toBe(true);
      expect(shouldApplyInitialActiveTurn(current, otherRun)).toBe(true);
    });
  });

  describe("selectActiveTurnViewModel", () => {
    it("combines all selectors into a single view model", () => {
      const state = stateWith({
        text: "Hello",
        status: "streaming",
        toolCallsById: {
          "tc-1": {
            id: "tc-1",
            tool: "search_knowledge",
            args: { query: "x" },
            status: "running",
          },
        },
        citationsByKey: {
          Guide: { title: "Guide", summary: "A guide" },
        },
        knowledgeGapsByKey: {
          posture: { query: "posture", message: "Not found" },
        },
      });

      const vm = selectActiveTurnViewModel(state);

      expect(vm.streamingMarkdown).toBe("Hello");
      expect(vm.toolCalls).toHaveLength(1);
      expect(vm.citations).toHaveLength(1);
      expect(vm.knowledgeGaps).toHaveLength(1);
      expect(vm.isRunning).toBe(true);
      expect(vm.isInterrupted).toBe(false);
      expect(vm.hasVisibleContent).toBe(true);
    });
  });
});
