/**
 * StreamEventReducer — pure reducer for consultation stream events.
 *
 * Consumes StreamEvent v1 and produces chat/runtime state without side effects.
 * Side effects (parent callbacks, assistant-ui updates) belong in the hook layer.
 */

import type {
  StreamEvent,
  ExtractedInfo,
  Citation,
  RedFlagEvent,
  PendingInteraction,
  AskUserQuestion,
} from '../types/consultation';

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

export interface ConsultationStreamState {
  /** Accumulated assistant text from message.text.delta events. */
  assistantText: string;
  /** Set on conversation.created. */
  conversationId?: string;
  /** Set on message.created for the assistant message. */
  assistantMessageId?: string;
  /** Set on message.persisted for the user message. */
  persistedUserMessageId?: string;
  /** Extracted info items, merged by body_part. */
  extractedInfo: ExtractedInfo[];
  /** Citations seen so far, deduped by title. */
  citations: Citation[];
  /** Most recent red flag event. */
  redFlags: RedFlagEvent | null;
  /** Knowledge gap queries. */
  knowledgeGaps: Array<{ query: string; message: string }>;
  /** Pending ask_user interaction, if any. */
  pendingInteraction: PendingInteraction | null;
  /** Overall stream status. */
  status: 'idle' | 'streaming' | 'completed' | 'failed';
  /** Error message if status is 'failed'. */
  error?: string;
}

export const INITIAL_STATE: ConsultationStreamState = {
  assistantText: '',
  extractedInfo: [],
  citations: [],
  redFlags: null,
  knowledgeGaps: [],
  pendingInteraction: null,
  status: 'idle',
};

// ---------------------------------------------------------------------------
// Effects — returned alongside state for the hook to execute
// ---------------------------------------------------------------------------

export type ReducerEffect =
  | { type: 'assistant_text_changed'; text: string }
  | { type: 'conversation_created'; conversationId: string; replacesDraftId?: string }
  | { type: 'message_persisted'; clientMessageId: string; messageId: string }
  | { type: 'extracted_info_updated'; info: ExtractedInfo }
  | { type: 'phase_changed'; from: string; to: string }
  | { type: 'red_flag'; flags: RedFlagEvent }
  | { type: 'citation_added'; citation: Citation }
  | { type: 'interaction_required'; interaction: PendingInteraction }
  | { type: 'interaction_answered'; interactionId: string }
  | { type: 'message_completed'; data: unknown }
  | { type: 'stream_error'; message: string };

// ---------------------------------------------------------------------------
// Reducer
// ---------------------------------------------------------------------------

export interface ReduceResult {
  state: ConsultationStreamState;
  effects: ReducerEffect[];
}

/**
 * Apply a single StreamEvent to the current state and return the new state
 * plus any side effects the hook should execute.
 */
export function reduceStreamEvent(
  current: ConsultationStreamState,
  event: StreamEvent,
): ReduceResult {
  const effects: ReducerEffect[] = [];
  let next = current;

  switch (event.type) {
    // --- Lifecycle ---------------------------------------------------------
    case 'conversation.created': {
      const payload = event.payload as { replaces_draft_id?: string };
      const conversationId = event.ids.conversation_id || '';
      next = { ...current, conversationId, status: 'streaming' };
      effects.push({
        type: 'conversation_created',
        conversationId,
        replacesDraftId: payload.replaces_draft_id,
      });
      break;
    }

    case 'message.persisted': {
      const payload = event.payload as { client_message_id: string };
      const messageId = event.ids.message_id || '';
      next = { ...current, persistedUserMessageId: messageId };
      effects.push({
        type: 'message_persisted',
        clientMessageId: payload.client_message_id,
        messageId,
      });
      break;
    }

    case 'message.created': {
      const messageId = event.ids.message_id || '';
      next = { ...current, assistantMessageId: messageId, status: 'streaming' };
      break;
    }

    // --- Text streaming ----------------------------------------------------
    case 'message.text.delta': {
      const payload = event.payload as { delta: string };
      const newText = current.assistantText + payload.delta;
      next = { ...current, assistantText: newText };
      effects.push({ type: 'assistant_text_changed', text: newText });
      break;
    }

    // --- State events ------------------------------------------------------
    case 'state.extracted_info.upsert': {
      const payload = event.payload as { info: ExtractedInfo };
      const info = payload.info;
      next = {
        ...current,
        extractedInfo: upsertExtractedInfo(current.extractedInfo, info),
      };
      effects.push({ type: 'extracted_info_updated', info });
      break;
    }

    case 'state.phase.changed': {
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
      const payload = event.payload as { citation: Citation };
      const citation = payload.citation;
      next = {
        ...current,
        citations: dedupCitations(current.citations, citation),
      };
      effects.push({ type: 'citation_added', citation });
      break;
    }

    case 'source.knowledge_gap': {
      const payload = event.payload as { query: string; message: string };
      next = {
        ...current,
        knowledgeGaps: [...current.knowledgeGaps, { query: payload.query, message: payload.message }],
      };
      break;
    }

    // --- Safety events -----------------------------------------------------
    case 'safety.red_flag.detected': {
      const payload = event.payload as { has_red_flags: boolean; flags: unknown[] };
      const redFlagEvent = payload as unknown as RedFlagEvent;
      next = { ...current, redFlags: redFlagEvent };
      effects.push({ type: 'red_flag', flags: redFlagEvent });
      break;
    }

    // --- Completion --------------------------------------------------------
    case 'message.completed': {
      next = { ...current, status: 'completed' };
      effects.push({ type: 'message_completed', data: event.payload });
      break;
    }

    case 'message.failed': {
      const payload = event.payload as { error?: { message?: string } };
      next = {
        ...current,
        status: 'failed',
        error: payload.error?.message || 'stream failed',
      };
      break;
    }

    case 'stream.done': {
      // stream.done is terminal but doesn't change status if already completed
      if (current.status !== 'completed') {
        next = { ...current, status: 'completed' };
      }
      break;
    }

    case 'stream.error': {
      const payload = event.payload as { message: string };
      next = { ...current, status: 'failed', error: payload.message };
      effects.push({ type: 'stream_error', message: payload.message });
      break;
    }

    // --- Interaction events ------------------------------------------------
    case 'state.interaction.required': {
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
      next = { ...current, pendingInteraction: interaction, status: 'streaming' };
      effects.push({ type: 'interaction_required', interaction });
      break;
    }

    case 'state.interaction.answered': {
      const payload = event.payload as { interaction_id: string };
      next = {
        ...current,
        pendingInteraction: current.pendingInteraction
          ? { ...current.pendingInteraction, status: 'answered' }
          : null,
      };
      effects.push({ type: 'interaction_answered', interactionId: payload.interaction_id });
      break;
    }

    // --- Unknown events (including title.generated which is outside StreamEvent union) --
    default:
      break;
  }

  return { state: next, effects };
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Upsert extracted info by body_part — same behavior as current inline code. */
function upsertExtractedInfo(
  existing: ExtractedInfo[],
  incoming: ExtractedInfo,
): ExtractedInfo[] {
  const idx = existing.findIndex((e) => e.body_part === incoming.body_part);
  if (idx >= 0) {
    const updated = [...existing];
    updated[idx] = { ...updated[idx], ...incoming };
    return updated;
  }
  return [...existing, incoming];
}

/** Dedup citations by title — preserves current behavior. */
function dedupCitations(existing: Citation[], incoming: Citation): Citation[] {
  if (existing.some((c) => c.title === incoming.title)) {
    return existing;
  }
  return [...existing, incoming];
}
