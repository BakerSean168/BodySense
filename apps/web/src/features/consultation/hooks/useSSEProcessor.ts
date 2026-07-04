/**
 * SSE event processor — parses SSE text lines and dispatches to handlers.
 */

import type {
  SSEConversationCreated,
  SSEMessagePersisted,
  SSEMessageCreated,
  SSETextDelta,
  SSEToolCall,
  SSEToolResult,
  SSEExtractedInfo,
  SSEPhaseChange,
  SSECitation,
  SSEKnowledgeGap,
  SSERedFlag,
  SSEMessageCompleted,
  SSEMessageFailed,
  SSETitleGenerated,
  SSEStreamDone,
  SSEStreamError,
  StreamEvent,
} from '../types/consultation';

export interface SSEHandlers {
  onConversationCreated?: (data: SSEConversationCreated) => void;
  onRunStarted?: (data: StreamEvent) => void;
  onRunResumed?: (data: StreamEvent) => void;
  onRunInterrupted?: (data: StreamEvent) => void;
  onRunCompleted?: (data: StreamEvent) => void;
  onRunFailed?: (data: StreamEvent) => void;
  onMessagePersisted?: (data: SSEMessagePersisted) => void;
  onMessageCreated?: (data: SSEMessageCreated) => void;
  onTextDelta?: (data: SSETextDelta) => void;
  onToolCall?: (data: SSEToolCall) => void;
  onToolResult?: (data: SSEToolResult) => void;
  onExtractedInfo?: (data: SSEExtractedInfo) => void;
  onPhaseChange?: (data: SSEPhaseChange) => void;
  onCitation?: (data: SSECitation) => void;
  onKnowledgeGap?: (data: SSEKnowledgeGap) => void;
  onRedFlag?: (data: SSERedFlag) => void;
  onMessageCompleted?: (data: SSEMessageCompleted) => void;
  onMessageFailed?: (data: SSEMessageFailed) => void;
  onTitleGenerated?: (data: SSETitleGenerated) => void;
  onInteractionRequired?: (data: StreamEvent) => void;
  onInteractionAnswered?: (data: StreamEvent) => void;
  onDone?: (data: SSEStreamDone) => void;
  onStreamError?: (data: SSEStreamError) => void;
  onError?: (error: Error) => void;
}

const EVENT_MAP: Record<string, keyof SSEHandlers> = {
  'conversation.created': 'onConversationCreated',
  'run.started': 'onRunStarted',
  'run.resumed': 'onRunResumed',
  'run.interrupted': 'onRunInterrupted',
  'run.completed': 'onRunCompleted',
  'run.failed': 'onRunFailed',
  'message.persisted': 'onMessagePersisted',
  'message.created': 'onMessageCreated',
  'message.text.delta': 'onTextDelta',
  'tool.call': 'onToolCall',
  'tool.result': 'onToolResult',
  'state.extracted_info.upsert': 'onExtractedInfo',
  'state.phase.changed': 'onPhaseChange',
  'source.citation.added': 'onCitation',
  'source.knowledge_gap': 'onKnowledgeGap',
  'safety.red_flag.detected': 'onRedFlag',
  'message.completed': 'onMessageCompleted',
  'message.failed': 'onMessageFailed',
  'title.generated': 'onTitleGenerated',
  'state.interaction.required': 'onInteractionRequired',
  'state.interaction.answered': 'onInteractionAnswered',
  'stream.done': 'onDone',
  'stream.error': 'onStreamError',
};

type HandlerFn = (data: StreamEvent) => void;

export function processSSELine(
  line: string,
  state: { currentEvent: string },
  handlers: SSEHandlers
): void {
  const trimmed = line.trim();

  if (trimmed.startsWith('event: ')) {
    state.currentEvent = trimmed.slice(7).trim();
    return;
  }

  if (trimmed.startsWith('data: ')) {
    const dataStr = trimmed.slice(6);
    try {
      const data = JSON.parse(dataStr);
      const event = data as StreamEvent;
      const eventType = event.type || state.currentEvent;
      const handlerKey = EVENT_MAP[eventType];
      if (handlerKey) {
        const handler = handlers[handlerKey];
        if (handler) {
          (handler as HandlerFn)(event);
        }
      }
    } catch (err) {
      console.warn('[SSE] malformed JSON payload:', dataStr, err);
    }
  }
}

/**
 * Read an SSE Response body and dispatch events.
 */
export async function consumeSSEStream(
  response: Response,
  handlers: SSEHandlers
): Promise<void> {
  const reader = response.body?.getReader();
  if (!reader) {
    handlers.onError?.(new Error('No response body'));
    return;
  }

  const decoder = new TextDecoder();
  const state = { currentEvent: '' };
  let buffer = '';

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        processSSELine(line, state, handlers);
      }
    }

    // Process remaining buffer
    if (buffer.trim()) {
      processSSELine(buffer, state, handlers);
    }
  } catch (err) {
    handlers.onError?.(err instanceof Error ? err : new Error(String(err)));
  }
}
