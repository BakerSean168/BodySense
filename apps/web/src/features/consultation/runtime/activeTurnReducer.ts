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
 * - 用 `event.type` 可辨识联合做 switch，未覆盖的类型走 default（见底部）。
 * - 用 `lastSeqByType` 做基于 seq 的幂等守卫，避免后端重复事件重复生效。
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
  HealthFeatures,
  Citation,
  RedFlagEvent,
  PendingInteraction,
  AskUserQuestion,
  ToolCallInfo,
} from '../types/consultation';
import type { ThreadAssistantMessagePart } from '@assistant-ui/react';

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

export type StreamStatus = 'idle' | 'streaming' | 'interrupted' | 'completed' | 'failed';

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
  /** Highest seq processed per event type (backend mixes multiple seq sources). */
  lastSeqByType: Partial<Record<StreamEvent['type'], number>>;
  /** Error message if status is 'failed'. */
  error?: string;
}

export const INITIAL_ACTIVE_TURN_STATE: ActiveTurnState = {
  runId: null,
  conversationId: null,
  assistantMessageId: null,
  status: 'idle',
  text: '',
  toolCallsById: {},
  citationsByKey: {},
  knowledgeGapsByKey: {},
  redFlag: null,
  pendingInteraction: null,
  extractedInfoByBodyPart: {},
  finalParts: [],
  lastSeqByType: {},
};

/** Return a fresh initial state (factory for reset). */
export function resetActiveTurnState(): ActiveTurnState {
  return { ...INITIAL_ACTIVE_TURN_STATE };
}

// ---------------------------------------------------------------------------
// Effects — parent-level callbacks only (no UI-level effects)
// ---------------------------------------------------------------------------

export type ActiveTurnEffect =
  | { type: 'conversation_created'; conversationId: string; replacesDraftId?: string }
  | { type: 'message_persisted'; clientMessageId: string; messageId: string }
  | { type: 'extracted_info_updated'; info: ExtractedInfo }
  | { type: 'health_features_updated'; healthFeatures: HealthFeatures }
  | { type: 'phase_changed'; from: string; to: string }
  | { type: 'red_flag'; flags: RedFlagEvent }
  | { type: 'citation_added'; citation: Citation }
  | { type: 'interaction_required'; interaction: PendingInteraction }
  | { type: 'interaction_answered'; interactionId: string }
  | { type: 'message_completed'; data: unknown }
  | { type: 'title_generated'; title: string }
  | { type: 'stream_error'; message: string };

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
  // Seq-based idempotency guard. Backend events currently mix locally-generated
  // SSE events with AI-stream events that use different sequence spaces, so the
  // safest comparable scope is the event type itself.
  const lastSeqForType = current.lastSeqByType[event.type] ?? -1;
  if (event.seq !== undefined && event.seq <= lastSeqForType) {
    return { state: current, effects: [] };
  }

  const effects: ActiveTurnEffect[] = [];
  let next = current;
  let processed = false;

  switch (event.type) {
    // --- Lifecycle ---------------------------------------------------------
    case 'conversation.created': {
      processed = true;
      const payload = event.payload as { replaces_draft_id?: string };
      const conversationId = event.ids.conversation_id || '';
      console.debug('[SSE] ③ Reducer 处理 conversation.created → 产生 effect', {
        conversationId,
        runId: event.ids.run_id,
        replacesDraftId: payload.replaces_draft_id,
        prevStatus: current.status,
        nextStatus: 'streaming',
      });
      next = {
        ...current,
        conversationId,
        runId: event.ids.run_id || null,
        status: 'streaming',
        error: undefined,
      };
      effects.push({
        type: 'conversation_created',
        conversationId,
        replacesDraftId: payload.replaces_draft_id,
      });
      break;
    }

    case 'run.started':
    case 'run.resumed': {
      processed = true;
      next = {
        ...current,
        conversationId: event.ids.conversation_id || current.conversationId,
        runId: event.ids.run_id || current.runId,
        status: 'streaming',
        pendingInteraction: event.type === 'run.resumed' ? null : current.pendingInteraction,
        error: undefined,
      };
      break;
    }

    case 'run.interrupted': {
      processed = true;
      next = { ...current, status: 'interrupted' };
      break;
    }

    case 'run.completed': {
      processed = true;
      next = { ...current, runId: event.ids.run_id || current.runId };
      break;
    }

    case 'run.failed': {
      processed = true;
      const payload = event.payload as { error?: { message?: string } };
      next = {
        ...current,
        status: 'failed',
        error: payload.error?.message || current.error || 'run failed',
      };
      break;
    }

    case 'message.persisted': {
      processed = true;
      const payload = event.payload as { client_message_id: string };
      const messageId = event.ids.message_id || '';
      next = { ...current };
      effects.push({
        type: 'message_persisted',
        clientMessageId: payload.client_message_id,
        messageId,
      });
      break;
    }

    case 'message.created': {
      processed = true;
      const messageId = event.ids.message_id || '';
      next = { ...current, assistantMessageId: messageId, status: 'streaming' };
      break;
    }

    // --- Text streaming ----------------------------------------------------
    case 'message.text.delta': {
      processed = true;
      const payload = event.payload as { delta: string };
      next = { ...current, text: current.text + payload.delta };
      break;
    }

    // --- State events ------------------------------------------------------
    case 'state.extracted_info.upsert': {
      processed = true;
      const payload = event.payload as { info: ExtractedInfo };
      const info = payload.info;
      const prev = current.extractedInfoByBodyPart[info.body_part] || {};
      next = {
        ...current,
        extractedInfoByBodyPart: {
          ...current.extractedInfoByBodyPart,
          [info.body_part]: { ...prev, ...info },
        },
      };
      effects.push({ type: 'extracted_info_updated', info });
      break;
    }

    case 'state.health_features.upsert': {
      processed = true;
      const payload = event.payload as { health_features: HealthFeatures };
      next = { ...current };
      effects.push({
        type: 'health_features_updated',
        healthFeatures: payload.health_features,
      });
      break;
    }

    case 'state.phase.changed': {
      processed = true;
      const payload = event.payload as { from?: string; to: string };
      effects.push({
        type: 'phase_changed',
        from: payload.from || '',
        to: payload.to,
      });
      break;
    }

    // --- Source events ------------------------------------------------------
    case 'source.citation.added': {
      processed = true;
      const payload = event.payload as { citation: Citation };
      const citation = payload.citation;
      const key = citation.title;
      if (!current.citationsByKey[key]) {
        next = {
          ...current,
          citationsByKey: { ...current.citationsByKey, [key]: citation },
        };
        effects.push({ type: 'citation_added', citation });
      }
      break;
    }

    case 'source.knowledge_gap': {
      processed = true;
      const payload = event.payload as { query: string; message: string };
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
    case 'safety.red_flag.detected': {
      processed = true;
      const payload = event.payload as { has_red_flags: boolean; flags: unknown[] };
      // 运行时边界：契约只保证 payload 是 unknown，把“已确认形状”的类型断言收口到
      // 这一处，避免到处 as。详见 typescript-unknown-vs-any / static-types-and-runtime-validation。
      const redFlagEvent = payload as unknown as RedFlagEvent;
      next = { ...current, redFlag: redFlagEvent };
      effects.push({ type: 'red_flag', flags: redFlagEvent });
      break;
    }

    // --- Completion --------------------------------------------------------
    case 'message.completed': {
      processed = true;
      next = {
        ...current,
        status: 'completed',
        finalParts: buildFinalMessageParts(current),
      };
      effects.push({ type: 'message_completed', data: event.payload });
      break;
    }

    case 'message.failed': {
      processed = true;
      const payload = event.payload as { status: 'failed'; error: { message: string } };
      next = {
        ...current,
        status: 'failed',
        error: payload.error?.message || 'stream failed',
      };
      break;
    }

    case 'stream.done': {
      processed = true;
      if (current.status === 'interrupted') {
        next = current;
        break;
      }
      if (current.status !== 'completed' && current.status !== 'failed') {
        next = {
          ...current,
          status: 'completed',
          finalParts: buildFinalMessageParts(current),
        };
      }
      break;
    }

    case 'title.generated': {
      processed = true;
      const payload = event.payload as { title: string };
      console.debug('[SSE] ③ Reducer 处理 title.generated → 产生 effect', {
        title: payload.title,
        currentConversationId: current.conversationId,
      });
      effects.push({ type: 'title_generated', title: payload.title });
      break;
    }

    case 'stream.error': {
      processed = true;
      const payload = event.payload as { message: string };
      next = { ...current, status: 'failed', error: payload.message };
      effects.push({ type: 'stream_error', message: payload.message });
      break;
    }

    // --- Interaction events ------------------------------------------------
    case 'state.interaction.required': {
      processed = true;
      const payload = event.payload as {
        interaction_id: string;
        question: AskUserQuestion;
      };
      const interaction: PendingInteraction = {
        id: payload.interaction_id,
        run_id: event.ids.run_id || '',
        conversation_id: event.ids.conversation_id || '',
        tool_call_id: event.ids.tool_call_id || '',
        tool_name: 'ask_user',
        question: payload.question,
        status: 'pending',
        created_at: new Date().toISOString(),
      };
      next = { ...current, pendingInteraction: interaction, status: 'interrupted' };
      effects.push({ type: 'interaction_required', interaction });
      break;
    }

    case 'state.interaction.answered': {
      processed = true;
      const payload = event.payload as { interaction_id: string; answer?: unknown };
      next = {
        ...current,
        pendingInteraction: current.pendingInteraction
          ? {
              ...current.pendingInteraction,
              status: 'answered',
              answer: payload.answer,
            }
          : null,
      };
      effects.push({ type: 'interaction_answered', interactionId: payload.interaction_id });
      break;
    }

    // --- Tool events ---------------------------------------------------------
    case 'tool.call': {
      processed = true;
      const payload = event.payload as { tool: string; args: unknown };
      const toolCallId = event.ids.tool_call_id || `tc_${crypto.randomUUID().slice(0, 8)}`;
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
              status: 'running',
            },
          },
        };
      }
      break;
    }

    case 'tool.result': {
      processed = true;
      const payload = event.payload as { tool: string; result: unknown };
      const toolCallId = event.ids.tool_call_id || '';
      const toolName = payload.tool.trim();

      // Exact match by tool_call_id
      if (toolCallId && current.toolCallsById[toolCallId]) {
        const existing = current.toolCallsById[toolCallId];
        if (existing.status === 'completed') break; // already completed
        next = {
          ...current,
          toolCallsById: {
            ...current.toolCallsById,
            [toolCallId]: { ...existing, result: payload.result, status: 'completed' },
          },
        };
        break;
      }

      // Conservative fallback: only match by tool name when there is exactly
      // one running candidate. Multiple same-name tools would be ambiguous.
      const fallbackEntries = Object.entries(current.toolCallsById).filter(
        ([, tc]) => tc.tool === toolName && tc.status === 'running',
      );
      if (fallbackEntries.length === 1) {
        console.warn(
          '[activeTurnReducer] tool.result matched by name fallback (no exact tool_call_id match)',
          { toolName: toolName, fallbackId: fallbackEntries[0][0] },
        );
        const [fallbackId, tc] = fallbackEntries[0];
        next = {
          ...current,
          toolCallsById: {
            ...current.toolCallsById,
            [fallbackId]: { ...tc, result: payload.result, status: 'completed' },
          },
        };
        break;
      }

      // Placeholder
      const placeholderId = toolCallId || `tc_ph_${Object.keys(current.toolCallsById).length}`;
      next = {
        ...current,
        toolCallsById: {
          ...current.toolCallsById,
          [placeholderId]: {
            id: placeholderId,
            tool: toolName,
            args: null,
            result: payload.result,
            status: 'completed',
          },
        },
      };
      break;
    }

    // --- Unknown events ----------------------------------------------------
    // default 分支处理“契约之外”的事件：保持幂等，不抛错、也不丢失已累积状态。
    // 可辨识联合要求穷尽所有已知类型；未知类型属于运行时边界，交给上层决定。
    default:
      break;
  }

  if (processed && event.seq !== undefined) {
    next = {
      ...next,
      lastSeqByType: {
        ...next.lastSeqByType,
        [event.type]: event.seq,
      },
    };
  }

  return { state: next, effects };
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Build final message parts from the accumulated active turn state. */
function buildFinalMessageParts(state: ActiveTurnState): ThreadAssistantMessagePart[] {
  const parts: ThreadAssistantMessagePart[] = [];

  if (state.text) {
    parts.push({ type: 'text', text: state.text });
  }

  for (const citation of Object.values(state.citationsByKey)) {
    const bodysenseMetadata: Record<string, string> = {};
    if (citation.summary) bodysenseMetadata.summary = citation.summary;
    if (citation.snippet) bodysenseMetadata.snippet = citation.snippet;
    if (citation.source_title) bodysenseMetadata.source_title = citation.source_title;
    if (citation.source_author) bodysenseMetadata.source_author = citation.source_author;
    parts.push({
      type: 'source',
      sourceType: 'url',
      id: `src_${crypto.randomUUID().slice(0, 8)}`,
      url: (citation as { url?: string }).url || '',
      title: citation.title,
      providerMetadata:
        Object.keys(bodysenseMetadata).length > 0
          ? { bodysense: bodysenseMetadata }
          : undefined,
    });
  }

  if (state.redFlag?.has_red_flags) {
    parts.push({ type: 'data', name: 'red_flag', data: state.redFlag });
  }

  for (const gap of Object.values(state.knowledgeGapsByKey)) {
    parts.push({ type: 'data', name: 'knowledge_gap', data: gap });
  }

  return parts;
}
