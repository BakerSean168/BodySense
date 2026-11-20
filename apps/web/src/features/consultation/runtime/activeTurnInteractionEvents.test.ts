/**
 * Ask User interaction tests for ActiveTurnReducer.
 */

import { describe, it, expect } from "vitest";
import {
  reduceActiveTurnEvent,
  INITIAL_ACTIVE_TURN_STATE,
  type ActiveTurnState,
} from "./activeTurnReducer";
import { makeEvent } from "./activeTurnReducer.testSupport";

describe("ActiveTurnReducer", () => {
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
  describe("state.interaction.expired", () => {
    it("marks the matching pending interaction expired and unlocks the composer", () => {
      const current: ActiveTurnState = {
        ...INITIAL_ACTIVE_TURN_STATE,
        status: "interrupted",
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

      const { state } = reduceActiveTurnEvent(
        current,
        makeEvent(
          "state.interaction.expired",
          {
            interaction_id: "int-1",
            expired_at: "2026-08-23T00:05:00Z",
            reason: "ttl_elapsed",
          },
          { interaction_id: "int-1" },
          "state",
        ),
      );

      expect(state.pendingInteraction?.status).toBe("expired");
      expect(state.status).toBe("failed");
      expect(state.error).toBe("ttl_elapsed");
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
});

describe("same-Run Ask User continuation", () => {
  it("starts a new response segment without copying prior text or tools", () => {
    let state = INITIAL_ACTIVE_TURN_STATE;
    for (const event of [
      makeEvent("message.created", {}, { message_id: "old-response" }),
      makeEvent("message.text.delta", { delta: "Old response" }),
      makeEvent(
        "tool.call",
        { tool: "search_knowledge", args: {} },
        { tool_call_id: "old-tool" },
      ),
      makeEvent("run.interrupted"),
      makeEvent("stream.done"),
      makeEvent("run.resumed"),
      makeEvent("message.created", {}, { message_id: "new-response" }),
      makeEvent("message.text.delta", { delta: "New response" }),
    ])
      state = reduceActiveTurnEvent(state, event).state;
    expect(state.runId).toBe("run-1");
    expect(state.assistantMessageId).toBe("new-response");
    expect(state.text).toBe("New response");
    expect(state.toolCallsById).toEqual({});
    expect(state.finalParts).toEqual([]);
    const stale = makeEvent("message.text.delta", { delta: "duplicate" });
    stale.seq = state.lastSeq;
    expect(reduceActiveTurnEvent(state, stale).state).toBe(state);
  });

  it("an old answered event does not acknowledge a different pending question", () => {
    const pending = makeEvent("state.interaction.required", {
      interaction_id: "new-interaction",
      question: { question: "Describe yourself", answer_type: "text" },
      created_at: "2026-10-13T00:00:00Z",
    });
    const state = reduceActiveTurnEvent(
      INITIAL_ACTIVE_TURN_STATE,
      pending,
    ).state;
    const answered = makeEvent("state.interaction.answered", {
      interaction_id: "old-interaction",
      answer: { text: "different answer" },
    });
    const result = reduceActiveTurnEvent(state, answered).state;
    expect(result.pendingInteraction?.id).toBe("new-interaction");
    expect(result.pendingInteraction?.status).toBe("pending");
  });

  describe("BodyState suggestion receipts", () => {
    const suggestion = {
      question: "要把你说过的这句话记录为你自己的猜测吗？",
      answer_type: "single_choice",
      options: ["记录为我的猜测", "不记录"],
      allow_custom_input: false,
      required: true,
      purpose: "body_state_suggestion",
      quote: "我觉得是久坐导致的",
      state_binding: {
        revision: "body-state-suggestion-v1",
        kind: "hypothesis",
        source: {
          message_id: "msg-user-1",
          message_seq: 1,
          span_start: 0,
          span_end: 9,
          excerpt_sha256: "a".repeat(64),
        },
        base_body_state_revision: 0,
      },
    };
    const receipt = {
      kind: "hypothesis",
      commit_id: "commit-1",
      hypothesis_id: "hyp-1",
      source_evidence_id: "ev-1",
      revision: 1,
      event_id: "event-1",
      committed_at: "2026-10-19T02:00:00Z",
    };

    function askSuggestion(): ActiveTurnState {
      return reduceActiveTurnEvent(
        INITIAL_ACTIVE_TURN_STATE,
        makeEvent("state.interaction.required", {
          interaction_id: "int-s",
          question: suggestion,
        }),
      ).state;
    }

    it("keeps the receipt of an answered suggestion across run.resumed", () => {
      const answered = reduceActiveTurnEvent(
        askSuggestion(),
        makeEvent("state.interaction.answered", {
          interaction_id: "int-s",
          answer: { text: "记录为我的猜测", selected: ["记录为我的猜测"] },
          body_state_receipt: receipt,
        }),
      ).state;
      expect(answered.pendingInteraction?.body_state_receipt).toEqual(receipt);

      const resumed = reduceActiveTurnEvent(
        answered,
        makeEvent("run.resumed", { interaction_id: "int-s" }),
      ).state;
      expect(resumed.pendingInteraction).toBeNull();
      expect(resumed.answeredInteractions).toHaveLength(1);
      expect(resumed.answeredInteractions[0]).toMatchObject({
        id: "int-s",
        status: "answered",
        body_state_receipt: receipt,
      });

      const nextTurn = reduceActiveTurnEvent(
        resumed,
        makeEvent("run.started", {}, { run_id: "run-2" }),
      ).state;
      expect(nextTurn.answeredInteractions).toEqual([]);
    });

    it("records a decline with a null receipt and never twice", () => {
      const asked = askSuggestion();
      const decline = makeEvent("state.interaction.answered", {
        interaction_id: "int-s",
        answer: { text: "不记录", selected: ["不记录"] },
        body_state_receipt: null,
      });
      const once = reduceActiveTurnEvent(asked, decline).state;
      const twice = reduceActiveTurnEvent(once, decline).state;
      expect(twice.answeredInteractions).toHaveLength(1);
      expect(twice.answeredInteractions[0].body_state_receipt).toBeNull();
    });

    it("keeps an ordinary Ask User answer across run.resumed", () => {
      const state = reduceActiveTurnEvent(
        INITIAL_ACTIVE_TURN_STATE,
        makeEvent("state.interaction.required", { interaction_id: "int-o" }),
      ).state;
      const answered = reduceActiveTurnEvent(
        state,
        makeEvent("state.interaction.answered", { interaction_id: "int-o" }),
      ).state;
      expect(answered.answeredInteractions).toHaveLength(1);
      expect(answered.pendingInteraction?.body_state_receipt).toBeUndefined();
      const resumed = reduceActiveTurnEvent(
        answered,
        makeEvent("run.resumed", { interaction_id: "int-o" }),
      ).state;
      expect(resumed.pendingInteraction).toBeNull();
      expect(resumed.answeredInteractions).toMatchObject([
        { id: "int-o", status: "answered" },
      ]);
    });
  });
});
