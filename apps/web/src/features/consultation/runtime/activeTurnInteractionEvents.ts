/**
 * Ask User interaction event handlers for the active-turn reducer.
 */

import type { PendingInteraction, StreamEvent } from "../types/consultation";
import { normalizeAskUserQuestion } from "./askUserQuestion";
import { assertNever } from "./activeTurnHelpers";
import type { ActiveTurnEffect, ActiveTurnState } from "./activeTurnState";

export type ActiveTurnInteractionEvent = Extract<
  StreamEvent,
  {
    type:
      | "state.interaction.required"
      | "state.interaction.answered"
      | "state.interaction.expired";
  }
>;

export function applyInteractionEvent(
  current: ActiveTurnState,
  event: ActiveTurnInteractionEvent,
  effects: ActiveTurnEffect[],
): ActiveTurnState {
  switch (event.type) {
    case "state.interaction.required": {
      const payload = event.payload;
      const interaction: PendingInteraction = {
        id: payload.interaction_id,
        run_id: event.ids.run_id || "",
        conversation_id: event.ids.conversation_id || "",
        tool_call_id: event.ids.tool_call_id || "",
        tool_name: "ask_user",
        question: normalizeAskUserQuestion(payload.question),
        status: "pending",
        created_at: payload.created_at,
      };
      effects.push({ type: "interaction_required", interaction });
      return {
        ...current,
        pendingInteraction: interaction,
        status: "interrupted",
      };
    }

    case "state.interaction.answered": {
      const payload = event.payload;
      const answered: PendingInteraction | null =
        current.pendingInteraction?.id === payload.interaction_id
          ? {
              ...current.pendingInteraction,
              status: "answered",
              answer: payload.answer,
              ...("body_state_receipt" in payload
                ? { body_state_receipt: payload.body_state_receipt ?? null }
                : {}),
            }
          : null;
      const answeredInteractions = current.answeredInteractions ?? [];
      effects.push({
        type: "interaction_answered",
        interactionId: payload.interaction_id,
      });
      return {
        ...current,
        pendingInteraction: answered ?? current.pendingInteraction,
        answeredInteractions:
          answered &&
          !answeredInteractions.some((item) => item.id === answered.id)
            ? [...answeredInteractions, answered]
            : answeredInteractions,
      };
    }

    case "state.interaction.expired": {
      const payload = event.payload;
      if (current.pendingInteraction?.id !== payload.interaction_id) {
        return current;
      }
      return {
        ...current,
        pendingInteraction: {
          ...current.pendingInteraction,
          status: "expired",
        },
        status: "failed",
        error: payload.reason || "interaction expired",
      };
    }

    default:
      return assertNever(event);
  }
}
