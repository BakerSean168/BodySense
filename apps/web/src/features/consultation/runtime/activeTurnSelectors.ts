/**
 * ActiveTurnSelectors — derived view model from ActiveTurnState.
 *
 * UI components should consume these selectors instead of reading raw reducer state
 * to avoid duplicating sort/dedup/filter logic across components.
 */

import type { ActiveTurnState } from "./activeTurnReducer";
import type {
  ToolCallInfo,
  Citation,
  RedFlagEvent,
  PendingInteraction,
} from "../types/consultation";

// ---------------------------------------------------------------------------
// View Model
// ---------------------------------------------------------------------------

export interface ActiveTurnViewModel {
  streamingMarkdown: string;
  toolCalls: ToolCallInfo[];
  citations: Citation[];
  knowledgeGaps: Array<{ query: string; message: string }>;
  redFlag: RedFlagEvent | null;
  pendingInteraction: PendingInteraction | null;
  isRunning: boolean;
  isInterrupted: boolean;
  isFailed: boolean;
  isCancelled: boolean;
  error: string | undefined;
  hasVisibleContent: boolean;
  hasRenderableContent: boolean;
}

// ---------------------------------------------------------------------------
// Selectors
// ---------------------------------------------------------------------------

export function selectActiveTurnViewModel(
  state: ActiveTurnState,
): ActiveTurnViewModel {
  return {
    streamingMarkdown: selectStreamingMarkdown(state),
    toolCalls: selectVisibleToolCalls(state),
    citations: selectCitations(state),
    knowledgeGaps: selectKnowledgeGaps(state),
    redFlag: state.redFlag,
    pendingInteraction: state.pendingInteraction,
    isRunning: state.status === "streaming",
    isInterrupted: state.status === "interrupted",
    isFailed: state.status === "failed",
    isCancelled: state.status === "cancelled",
    error: state.error,
    hasVisibleContent: selectHasVisibleContent(state),
    hasRenderableContent: selectHasRenderableContent(state),
  };
}

/** Raw streaming text. */
export function selectStreamingMarkdown(state: ActiveTurnState): string {
  return state.text;
}

/** Tool calls sorted (running first, completed last), ask_user filtered out. */
export function selectVisibleToolCalls(state: ActiveTurnState): ToolCallInfo[] {
  const calls = Object.values(state.toolCallsById).filter(
    (tc) => tc.tool !== "ask_user",
  );
  calls.sort((a, b) => {
    if (a.status === b.status) return 0;
    return a.status === "running" ? -1 : 1;
  });
  return calls;
}

/** All tool calls including ask_user (for internal use). */
export function selectAllToolCalls(state: ActiveTurnState): ToolCallInfo[] {
  return Object.values(state.toolCallsById);
}

/** Citations as an array. */
export function selectCitations(state: ActiveTurnState): Citation[] {
  return Object.values(state.citationsByKey);
}

/** Knowledge gaps as an array. */
export function selectKnowledgeGaps(
  state: ActiveTurnState,
): Array<{ query: string; message: string }> {
  return Object.values(state.knowledgeGapsByKey);
}

/** Whether the turn is interrupted (waiting for user input). */
export function selectIsInterrupted(state: ActiveTurnState): boolean {
  return state.status === "interrupted";
}

/** Whether the composer should stay locked until the current turn fully settles. */
export function selectIsComposerLocked(state: ActiveTurnState): boolean {
  return state.status === "streaming" || state.status === "interrupted";
}

/** Whether the active turn has any visible content to render. */
export function selectHasVisibleContent(state: ActiveTurnState): boolean {
  return selectHasRenderableContent(state) || state.pendingInteraction !== null;
}

/** Whether the active turn has content that should actually render in the message timeline. */
export function selectHasRenderableContent(state: ActiveTurnState): boolean {
  return (
    state.text.length > 0 ||
    selectVisibleToolCalls(state).length > 0 ||
    Object.keys(state.citationsByKey).length > 0 ||
    Object.keys(state.knowledgeGapsByKey).length > 0 ||
    state.redFlag !== null
  );
}
