/**
 * ActiveTurnReducer —— 当前 AI 回复这一“轮(turn)”流式状态的纯 reducer。
 *
 * ===== 它解决什么问题 =====
 * 流式事件是一条一条到达的；UI 需要在不丢失已累积内容的前提下，把每个
 * StreamEvent 合并进当前状态。这里把它做成纯函数：
 *   (current: ActiveTurnState, event: StreamEvent) => { state, effects }
 * 不直接改 UI、不发请求——只算“下一步状态”，并把需要副作用的事（建会话、
 * 落库、阶段切换等）打包成 effects 交还给上层 hook 去执行。
 *
 * ===== 几个值得注意的设计点 =====
 * - 用 `Record<id, T>` 做天然 upsert（toolCalls / citations / knowledgeGaps）。
 * - 用 `event.type` 可辨识联合做穷尽 switch；每个合法 variant 必须处理或显式 no-op，default 仅作 `assertNever` 编译期保护。
 * - 用 `(run_id, seq)` 做幂等守卫；后端是每个 run 的唯一公共序号所有者。
 * - 所有更新都是不可变展开（{ ...current, ... }），不原地改对象。
 *
 * 事件类别按处理逻辑拆在同目录：
 * - activeTurnLifecycleEvents.ts —— 会话/运行/消息生命周期
 * - activeTurnContentEvents.ts —— 文本、结构化状态、来源、安全
 * - activeTurnCompletionEvents.ts —— 完成与流终止
 * - activeTurnInteractionEvents.ts —— ask_user 交互
 * - activeTurnToolEvents.ts —— tool.call / tool.result
 * - activeTurnState.ts —— 状态、初始值、effects 契约
 *
 * 深入笔记（Thought Forest 文件名）：
 * - react-use-reducer.md
 * - react-context-and-state-management.md
 * - typescript-discriminated-unions-and-exhaustiveness.md
 * - typescript-unknown-vs-any.md
 * - typescript-static-types-and-runtime-validation.md
 */

import type { StreamEvent } from "../types/consultation";
import { applyCompletionEvent } from "./activeTurnCompletionEvents";
import { applyContentEvent } from "./activeTurnContentEvents";
import { assertNever } from "./activeTurnHelpers";
import { applyInteractionEvent } from "./activeTurnInteractionEvents";
import { applyLifecycleEvent } from "./activeTurnLifecycleEvents";
import type {
  ActiveTurnEffect,
  ActiveTurnState,
  ReduceResult,
} from "./activeTurnState";
import { applyToolEvent } from "./activeTurnToolEvents";

export {
  EXECUTION_LOST_USER_MESSAGE,
  INITIAL_ACTIVE_TURN_STATE,
  resetActiveTurnState,
  type ActiveTurnEffect,
  type ActiveTurnState,
  type ReduceResult,
  type StreamStatus,
} from "./activeTurnState";

// ---------------------------------------------------------------------------
// Reducer
// ---------------------------------------------------------------------------

export function reduceActiveTurnEvent(
  current: ActiveTurnState,
  event: StreamEvent,
): ReduceResult {
  // Public seq has one owner (Go StreamWriter / transactional OOB allocator).
  // Compare only within the same Run. V3 resumes keep the Run ID and continue
  // its sequence; explicit historical continuations may still have a new ID.
  const eventRunId = event.ids.run_id || current.runId;
  const sameSequenceRun =
    eventRunId !== null && eventRunId === current.sequenceRunId;
  if (sameSequenceRun && event.seq <= current.lastSeq) {
    return { state: current, effects: [] };
  }

  const effects: ActiveTurnEffect[] = [];
  let next: ActiveTurnState;
  switch (event.type) {
    // --- Lifecycle ---------------------------------------------------------
    case "conversation.created":
    case "run.started":
    case "run.resumed":
    case "run.interrupted":
    case "run.completed":
    case "run.failed":
    case "run.cancelled":
    case "message.persisted":
    case "message.created": {
      next = applyLifecycleEvent(current, event, effects);
      break;
    }

    // --- Text streaming / state / source / safety --------------------------
    case "message.text.delta":
    case "state.extracted_info.upsert":
    case "state.phase.changed":
    case "source.citation.added":
    case "source.knowledge_gap":
    case "safety.red_flag.detected": {
      next = applyContentEvent(current, event, effects);
      break;
    }

    // --- Completion --------------------------------------------------------
    case "message.completed":
    case "message.failed":
    case "stream.done":
    case "stream.error": {
      next = applyCompletionEvent(current, event, effects);
      break;
    }

    // --- Interaction events ------------------------------------------------
    case "state.interaction.required":
    case "state.interaction.answered":
    case "state.interaction.expired": {
      next = applyInteractionEvent(current, event, effects);
      break;
    }

    // --- Tool events -------------------------------------------------------
    case "tool.call":
    case "tool.result": {
      next = applyToolEvent(current, event, eventRunId);
      break;
    }

    // --- Explicitly observed/no-op protocol events -------------------------
    // These are valid public events that this reducer does not project into UI
    // state today. They still advance the canonical sequence watermark.
    case "state.lifestyle_context.upsert":
    case "source.answer_attribution.added":
    case "safety.output_reviewed":
    case "safety.output_rejected":
    case "usage.reported":
    case "job.created":
    case "job.progress":
    case "job.completed":
    case "job.failed": {
      next = current;
      break;
    }

    default:
      return assertNever(event);
  }

  next = {
    ...next,
    sequenceRunId: eventRunId,
    lastSeq: event.seq,
  };

  return { state: next, effects };
}
