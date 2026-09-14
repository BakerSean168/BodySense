import { describe, expect, it } from "vitest";
import {
  parsePendingInteraction,
  projectPendingInteraction,
} from "./pendingInteractionProjection";

const validInteraction = {
  id: "11111111-1111-4111-8111-111111111111",
  run_id: "22222222-2222-4222-8222-222222222222",
  conversation_id: "33333333-3333-4333-8333-333333333333",
  tool_call_id: "tool-ask-1",
  tool_name: "ask_user",
  question: {
    question: "疼痛会向手臂放射吗？",
    answer_type: "single_choice",
    options: ["会", "不会"],
    purpose: "symptom_intake",
  },
  status: "answered" as const,
  answer: { value: "不会" },
  created_at: "2026-09-14T10:00:00Z",
  answered_at: "2026-09-14T10:01:00Z",
  metadata: {},
};

describe("pending interaction projection", () => {
  it("re-establishes the feature interaction after assistant-ui metadata widening", () => {
    const interaction = parsePendingInteraction(validInteraction);

    expect(interaction.question.answer_type).toBe("single_choice");
    expect(interaction.question.purpose).toBe("symptom_intake");
    expect(interaction.status).toBe("answered");
  });

  it("normalizes canonical select questions through the same projection used by REST", () => {
    const interaction = projectPendingInteraction({
      ...validInteraction,
      question: {
        question: "请选择主要诱因",
        answer_type: "select",
        options: ["久坐", "运动"],
      },
    });

    expect(interaction.question.answer_type).toBe("single_choice");
  });

  it("fails closed when assistant-ui custom metadata no longer matches AgentInteraction", () => {
    expect(() =>
      parsePendingInteraction({
        ...validInteraction,
        id: "not-a-uuid",
        question: { question: 42 },
      }),
    ).toThrow();
  });
});
