import { parseInteractionQuestion } from "@bodysense/contracts";
import { AgentInteraction as AgentInteractionSchema } from "@/generated/api/model/agentInteraction.zod";
import type { AgentInteractionOutput as PublicAgentInteraction } from "@/generated/api/model";
import type { PendingInteraction } from "../types/consultation";
import { normalizeAskUserQuestion } from "./askUserQuestion";

export function projectPendingInteraction(
  input: PublicAgentInteraction,
): PendingInteraction {
  return {
    id: input.id,
    run_id: input.run_id,
    conversation_id: input.conversation_id,
    tool_call_id: input.tool_call_id,
    tool_name: input.tool_name,
    question: normalizeAskUserQuestion(
      parseInteractionQuestion(input.question),
    ),
    status: input.status,
    answer: input.answer,
    created_at: input.created_at,
    answered_at: input.answered_at ?? null,
    metadata: input.metadata,
  };
}

/**
 * Re-establish the feature-domain interaction after a third-party runtime has
 * widened custom message metadata back to unknown.
 */
export function parsePendingInteraction(input: unknown): PendingInteraction {
  return projectPendingInteraction(AgentInteractionSchema.parse(input));
}

/**
 * History shape of an answered BodyState suggestion: Go's receipt lives in
 * metadata.body_state_receipt, exactly as interaction history returns it, so
 * a completed turn and a reloaded thread render the same "saved" state.
 */
export function suggestionHistoryItem(
  interaction: PendingInteraction,
): PendingInteraction {
  const { body_state_receipt: receipt, ...rest } = interaction;
  return {
    ...rest,
    metadata: {
      ...interaction.metadata,
      ...(receipt ? { body_state_receipt: receipt } : {}),
    },
  };
}

/** Answered suggestions a completed turn carried in its message metadata. */
export function readSuggestionHistory(value: unknown): PendingInteraction[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => {
    try {
      return [parsePendingInteraction(item)];
    } catch (error) {
      console.error("Invalid BodyState suggestion metadata", error);
      return [];
    }
  });
}
