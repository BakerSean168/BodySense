/**
 * ActiveTurnState —— 当前 AI 回复这一“轮(turn)”的流式状态与 reducer 契约。
 *
 * 从 activeTurnReducer.ts 拆出；对外导出仍由 activeTurnReducer.ts 统一转发，
 * 既有导入路径保持兼容。
 */

import type { ThreadAssistantMessagePart } from "@assistant-ui/react";
import type {
  Citation,
  ExtractedInfo,
  PendingInteraction,
  RedFlagEvent,
  StreamEvent,
  ToolCallInfo,
} from "../types/consultation";

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

export const EXECUTION_LOST_USER_MESSAGE =
  "本次执行因服务实例中断而停止，系统已安全回收。你可以继续输入，发起一次新的执行。";

export type StreamStatus =
  | "idle"
  | "streaming"
  | "interrupted"
  | "completed"
  | "failed"
  | "cancelled";

export interface ActiveTurnState {
  runId: string | null;
  conversationId: string | null;
  assistantMessageId: string | null;
  status: StreamStatus;
  /** Accumulated streaming markdown text. */
  text: string;
  /** Tool calls keyed by tool_call_id (upsert-friendly). */
  toolCallsById: Record<string, ToolCallInfo>;
  /** Citations keyed by title. */
  citationsByKey: Record<string, Citation>;
  /** Knowledge gaps keyed by query. */
  knowledgeGapsByKey: Record<string, { query: string; message: string }>;
  /** Latest red flag event. */
  redFlag: RedFlagEvent | null;
  /** Pending ask_user interaction. */
  pendingInteraction: PendingInteraction | null;
  /**
   * Ask User interactions answered in this turn (open questions and BodyState
   * suggestions with Go's receipt). They outlive run.resumed, which clears
   * pendingInteraction, so the answered card stays in the thread above the
   * running pill and a suggestion shows "saved" exactly when Go committed.
   */
  answeredInteractions: PendingInteraction[];
  /** Extracted info keyed by body_part. */
  extractedInfoByBodyPart: Record<string, ExtractedInfo>;
  /** Final message parts generated on completion, compatible with assistant-ui. */
  finalParts: ThreadAssistantMessagePart[];
  /** Public event sequence is monotonic within a run and resets on run change. */
  sequenceRunId: string | null;
  lastSeq: number;
  /** Error message if status is 'failed'. */
  error?: string;
}

export const INITIAL_ACTIVE_TURN_STATE: ActiveTurnState = {
  runId: null,
  conversationId: null,
  assistantMessageId: null,
  status: "idle",
  text: "",
  toolCallsById: {},
  citationsByKey: {},
  knowledgeGapsByKey: {},
  redFlag: null,
  pendingInteraction: null,
  answeredInteractions: [],
  extractedInfoByBodyPart: {},
  finalParts: [],
  sequenceRunId: null,
  lastSeq: 0,
};

/** Return a fresh initial state (factory for reset). */
export function resetActiveTurnState(): ActiveTurnState {
  return { ...INITIAL_ACTIVE_TURN_STATE };
}

// ---------------------------------------------------------------------------
// Effects — parent-level callbacks only (no UI-level effects)
// ---------------------------------------------------------------------------

export type ActiveTurnEffect =
  | {
      type: "conversation_created";
      conversationId: string;
    }
  | { type: "message_persisted"; clientMessageId: string; messageId: string }
  | { type: "extracted_info_updated"; info: ExtractedInfo }
  | { type: "phase_changed"; from: string; to: string }
  | { type: "red_flag"; flags: RedFlagEvent }
  | { type: "citation_added"; citation: Citation }
  | { type: "interaction_required"; interaction: PendingInteraction }
  | { type: "interaction_answered"; interactionId: string }
  | {
      type: "message_completed";
      data: Extract<StreamEvent, { type: "message.completed" }>;
    }
  | { type: "stream_error"; message: string };

export interface ReduceResult {
  state: ActiveTurnState;
  effects: ActiveTurnEffect[];
}
