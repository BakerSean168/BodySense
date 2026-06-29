import { useRef, useState } from 'react';
import { useLocalRuntime, type ChatModelAdapter, type ChatModelRunResult } from '@assistant-ui/react';
import { consultationApi } from '../services/consultationService';
import { consumeSSEStream } from './useSSEProcessor';
import type { ExtractedInfo, Citation, SSERedFlag, SSEMessageCompleted } from '../types/consultation';

export interface ConsultationAdapterOptions {
  onConversationCreated?: (conversationId: string, replacesDraftId?: string) => void;
  onMessagePersisted?: (clientMessageId: string, messageId: string) => void;
  onExtractedInfoUpdate?: (info: ExtractedInfo) => void;
  onPhaseChange?: (from: string, to: string) => void;
  onRedFlag?: (flag: SSERedFlag['payload']) => void;
  onCitation?: (citation: Citation) => void;
  onTitleGenerated?: (title: string) => void;
  onMessageCompleted?: (data: SSEMessageCompleted) => void;
  isDraft?: boolean;
  clientDraftId?: string;
}

export function useAssistantChatRuntime(
  conversationId: string | null,
  options: ConsultationAdapterOptions = {}
) {
  const [isStreaming, setIsStreaming] = useState(false);
  const optionsRef = useRef(options);
  optionsRef.current = options;

  const adapter: ChatModelAdapter = {
    async *run({ messages }): AsyncGenerator<ChatModelRunResult> {
      const lastMessage = messages[messages.length - 1];
      const content = lastMessage.content
        .filter((p): p is { type: 'text'; text: string } => p.type === 'text')
        .map(p => p.text)
        .join('');

      const clientMessageId = `tmp_${crypto.randomUUID()}`;
      const requestId = crypto.randomUUID();

      setIsStreaming(true);

      // Queue for passing results from SSE callbacks to the generator
      const queue: ChatModelRunResult[] = [];
      let resolveNext: (() => void) | null = null;
      let streamDone = false;
      let streamError: Error | null = null;

      function pushResult(result: ChatModelRunResult) {
        queue.push(result);
        resolveNext?.();
      }

      function waitForNext(): Promise<boolean> {
        if (queue.length > 0) return Promise.resolve(true);
        if (streamDone || streamError) return Promise.resolve(false);
        return new Promise<boolean>((resolve) => {
          resolveNext = () => resolve(queue.length > 0);
        });
      }

      try {
        const response = await consultationApi.sendMessage({
          conversationId: conversationId,
          clientDraftId: optionsRef.current.isDraft ? optionsRef.current.clientDraftId : undefined,
          clientMessageId,
          requestId,
          message: {
            role: 'user',
            parts: [{ type: 'text', text: content }],
          },
          context: {
            entry: 'consultation',
          },
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        // Start consuming the SSE stream in the background.
        // Results are pushed to the queue and yielded below.
        const streamPromise = consumeSSEStream(response, {
          onConversationCreated: (data) => {
            optionsRef.current.onConversationCreated?.(
              data.ids.conversation_id || '',
              data.payload.replaces_draft_id
            );
          },
          onMessagePersisted: (data) => {
            optionsRef.current.onMessagePersisted?.(
              data.payload.client_message_id,
              data.ids.message_id || ''
            );
          },
          onTextDelta: (data) => {
            fullText += data.payload.delta;
            pushResult({
              content: [{ type: 'text', text: fullText }],
            });
          },
          onExtractedInfo: (data) => {
            optionsRef.current.onExtractedInfoUpdate?.(data.payload.info as ExtractedInfo);
          },
          onPhaseChange: (data) => {
            optionsRef.current.onPhaseChange?.(data.payload.from || '', data.payload.to);
          },
          onRedFlag: (data) => {
            optionsRef.current.onRedFlag?.(data.payload);
          },
          onCitation: (data) => {
            optionsRef.current.onCitation?.(data.payload.citation as Citation);
          },
          onTitleGenerated: (data) => {
            optionsRef.current.onTitleGenerated?.(data.payload.title);
          },
          onMessageCompleted: (data) => {
            optionsRef.current.onMessageCompleted?.(data);
          },
          onDone: () => {
            streamDone = true;
            resolveNext?.();
          },
          onStreamError: (data) => {
            streamError = new Error(data.payload.message);
            resolveNext?.();
          },
          onError: (err) => {
            streamError = err;
            resolveNext?.();
          },
        });

        let fullText = '';

        // Yield results as they arrive from the SSE stream
        while (await waitForNext()) {
          while (queue.length > 0) {
            yield queue.shift()!;
          }
        }

        // Wait for the stream to fully finish
        await streamPromise;

        if (streamError) {
          throw streamError;
        }

        // Final yield with complete text
        if (fullText) {
          yield {
            content: [{ type: 'text', text: fullText }],
          };
        }
      } finally {
        setIsStreaming(false);
      }
    },
  };

  const runtime = useLocalRuntime(adapter);

  return { runtime, isStreaming };
}
