import { describe, expect, it } from "vitest";
import { validateGeneratedStreamEvent } from "./generated-stream-event";
import { parseStreamEvent } from "./stream-event-parser";

// BS-AGENT-001: a Go-bound BodyState suggestion question and the receipt on
// its answered milestone are part of the public StreamEvent contract.

function parserAccepts(event: unknown): boolean {
  try {
    parseStreamEvent(event);
    return true;
  } catch {
    return false;
  }
}

const binding = {
  revision: "body-state-suggestion-v1",
  kind: "hypothesis",
  source: {
    message_id: "6f1c1f9e-2b0e-4d7a-9b8e-3c2a1d0e9f11",
    message_seq: 3,
    span_start: 4,
    span_end: 13,
    excerpt_sha256: "a".repeat(64),
  },
  base_body_state_revision: 7,
};

function required(stateBinding: unknown) {
  return {
    version: 1,
    seq: 1,
    channel: "state",
    type: "state.interaction.required",
    ids: { conversation_id: "conv-1", run_id: "run-1", interaction_id: "i" },
    payload: {
      interaction_id: "i",
      created_at: "2026-10-17T08:00:00Z",
      question: {
        purpose: "body_state_suggestion",
        question: "要把你说过的这句话记录为你自己的猜测吗？",
        answer_type: "single_choice",
        options: ["记录为我的猜测", "不记录"],
        allow_custom_input: false,
        required: true,
        quote: "跑步太多导致膝盖疼",
        state_binding: stateBinding,
      },
    },
  };
}

const receipt = {
  kind: "hypothesis",
  commit_id: "c",
  hypothesis_id: "h",
  source_evidence_id: "e",
  revision: 8,
  event_id: "ev",
  committed_at: "2026-10-17T08:00:00Z",
};

function answered(extra: Record<string, unknown>) {
  return {
    version: 1,
    seq: 2,
    channel: "state",
    type: "state.interaction.answered",
    ids: { conversation_id: "conv-1", run_id: "run-1", interaction_id: "i" },
    payload: {
      interaction_id: "i",
      answer: { text: "记录为我的猜测", selected: ["记录为我的猜测"] },
      ...extra,
    },
  };
}

const cases: Array<[string, unknown, boolean]> = [
  ["bound suggestion question", required(binding), true],
  [
    "binding with a model-authored statement",
    required({ ...binding, statement: "模型改写" }),
    false,
  ],
  [
    "binding with an uppercase digest",
    required({
      ...binding,
      source: { ...binding.source, excerpt_sha256: "A".repeat(64) },
    }),
    false,
  ],
  [
    "binding of an unsupported kind",
    required({ ...binding, kind: "diagnosis" }),
    false,
  ],
  [
    "symptom intake binding still accepted",
    required({
      revision: "symptom-intake-binding-v2",
      capture_id: "0".repeat(24),
      body_fact_id: "f",
      field_map: { safety_signals: "safety_signals" },
    }),
    true,
  ],
  [
    "answered with a commit receipt",
    answered({ body_state_receipt: receipt }),
    true,
  ],
  [
    "declined answer carries a null receipt",
    answered({ body_state_receipt: null }),
    true,
  ],
  ["ordinary answer without a receipt", answered({}), true],
  [
    "receipt smuggling a statement",
    answered({ body_state_receipt: { ...receipt, statement: "x" } }),
    false,
  ],
  [
    "receipt without a revision",
    answered({ body_state_receipt: { ...receipt, revision: 0 } }),
    false,
  ],
];

describe("BodyState suggestion stream contract", () => {
  it.each(cases)("%s", (_name, event, shouldAccept) => {
    expect(validateGeneratedStreamEvent(event)).toBe(shouldAccept);
    expect(parserAccepts(event)).toBe(shouldAccept);
  });
});
