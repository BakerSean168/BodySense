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
 * 深入笔记（Thought Forest 文件名）：
 * - react-use-reducer.md
 * - react-context-and-state-management.md
 * - typescript-discriminated-unions-and-exhaustiveness.md
 * - typescript-unknown-vs-any.md
 * - typescript-static-types-and-runtime-validation.md
 */

import type {
  StreamEvent,
  ExtractedInfo,
  Citation,
  RedFlagEvent,
  PendingInteraction,
  AskUserQuestion,
  ToolCallInfo,
} from "../types/consultation";
import type { ThreadAssistantMessagePart } from "@assistant-ui/react";

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

export const EXECUTION_LOST_USER_MESSAGE =
  "本次执行因服务实例中断而停止，系统已安全回收。你可以继续输入，发起一次新的执行。";

export type StreamStatus =
  "idle" | "streaming" | "interrupted" | "completed" | "failed" | "cancelled";

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
  | { type: "title_generated"; title: string }
  | { type: "stream_error"; message: string };

export interface ReduceResult {
  state: ActiveTurnState;
  effects: ActiveTurnEffect[];
}

// ---------------------------------------------------------------------------
// Reducer
// ---------------------------------------------------------------------------

export function reduceActiveTurnEvent(
  current: ActiveTurnState,
  event: StreamEvent,
): ReduceResult {
  // Public seq has one owner (Go StreamWriter / transactional OOB allocator).
  // Compare only within the same run; a resumed run has a new run_id and starts
  // again at seq=1 while the logical LangGraph thread may continue.
  const eventRunId = event.ids.run_id || current.runId;
  const sameSequenceRun =
    eventRunId !== null && eventRunId === current.sequenceRunId;
  if (sameSequenceRun && event.seq <= current.lastSeq) {
    return { state: current, effects: [] };
  }

  const effects: ActiveTurnEffect[] = [];
  let next = current;
  switch (event.type) {
    // --- Lifecycle ---------------------------------------------------------
    case "conversation.created": {
      const conversationId = event.ids.conversation_id || "";
      next = {
        ...current,
        conversationId,
        runId: event.ids.run_id || null,
        status: "streaming",
        error: undefined,
      };
      effects.push({
        type: "conversation_created",
        conversationId,
      });
      break;
    }

    case "run.started":
    case "run.resumed": {
      next = {
        ...current,
        conversationId: event.ids.conversation_id || current.conversationId,
        runId: event.ids.run_id || current.runId,
        status: "streaming",
        pendingInteraction:
          event.type === "run.resumed" ? null : current.pendingInteraction,
        error: undefined,
      };
      break;
    }

    case "run.interrupted": {
      next = { ...current, status: "interrupted" };
      break;
    }

    case "run.completed": {
      next = { ...current, runId: event.ids.run_id || current.runId };
      break;
    }

    case "run.failed": {
      const payload = event.payload;
      const executionLost = payload.reason === "execution_lost";
      next = {
        ...current,
        runId: event.ids.run_id || current.runId,
        status: "failed",
        pendingInteraction: null,
        error: executionLost
          ? EXECUTION_LOST_USER_MESSAGE
          : payload.error?.message ||
            current.error ||
            "本次执行未能完成，请继续输入后重试。",
      };
      break;
    }

    case "run.cancelled": {
      const payload = event.payload;
      next = {
        ...current,
        runId: event.ids.run_id || current.runId,
        status: "cancelled",
        error: payload.reason,
        pendingInteraction: null,
      };
      break;
    }

    case "message.persisted": {
      const payload = event.payload;
      const messageId = event.ids.message_id || "";
      next = { ...current };
      effects.push({
        type: "message_persisted",
        clientMessageId: payload.client_message_id,
        messageId,
      });
      break;
    }

    case "message.created": {
      const messageId = event.ids.message_id || "";
      next = { ...current, assistantMessageId: messageId, status: "streaming" };
      break;
    }

    // --- Text streaming ----------------------------------------------------
    case "message.text.delta": {
      const payload = event.payload;
      next = { ...current, text: current.text + payload.delta };
      break;
    }

    // --- State events ------------------------------------------------------
    case "state.extracted_info.upsert": {
      const payload = event.payload;
      const info = payload.info;
      const prev = current.extractedInfoByBodyPart[info.body_part] || {};
      next = {
        ...current,
        extractedInfoByBodyPart: {
          ...current.extractedInfoByBodyPart,
          [info.body_part]: { ...prev, ...info },
        },
      };
      effects.push({ type: "extracted_info_updated", info });
      break;
    }

    case "state.phase.changed": {
      const payload = event.payload;
      effects.push({
        type: "phase_changed",
        from: payload.from || "",
        to: payload.to,
      });
      break;
    }

    // --- Source events ------------------------------------------------------
    case "source.citation.added": {
      const payload = event.payload;
      const citation = payload.citation;
      const key = citation.title;
      if (!current.citationsByKey[key]) {
        next = {
          ...current,
          citationsByKey: { ...current.citationsByKey, [key]: citation },
        };
        effects.push({ type: "citation_added", citation });
      }
      break;
    }

    case "source.knowledge_gap": {
      const payload = event.payload;
      const key = payload.query;
      if (!current.knowledgeGapsByKey[key]) {
        next = {
          ...current,
          knowledgeGapsByKey: {
            ...current.knowledgeGapsByKey,
            [key]: { query: payload.query, message: payload.message },
          },
        };
      }
      break;
    }

    // --- Safety events -----------------------------------------------------
    case "safety.red_flag.detected": {
      const payload = event.payload;
      const redFlagEvent: RedFlagEvent = {
        has_red_flags: payload.has_red_flags,
        flags: payload.flags,
      };
      next = { ...current, redFlag: redFlagEvent };
      effects.push({ type: "red_flag", flags: redFlagEvent });
      break;
    }

    // --- Completion --------------------------------------------------------
    case "message.completed": {
      next = {
        ...current,
        status: "completed",
        finalParts: buildFinalMessageParts(current),
      };
      effects.push({ type: "message_completed", data: event });
      break;
    }

    case "message.failed": {
      const payload = event.payload;
      next = {
        ...current,
        status: "failed",
        error:
          payload.error?.code === "execution_lost"
            ? EXECUTION_LOST_USER_MESSAGE
            : payload.error?.message || "stream failed",
      };
      break;
    }

    case "stream.done": {
      if (current.status === "interrupted") {
        next = current;
        break;
      }
      if (
        current.status !== "completed" &&
        current.status !== "failed" &&
        current.status !== "cancelled"
      ) {
        next = {
          ...current,
          status: "completed",
          finalParts: buildFinalMessageParts(current),
        };
      }
      break;
    }

    case "title.generated": {
      const payload = event.payload;
      effects.push({ type: "title_generated", title: payload.title });
      break;
    }

    case "stream.error": {
      const payload = event.payload;
      next = { ...current, status: "failed", error: payload.message };
      effects.push({ type: "stream_error", message: payload.message });
      break;
    }

    // --- Interaction events ------------------------------------------------
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
      next = {
        ...current,
        pendingInteraction: interaction,
        status: "interrupted",
      };
      effects.push({ type: "interaction_required", interaction });
      break;
    }

    case "state.interaction.answered": {
      const payload = event.payload;
      next = {
        ...current,
        pendingInteraction: current.pendingInteraction
          ? {
              ...current.pendingInteraction,
              status: "answered",
              answer: payload.answer,
            }
          : null,
      };
      effects.push({
        type: "interaction_answered",
        interactionId: payload.interaction_id,
      });
      break;
    }

    case "state.interaction.expired": {
      const payload = event.payload;
      if (current.pendingInteraction?.id === payload.interaction_id) {
        next = {
          ...current,
          pendingInteraction: {
            ...current.pendingInteraction,
            status: "expired",
          },
          status: "failed",
          error: payload.reason || "interaction expired",
        };
      }
      break;
    }

    // --- Tool events ---------------------------------------------------------
    case "tool.call": {
      const payload = event.payload;
      const toolCallId =
        event.ids.tool_call_id || `tc_${eventRunId || "run"}_${event.seq}`;
      const toolName = payload.tool.trim();
      if (!current.toolCallsById[toolCallId]) {
        next = {
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
      break;
    }

    case "tool.result": {
      const payload = event.payload;
      const toolCallId = event.ids.tool_call_id || "";
      const toolName = payload.tool.trim();

      // Exact match by tool_call_id
      if (toolCallId && current.toolCallsById[toolCallId]) {
        const existing = current.toolCallsById[toolCallId];
        if (existing.status === "completed") break; // already completed
        next = {
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
        break;
      }

      // Conservative fallback: only match by tool name when there is exactly
      // one running candidate. Multiple same-name tools would be ambiguous.
      const fallbackEntries = Object.entries(current.toolCallsById).filter(
        ([, tc]) => tc.tool === toolName && tc.status === "running",
      );
      if (fallbackEntries.length === 1) {
        const [fallbackId, tc] = fallbackEntries[0];
        next = {
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
        break;
      }

      // Placeholder
      const placeholderId =
        toolCallId || `tc_ph_${Object.keys(current.toolCallsById).length}`;
      next = {
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type InteractionRequiredQuestion = Extract<
  StreamEvent,
  { type: "state.interaction.required" }
>["payload"]["question"];
type InteractionRequiredField = NonNullable<
  InteractionRequiredQuestion["fields"]
>[number];

function normalizeAnswerType(
  answerType:
    | InteractionRequiredQuestion["answer_type"]
    | InteractionRequiredField["answer_type"],
): AskUserQuestion["answer_type"] {
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

function normalizeAskUserQuestion(
  question: InteractionRequiredQuestion,
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

function assertNever(event: never): never {
  throw new Error(`Unhandled validated StreamEvent: ${JSON.stringify(event)}`);
}

/** Build final message parts from the accumulated active turn state. */
function buildFinalMessageParts(
  state: ActiveTurnState,
): ThreadAssistantMessagePart[] {
  const parts: ThreadAssistantMessagePart[] = [];

  if (state.text) {
    parts.push({ type: "text", text: state.text });
  }

  for (const citation of Object.values(state.citationsByKey)) {
    const bodysenseMetadata: Record<string, string> = {};
    if (citation.summary) bodysenseMetadata.summary = citation.summary;
    if (citation.snippet) bodysenseMetadata.snippet = citation.snippet;
    if (citation.source_title)
      bodysenseMetadata.source_title = citation.source_title;
    if (citation.source_author)
      bodysenseMetadata.source_author = citation.source_author;
    parts.push({
      type: "source",
      sourceType: "url",
      id: `src_${crypto.randomUUID().slice(0, 8)}`,
      url: citation.url || "",
      title: citation.title,
      providerMetadata:
        Object.keys(bodysenseMetadata).length > 0
          ? { bodysense: bodysenseMetadata }
          : undefined,
    });
  }

  if (state.redFlag?.has_red_flags) {
    parts.push({ type: "data", name: "red_flag", data: state.redFlag });
  }

  for (const gap of Object.values(state.knowledgeGapsByKey)) {
    parts.push({ type: "data", name: "knowledge_gap", data: gap });
  }

  return parts;
}
