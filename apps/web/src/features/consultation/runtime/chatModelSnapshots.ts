import type { ChatModelRunResult } from "@assistant-ui/react";
import type { ActiveTurnState } from "./activeTurnReducer";
import { suggestionHistoryItem } from "./pendingInteractionProjection";

export function buildStreamingSnapshot(
  state: ActiveTurnState,
): ChatModelRunResult | null {
  if (!state.text) {
    return null;
  }

  return {
    content: [{ type: "text", text: state.text }],
  };
}

export function buildCompletedSnapshot(
  state: ActiveTurnState,
): ChatModelRunResult | null {
  if (state.status !== "completed") {
    return null;
  }

  // Only suggestions ride on the message; open questions come back with the
  // thread's interaction history.
  const suggestions = (state.answeredInteractions ?? []).filter(
    (item) => item.question.purpose === "body_state_suggestion",
  );
  // The answered suggestions (with Go's receipts) stay on the completed
  // message, so "saved" survives the active turn being reset.
  const metadata =
    suggestions.length > 0
      ? {
          custom: {
            body_state_suggestions: suggestions.map(suggestionHistoryItem),
          },
        }
      : undefined;
  const content =
    state.finalParts.length > 0
      ? [...state.finalParts]
      : (buildStreamingSnapshot(state)?.content ?? null);
  if (!content && !metadata) return null;
  return {
    content: content ?? [],
    ...(metadata ? { metadata } : {}),
  };
}
