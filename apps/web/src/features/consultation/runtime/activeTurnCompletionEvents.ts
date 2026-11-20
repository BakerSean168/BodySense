/**
 * Completion and stream event handlers for the active-turn reducer.
 */

import type { StreamEvent } from "../types/consultation";
import { assertNever, buildFinalMessageParts } from "./activeTurnHelpers";
import type { ActiveTurnEffect, ActiveTurnState } from "./activeTurnState";
import { EXECUTION_LOST_USER_MESSAGE } from "./activeTurnState";

export type ActiveTurnCompletionEvent = Extract<
  StreamEvent,
  {
    type:
      | "message.completed"
      | "message.failed"
      | "stream.done"
      | "stream.error";
  }
>;

export function applyCompletionEvent(
  current: ActiveTurnState,
  event: ActiveTurnCompletionEvent,
  effects: ActiveTurnEffect[],
): ActiveTurnState {
  switch (event.type) {
    case "message.completed": {
      effects.push({ type: "message_completed", data: event });
      return {
        ...current,
        status: "completed",
        finalParts: buildFinalMessageParts(current),
      };
    }

    case "message.failed": {
      const payload = event.payload;
      return {
        ...current,
        status: "failed",
        error:
          payload.error?.code === "execution_lost"
            ? EXECUTION_LOST_USER_MESSAGE
            : payload.error?.message || "stream failed",
      };
    }

    case "stream.done": {
      if (current.status === "interrupted") {
        return current;
      }
      if (
        current.status !== "completed" &&
        current.status !== "failed" &&
        current.status !== "cancelled"
      ) {
        return {
          ...current,
          status: "completed",
          finalParts: buildFinalMessageParts(current),
        };
      }
      return current;
    }

    case "stream.error": {
      const payload = event.payload;
      effects.push({ type: "stream_error", message: payload.message });
      return { ...current, status: "failed", error: payload.message };
    }

    default:
      return assertNever(event);
  }
}
