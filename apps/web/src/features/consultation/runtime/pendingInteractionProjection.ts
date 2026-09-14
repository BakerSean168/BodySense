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
