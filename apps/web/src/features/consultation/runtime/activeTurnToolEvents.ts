/**
 * Tool call event handlers for the active-turn reducer.
 */

import type { StreamEvent } from "../types/consultation";
import { assertNever } from "./activeTurnHelpers";
import type { ActiveTurnState } from "./activeTurnState";

export type ActiveTurnToolEvent = Extract<
  StreamEvent,
  { type: "tool.call" | "tool.result" }
>;

export function applyToolEvent(
  current: ActiveTurnState,
  event: ActiveTurnToolEvent,
  eventRunId: string | null,
): ActiveTurnState {
  switch (event.type) {
    case "tool.call": {
      const payload = event.payload;
      const toolCallId =
        event.ids.tool_call_id || `tc_${eventRunId || "run"}_${event.seq}`;
      const toolName = payload.tool.trim();
      if (current.toolCallsById[toolCallId]) return current;
      return {
        ...current,
        toolCallsById: {
          ...current.toolCallsById,
          [toolCallId]: {
            id: toolCallId,
            tool: toolName,
            args: payload.args,
            status: "running",
          },
        },
      };
    }

    case "tool.result": {
      const payload = event.payload;
      const toolCallId = event.ids.tool_call_id || "";
      const toolName = payload.tool.trim();

      // Exact match by tool_call_id
      if (toolCallId && current.toolCallsById[toolCallId]) {
        const existing = current.toolCallsById[toolCallId];
        if (existing.status === "completed") return current; // already completed
        return {
          ...current,
          toolCallsById: {
            ...current.toolCallsById,
            [toolCallId]: {
              ...existing,
              result: payload.result,
              status: "completed",
            },
          },
        };
      }

      // Conservative fallback: only match by tool name when there is exactly
      // one running candidate. Multiple same-name tools would be ambiguous.
      const fallbackEntries = Object.entries(current.toolCallsById).filter(
        ([, tc]) => tc.tool === toolName && tc.status === "running",
      );
      if (fallbackEntries.length === 1) {
        const [fallbackId, tc] = fallbackEntries[0];
        return {
          ...current,
          toolCallsById: {
            ...current.toolCallsById,
            [fallbackId]: {
              ...tc,
              result: payload.result,
              status: "completed",
            },
          },
        };
      }

      // Placeholder
      const placeholderId =
        toolCallId || `tc_ph_${Object.keys(current.toolCallsById).length}`;
      return {
        ...current,
        toolCallsById: {
          ...current.toolCallsById,
          [placeholderId]: {
            id: placeholderId,
            tool: toolName,
            args: null,
            result: payload.result,
            status: "completed",
          },
        },
      };
    }

    default:
      return assertNever(event);
  }
}
