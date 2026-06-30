import { useRef, useState } from 'react';
import { useLocalRuntime, type ChatModelAdapter, type ChatModelRunResult } from '@assistant-ui/react';
import { consultationApi } from '../services/consultationService';
import { consumeSSEStream } from './useSSEProcessor';
import { reduceStreamEvent, INITIAL_STATE, type ConsultationStreamState, type ReducerEffect } from '../runtime/streamEventReducer';
import type { ExtractedInfo, Citation, SSERedFlag, SSEMessageCompleted, StreamEvent, PendingInteraction } from '../types/consultation';

export interface ConsultationAdapterOptions {
  onConversationCreated?: (conversationId: string, replacesDraftId?: string) => void;
  onMessagePersisted?: (clientMessageId: string, messageId: string) => void;
  onExtractedInfoUpdate?: (info: ExtractedInfo) => void;
  onPhaseChange?: (from: string, to: string) => void;
  onRedFlag?: (flag: SSERedFlag['payload']) => void;
  onCitation?: (citation: Citation) => void;
  onTitleGenerated?: (title: string) => void;
  onMessageCompleted?: (data: SSEMessageCompleted) => void;
  onInteractionRequired?: (interaction: PendingInteraction) => void;
  isDraft?: boolean;
  clientDraftId?: string;
}

export interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
}

export function useAssistantChatRuntime(
  conversationId: string | null,
  initialMessages: ChatMessage[] = [],
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

      // Reducer state — replaces ad hoc fullText mutation
      let reducerState: ConsultationStreamState = INITIAL_STATE;

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

      /** Execute side effects emitted by the reducer. */
      function applyEffects(effects: ReducerEffect[]) {
        for (const effect of effects) {
          switch (effect.type) {
            case 'assistant_text_changed':
              pushResult({
                content: [{ type: 'text', text: effect.text }],
              });
              break;
            case 'conversation_created':
              optionsRef.current.onConversationCreated?.(
                effect.conversationId,
                effect.replacesDraftId,
              );
              break;
            case 'message_persisted':
              optionsRef.current.onMessagePersisted?.(
                effect.clientMessageId,
                effect.messageId,
              );
              break;
            case 'extracted_info_updated':
              optionsRef.current.onExtractedInfoUpdate?.(effect.info);
              break;
            case 'phase_changed':
              optionsRef.current.onPhaseChange?.(effect.from, effect.to);
              break;
            case 'red_flag':
              optionsRef.current.onRedFlag?.(effect.flags as SSERedFlag['payload']);
              break;
            case 'citation_added':
              optionsRef.current.onCitation?.(effect.citation);
              break;
            case 'interaction_required':
              optionsRef.current.onInteractionRequired?.(effect.interaction);
              break;
            case 'interaction_answered':
              // Interaction answered — no-op in hook, handled by resume flow
              break;
            case 'message_completed':
              optionsRef.current.onMessageCompleted?.(effect.data as SSEMessageCompleted);
              break;
            case 'stream_error':
              // Handled via streamError below
              break;
          }
        }
      }

      /** Dispatch a StreamEvent through the reducer and apply effects. */
      function dispatch(event: StreamEvent) {
        const result = reduceStreamEvent(reducerState, event);
        reducerState = result.state;
        applyEffects(result.effects);
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
        // Events are dispatched through the reducer; effects push to the queue.
        const streamPromise = consumeSSEStream(response, {
          onConversationCreated: (data) => dispatch(data as unknown as StreamEvent),
          onMessagePersisted: (data) => dispatch(data as unknown as StreamEvent),
          onTextDelta: (data) => dispatch(data as unknown as StreamEvent),
          onExtractedInfo: (data) => dispatch(data as unknown as StreamEvent),
          onPhaseChange: (data) => dispatch(data as unknown as StreamEvent),
          onRedFlag: (data) => dispatch(data as unknown as StreamEvent),
          onCitation: (data) => dispatch(data as unknown as StreamEvent),
          onTitleGenerated: (data) => {
            // title.generated is outside the StreamEvent union; handle directly
            optionsRef.current.onTitleGenerated?.(data.payload.title);
          },
          onInteractionRequired: (data) => dispatch(data as unknown as StreamEvent),
          onInteractionAnswered: (data) => dispatch(data as unknown as StreamEvent),
          onMessageCompleted: (data) => dispatch(data as unknown as StreamEvent),
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

        // Final yield with complete text from reducer state
        if (reducerState.assistantText) {
          yield {
            content: [{ type: 'text', text: reducerState.assistantText }],
          };
        }
      } finally {
        setIsStreaming(false);
      }
    },
  };

  const runtime = useLocalRuntime(adapter, {
    initialMessages: initialMessages.map((m) => ({
      role: m.role,
      content: [{ type: 'text', text: m.content }],
    })),
  });

  return { runtime, isStreaming };
}
