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
  SSERedFlag,
  SSEMessageCompleted,
  SSEMessageFailed,
  SSETitleGenerated,
} from '../types/consultation';

export interface SSEHandlers {
  onConversationCreated?: (data: SSEConversationCreated) => void;
  onMessagePersisted?: (data: SSEMessagePersisted) => void;
  onMessageCreated?: (data: SSEMessageCreated) => void;
  onTextDelta?: (data: SSETextDelta) => void;
  onToolCall?: (data: SSEToolCall) => void;
  onToolResult?: (data: SSEToolResult) => void;
  onExtractedInfo?: (data: SSEExtractedInfo) => void;
  onPhaseChange?: (data: SSEPhaseChange) => void;
  onCitation?: (data: SSECitation) => void;
  onRedFlag?: (data: SSERedFlag) => void;
  onMessageCompleted?: (data: SSEMessageCompleted) => void;
  onMessageFailed?: (data: SSEMessageFailed) => void;
  onTitleGenerated?: (data: SSETitleGenerated) => void;
  onDone?: () => void;
  onError?: (error: Error) => void;
}

const EVENT_MAP: Record<string, keyof SSEHandlers> = {
  'conversation.created': 'onConversationCreated',
  'message.persisted': 'onMessagePersisted',
  'message.created': 'onMessageCreated',
  'text.delta': 'onTextDelta',
  'tool.call': 'onToolCall',
  'tool.result': 'onToolResult',
  'extracted_info': 'onExtractedInfo',
  'phase_change': 'onPhaseChange',
  'citation': 'onCitation',
  'red_flag': 'onRedFlag',
  'message.completed': 'onMessageCompleted',
  'message.failed': 'onMessageFailed',
  'title.generated': 'onTitleGenerated',
  'done': 'onDone',
};

type HandlerFn = (data: unknown) => void;

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
      const handlerKey = EVENT_MAP[state.currentEvent];
      if (handlerKey) {
        const handler = handlers[handlerKey];
        if (handler) {
          (handler as HandlerFn)(handlerKey === 'onDone' ? undefined : data);
        }
      }
    } catch {
      // skip malformed JSON
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
