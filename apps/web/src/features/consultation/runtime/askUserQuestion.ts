import type { InteractionQuestion } from "@bodysense/contracts";
import type { AskUserQuestion } from "../types/consultation";

type CanonicalAnswerType = InteractionQuestion["answer_type"];
type CanonicalField = NonNullable<InteractionQuestion["fields"]>[number];
type UiAnswerType = AskUserQuestion["answer_type"];

function normalizeAnswerType(
  answerType: CanonicalAnswerType | CanonicalField["answer_type"],
): UiAnswerType {
  switch (answerType) {
    case "single_choice":
    case "multi_choice":
    case "number":
    case "date":
    case "text":
      return answerType;
    case "select":
      return "single_choice";
    case "scale":
      return "number";
    case undefined:
      return "text";
  }
}

/**
 * Project the validated public interaction contract into the UI's normalized
 * input vocabulary. This is a domain mapping step, not a trust boundary: the
 * caller must parse unknown transport data before calling this function.
 */
export function normalizeAskUserQuestion(
  question: InteractionQuestion,
): AskUserQuestion {
  return {
    ...question,
    answer_type: normalizeAnswerType(question.answer_type),
    fields: question.fields?.map((field) => ({
      ...field,
      answer_type: normalizeAnswerType(field.answer_type),
    })),
  };
}
