import { describe, expect, it } from "vitest";
import { parseStreamEvent } from "@bodysense/contracts";
import {
  INITIAL_ACTIVE_TURN_STATE,
  reduceActiveTurnEvent,
} from "./activeTurnReducer";
import { selectActiveTurnViewModel } from "./activeTurnSelectors";
import {
  readSuggestionHistory,
  suggestionHistoryItem,
} from "./pendingInteractionProjection";

const ids = {
  conversation_id: "6f1d9a8e-3b7c-4d2a-9e5f-1a2b3c4d5e6f",
  run_id: "7a2e8b9f-4c8d-4e3b-8f6a-2b3c4d5e6f70",
  turn_id: "8b3f9c0a-5d9e-4f4c-9a7b-3c4d5e6f7081",
  message_id: "9c4a0d1b-6e0f-4a5d-8b8c-4d5e6f708192",
  tool_call_id: "ask-suggestion-1",
  interaction_id: null,
  job_id: null,
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

function answeredTurn(bodyStateReceipt: typeof receipt | null) {
  const required = parseStreamEvent({
    version: 1,
    seq: 1,
    channel: "state",
    type: "state.interaction.required",
    ids,
    payload: {
      interaction_id: "0d5b1e2c-7f1a-4b6e-9c9d-5e6f708192a3",
      created_at: "2026-10-19T02:00:00Z",
      question: {
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
            message_id: ids.message_id,
            message_seq: 1,
            span_start: 0,
            span_end: 9,
            excerpt_sha256: "a".repeat(64),
          },
          base_body_state_revision: 0,
        },
      },
    },
  });
  const option = bodyStateReceipt ? "记录为我的猜测" : "不记录";
  const answered = parseStreamEvent({
    version: 1,
    seq: 2,
    channel: "state",
    type: "state.interaction.answered",
    ids,
    payload: {
      interaction_id: "0d5b1e2c-7f1a-4b6e-9c9d-5e6f708192a3",
      answer: { text: option, selected: [option] },
      body_state_receipt: bodyStateReceipt,
    },
  });
  const asked = reduceActiveTurnEvent(INITIAL_ACTIVE_TURN_STATE, required);
  return reduceActiveTurnEvent(asked.state, answered).state;
}

describe("answered BodyState suggestion history", () => {
  it("moves Go's receipt into history metadata and parses back", () => {
    const [live] = answeredTurn(receipt).answeredInteractions;
    const item = suggestionHistoryItem(live);
    expect(item).not.toHaveProperty("body_state_receipt");
    expect(item.metadata?.body_state_receipt).toEqual(receipt);
    const [restored] = readSuggestionHistory(
      JSON.parse(JSON.stringify([item])),
    );
    expect(restored.id).toBe(live.id);
    expect(restored.status).toBe("answered");
    expect(restored.question.quote).toBe("我觉得是久坐导致的");
    expect(restored.metadata?.body_state_receipt).toEqual(receipt);
  });

  it("keeps a decline without any receipt", () => {
    const [live] = answeredTurn(null).answeredInteractions;
    const [restored] = readSuggestionHistory([suggestionHistoryItem(live)]);
    expect(restored.metadata).toEqual({});
    expect(restored.answer).toEqual({ text: "不记录", selected: ["不记录"] });
  });

  it("drops malformed metadata instead of rendering it", () => {
    expect(readSuggestionHistory([{ id: "x" }])).toEqual([]);
    expect(readSuggestionHistory("nope")).toEqual([]);
  });

  it("shows answered suggestions only while the turn is live", () => {
    const answered = answeredTurn(receipt);
    // Until run.resumed the answered card is still the pending interaction.
    expect(selectActiveTurnViewModel(answered).answeredInteractions).toEqual(
      [],
    );
    const state = {
      ...answered,
      status: "streaming" as const,
      pendingInteraction: null,
    };
    expect(selectActiveTurnViewModel(state).answeredInteractions).toHaveLength(
      1,
    );
    expect(
      selectActiveTurnViewModel({ ...state, status: "completed" })
        .answeredInteractions,
    ).toEqual([]);
  });
});
